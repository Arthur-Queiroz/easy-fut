package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/matches"
)

// setupVoteData cria partida em votação, admin votante e 2 candidatos.
func setupVoteData(t *testing.T, s *Store, status string) (matchID, voterID, candA, candB string) {
	t.Helper()
	ctx := context.Background()
	creator := votingTestUser(t, s, "active")
	matchID = votingTestMatch(t, s, status, time.Now().Format("2006-01-02"), "23:00")
	voterID = votingTestUser(t, s, "active")
	candA = votingTestUser(t, s, "active")
	candB = votingTestUser(t, s, "active")
	for _, uid := range []string{voterID, candA, candB} {
		if _, err := s.pool.Exec(ctx, `insert into match_confirmations (match_id, user_id, response) values ($1,$2,'going')`, matchID, uid); err != nil {
			t.Fatal(err)
		}
	}
	_ = creator
	return matchID, voterID, candA, candB
}

func TestBestGoalCategoryAccepted(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	matchID, voterID, candA, candB := setupVoteData(t, s, "voting")

	for _, cand := range []string{candA, candB} {
		if err := s.CastVote(ctx, matchID, voterID, "best_goal", cand, nil); err != nil {
			t.Fatalf("voto best_goal recusado: %v", err)
		}
	}
	// 2 votos em jogadores distintos: ok. Terceiro em outro jogador viola apenas regra do handler, não do store.
	if err := s.CastVote(ctx, matchID, voterID, "best_goal", candA, nil); !errors.Is(err, matches.ErrDuplicateVote) {
		t.Fatalf("duplicado deveria dar ErrDuplicateVote, veio %v", err)
	}
}

func TestVoteTallyByMatchFull(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	matchID, voterA, candA, candB := setupVoteData(t, s, "voting")
	voterB := votingTestUser(t, s, "active")
	if _, err := s.pool.Exec(ctx, `insert into match_confirmations (match_id, user_id, response) values ($1,$2,'going')`, matchID, voterB); err != nil {
		t.Fatal(err)
	}

	// top_scorer: candA 2 votos; worst_player: candB 1; best_goal: candA 1
	for _, v := range []struct{ voter, cat, cand string }{
		{voterA, "top_scorer", candA}, {voterB, "top_scorer", candA},
		{voterA, "worst_player", candB}, {voterA, "best_goal", candA},
	} {
		if err := s.CastVote(ctx, matchID, v.voter, v.cat, v.cand, nil); err != nil {
			t.Fatalf("CastVote %s→%s falhou: %v", v.voter, v.cat, err)
		}
	}

	tally, err := s.VoteTallyByMatch(ctx, matchID)
	if err != nil {
		t.Fatal(err)
	}
	// 3 linhas (uma por candidato com ≥1 voto em cada categoria agregada):
	// candA (top_scorer 2 + best_goal 1 → 2 linhas), candB (worst_player → 1 linha)
	if len(tally) != 3 {
		t.Fatalf("tally = %d linhas, queria 3 (candA ts + candA bg + candB wp)", len(tally))
	}
	counts := map[string]int{}
	for _, row := range tally {
		counts[row.Category+"|"+row.CandidateID] = row.Votes
	}
	if counts["top_scorer|"+candA] != 2 || counts["worst_player|"+candB] != 1 || counts["best_goal|"+candA] != 1 {
		t.Fatalf("tally incorreto: %v", counts)
	}
}

func TestVotesFlatAndUserHistory(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	matchID, voterID, candA, _ := setupVoteData(t, s, "voting")
	_ = s.CastVote(ctx, matchID, voterID, "top_scorer", candA, nil)
	_ = s.CastVote(ctx, matchID, voterID, "best_goal", candA, nil)

	// flat sem filtro
	flat, err := s.MatchesVotesFlat(ctx, matchID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) != 2 {
		t.Fatalf("flat = %d, queria 2", len(flat))
	}
	// flat filtrado por votante
	flat, err = s.MatchesVotesFlat(ctx, matchID, voterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) != 2 || flat[0].VoterName == "" || flat[0].CandidateName == "" {
		t.Fatalf("flat filtrado incorreto: %+v", flat)
	}
	// outros: vazio
	other := uuid.NewString()
	flat, _ = s.MatchesVotesFlat(ctx, matchID, other)
	if len(flat) != 0 {
		t.Fatalf("filtro por votante inexistente devia ser vazio, veio %d", len(flat))
	}
	// histórico do usuário entre partidas
	match2 := votingTestMatch(t, s, "voting", "2026-10-25", "23:00")
	if _, err := s.pool.Exec(ctx, `insert into match_confirmations (match_id, user_id, response) values ($1,$2,'going')`, match2, voterID); err != nil {
		t.Fatal(err)
	}
	_ = s.CastVote(ctx, match2, voterID, "worst_player", candA, nil)
	hist, err := s.UserVoteHistory(ctx, voterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 3 {
		t.Fatalf("histórico = %d, queria 3", len(hist))
	}
	if hist[0].Category != "worst_player" { // desc por created_at
		t.Fatalf("histórico não está desc: primeira categoria = %s", hist[0].Category)
	}
}

func TestMatchesVoteSummary(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	matchID, voterID, candA, _ := setupVoteData(t, s, "voting")
	_ = s.CastVote(ctx, matchID, voterID, "top_scorer", candA, nil)

	out, err := s.MatchesVoteSummary(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("summary vazio com partida existente")
	}
	var found bool
	for _, m := range out {
		if m.MatchID == matchID {
			found = true
			if m.Voters != 1 || m.Votes != 1 {
				t.Fatalf("summary da partida: voters=%d votes=%d, queria 1/1", m.Voters, m.Votes)
			}
		}
	}
	if !found {
		t.Fatal("partida não apareceu no summary")
	}
}
