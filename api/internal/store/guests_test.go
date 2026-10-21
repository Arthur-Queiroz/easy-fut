package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
)

// guestTestMatch cria uma partida aberta, com prazo no futuro e venda de
// `slots` vagas a R$ 25,00 (total R$ 25,25 com a taxa do Pix).
func guestTestMatch(t *testing.T, s *Store, slots int) (Match, string) {
	t.Helper()
	admin := votingTestUser(t, s, "active")
	m, err := s.CreateMatch(context.Background(), MatchInput{
		MatchDate: "2099-01-10", StartTime: "20:00", EndTime: "22:00",
		Venue: "Arena Teste", Address: "Rua Teste, 1",
		ConfirmationDeadline: time.Now().Add(2 * time.Hour),
	}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetGuestSale(context.Background(), m.ID, true, slots, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	return m, admin
}

func guestInput(matchID, phone string) GuestBookingInput {
	return GuestBookingInput{
		MatchID: matchID, Name: "Avulso Teste", Phone: phone,
		Email: "avulso@example.invalid", AccessTokenHash: "hash-" + phone,
	}
}

func mustBook(t *testing.T, s *Store, matchID, phone string) GuestBooking {
	t.Helper()
	b, err := s.CreateGuestBooking(context.Background(), guestInput(matchID, phone), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustPending(t *testing.T, s *Store, b GuestBooking) GuestBooking {
	t.Helper()
	b, err := s.MarkGuestBookingPendingPayment(context.Background(), b.ID, GuestPix{
		OrderID: "order-" + b.ID, Status: "action_required", Payload: "pix-copia-e-cola",
		TicketURL: "https://example.invalid/ticket", QRCodeBase64: "qr",
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Abrir a venda congela o preço vigente e o total com a taxa embutida.
func TestSetGuestSaleFreezesPrice(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 4)

	var open bool
	var slots int
	var price, total int64
	read := func() {
		t.Helper()
		if err := s.pool.QueryRow(ctx, `select guest_sale_open, guest_slots, guest_price_cents, guest_total_cents
			from matches where id = $1`, m.ID).Scan(&open, &slots, &price, &total); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if !open || slots != 4 || price != 2500 || total != 2525 {
		t.Fatalf("venda = %v, %d vagas, %d/%d; want aberta, 4, 2500/2525", open, slots, price, total)
	}
	// Fechar não apaga o preço congelado.
	if err := s.SetGuestSale(ctx, m.ID, false, 0, 9999, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	read()
	if open || slots != 4 || price != 2500 || total != 2525 {
		t.Fatalf("depois de fechar: %v, %d, %d/%d", open, slots, price, total)
	}
	if err := s.SetGuestSale(ctx, "00000000-0000-0000-0000-000000000000", true, 1, 2500, billing.DefaultPixRate); !errors.Is(err, ErrNotFound) {
		t.Errorf("partida inexistente: %v", err)
	}
}

// A inscrição cria um avulso novo (sem e-mail, sem senha, nunca admin) e
// reserva a vaga por 30 minutos com o preço congelado na partida.
func TestCreateGuestBooking(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 2)
	before := time.Now()

	b, err := s.CreateGuestBooking(ctx, GuestBookingInput{
		MatchID: m.ID, Name: "  Zé   Avulso ", Phone: "(11) 98765-4321",
		Email: " ZE@Example.com ", AccessTokenHash: "token-hash",
	}, before)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != guests.StatusReserved || b.Name != "Zé Avulso" || b.Phone != "+5511987654321" || b.Email != "ze@example.com" {
		t.Fatalf("inscrição = %+v", b)
	}
	if b.BaseCents != 2500 || b.TotalCents != 2525 || b.FeeCents != 25 {
		t.Errorf("valores = %d/%d/%d, want 2500/2525/25", b.BaseCents, b.TotalCents, b.FeeCents)
	}
	if d := b.ExpiresAt.Sub(before); d < 29*time.Minute || d > 31*time.Minute {
		t.Errorf("reserva vence em %v, want 30 min", d)
	}
	if b.AccessTokenHash != "token-hash" {
		t.Errorf("hash do token = %q", b.AccessTokenHash)
	}

	u, err := s.UserByID(ctx, b.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Kind != KindGuest || u.Role != "player" || u.Email != "" || u.Phone != "+5511987654321" {
		t.Errorf("avulso = %+v", u)
	}
	var email, hash *string
	if err := s.pool.QueryRow(ctx, `select email, password_hash from users where id = $1`, b.UserID).Scan(&email, &hash); err != nil {
		t.Fatal(err)
	}
	if email != nil || hash != nil {
		t.Errorf("avulso com credenciais: e-mail %v, senha %v", email, hash)
	}

	// Cada compra é independente: outro número na mesma partida cria outro avulso.
	other := mustBook(t, s, m.ID, "21998765432")
	if other.UserID == b.UserID {
		t.Error("duas compras dividindo o mesmo avulso")
	}
	got, err := s.GuestBookingByID(ctx, b.ID)
	if err != nil || got.ID != b.ID || got.MatchID != m.ID {
		t.Fatalf("GuestBookingByID = %+v, %v", got, err)
	}
	if _, err := s.GuestBookingByID(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("inscrição inexistente: %v", err)
	}
}

func TestCreateGuestBookingRules(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 1)

	if _, err := s.CreateGuestBooking(ctx, guestInput(m.ID, "1133334444"), time.Now()); !errors.Is(err, guests.ErrInvalidPhone) {
		t.Errorf("telefone fixo: %v", err)
	}
	in := guestInput(m.ID, "11987654321")
	in.Email = "sem-arroba"
	if _, err := s.CreateGuestBooking(ctx, in, time.Now()); !errors.Is(err, guests.ErrInvalidEmail) {
		t.Errorf("e-mail inválido: %v", err)
	}
	if _, err := s.CreateGuestBooking(ctx, guestInput("00000000-0000-0000-0000-000000000000", "11987654321"), time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("partida inexistente: %v", err)
	}

	first := mustBook(t, s, m.ID, "11987654321")
	// Mesmo WhatsApp em outra formatação ainda é o mesmo número.
	if _, err := s.CreateGuestBooking(ctx, guestInput(m.ID, "+55 (11) 98765-4321"), time.Now()); !errors.Is(err, guests.ErrPhoneAlreadyBooked) {
		t.Errorf("mesmo telefone: %v", err)
	}
	if _, err := s.CreateGuestBooking(ctx, guestInput(m.ID, "21998765432"), time.Now()); !errors.Is(err, guests.ErrSoldOut) {
		t.Errorf("sem vaga: %v", err)
	}

	// Reserva vencida libera a vaga e o número, mesmo antes do cron.
	if _, err := s.pool.Exec(ctx, `update guest_bookings set expires_at = now() - interval '1 minute' where id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	again := mustBook(t, s, m.ID, "11987654321")
	if old, _ := s.GuestBookingByID(ctx, first.ID); old.Status != guests.StatusExpired {
		t.Errorf("reserva vencida ficou %s", old.Status)
	}
	if again.ID == first.ID {
		t.Error("nova tentativa reaproveitou a inscrição expirada")
	}

	// Venda fechada pelo admin ou partida fora de open.
	if err := s.SetGuestSale(ctx, m.ID, false, 5, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGuestBooking(ctx, guestInput(m.ID, "31998765432"), time.Now()); !errors.Is(err, guests.ErrSaleClosed) {
		t.Errorf("venda fechada: %v", err)
	}
	if err := s.SetGuestSale(ctx, m.ID, true, 5, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMatchStatus(ctx, m.ID, "closed", "open"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGuestBooking(ctx, guestInput(m.ID, "31998765432"), time.Now()); !errors.Is(err, guests.ErrSaleClosed) {
		t.Errorf("partida fechada: %v", err)
	}
}

// Compras simultâneas não levam mais vagas do que existem: a partida fica
// travada durante a contagem.
func TestCreateGuestBookingConcurrentSlots(t *testing.T) {
	s := newTestStore(t, "guests_test")
	m, _ := guestTestMatch(t, s, 3)

	const buyers = 10
	var wg sync.WaitGroup
	errs := make([]error, buyers)
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.CreateGuestBooking(context.Background(),
				guestInput(m.ID, fmt.Sprintf("119%08d", i)), time.Now())
		}()
	}
	wg.Wait()

	ok, soldOut := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, guests.ErrSoldOut):
			soldOut++
		default:
			t.Errorf("erro inesperado: %v", err)
		}
	}
	if ok != 3 || soldOut != buyers-3 {
		t.Fatalf("%d vendidas e %d esgotadas, want 3 e %d", ok, soldOut, buyers-3)
	}
}

func TestMarkGuestBookingPendingPayment(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 3)
	b := mustBook(t, s, m.ID, "11987654321")

	pending := mustPending(t, s, b)
	if pending.Status != guests.StatusPendingPayment || pending.ProviderOrderID != "order-"+b.ID ||
		pending.PixPayload != "pix-copia-e-cola" || pending.PixTicketURL == "" || pending.PixQRCodeBase64 != "qr" {
		t.Fatalf("pendente = %+v", pending)
	}
	// A mesma order pode ser gravada de novo (idempotência do Mercado Pago).
	mustPending(t, s, b)
	// Outra order para a mesma inscrição, não.
	if _, err := s.MarkGuestBookingPendingPayment(ctx, b.ID, GuestPix{OrderID: "outra"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("order diferente: %v", err)
	}
	// Reserva que já expirou não recebe Pix.
	expired := mustBook(t, s, m.ID, "21998765432")
	if _, err := s.pool.Exec(ctx, `update guest_bookings set status = 'expired' where id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkGuestBookingPendingPayment(ctx, expired.ID, GuestPix{OrderID: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("reserva expirada: %v", err)
	}
}

// Pagamento aprovado dentro do prazo: presença, caixa e uma única mensagem
// de confirmação, mesmo com o webhook repetido.
func TestConfirmGuestPayment(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 3)
	b := mustPending(t, s, mustBook(t, s, m.ID, "11987654321"))

	if _, _, err := s.ConfirmGuestPayment(ctx, b.ID, "order-errada", "pay-1", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("order de outra inscrição: %v", err)
	}

	for attempt := range 3 {
		got, changed, err := s.ConfirmGuestPayment(ctx, b.ID, b.ProviderOrderID, "pay-1", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if changed != (attempt == 0) || got.Status != guests.StatusConfirmed || got.ConfirmedAt == nil || got.ProviderPaymentID != "pay-1" {
			t.Fatalf("tentativa %d: changed %v, %+v", attempt, changed, got)
		}
	}

	going, err := s.IsParticipant(ctx, m.ID, b.UserID)
	if err != nil || !going {
		t.Fatalf("avulso pago não está na presença: %v %v", going, err)
	}
	var kind string
	var amount, fee int64
	var bookingID *string
	var entries int
	if err := s.pool.QueryRow(ctx, `select count(*) over (), kind, amount_cents, fee_cents, booking_id
		from cash_entries`).Scan(&entries, &kind, &amount, &fee, &bookingID); err != nil {
		t.Fatal(err)
	}
	if entries != 1 || kind != guests.CashGuestPayment || amount != 2500 || fee != 25 || bookingID == nil || *bookingID != b.ID {
		t.Errorf("caixa = %d lançamentos, %s %d (taxa %d)", entries, kind, amount, fee)
	}

	var messages int
	var phone, text string
	if err := s.pool.QueryRow(ctx, `select count(*) over (), n.phone, n.message
		from guest_messages gm join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'confirmation' and not gm.resend`, b.ID).Scan(&messages, &phone, &text); err != nil {
		t.Fatal(err)
	}
	if messages != 1 || phone != "5511987654321" || text != guests.ConfirmationMessage("2099-01-10", "20:00", "Arena Teste") {
		t.Errorf("mensagem = %d, %q, %q", messages, phone, text)
	}
	var queued int
	if err := s.pool.QueryRow(ctx, `select count(*) from notifications where user_id = $1`, b.UserID).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Errorf("%d WhatsApps na fila, want 1", queued)
	}
}

// Pagamento depois da reserva vencida: sempre reembolsa e nunca vira vaga.
// O dinheiro entrou, então o caixa registra a entrada (o estorno sai depois).
func TestConfirmGuestPaymentAfterExpiry(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 3)
	late := mustPending(t, s, mustBook(t, s, m.ID, "11987654321"))
	expired := mustPending(t, s, mustBook(t, s, m.ID, "21998765432"))
	if _, err := s.pool.Exec(ctx, `update guest_bookings set expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExpireGuestBookings(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}

	for _, b := range []GuestBooking{late, expired} {
		got, changed, err := s.ConfirmGuestPayment(ctx, b.ID, b.ProviderOrderID, "pay", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if !changed || got.Status != guests.StatusRefundPending || got.ConfirmedAt != nil || got.RefundReason == "" {
			t.Errorf("pagamento tardio: changed %v, %+v", changed, got)
		}
		if going, _ := s.IsParticipant(ctx, m.ID, b.UserID); going {
			t.Error("pagamento tardio entrou na presença")
		}
	}
	var payments, messages int
	if err := s.pool.QueryRow(ctx, `select (select count(*) from cash_entries where kind = 'guest_payment'),
		(select count(*) from guest_messages)`).Scan(&payments, &messages); err != nil {
		t.Fatal(err)
	}
	if payments != 2 || messages != 0 {
		t.Errorf("caixa %d entradas (want 2), %d mensagens (want 0)", payments, messages)
	}
}

func TestExpireGuestBookings(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 5)
	reserved := mustBook(t, s, m.ID, "11987654321")
	pending := mustPending(t, s, mustBook(t, s, m.ID, "21998765432"))
	confirmed := mustPending(t, s, mustBook(t, s, m.ID, "31998765432"))
	if _, _, err := s.ConfirmGuestPayment(ctx, confirmed.ID, confirmed.ProviderOrderID, "pay", time.Now()); err != nil {
		t.Fatal(err)
	}
	fresh := mustBook(t, s, m.ID, "41998765432")

	if _, err := s.pool.Exec(ctx, `update guest_bookings set expires_at = now() - interval '1 minute' where id <> $1`, fresh.ID); err != nil {
		t.Fatal(err)
	}
	ids, err := s.ExpireGuestBookings(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expiradas = %v, want a reservada e a pendente", ids)
	}
	want := map[string]string{
		reserved.ID: guests.StatusExpired, pending.ID: guests.StatusExpired,
		confirmed.ID: guests.StatusConfirmed, fresh.ID: guests.StatusReserved,
	}
	for id, status := range want {
		if b, _ := s.GuestBookingByID(ctx, id); b.Status != status {
			t.Errorf("%s ficou %s, want %s", id, b.Status, status)
		}
	}
	if again, _ := s.ExpireGuestBookings(ctx, time.Now()); len(again) != 0 {
		t.Errorf("segunda execução expirou %v", again)
	}
}

// Partida cancelada: quem pagou vai para reembolso, reservas são canceladas.
func TestCancelMatchSettlesGuestBookings(t *testing.T) {
	s := newTestStore(t, "guests_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 5)
	reserved := mustBook(t, s, m.ID, "11987654321")
	pending := mustPending(t, s, mustBook(t, s, m.ID, "21998765432"))
	confirmed := mustPending(t, s, mustBook(t, s, m.ID, "31998765432"))
	if _, _, err := s.ConfirmGuestPayment(ctx, confirmed.ID, confirmed.ProviderOrderID, "pay", time.Now()); err != nil {
		t.Fatal(err)
	}
	expired := mustBook(t, s, m.ID, "41998765432")
	if _, err := s.pool.Exec(ctx, `update guest_bookings set status = 'expired' where id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.CancelMatch(ctx, m.ID, "chuva"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		reserved.ID:  guests.StatusCancelled,
		pending.ID:   guests.StatusCancelled,
		confirmed.ID: guests.StatusRefundPending,
		expired.ID:   guests.StatusExpired,
	}
	for id, status := range want {
		b, _ := s.GuestBookingByID(ctx, id)
		if b.Status != status {
			t.Errorf("%s ficou %s, want %s", id, b.Status, status)
		}
		if status == guests.StatusRefundPending && b.RefundReason == "" {
			t.Error("reembolso sem motivo")
		}
	}
	if err := s.CancelMatch(ctx, m.ID, "de novo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cancelar duas vezes: %v", err)
	}
}
