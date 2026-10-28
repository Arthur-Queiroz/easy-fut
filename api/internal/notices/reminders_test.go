package notices

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCheckMatchReminder(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Minute)
	for _, tc := range []struct {
		name, kind, status       string
		deadline, votingClosesAt *time.Time
		want                     error
	}{
		{"presença aberta", "confirmation", "open", &future, nil, nil},
		{"presença encerrada", "confirmation", "open", &past, nil, ErrReminderClosed},
		{"partida fechada", "confirmation", "closed", &future, nil, ErrReminderClosed},
		{"votação aberta", "vote", "voting", nil, &future, nil},
		{"votação encerrada", "vote", "voting", nil, &past, ErrReminderClosed},
		{"partida finalizada", "vote", "finished", nil, &future, ErrReminderClosed},
		{"tipo inválido", "other", "open", &future, nil, ErrInvalidReminderKind},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckMatchReminder(tc.kind, tc.status, tc.deadline, tc.votingClosesAt, now); !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, esperado %v", err, tc.want)
			}
		})
	}
}

func TestMatchReminderMessage(t *testing.T) {
	message := MatchReminderMessage("vote", "Ana", "2026-10-28", "Arena 1", "https://fut.example", "abc")
	for _, part := range []string{"Ana", "28/10", "votação", "https://fut.example/partida?match=abc"} {
		if !strings.Contains(message, part) {
			t.Fatalf("mensagem %q não contém %q", message, part)
		}
	}
}
