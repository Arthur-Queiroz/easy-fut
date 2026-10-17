package store

import (
	"context"
	"errors"
	"futdarapaziada/api/internal/push"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrNoPushRecipients = errors.New("nenhum destinatário com notificações ativas")

// Manual notices expire: a device offline for longer than this never gets a stale notice.
const BroadcastMaxAge = 24 * time.Hour

type PushRecipient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	AvatarColor string `json:"avatar_color"`
	Devices     int    `json:"devices"`
}

// Everyone not archived can be picked; "all" reaches only active users.
func (s *Store) ListPushRecipients(ctx context.Context) ([]PushRecipient, error) {
	rows, err := s.pool.Query(ctx, `select u.id,u.name,u.status,u.avatar_color,count(p.id)::int from users u
 left join push_subscriptions p on p.user_id=u.id
 where u.status<>'archived' group by u.id order by u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recipients []PushRecipient
	for rows.Next() {
		var r PushRecipient
		if err := rows.Scan(&r.ID, &r.Name, &r.Status, &r.AvatarColor, &r.Devices); err != nil {
			return nil, err
		}
		recipients = append(recipients, r)
	}
	return recipients, rows.Err()
}

type BroadcastInput struct {
	Title, Body, Audience string
	UserIDs               []string
}

type PushBroadcast struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Audience    string    `json:"audience"`
	CreatorName string    `json:"creator_name"`
	CreatedAt   time.Time `json:"created_at"`
	Users       int       `json:"users"`
	Devices     int       `json:"devices"`
	Sent        int       `json:"sent"`
	Pending     int       `json:"pending"`
	Failed      int       `json:"failed"`
}

// The queue is written in the same transaction; nothing is saved without a recipient device.
func (s *Store) CreatePushBroadcast(ctx context.Context, in BroadcastInput, createdBy string) (PushBroadcast, error) {
	b := PushBroadcast{Title: in.Title, Body: in.Body, Audience: in.Audience}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return b, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `insert into push_broadcasts(title,body,audience,created_by) values($1,$2,$3,$4)
 returning id,created_at,coalesce((select name from users where id=$4),'')`, in.Title, in.Body, in.Audience, createdBy).Scan(&b.ID, &b.CreatedAt, &b.CreatorName)
	if err != nil {
		return b, err
	}
	ids := in.UserIDs
	if ids == nil {
		ids = []string{}
	}
	tag, err := tx.Exec(ctx, `insert into broadcast_push_deliveries(subscription_id,broadcast_id)
 select p.id,$1 from push_subscriptions p join users u on u.id=p.user_id
 where ($2='all' and u.status='active') or ($2='selected' and u.status<>'archived' and u.id=any($3::uuid[]))
 on conflict do nothing`, b.ID, in.Audience, ids)
	if err != nil {
		return b, err
	}
	if tag.RowsAffected() == 0 {
		return b, ErrNoPushRecipients
	}
	b.Devices, b.Pending = int(tag.RowsAffected()), int(tag.RowsAffected())
	err = tx.QueryRow(ctx, `select count(distinct p.user_id)::int from broadcast_push_deliveries d
 join push_subscriptions p on p.id=d.subscription_id where d.broadcast_id=$1`, b.ID).Scan(&b.Users)
	if err != nil {
		return b, err
	}
	return b, tx.Commit(ctx)
}

func (s *Store) ListPushBroadcasts(ctx context.Context, limit int) ([]PushBroadcast, error) {
	rows, err := s.pool.Query(ctx, `select b.id,b.title,b.body,b.audience,coalesce(u.name,''),b.created_at,
 count(distinct p.user_id)::int,count(d.id)::int,
 count(d.id) filter (where d.status='sent')::int,
 count(d.id) filter (where d.status='pending')::int,
 count(d.id) filter (where d.status='failed')::int
 from push_broadcasts b left join users u on u.id=b.created_by
 left join broadcast_push_deliveries d on d.broadcast_id=b.id
 left join push_subscriptions p on p.id=d.subscription_id
 group by b.id,u.name order by b.created_at desc limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var broadcasts []PushBroadcast
	for rows.Next() {
		var b PushBroadcast
		if err := rows.Scan(&b.ID, &b.Title, &b.Body, &b.Audience, &b.CreatorName, &b.CreatedAt, &b.Users, &b.Devices, &b.Sent, &b.Pending, &b.Failed); err != nil {
			return nil, err
		}
		broadcasts = append(broadcasts, b)
	}
	return broadcasts, rows.Err()
}

type BroadcastDelivery struct {
	ID, SubscriptionID, BroadcastID, Title, Body string
	Subscription                                 push.Subscription
	CreatedAt                                    time.Time
	Attempts                                     int
}

// Status is checked again at send time: an archived user, or an inactivated one in an
// "all" notice, gets nothing even if queued before the change.
func (s *Store) ClaimBroadcastDelivery(ctx context.Context) (BroadcastDelivery, error) {
	var d BroadcastDelivery
	err := s.pool.QueryRow(ctx, `with candidate as (
 select d.id from broadcast_push_deliveries d
 join push_subscriptions p on p.id=d.subscription_id
 join users u on u.id=p.user_id
 join push_broadcasts b on b.id=d.broadcast_id
 where d.status='pending' and d.next_attempt_at<=now() and d.attempts<5
 and u.status<>'archived' and (b.audience='selected' or u.status='active')
 and b.created_at>now()-($1*interval '1 second')
 order by d.next_attempt_at,d.id for update of d skip locked limit 1
 ), claimed as (
 update broadcast_push_deliveries d set attempts=attempts+1,next_attempt_at=now()+interval '2 minutes'
 from candidate c where d.id=c.id returning d.*
 ) select d.id,d.subscription_id,d.broadcast_id,d.attempts,p.endpoint,p.p256dh,p.auth,b.title,b.body,b.created_at
 from claimed d join push_subscriptions p on p.id=d.subscription_id join push_broadcasts b on b.id=d.broadcast_id`, BroadcastMaxAge.Seconds()).Scan(
		&d.ID, &d.SubscriptionID, &d.BroadcastID, &d.Attempts, &d.Subscription.Endpoint, &d.Subscription.Keys.P256dh, &d.Subscription.Keys.Auth, &d.Title, &d.Body, &d.CreatedAt)
	if err == pgx.ErrNoRows {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) CompleteBroadcastDelivery(ctx context.Context, d BroadcastDelivery, status int, sendErr error) error {
	return s.completeDelivery(ctx, "broadcast_push_deliveries", d.ID, d.SubscriptionID, d.Subscription.Endpoint, d.Attempts, status, sendErr)
}
