package store

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/matches"
)

func voteTestUser(t *testing.T, s *Store, name string) string {
	t.Helper()
	var id string
	err := s.pool.QueryRow(context.Background(),
		`insert into users(name,email,password_hash) values($1,$2,'test') returning id`,
		name, uuid.NewString()+"@example.invalid").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// voteTestMatch cria uma partida, confirma os jogadores em going e a leva
// até a votação pelo mesmo caminho do admin (fechar confirmações → encerrar).
func voteTestMatch(t *testing.T, s *Store, creator string, going ...string) string {
	t.Helper()
	ctx := context.Background()
	m, err := s.CreateMatch(ctx, MatchInput{MatchDate: "2099-01-01", StartTime: "20:00", EndTime: "22:00", Venue: "Test venue", ConfirmationDeadline: time.Now().Add(time.Hour)}, creator)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range going {
		if err := s.Confirm(ctx, m.ID, id, "going"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetMatchStatus(ctx, m.ID, "closed", "open"); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishMatch(ctx, m.ID, creator, time.Now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestVoteRepeatedOnSameCandidateIsRejected(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia")
	match := voteTestMatch(t, s, ana, ana, bia)

	if err := s.CastVote(ctx, match, ana, "top_scorer", bia, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.CastVote(ctx, match, ana, "top_scorer", bia, nil); !errors.Is(err, matches.ErrDuplicateVote) {
		t.Fatalf("segundo voto igual: err = %v, want ErrDuplicateVote", err)
	}
	// O mesmo candidato em outra categoria é outro voto.
	if err := s.CastVote(ctx, match, ana, "worst_player", bia, nil); err != nil {
		t.Fatal(err)
	}
}

// limitCheck é a parte de matches.Ballot que depende dos votos já gravados.
func limitCheck(previous []string) error {
	if len(previous) >= matches.MaxVotesPerCategory {
		return matches.ErrVoteLimit
	}
	return nil
}

// O handler valida o limite com os votos lidos dentro do lock do eleitor:
// envios simultâneos não passam de 2.
func TestCastVoteLimitHoldsUnderConcurrency(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana := voteTestUser(t, s, "Ana")
	others := []string{voteTestUser(t, s, "Bia"), voteTestUser(t, s, "Caio"), voteTestUser(t, s, "Davi"), voteTestUser(t, s, "Edu")}
	match := voteTestMatch(t, s, ana, append([]string{ana}, others...)...)

	for round := range 10 {
		if _, err := s.pool.Exec(ctx, `delete from votes where match_id = $1`, match); err != nil {
			t.Fatal(err)
		}
		errs := make(chan error, len(others))
		var wg sync.WaitGroup
		for _, candidate := range others {
			wg.Go(func() { errs <- s.CastVote(ctx, match, ana, "top_scorer", candidate, limitCheck) })
		}
		wg.Wait()
		close(errs)
		rejected := 0
		for err := range errs {
			switch {
			case errors.Is(err, matches.ErrVoteLimit):
				rejected++
			case err != nil:
				t.Fatal(err)
			}
		}
		votes, err := s.MyVotes(ctx, match, ana)
		if err != nil {
			t.Fatal(err)
		}
		if len(votes["top_scorer"]) != 2 || rejected != 2 {
			t.Fatalf("rodada %d: %d votos gravados e %d recusados, want 2 e 2", round, len(votes["top_scorer"]), rejected)
		}
	}
}

// Trocar um voto com 2 já gravados é uma operação só (FUT-003).
func TestReplaceVotesSwapsWithinTheLimit(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia, caio, davi := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia"), voteTestUser(t, s, "Caio"), voteTestUser(t, s, "Davi")
	match := voteTestMatch(t, s, ana, ana, bia, caio, davi)
	for _, v := range [][2]string{{"top_scorer", bia}, {"top_scorer", caio}, {"worst_player", davi}} {
		if err := s.CastVote(ctx, match, ana, v[0], v[1], nil); err != nil {
			t.Fatal(err)
		}
	}

	err := s.ReplaceVotes(ctx, match, ana, map[matches.Category][]string{
		matches.TopScorer: {bia, davi},
		matches.BestGoal:  {ana},
	})
	if err != nil {
		t.Fatal(err)
	}
	votes, err := s.MyVotes(ctx, match, ana)
	if err != nil {
		t.Fatal(err)
	}
	// Bia continua primeiro (voto mantido, não regravado); categoria ausente
	// no envio (worst_player) fica como estava.
	want := map[string][]string{"top_scorer": {bia, davi}, "worst_player": {davi}, "best_goal": {ana}}
	for category, ids := range want {
		if !slices.Equal(votes[category], ids) {
			t.Errorf("%s = %v, want %v", category, votes[category], ids)
		}
	}

	// Lista vazia desfaz a categoria.
	if err := s.ReplaceVotes(ctx, match, ana, map[matches.Category][]string{matches.TopScorer: nil}); err != nil {
		t.Fatal(err)
	}
	if votes, _ = s.MyVotes(ctx, match, ana); len(votes["top_scorer"]) != 0 {
		t.Errorf("top_scorer depois de lista vazia = %v", votes["top_scorer"])
	}
}

func TestReplaceVotesRejectsClosedVoting(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia")
	match := voteTestMatch(t, s, ana, ana, bia)
	if _, err := s.pool.Exec(ctx, `update matches set status = 'finished' where id = $1`, match); err != nil {
		t.Fatal(err)
	}

	err := s.ReplaceVotes(ctx, match, ana, map[matches.Category][]string{matches.TopScorer: {bia}})
	if !errors.Is(err, matches.ErrVotingClosed) {
		t.Fatalf("err = %v, want ErrVotingClosed", err)
	}
	if votes, _ := s.MyVotes(ctx, match, ana); len(votes) != 0 {
		t.Errorf("votos gravados com a votação fechada: %v", votes)
	}
}

func TestMyVotesKeepsOrderAndRemoveVoteUndoes(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia, caio := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia"), voteTestUser(t, s, "Caio")
	match := voteTestMatch(t, s, ana, ana, bia, caio)

	for _, candidate := range []string{caio, bia} {
		if err := s.CastVote(ctx, match, ana, "top_scorer", candidate, nil); err != nil {
			t.Fatal(err)
		}
	}
	votes, err := s.MyVotes(ctx, match, ana)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(votes["top_scorer"], []string{caio, bia}) {
		t.Fatalf("MyVotes = %v, want [caio bia] na ordem do voto", votes["top_scorer"])
	}

	if err := s.RemoveVote(ctx, match, ana, "top_scorer", caio); err != nil {
		t.Fatal(err)
	}
	// Desfazer um voto inexistente não é erro.
	if err := s.RemoveVote(ctx, match, ana, "top_scorer", caio); err != nil {
		t.Fatal(err)
	}
	votes, err = s.MyVotes(ctx, match, ana)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(votes["top_scorer"], []string{bia}) {
		t.Fatalf("depois de desfazer: %v, want [bia]", votes["top_scorer"])
	}
}

func TestVoteResultsTieReturnsEveryWinnerByName(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia, caio, davi := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia"), voteTestUser(t, s, "Caio"), voteTestUser(t, s, "Davi")
	match := voteTestMatch(t, s, ana, ana, bia, caio, davi)

	// Caio 2, Bia 2, Davi 1 em top_scorer; ninguém nas outras categorias.
	for _, v := range [][2]string{{ana, caio}, {davi, caio}, {ana, bia}, {caio, bia}, {bia, davi}} {
		if err := s.CastVote(ctx, match, v[0], "top_scorer", v[1], nil); err != nil {
			t.Fatal(err)
		}
	}
	results, err := s.VoteResults(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Category != "top_scorer" || results[1].Category != "worst_player" || results[2].Category != "best_goal" {
		t.Fatalf("categorias = %+v", results)
	}
	top := results[0]
	if top.VoteCount != 5 {
		t.Errorf("VoteCount = %d, want 5 (total de votos da categoria)", top.VoteCount)
	}
	var names []string
	for _, w := range top.Winners {
		names = append(names, w.Name)
		if w.Votes != 2 {
			t.Errorf("%s com %d votos, want 2", w.Name, w.Votes)
		}
	}
	if !slices.Equal(names, []string{"Bia", "Caio"}) {
		t.Errorf("vencedores = %v, want [Bia Caio]", names)
	}
	for _, empty := range results[1:] {
		if empty.VoteCount != 0 || empty.Winners == nil || len(empty.Winners) != 0 {
			t.Errorf("categoria sem votos = %+v, want lista vazia e 0", empty)
		}
	}
}

func TestIsParticipantMeansConfirmedGoing(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana, bia, caio := voteTestUser(t, s, "Ana"), voteTestUser(t, s, "Bia"), voteTestUser(t, s, "Caio")
	m, err := s.CreateMatch(ctx, MatchInput{MatchDate: "2099-01-01", StartTime: "20:00", EndTime: "22:00", Venue: "Test venue", ConfirmationDeadline: time.Now().Add(time.Hour)}, ana)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Confirm(ctx, m.ID, ana, "going"); err != nil {
		t.Fatal(err)
	}
	if err := s.Confirm(ctx, m.ID, bia, "not_going"); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{ana: true, bia: false, caio: false} {
		got, err := s.IsParticipant(ctx, m.ID, id)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("IsParticipant(%s) = %v, want %v", id, got, want)
		}
	}
}

func TestCloseExpiredVotingOnlyClosesPastDeadline(t *testing.T) {
	s := newTestStore(t, "votes_test")
	ctx := context.Background()
	ana := voteTestUser(t, s, "Ana")
	open := voteTestMatch(t, s, ana, ana)
	expired := voteTestMatch(t, s, ana, ana)
	if _, err := s.pool.Exec(ctx, `update matches set voting_closes_at = now() - interval '1 minute' where id = $1`, expired); err != nil {
		t.Fatal(err)
	}

	ids, err := s.CloseExpiredVoting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{expired}) {
		t.Fatalf("fechadas = %v, want só a vencida", ids)
	}
	for id, want := range map[string]string{open: "voting", expired: "finished"} {
		m, err := s.MatchByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != want {
			t.Errorf("status = %s, want %s", m.Status, want)
		}
	}
}
