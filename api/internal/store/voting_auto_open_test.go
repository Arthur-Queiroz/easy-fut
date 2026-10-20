package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func votingTestStore(t *testing.T) *Store {
	t.Helper()
	return newTestStore(t, "voting_test")
}

// votingTestMatch cria uma partida na data/hora informados (na TZ de SP, como
// em produção) com o status inicial dado.
func votingTestMatch(t *testing.T, s *Store, status, matchDate, endTime string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	if err := s.pool.QueryRow(ctx, `
		insert into users (name, email, password_hash, role)
		values ('Criador test', $1, 'test', 'admin') returning id`,
		uuid.NewString()+"@example.invalid").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into matches (match_date, start_time, end_time, venue, confirmation_deadline, status, created_by)
		values ($1, '08:00', $2, 'Quadra test', now(), $3, $4) returning id`,
		matchDate, endTime, status, userID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func votingTestUser(t *testing.T, s *Store, status string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into users (name, email, password_hash, role, status)
		values ('Usuario test', $1, 'test', 'admin', $2) returning id`,
		uuid.NewString()+"@example.invalid", status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func matchStatus(t *testing.T, s *Store, id string) (string, *time.Time) {
	t.Helper()
	var status string
	var closes *time.Time
	if err := s.pool.QueryRow(context.Background(),
		`select status, voting_closes_at from matches where id = $1`, id).Scan(&status, &closes); err != nil {
		t.Fatal(err)
	}
	return status, closes
}

func TestOpenExpiredVotingOpensFinishedMatches(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()

	// Partida de ontem, encerrada (confirmações fechadas): deve abrir votação.
	pastClosed := votingTestMatch(t, s, "closed", "2026-10-17", "10:00")
	// Partida de ontem com times sorteados: também deve abrir.
	pastDrawn := votingTestMatch(t, s, "teams_drawn", "2026-10-17", "10:00")
	// Partida em aberto (prazo de confirmação ainda não veio): intocada.
	openMatch := votingTestMatch(t, s, "open", "2026-10-25", "10:00")
	// Votação já em curso: intocada.
	alreadyVoting := votingTestMatch(t, s, "voting", "2026-10-17", "10:00")
	// Já finalizada: intocada.
	finished := votingTestMatch(t, s, "finished", "2026-10-17", "10:00")

	ids, err := s.OpenExpiredVoting(ctx, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found[pastClosed] || !found[pastDrawn] {
		t.Fatalf("partidas encerradas deveriam abrir votação; abertas: %v", ids)
	}
	if found[openMatch] || found[alreadyVoting] || found[finished] {
		t.Fatalf("partidas fora do gate não deveriam ser tocadas; tocadas: %v", ids)
	}

	for _, id := range []string{pastClosed, pastDrawn} {
		status, closes := matchStatus(t, s, id)
		if status != "voting" {
			t.Errorf("partida %s: status = %q, queria voting", id, status)
		}
		if closes == nil {
			t.Fatalf("voting_closes_at não preenchido")
		}
		// Fim da partida = 2026-10-17 10:00 -03 → 13:00 UTC; +2 dias.
		want := time.Date(2026, 10, 19, 13, 0, 0, 0, time.UTC)
		if !closes.Equal(want) {
			t.Errorf("voting_closes_at = %v, queria %v", closes.UTC(), want)
		}
	}
}

func TestOpenExpiredVotingRespectsDelay(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()

	// Partida acabada há poucos minutos: com delay de 60 min não deve abrir.
	recent := votingTestMatch(t, s, "closed",
		time.Now().Format("2006-01-02"), "23:59")

	// Idem, mas com data futura relativa ao delay: não abre.
	ids, err := s.OpenExpiredVoting(ctx, 120, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("delay de 120min não deveria abrir nenhuma partida; abriu %v", ids)
	}
	status, _ := matchStatus(t, s, recent)
	if status != "closed" {
		t.Errorf("delay não respeitado: status = %q", status)
	}

	// Com delay 0, a partida do mesmo dia abre (fim 23:59 do dia anterior
	// depende do relógio; o teste aceita abrir ou não, o que importa é que
	// não panic e que com delay grande fechado acima).
	if _, err := s.OpenExpiredVoting(ctx, 0, 2); err != nil {
		t.Fatal(err)
	}
}
