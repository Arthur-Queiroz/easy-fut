package guests

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Link privado de voto do avulso (fase C): enviado por WhatsApp quando a
// votação abre, reenviável pelo admin. Só o hash do token vai ao banco.

// Tipos de mensagem de WhatsApp de uma inscrição (guest_messages.kind).
const (
	MessageConfirmation = "confirmation"
	MessageVoteLink     = "vote_link"
)

const matchVoting = "voting"

var (
	ErrInvalidMessageKind   = errors.New("tipo de mensagem inválido")
	ErrResendNotConfirmed   = errors.New("só dá para reenviar mensagens de uma inscrição confirmada")
	ErrVoteLinkVotingClosed = errors.New("o link de voto só pode ser reenviado com a votação da partida aberta")
)

// voteTokenBytes é o tamanho do token: 256 bits, impossível de adivinhar.
const voteTokenBytes = 32

// NewVoteToken gera um token aleatório em base64url e o hash que fica no
// banco.
func NewVoteToken() (token, hash string, err error) {
	buf := make([]byte, voteTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashVoteToken(token), nil
}

// HashVoteToken é o SHA-256 (hex) do token recebido na URL.
func HashVoteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VoteLinkURL monta o link da página de voto da PWA (/votar/:token).
func VoteLinkURL(appURL, token string) string {
	return strings.TrimRight(appURL, "/") + "/votar/" + token
}

// saoPaulo é o fuso em que o grupo lê os prazos. Sem a base de fusos no
// sistema, cai no -03:00 fixo (o Brasil não tem horário de verão desde 2019).
var saoPaulo = func() *time.Location {
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		return loc
	}
	return time.FixedZone("BRT", -3*60*60)
}()

// VoteLinkMessage é o WhatsApp com o link de voto. matchDate vem do banco
// como YYYY-MM-DD; o prazo é o fim da votação, mostrado no horário de SP.
func VoteLinkMessage(matchDate string, closesAt time.Time, link string) string {
	date := matchDate
	if d, err := time.Parse("2006-01-02", matchDate); err == nil {
		date = d.Format("02/01/2006")
	}
	deadline := closesAt.In(saoPaulo).Format("02/01/2006 às 15:04")
	return fmt.Sprintf("A votação da partida de %s está aberta até %s. Vote no app: %s", date, deadline, link)
}

// VoteLinkOpen diz se o link aceita leitura e voto: a partida está em
// votação e o prazo do link (o fim da votação) não passou. O job fecha a
// votação a cada 10 minutos; o prazo do link vale mesmo antes disso.
func VoteLinkOpen(matchStatus string, expiresAt, now time.Time) bool {
	return matchStatus == matchVoting && now.Before(expiresAt)
}

// Resend é o pedido do admin para reenviar uma mensagem da inscrição.
type Resend struct {
	Kind           string
	BookingStatus  string
	MatchStatus    string
	VotingClosesAt *time.Time
	Now            time.Time
}

// Validate: só inscrição confirmada recebe mensagens; o link de voto exige
// a votação aberta, porque o token novo vale só até o fim dela.
func (r Resend) Validate() error {
	if r.Kind != MessageConfirmation && r.Kind != MessageVoteLink {
		return ErrInvalidMessageKind
	}
	if r.BookingStatus != StatusConfirmed {
		return ErrResendNotConfirmed
	}
	if r.Kind == MessageVoteLink &&
		(r.VotingClosesAt == nil || !VoteLinkOpen(r.MatchStatus, *r.VotingClosesAt, r.Now)) {
		return ErrVoteLinkVotingClosed
	}
	return nil
}
