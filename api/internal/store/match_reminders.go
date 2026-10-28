package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"futdarapaziada/api/internal/notices"
	"futdarapaziada/api/internal/notify"
)

var ErrNoReminderRecipients = errors.New("nenhum membro pendente com WhatsApp disponível")

type MatchReminderRecipient struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AvatarColor    string `json:"avatar_color"`
	AvatarURL      string `json:"avatar_url"`
	HasWhatsApp    bool   `json:"has_whatsapp"`
	ReminderStatus string `json:"reminder_status"`
	phone          string
}

// ReminderRecipients mostra apenas membros ativos que ainda devem responder
// ou votar. Um envio agendado/entregue não aparece como ação disponível.
func (s *Store) ReminderRecipients(ctx context.Context, matchID, kind string, now time.Time) ([]MatchReminderRecipient, error) {
	m, err := s.MatchByID(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if err := notices.CheckMatchReminder(kind, m.Status, &m.ConfirmationDeadline, m.VotingClosesAt, now); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		select u.id, u.name, u.avatar_color, u.avatar_url, u.phone,
		       coalesce((select n.status from notifications n where n.match_id=$1 and n.user_id=u.id
		                 and n.reminder_kind=$2), '')
		from match_confirmations mc join users u on u.id=mc.user_id
		where mc.match_id=$1 and u.kind='member' and u.status='active'
		  and (($2='confirmation' and mc.response='no_response') or
		       ($2='vote' and mc.response='going' and not exists (
		           select 1 from votes v where v.match_id=$1 and v.voter_id=u.id)))
		order by u.name`, matchID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recipients []MatchReminderRecipient
	for rows.Next() {
		var r MatchReminderRecipient
		if err := rows.Scan(&r.ID, &r.Name, &r.AvatarColor, &r.AvatarURL, &r.phone, &r.ReminderStatus); err != nil {
			return nil, err
		}
		_, err := notify.NormalizeNumber(r.phone)
		r.HasWhatsApp = err == nil
		recipients = append(recipients, r)
	}
	return recipients, rows.Err()
}

// QueueManualMatchReminders registra no máximo um lembrete por membro, partida e
// tipo. Uma falha de entrega pode ser reenfileirada manualmente.
func (s *Store) QueueManualMatchReminders(ctx context.Context, matchID, kind string, userIDs []string, appURL string, now time.Time) (int, error) {
	m, err := s.MatchByID(ctx, matchID)
	if err != nil {
		return 0, err
	}
	recipients, err := s.ReminderRecipients(ctx, matchID, kind, now)
	if err != nil {
		return 0, err
	}
	chosen := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		chosen[id] = true
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	queued := 0
	for _, r := range recipients {
		if !chosen[r.ID] || !r.HasWhatsApp || r.ReminderStatus == "scheduled" || r.ReminderStatus == "sent" {
			continue
		}
		phone, _ := notify.NormalizeNumber(r.phone)
		message := notices.MatchReminderMessage(kind, r.Name, m.MatchDate, m.Venue, appURL, matchID)
		var id string
		err := tx.QueryRow(ctx, `
			insert into notifications (user_id, phone, message, scheduled_at, match_id, reminder_kind)
			select $2, $3, $4, $5, m.id, $6 from matches m
			join match_confirmations mc on mc.match_id=m.id and mc.user_id=$2
			join users u on u.id=$2
			where m.id=$1 and u.kind='member' and u.status='active'
			  and (($6='confirmation' and m.status='open' and m.confirmation_deadline>$5
			        and mc.response='no_response') or
			       ($6='vote' and m.status='voting' and m.voting_closes_at>$5
			        and mc.response='going' and not exists (
			            select 1 from votes v where v.match_id=m.id and v.voter_id=u.id)))
			on conflict (match_id,user_id,reminder_kind) where match_id is not null
			do update set phone=excluded.phone, message=excluded.message,
			              scheduled_at=excluded.scheduled_at, status='scheduled', error=''
			where notifications.status='failed'
			returning id`, matchID, r.ID, phone, message, now, kind).Scan(&id)
		if err == nil {
			queued++
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	}
	if queued == 0 {
		return 0, ErrNoReminderRecipients
	}
	return queued, tx.Commit(ctx)
}
