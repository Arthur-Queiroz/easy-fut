// Package matches reúne as regras de domínio das partidas. Ele não conhece
// HTTP nem banco: recebe os fatos já carregados e decide se a ação vale.
package matches

import (
	"errors"
	"fmt"
	"slices"
)

// Estados da partida usados pelas regras de votação.
const (
	StatusVoting   = "voting"
	StatusFinished = "finished"
)

// Category é uma das categorias da votação pós-jogo.
type Category string

const (
	TopScorer   Category = "top_scorer"
	WorstPlayer Category = "worst_player"
	BestGoal    Category = "best_goal" // FUT-005: mesmas regras do artilheiro
)

// Categories lista as categorias na ordem em que são apresentadas.
var Categories = []Category{TopScorer, WorstPlayer, BestGoal}

// MaxVotesPerCategory é quantos jogadores distintos cada participante pode
// votar em uma categoria. best_goal é exceção: apenas 1 voto (o gol da
// partida é único — decisão do Arthur em 2026-10-25).
const MaxVotesPerCategory = 2

// MaxVotesFor devolve o limite por categoria (2 votos; 1 no melhor gol).
func MaxVotesFor(c Category) int {
	if c == BestGoal {
		return 1
	}
	return MaxVotesPerCategory
}

var (
	ErrInvalidCategory = errors.New("categoria inválida")
	ErrVotingClosed    = errors.New("a votação desta partida não está aberta")
	ErrNotParticipant  = errors.New("somente quem participou da partida pode votar e receber votos")
	ErrSelfWorstVote   = errors.New("não vale votar em si mesmo nessa categoria")
	ErrDuplicateVote   = errors.New("você já votou neste jogador nesta categoria")
	// ErrVoteLimit é o sentinel; a mensagem exibida vem de VoteLimitError,
	// que envolve este erro com o número correto da categoria (2 ou 1).
	ErrVoteLimit = errors.New("você já usou seu(s) 2 voto(s) nesta categoria")
)

// VoteLimitError envolve o sentinel com o número correto da categoria
// (2 votos; 1 no melhor gol). O texto do sentinel cobre o caso comum (2).
func VoteLimitError(c Category) error {
	n := MaxVotesFor(c)
	if n == MaxVotesPerCategory {
		return ErrVoteLimit
	}
	return fmt.Errorf("%w Sendo %s, vale %d voto.", ErrVoteLimit, c.Label(), n)
}

func ParseCategory(raw string) (Category, error) {
	if c := Category(raw); slices.Contains(Categories, c) {
		return c, nil
	}
	return "", ErrInvalidCategory
}

// Label é o nome da categoria como o grupo fala.
func (c Category) Label() string {
	switch c {
	case WorstPlayer:
		return "perna de pau"
	case BestGoal:
		return "melhor gol"
	}
	return "artilheiro"
}

// CheckVotingOpen vale para votar e para desfazer um voto.
func CheckVotingOpen(matchStatus string) error {
	if matchStatus != StatusVoting {
		return ErrVotingClosed
	}
	return nil
}

// ResultsVisible esconde a apuração enquanto a votação está aberta, para não
// influenciar quem ainda vai votar.
func ResultsVisible(matchStatus string) bool {
	return matchStatus == StatusFinished
}

// Ballot reúne o que é preciso saber para decidir se um voto vale.
// Participar da partida é ter confirmado presença (going).
type Ballot struct {
	MatchStatus     string
	VoterID         string
	CandidateID     string
	Category        Category
	VoterPlayed     bool
	CandidatePlayed bool
	PreviousVotes   []string // candidatos que o eleitor já votou nesta categoria
}

// Validate aplica as regras na ordem em que a mensagem é mais útil a quem
// vota: votação aberta, participação, voto em si mesmo, repetição e limite.
func (b Ballot) Validate() error {
	if err := CheckVotingOpen(b.MatchStatus); err != nil {
		return err
	}
	if !b.VoterPlayed || !b.CandidatePlayed {
		return ErrNotParticipant
	}
	if b.Category == WorstPlayer && b.CandidateID == b.VoterID {
		return ErrSelfWorstVote
	}
	if slices.Contains(b.PreviousVotes, b.CandidateID) {
		return ErrDuplicateVote
	}
	if len(b.PreviousVotes) >= MaxVotesFor(b.Category) {
		return VoteLimitError(b.Category)
	}
	return nil
}

// Submission é o voto completo de um participante, como a PWA envia em
// "Enviar votos": para cada categoria, os candidatos que devem valer. Uma
// lista vazia desfaz os votos da categoria; uma categoria ausente fica como
// está.
type Submission struct {
	MatchStatus  string
	VoterID      string
	Votes        map[Category][]string
	Participants []string // quem confirmou presença (going)
}

// Validate aplica a cada candidato as regras de Ballot, como se os votos da
// categoria fossem dados um a um na ordem enviada.
func (s Submission) Validate() error {
	if err := CheckVotingOpen(s.MatchStatus); err != nil {
		return err
	}
	if !slices.Contains(s.Participants, s.VoterID) {
		return ErrNotParticipant
	}
	for _, c := range Categories {
		candidates := s.Votes[c]
		for i, candidate := range candidates {
			b := Ballot{
				MatchStatus:     s.MatchStatus,
				VoterID:         s.VoterID,
				CandidateID:     candidate,
				Category:        c,
				VoterPlayed:     true,
				CandidatePlayed: slices.Contains(s.Participants, candidate),
				PreviousVotes:   candidates[:i],
			}
			if err := b.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// WinnerAnnouncement é o texto do feed de atividade para cada vencedor.
func WinnerAnnouncement(c Category, winnerName string) string {
	return fmt.Sprintf("%s foi eleito %s da partida", winnerName, c.Label())
}
