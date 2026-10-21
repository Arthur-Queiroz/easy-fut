package guests

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestVoteLinkURL(t *testing.T) {
	for _, appURL := range []string{"https://fut.example.invalid", "https://fut.example.invalid/"} {
		if got := VoteLinkURL(appURL, "abc_-123"); got != "https://fut.example.invalid/votar/abc_-123" {
			t.Errorf("VoteLinkURL(%q) = %q", appURL, got)
		}
	}
}

// O prazo aparece no horário de São Paulo, qualquer que seja o fuso do
// processo ou do banco.
func TestVoteLinkMessage(t *testing.T) {
	closes := time.Date(2099, 1, 13, 1, 0, 0, 0, time.UTC) // 12/01 22:00 em SP
	got := VoteLinkMessage("2099-01-10", closes, "https://fut.example.invalid/votar/tok")
	want := "A votação da partida de 10/01/2099 está aberta até 12/01/2099 às 22:00. " +
		"Vote no app: https://fut.example.invalid/votar/tok"
	if got != want {
		t.Errorf("VoteLinkMessage() = %q, want %q", got, want)
	}
	// Data fora do formato do banco aparece como veio.
	if got := VoteLinkMessage("amanhã", closes, "x"); got != "A votação da partida de amanhã está aberta até 12/01/2099 às 22:00. Vote no app: x" {
		t.Errorf("data inválida: %q", got)
	}
}

// O token tem pelo menos 32 bytes aleatórios em base64url; só o hash vai ao
// banco, e o hash não revela o token.
func TestNewVoteToken(t *testing.T) {
	token, hash, err := NewVoteToken()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < 32 {
		t.Fatalf("token %q: %d bytes, %v", token, len(raw), err)
	}
	if hash != HashVoteToken(token) || hash == token || len(hash) != 64 {
		t.Errorf("hash = %q", hash)
	}
	other, _, err := NewVoteToken()
	if err != nil || other == token {
		t.Errorf("dois tokens iguais: %q", other)
	}
}

func TestVoteLinkOpen(t *testing.T) {
	now := time.Date(2099, 1, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		matchStatus string
		expiresAt   time.Time
		want        bool
	}{
		{"votação aberta e link válido", "voting", now.Add(time.Hour), true},
		{"votação encerrada", "finished", now.Add(time.Hour), false},
		{"partida cancelada", "cancelled", now.Add(time.Hour), false},
		{"partida ainda sem votação", "teams_drawn", now.Add(time.Hour), false},
		{"prazo do link passou antes do job fechar", "voting", now, false},
	}
	for _, c := range cases {
		if got := VoteLinkOpen(c.matchStatus, c.expiresAt, now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Reenvio pelo admin: confirmação só de inscrição confirmada; link de voto
// também exige a votação aberta.
func TestResendValidate(t *testing.T) {
	now := time.Date(2099, 1, 11, 12, 0, 0, 0, time.UTC)
	closes := now.Add(24 * time.Hour)
	past := now.Add(-time.Minute)
	valid := Resend{Kind: MessageVoteLink, BookingStatus: StatusConfirmed, MatchStatus: "voting", VotingClosesAt: &closes, Now: now}

	cases := []struct {
		name   string
		change func(*Resend)
		want   error
	}{
		{"link com votação aberta", func(*Resend) {}, nil},
		{"confirmação de inscrição confirmada", func(r *Resend) { r.Kind = MessageConfirmation; r.MatchStatus = "open"; r.VotingClosesAt = nil }, nil},
		{"confirmação durante a votação", func(r *Resend) { r.Kind = MessageConfirmation }, nil},
		{"tipo desconhecido", func(r *Resend) { r.Kind = "pix" }, ErrInvalidMessageKind},
		{"confirmação de reserva", func(r *Resend) { r.Kind = MessageConfirmation; r.BookingStatus = StatusPendingPayment }, ErrResendNotConfirmed},
		{"confirmação de inscrição a reembolsar", func(r *Resend) { r.Kind = MessageConfirmation; r.BookingStatus = StatusRefundPending }, ErrResendNotConfirmed},
		{"link de inscrição expirada", func(r *Resend) { r.BookingStatus = StatusExpired }, ErrResendNotConfirmed},
		{"link antes da votação", func(r *Resend) { r.MatchStatus = "teams_drawn"; r.VotingClosesAt = nil }, ErrVoteLinkVotingClosed},
		{"link depois da votação", func(r *Resend) { r.MatchStatus = "finished" }, ErrVoteLinkVotingClosed},
		{"link com prazo vencido", func(r *Resend) { r.VotingClosesAt = &past }, ErrVoteLinkVotingClosed},
	}
	for _, c := range cases {
		r := valid
		c.change(&r)
		if err := r.Validate(); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}
