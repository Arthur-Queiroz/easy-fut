package matches

import (
	"errors"
	"testing"
)

const (
	ana = "11111111-1111-1111-1111-111111111111"
	bia = "22222222-2222-2222-2222-222222222222"
	cai = "33333333-3333-3333-3333-333333333333"
	dav = "44444444-4444-4444-4444-444444444444"
)

// validBallot é um voto que passa em todas as regras; cada caso altera um campo.
func validBallot() Ballot {
	return Ballot{
		MatchStatus:     StatusVoting,
		VoterID:         ana,
		CandidateID:     bia,
		Category:        TopScorer,
		VoterPlayed:     true,
		CandidatePlayed: true,
	}
}

func TestBallotValidate(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Ballot)
		want   error
	}{
		{"voto válido", func(*Ballot) {}, nil},
		{"votação ainda não aberta", func(b *Ballot) { b.MatchStatus = "teams_drawn" }, ErrVotingClosed},
		{"votação encerrada", func(b *Ballot) { b.MatchStatus = StatusFinished }, ErrVotingClosed},
		{"eleitor não jogou", func(b *Ballot) { b.VoterPlayed = false }, ErrNotParticipant},
		{"candidato não jogou", func(b *Ballot) { b.CandidatePlayed = false }, ErrNotParticipant},
		{"perna de pau em si mesmo", func(b *Ballot) { b.Category, b.CandidateID = WorstPlayer, ana }, ErrSelfWorstVote},
		{"artilheiro em si mesmo vale", func(b *Ballot) { b.CandidateID = ana }, nil},
		{"melhor gol em si mesmo vale", func(b *Ballot) { b.Category, b.CandidateID = BestGoal, ana }, nil},
		{"melhor gol também tem limite", func(b *Ballot) { b.Category, b.PreviousVotes = BestGoal, []string{cai, dav} }, ErrVoteLimit},
		{"mesmo jogador de novo", func(b *Ballot) { b.PreviousVotes = []string{bia} }, ErrDuplicateVote},
		{"segundo voto vale", func(b *Ballot) { b.PreviousVotes = []string{cai} }, nil},
		{"terceiro voto", func(b *Ballot) { b.PreviousVotes = []string{cai, dav} }, ErrVoteLimit},
		// Ordem das regras: a resposta mais informativa vem primeiro.
		{"fechada vence não participante", func(b *Ballot) { b.MatchStatus, b.VoterPlayed = StatusFinished, false }, ErrVotingClosed},
		{"repetido vence limite", func(b *Ballot) { b.PreviousVotes = []string{bia, cai} }, ErrDuplicateVote},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := validBallot()
			tc.change(&b)
			if err := b.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseCategory(t *testing.T) {
	for _, raw := range []string{"top_scorer", "worst_player", "best_goal"} {
		got, err := ParseCategory(raw)
		if err != nil || string(got) != raw {
			t.Errorf("ParseCategory(%q) = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "mvp", "best_player", "TOP_SCORER"} {
		if _, err := ParseCategory(raw); !errors.Is(err, ErrInvalidCategory) {
			t.Errorf("ParseCategory(%q) err = %v, want ErrInvalidCategory", raw, err)
		}
	}
}

func TestCheckVotingOpen(t *testing.T) {
	if err := CheckVotingOpen(StatusVoting); err != nil {
		t.Errorf("voting: %v", err)
	}
	for _, status := range []string{"open", "closed", "teams_drawn", StatusFinished, "cancelled"} {
		if err := CheckVotingOpen(status); !errors.Is(err, ErrVotingClosed) {
			t.Errorf("%s: err = %v, want ErrVotingClosed", status, err)
		}
	}
}

// O resultado só aparece depois do encerramento, para não influenciar votos.
func TestResultsVisible(t *testing.T) {
	if !ResultsVisible(StatusFinished) {
		t.Error("finished deveria mostrar o resultado")
	}
	if ResultsVisible(StatusVoting) {
		t.Error("voting não pode mostrar o resultado")
	}
}

func TestWinnerAnnouncement(t *testing.T) {
	cases := map[Category]string{
		TopScorer:   "Bia foi eleito artilheiro da partida",
		WorstPlayer: "Bia foi eleito perna de pau da partida",
		BestGoal:    "Bia foi eleito melhor gol da partida",
	}
	for category, want := range cases {
		if got := WinnerAnnouncement(category, "Bia"); got != want {
			t.Errorf("WinnerAnnouncement(%s) = %q, want %q", category, got, want)
		}
	}
}
