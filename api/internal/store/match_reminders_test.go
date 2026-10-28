package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManualMatchRemindersTargetOnlyPendingMembers(t *testing.T) {
	s := newTestStore(t, "match_reminders_test")
	ctx := context.Background()
	now := time.Now()
	admin, err := s.CreateUser(ctx, "Admin", "reminder-admin@example.invalid", "11999990001", "hash", "#123456", "admin")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.CreateUser(ctx, "Pendente", "reminder-pending@example.invalid", "11999990002", "hash", "#123456", "player")
	if err != nil {
		t.Fatal(err)
	}
	noPhone, err := s.CreateUser(ctx, "Sem telefone", "reminder-nophone@example.invalid", "", "hash", "#123456", "player")
	if err != nil {
		t.Fatal(err)
	}
	responded, err := s.CreateUser(ctx, "Respondeu", "reminder-responded@example.invalid", "11999990003", "hash", "#123456", "player")
	if err != nil {
		t.Fatal(err)
	}
	var matchID string
	err = s.pool.QueryRow(ctx, `insert into matches(match_date,start_time,end_time,venue,confirmation_deadline,created_by)
		values(current_date,'20:00','22:00','Arena 1',$1,$2) returning id`, now.Add(time.Hour), admin.ID).Scan(&matchID)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []User{pending, noPhone, responded} {
		response := "no_response"
		if u.ID == responded.ID {
			response = "going"
		}
		if _, err := s.pool.Exec(ctx, `insert into match_confirmations(match_id,user_id,response) values($1,$2,$3)`, matchID, u.ID, response); err != nil {
			t.Fatal(err)
		}
	}
	recipients, err := s.ReminderRecipients(ctx, matchID, "confirmation", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 2 || recipients[0].Name != "Pendente" || !recipients[0].HasWhatsApp || recipients[1].HasWhatsApp {
		t.Fatalf("destinatários = %+v", recipients)
	}
	queued, err := s.QueueManualMatchReminders(ctx, matchID, "confirmation", []string{pending.ID, noPhone.ID, responded.ID}, "https://fut.example", now)
	if err != nil || queued != 1 {
		t.Fatalf("enfileirados = %d, erro = %v", queued, err)
	}
	if _, err := s.QueueManualMatchReminders(ctx, matchID, "confirmation", []string{pending.ID}, "https://fut.example", now); !errors.Is(err, ErrNoReminderRecipients) {
		t.Fatalf("segundo envio = %v", err)
	}
	due, err := s.DueNotifications(ctx)
	if err != nil || len(due) != 1 || due[0].Phone != "5511999990002" {
		t.Fatalf("fila = %+v, erro = %v", due, err)
	}
	if err := s.MarkNotificationFailed(ctx, due[0].ID, "falha simulada"); err != nil {
		t.Fatal(err)
	}
	queued, err = s.QueueManualMatchReminders(ctx, matchID, "confirmation", []string{pending.ID}, "https://fut.example", now)
	if err != nil || queued != 1 {
		t.Fatalf("reenvio após falha = %d, erro = %v", queued, err)
	}
	if _, err := s.pool.Exec(ctx, `update match_confirmations set response='going' where match_id=$1 and user_id=$2`, matchID, pending.ID); err != nil {
		t.Fatal(err)
	}
	due, err = s.DueNotifications(ctx)
	if err != nil || len(due) != 0 {
		t.Fatalf("lembrete após confirmação = %+v, erro = %v", due, err)
	}
}

func TestVoteReminderSkipsMembersWhoAlreadyVoted(t *testing.T) {
	s := newTestStore(t, "match_reminders_test")
	ctx := context.Background()
	now := time.Now()
	admin, _ := s.CreateUser(ctx, "Admin", "vote-admin@example.invalid", "11999990001", "hash", "#123456", "admin")
	pending, _ := s.CreateUser(ctx, "Pendente", "vote-pending@example.invalid", "11999990002", "hash", "#123456", "player")
	voted, _ := s.CreateUser(ctx, "Votou", "vote-voted@example.invalid", "11999990003", "hash", "#123456", "player")
	var matchID string
	if err := s.pool.QueryRow(ctx, `insert into matches(match_date,start_time,end_time,venue,confirmation_deadline,status,voting_closes_at,created_by)
		values(current_date,'20:00','22:00','Arena 1',$1,'voting',$2,$3) returning id`, now.Add(-time.Hour), now.Add(time.Hour), admin.ID).Scan(&matchID); err != nil {
		t.Fatal(err)
	}
	for _, u := range []User{pending, voted} {
		if _, err := s.pool.Exec(ctx, `insert into match_confirmations(match_id,user_id,response) values($1,$2,'going')`, matchID, u.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `insert into votes(match_id,voter_id,category,candidate_id) values($1,$2,'top_scorer',$3)`, matchID, voted.ID, pending.ID); err != nil {
		t.Fatal(err)
	}
	recipients, err := s.ReminderRecipients(ctx, matchID, "vote", now)
	if err != nil || len(recipients) != 1 || recipients[0].ID != pending.ID {
		t.Fatalf("pendentes de voto = %+v, erro = %v", recipients, err)
	}
	queued, err := s.QueueManualMatchReminders(ctx, matchID, "vote", []string{pending.ID, voted.ID}, "https://fut.example", now)
	if err != nil || queued != 1 {
		t.Fatalf("enfileirados = %d, erro = %v", queued, err)
	}
	if _, err := s.pool.Exec(ctx, `insert into votes(match_id,voter_id,category,candidate_id) values($1,$2,'top_scorer',$3)`, matchID, pending.ID, voted.ID); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueNotifications(ctx)
	if err != nil || len(due) != 0 {
		t.Fatalf("lembrete após voto = %+v, erro = %v", due, err)
	}
}
