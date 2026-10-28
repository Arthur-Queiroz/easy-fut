package store

import (
	"context"
	"testing"
	"time"
)

// TestNextMatchIncludesVoting cobre o FUT-002: partida em 'voting' aparece
// como "próxima partida" (mesmo com data passada) e tem prioridade sobre
// partidas futuras ainda em aberto.
func TestNextMatchIncludesVoting(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()

	// Partida de 3 dias atrás, em votação (aberta automaticamente pelo FUT-001).
	inVoting := votingTestMatch(t, s, "voting", time.Now().AddDate(0, 0, -3).Format("2006-01-02"), "10:00")
	// Partida futura em aberto.
	futureOpen := votingTestMatch(t, s, "open", time.Now().AddDate(0, 0, 7).Format("2006-01-02"), "10:00")
	// Partida antiga finalizada: nunca é candidata.
	oldFinished := votingTestMatch(t, s, "finished", time.Now().AddDate(0, 0, -30).Format("2006-01-02"), "10:00")

	// Com votação em aberto, ela vence a futura em aberto.
	next, err := s.NextMatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != inVoting {
		t.Fatalf("NextMatch = %s, queria a partida em votação %s", next.ID, inVoting)
	}
	if next.Status != "voting" {
		t.Fatalf("NextMatch status = %q, queria voting", next.Status)
	}

	// Sem votação: a futura em aberto vence a finalizadora antiga.
	if _, err := s.pool.Exec(ctx, `update matches set status='finished' where id=$1`, inVoting); err != nil {
		t.Fatal(err)
	}
	next, err = s.NextMatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != futureOpen {
		t.Fatalf("NextMatch = %s, queria a futura em aberto %s", next.ID, futureOpen)
	}

	// Nada elegível: ErrNotFound.
	if _, err := s.pool.Exec(ctx, `update matches set status='cancelled' where id=$1`, futureOpen); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NextMatch(ctx); err != ErrNotFound {
		t.Fatalf("queria ErrNotFound, veio %v", err)
	}

	_ = oldFinished // verificada por exclusão via IDs acima
}
