package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"futdarapaziada/api/internal/guests"
)

const voteTestAppURL = "https://fut.example.invalid"

// mustConfirmedGuest inscreve e confirma o pagamento de um avulso.
func mustConfirmedGuest(t *testing.T, s *Store, matchID, phone string) GuestBooking {
	t.Helper()
	b := mustPending(t, s, mustBook(t, s, matchID, phone))
	b, _, err := s.ConfirmGuestPayment(context.Background(), b.ID, b.ProviderOrderID, "pay-"+phone, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// guestVotingMatch monta uma partida com um avulso confirmado, um com Pix
// pendente e um expirado, e devolve a partida, o admin e as inscrições.
func guestVotingMatch(t *testing.T, s *Store) (Match, string, GuestBooking, GuestBooking, GuestBooking) {
	t.Helper()
	ctx := context.Background()
	m, admin := guestTestMatch(t, s, 5)
	confirmed := mustConfirmedGuest(t, s, m.ID, "11987650001")
	pending := mustPending(t, s, mustBook(t, s, m.ID, "11987650002"))
	expired := mustBook(t, s, m.ID, "11987650003")
	if _, err := s.pool.Exec(ctx, `update guest_bookings set status = 'expired' where id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update matches set status = 'closed' where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	return m, admin, confirmed, pending, expired
}

type queuedVoteLink struct {
	message string
	phone   string
	resend  bool
}

// voteLinksOf lista os links de voto agendados para a inscrição, na ordem.
func voteLinksOf(t *testing.T, s *Store, bookingID string) []queuedVoteLink {
	t.Helper()
	rows, err := s.pool.Query(context.Background(), `
		select n.message, n.phone, gm.resend
		from guest_messages gm join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'vote_link'
		order by gm.created_at, n.created_at`, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []queuedVoteLink
	for rows.Next() {
		var q queuedVoteLink
		if err := rows.Scan(&q.message, &q.phone, &q.resend); err != nil {
			t.Fatal(err)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// tokenFromMessage tira o token do link que vai no WhatsApp.
func tokenFromMessage(t *testing.T, message string) string {
	t.Helper()
	_, token, ok := strings.Cut(message, voteTestAppURL+"/votar/")
	if !ok || token == "" {
		t.Fatalf("mensagem sem link de voto: %q", message)
	}
	return token
}

// Ao abrir a votação pelo admin, cada avulso confirmado recebe um link, uma
// vez só; pendentes e expirados não recebem.
func TestScheduleGuestVoteLinks(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, admin, confirmed, pending, expired := guestVotingMatch(t, s)

	// Antes da votação, nada é agendado.
	if n, err := s.ScheduleGuestVoteLinks(ctx, m.ID, voteTestAppURL, time.Now()); err != nil || n != 0 {
		t.Fatalf("antes da votação: %d, %v", n, err)
	}

	closes := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	if err := s.FinishMatch(ctx, m.ID, admin, closes); err != nil {
		t.Fatal(err)
	}
	for attempt := range 3 {
		n, err := s.ScheduleGuestVoteLinks(ctx, m.ID, voteTestAppURL, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if want := map[bool]int{true: 1, false: 0}[attempt == 0]; n != want {
			t.Errorf("tentativa %d: %d links, want %d", attempt, n, want)
		}
	}
	if n, err := s.ScheduleMissingGuestVoteLinks(ctx, voteTestAppURL, time.Now()); err != nil || n != 0 {
		t.Errorf("varredura do job depois do admin: %d, %v", n, err)
	}

	links := voteLinksOf(t, s, confirmed.ID)
	if len(links) != 1 || links[0].resend || links[0].phone != "5511987650001" {
		t.Fatalf("links do confirmado = %+v", links)
	}
	token := tokenFromMessage(t, links[0].message)
	if want := guests.VoteLinkMessage("2099-01-10", closes, guests.VoteLinkURL(voteTestAppURL, token)); links[0].message != want {
		t.Errorf("mensagem = %q, want %q", links[0].message, want)
	}
	voter, err := s.GuestVoterByToken(ctx, guests.HashVoteToken(token))
	if err != nil {
		t.Fatal(err)
	}
	if voter.BookingID != confirmed.ID || voter.MatchID != m.ID || voter.UserID != confirmed.UserID || !voter.ExpiresAt.Equal(closes) {
		t.Errorf("eleitor = %+v", voter)
	}
	for _, b := range []GuestBooking{pending, expired} {
		if got := voteLinksOf(t, s, b.ID); len(got) != 0 {
			t.Errorf("inscrição %s recebeu link: %+v", b.Status, got)
		}
	}
	var stored int
	if err := s.pool.QueryRow(ctx, `select count(*) from guest_vote_tokens where token_hash = $1 or token_hash = $2`,
		token, guests.HashVoteToken(token)).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 1 {
		t.Errorf("%d tokens guardados", stored)
	}
	var plain int
	if err := s.pool.QueryRow(ctx, `select count(*) from guest_vote_tokens where token_hash = $1`, token).Scan(&plain); err != nil || plain != 0 {
		t.Errorf("token guardado em claro: %d, %v", plain, err)
	}
}

// A abertura automática (job) usa a varredura: agenda o que falta em toda
// partida em votação, e não repete.
func TestScheduleMissingGuestVoteLinksAfterAutoOpen(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, _, confirmed, _, _ := guestVotingMatch(t, s)
	if _, err := s.pool.Exec(ctx, `update matches set match_date = current_date - 1 where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	opened, err := s.OpenExpiredVoting(ctx, 0, 2)
	if err != nil || len(opened) != 1 {
		t.Fatalf("abrir votação: %v, %v", opened, err)
	}
	for attempt := range 2 {
		n, err := s.ScheduleMissingGuestVoteLinks(ctx, voteTestAppURL, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if want := map[bool]int{true: 1, false: 0}[attempt == 0]; n != want {
			t.Errorf("varredura %d: %d links, want %d", attempt, n, want)
		}
	}
	if links := voteLinksOf(t, s, confirmed.ID); len(links) != 1 {
		t.Errorf("links = %+v", links)
	}

	// Votação com prazo vencido (o job de fechamento ainda não passou) não
	// recebe links novos.
	late := mustConfirmedGuestInVoting(t, s, time.Now().Add(-time.Minute))
	if n, err := s.ScheduleMissingGuestVoteLinks(ctx, voteTestAppURL, time.Now()); err != nil || n != 0 {
		t.Errorf("votação vencida: %d, %v", n, err)
	}
	if links := voteLinksOf(t, s, late.ID); len(links) != 0 {
		t.Errorf("link para votação vencida: %+v", links)
	}
}

// mustConfirmedGuestInVoting cria outra partida, já em votação com o prazo
// dado, com um avulso confirmado.
func mustConfirmedGuestInVoting(t *testing.T, s *Store, closes time.Time) GuestBooking {
	t.Helper()
	ctx := context.Background()
	m, admin := guestTestMatch(t, s, 2)
	b := mustConfirmedGuest(t, s, m.ID, "11987659999")
	if _, err := s.pool.Exec(ctx, `update matches set status = 'closed' where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishMatch(ctx, m.ID, admin, closes); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGuestVoterByToken(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, admin, confirmed, _, _ := guestVotingMatch(t, s)
	if err := s.FinishMatch(ctx, m.ID, admin, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScheduleGuestVoteLinks(ctx, m.ID, voteTestAppURL, time.Now()); err != nil {
		t.Fatal(err)
	}
	token := tokenFromMessage(t, voteLinksOf(t, s, confirmed.ID)[0].message)
	hash := guests.HashVoteToken(token)

	if _, err := s.GuestVoterByToken(ctx, hash); err != nil {
		t.Fatalf("token válido: %v", err)
	}
	if _, err := s.GuestVoterByToken(ctx, guests.HashVoteToken("outro")); !errors.Is(err, ErrNotFound) {
		t.Errorf("token desconhecido: %v", err)
	}
	if _, err := s.GuestVoterByToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("token em claro no lugar do hash: %v", err)
	}
	// Inscrição que deixou de estar confirmada perde o link.
	if _, err := s.pool.Exec(ctx, `update guest_bookings set status = 'refund_pending' where id = $1`, confirmed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GuestVoterByToken(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Errorf("inscrição a reembolsar: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `update guest_bookings set status = 'confirmed' where id = $1`, confirmed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update guest_vote_tokens set revoked_at = now() where booking_id = $1`, confirmed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GuestVoterByToken(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Errorf("token revogado: %v", err)
	}
}

// Reenvio do admin: grava com resend = true; o link novo gira o token.
func TestResendGuestMessage(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, admin, confirmed, pending, _ := guestVotingMatch(t, s)

	// Confirmação antes da votação: vale; link de voto: ainda não.
	if err := s.ResendGuestMessage(ctx, confirmed.ID, guests.MessageConfirmation, voteTestAppURL, time.Now()); err != nil {
		t.Fatal(err)
	}
	var resent int
	var text string
	if err := s.pool.QueryRow(ctx, `select count(*) over (), n.message
		from guest_messages gm join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'confirmation' and gm.resend`, confirmed.ID).Scan(&resent, &text); err != nil {
		t.Fatal(err)
	}
	if resent != 1 || text != guests.ConfirmationMessage("2099-01-10", "20:00", "Arena Teste") {
		t.Errorf("confirmação reenviada: %d, %q", resent, text)
	}
	if err := s.ResendGuestMessage(ctx, confirmed.ID, guests.MessageVoteLink, voteTestAppURL, time.Now()); !errors.Is(err, guests.ErrVoteLinkVotingClosed) {
		t.Errorf("link antes da votação: %v", err)
	}
	if err := s.ResendGuestMessage(ctx, pending.ID, guests.MessageConfirmation, voteTestAppURL, time.Now()); !errors.Is(err, guests.ErrResendNotConfirmed) {
		t.Errorf("confirmação de pendente: %v", err)
	}
	if err := s.ResendGuestMessage(ctx, confirmed.ID, "pix", voteTestAppURL, time.Now()); !errors.Is(err, guests.ErrInvalidMessageKind) {
		t.Errorf("tipo inválido: %v", err)
	}
	if err := s.ResendGuestMessage(ctx, "00000000-0000-0000-0000-000000000000", guests.MessageConfirmation, voteTestAppURL, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("inscrição inexistente: %v", err)
	}

	closes := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.FinishMatch(ctx, m.ID, admin, closes); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ScheduleGuestVoteLinks(ctx, m.ID, voteTestAppURL, time.Now()); err != nil {
		t.Fatal(err)
	}
	oldToken := tokenFromMessage(t, voteLinksOf(t, s, confirmed.ID)[0].message)

	if err := s.ResendGuestMessage(ctx, confirmed.ID, guests.MessageVoteLink, voteTestAppURL, time.Now()); err != nil {
		t.Fatal(err)
	}
	links := voteLinksOf(t, s, confirmed.ID)
	if len(links) != 2 || links[0].resend || !links[1].resend {
		t.Fatalf("links = %+v", links)
	}
	newToken := tokenFromMessage(t, links[1].message)
	if newToken == oldToken {
		t.Fatal("o reenvio não girou o token")
	}
	if _, err := s.GuestVoterByToken(ctx, guests.HashVoteToken(oldToken)); !errors.Is(err, ErrNotFound) {
		t.Errorf("token antigo ainda vale: %v", err)
	}
	voter, err := s.GuestVoterByToken(ctx, guests.HashVoteToken(newToken))
	if err != nil || !voter.ExpiresAt.Equal(closes) {
		t.Errorf("token novo: %+v, %v", voter, err)
	}
	// O reenvio não faz o automático sair de novo.
	if n, err := s.ScheduleMissingGuestVoteLinks(ctx, voteTestAppURL, time.Now()); err != nil || n != 0 {
		t.Errorf("varredura depois do reenvio: %d, %v", n, err)
	}
}

// Se o admin reenviou o link antes do envio automático (que falhou ao
// abrir a votação), a varredura não gira o token de novo.
func TestScheduleSkipsBookingWithResentVoteLink(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, admin, confirmed, _, _ := guestVotingMatch(t, s)
	if err := s.FinishMatch(ctx, m.ID, admin, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.ResendGuestMessage(ctx, confirmed.ID, guests.MessageVoteLink, voteTestAppURL, time.Now()); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ScheduleMissingGuestVoteLinks(ctx, voteTestAppURL, time.Now()); err != nil || n != 0 {
		t.Errorf("varredura: %d, %v", n, err)
	}
	links := voteLinksOf(t, s, confirmed.ID)
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	if _, err := s.GuestVoterByToken(ctx, guests.HashVoteToken(tokenFromMessage(t, links[0].message))); err != nil {
		t.Errorf("link reenviado deixou de valer: %v", err)
	}
}

// Selo Avulso: confirmações e times expõem o tipo do usuário.
func TestConfirmationsAndTeamsExposeKind(t *testing.T) {
	s := newTestStore(t, "guest_votes_test")
	ctx := context.Background()
	m, admin := guestTestMatch(t, s, 2)
	guest := mustConfirmedGuest(t, s, m.ID, "11987650001")
	if err := s.Confirm(ctx, m.ID, admin, "going"); err != nil {
		t.Fatal(err)
	}

	entries, err := s.Confirmations(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, e := range entries {
		kinds[e.UserID] = e.Kind
	}
	if kinds[admin] != KindMember || kinds[guest.UserID] != KindGuest {
		t.Errorf("confirmações: %+v", entries)
	}

	if _, err := s.pool.Exec(ctx, `update matches set status = 'closed' where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceTeams(ctx, m.ID, []Team{{TeamName: "A", TeamColor: "#000", Position: 0,
		Members: []TeamMember{{UserID: admin}, {UserID: guest.UserID}}}}); err != nil {
		t.Fatal(err)
	}
	teams, err := s.Teams(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) != 1 || len(teams[0].Members) != 2 {
		t.Fatalf("times = %+v", teams)
	}
	for _, member := range teams[0].Members {
		want := map[string]string{admin: KindMember, guest.UserID: KindGuest}[member.UserID]
		if member.Kind != want {
			t.Errorf("membro %s: kind %q, want %q", member.UserID, member.Kind, want)
		}
	}
}
