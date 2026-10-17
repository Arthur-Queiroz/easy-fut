package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func claimAllBroadcasts(t *testing.T, s *Store) map[string]BroadcastDelivery {
	t.Helper()
	got := map[string]BroadcastDelivery{}
	for {
		d, err := s.ClaimBroadcastDelivery(context.Background())
		if errors.Is(err, ErrNotFound) {
			return got
		}
		if err != nil {
			t.Fatal(err)
		}
		got[d.Subscription.Endpoint] = d
	}
}

func TestBroadcastToAllReachesOnlyActiveUsers(t *testing.T) {
	s := pushTestStore(t)
	ctx := context.Background()
	admin, adminSub := pushTestUser(t, s, "active")
	_, activeSub := pushTestUser(t, s, "active")
	pushTestUser(t, s, "inactive")
	pushTestUser(t, s, "archived")
	b, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Aviso", Body: "Jogo cancelado", Audience: "all"}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Users != 2 || b.Devices != 2 || b.CreatorName != "Push Test" {
		t.Fatalf("wrong summary: %+v", b)
	}
	got := claimAllBroadcasts(t, s)
	if len(got) != 2 || got[adminSub.Endpoint].ID == "" || got[activeSub.Endpoint].ID == "" {
		t.Fatalf("wrong recipients: %v", got)
	}
	d := got[activeSub.Endpoint]
	if d.Title != "Aviso" || d.Body != "Jogo cancelado" || d.BroadcastID != b.ID {
		t.Fatalf("wrong delivery: %+v", d)
	}
	if err := s.CompleteBroadcastDelivery(ctx, d, 201, nil); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListPushBroadcasts(ctx, 20)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if list[0].Sent != 1 || list[0].Pending != 1 || list[0].Devices != 2 || list[0].Users != 2 {
		t.Fatalf("wrong counts: %+v", list[0])
	}
}

func TestBroadcastToSelectedIncludesInactiveButNeverArchived(t *testing.T) {
	s := pushTestStore(t)
	ctx := context.Background()
	admin, _ := pushTestUser(t, s, "active")
	inactive, inactiveSub := pushTestUser(t, s, "inactive")
	archived, _ := pushTestUser(t, s, "archived")
	pushTestUser(t, s, "active") // not selected
	b, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Mensalidade", Body: "Regularize", Audience: "selected", UserIDs: []string{inactive, archived}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.Users != 1 {
		t.Fatalf("archived or unselected user queued: %+v", b)
	}
	got := claimAllBroadcasts(t, s)
	if len(got) != 1 || got[inactiveSub.Endpoint].ID == "" {
		t.Fatalf("wrong recipients: %v", got)
	}
}

func TestBroadcastWithoutDevicesIsNotSaved(t *testing.T) {
	s := pushTestStore(t)
	ctx := context.Background()
	admin, _ := pushTestUser(t, s, "active")
	var noDevice string
	if err := s.pool.QueryRow(ctx, `insert into users(name,email,password_hash) values('Sem aparelho','sem@example.invalid','x') returning id`).Scan(&noDevice); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Aviso", Body: "Oi", Audience: "selected", UserIDs: []string{noDevice}}, admin)
	if !errors.Is(err, ErrNoPushRecipients) {
		t.Fatalf("expected ErrNoPushRecipients, got %v", err)
	}
	var count int
	s.pool.QueryRow(ctx, `select count(*) from push_broadcasts`).Scan(&count)
	if count != 0 {
		t.Fatal("broadcast saved without recipients")
	}
}

func TestBroadcastRechecksStatusAndAgeAtSendTime(t *testing.T) {
	s := pushTestStore(t)
	ctx := context.Background()
	admin, _ := pushTestUser(t, s, "active")
	later, _ := pushTestUser(t, s, "active")
	if _, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Aviso", Body: "Oi", Audience: "all"}, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update users set status='inactive' where id=$1`, later); err != nil {
		t.Fatal(err)
	}
	if got := claimAllBroadcasts(t, s); len(got) != 1 {
		t.Fatalf("user inactivated after queueing still received an 'all' notice: %d", len(got))
	}
	if _, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Antigo", Body: "Oi", Audience: "all"}, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update push_broadcasts set created_at=now()-interval '25 hours' where title='Antigo'`); err != nil {
		t.Fatal(err)
	}
	if got := claimAllBroadcasts(t, s); len(got) != 0 {
		t.Fatal("stale notice delivered")
	}
}

func TestBroadcastRetryAndSharedDevice(t *testing.T) {
	s := pushTestStore(t)
	ctx := context.Background()
	admin, sub := pushTestUser(t, s, "active")
	if _, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "Aviso", Body: "Oi", Audience: "all"}, admin); err != nil {
		t.Fatal(err)
	}
	d, err := s.ClaimBroadcastDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteBroadcastDelivery(ctx, d, 503, fmt.Errorf("unavailable")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimBroadcastDelivery(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatal("retry has no backoff")
	}
	other, _ := pushTestUser(t, s, "active")
	if err := s.SavePushSubscription(ctx, other, sub); err != nil {
		t.Fatal(err)
	}
	var count int
	s.pool.QueryRow(ctx, `select count(*) from broadcast_push_deliveries`).Scan(&count)
	if count != 0 {
		t.Fatal("shared device inherited notice queued for previous account")
	}
}
