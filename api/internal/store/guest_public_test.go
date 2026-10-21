package store

import (
	"context"
	"testing"
	"time"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
)

// A lista pública só traz partidas com a venda valendo agora e conta como
// ocupadas só as inscrições que ainda prendem vaga (reserva vencida não conta,
// mesmo antes do cron passar).
func TestOpenGuestSales(t *testing.T) {
	s := newTestStore(t, "guest_public_test")
	ctx := context.Background()
	open, _ := guestTestMatch(t, s, 3)

	// Venda fechada, partida fechada e prazo vencido ficam de fora.
	closedSale, _ := guestTestMatch(t, s, 2)
	if err := s.SetGuestSale(ctx, closedSale.ID, false, 0, 0, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	closedMatch, _ := guestTestMatch(t, s, 2)
	if _, err := s.pool.Exec(ctx, `update matches set status = 'closed' where id = $1`, closedMatch.ID); err != nil {
		t.Fatal(err)
	}
	pastDeadline, _ := guestTestMatch(t, s, 2)
	if _, err := s.pool.Exec(ctx, `update matches set confirmation_deadline = now() - interval '1 minute' where id = $1`, pastDeadline.ID); err != nil {
		t.Fatal(err)
	}

	// Uma confirmada, uma reservada válida e uma reservada já vencida.
	confirmed := mustPending(t, s, mustBook(t, s, open.ID, "11987650001"))
	if _, _, err := s.ConfirmGuestPayment(ctx, confirmed.ID, confirmed.ProviderOrderID, "pay", time.Now()); err != nil {
		t.Fatal(err)
	}
	mustBook(t, s, open.ID, "11987650002")
	if _, err := s.CreateGuestBooking(ctx, guestInput(open.ID, "11987650003"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	sales, err := s.OpenGuestSales(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(sales) != 1 {
		t.Fatalf("vendas = %+v, want só a partida aberta", sales)
	}
	got := sales[0]
	if got.MatchID != open.ID || got.MatchDate != "2099-01-10" || got.StartTime != "20:00" || got.EndTime != "22:00" ||
		got.Venue != "Arena Teste" || got.Address != "Rua Teste, 1" || got.TotalCents != 2525 {
		t.Errorf("venda = %+v", got)
	}
	if got.Slots != 3 || got.ActiveBookings != 2 {
		t.Errorf("vagas = %d, ocupadas = %d; want 3 e 2", got.Slots, got.ActiveBookings)
	}
}

// Liberar a reserva (falha ao gerar o Pix) devolve a vaga e o telefone na
// hora; só vale para quem ainda está em reserved.
func TestReleaseGuestBooking(t *testing.T) {
	s := newTestStore(t, "guest_public_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 1)
	b := mustBook(t, s, m.ID, "11987650001")

	now := time.Now()
	if err := s.ReleaseGuestBooking(ctx, b.ID, now); err != nil {
		t.Fatal(err)
	}
	got, err := s.GuestBookingByID(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != guests.StatusExpired || got.ExpiresAt.After(now.Add(time.Second)) {
		t.Errorf("liberada = %s, vence %v", got.Status, got.ExpiresAt)
	}
	// A única vaga e o mesmo telefone voltam a valer.
	mustBook(t, s, m.ID, "11987650001")

	// Inscrição que já tem Pix não é liberada por aqui.
	other, _ := guestTestMatch(t, s, 1)
	pending := mustPending(t, s, mustBook(t, s, other.ID, "11987650002"))
	if err := s.ReleaseGuestBooking(ctx, pending.ID, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GuestBookingByID(ctx, pending.ID); got.Status != guests.StatusPendingPayment {
		t.Errorf("pendente virou %s", got.Status)
	}
}

// O estado do provedor é atualizado só para a order da inscrição e só antes
// do pagamento: uma notificação atrasada não apaga o "processed" gravado na
// confirmação.
func TestUpdateGuestProviderStatus(t *testing.T) {
	s := newTestStore(t, "guest_public_test")
	ctx := context.Background()
	m, _ := guestTestMatch(t, s, 2)
	b := mustPending(t, s, mustBook(t, s, m.ID, "11987650001"))

	if err := s.UpdateGuestProviderStatus(ctx, b.ID, "outra-order", "p", "x", "y"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateGuestProviderStatus(ctx, b.ID, b.ProviderOrderID, "pay-1", "processing", "in_process"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GuestBookingByID(ctx, b.ID)
	if got.ProviderPaymentID != "pay-1" || got.ProviderStatus != "processing" || got.ProviderStatusDetail != "in_process" {
		t.Errorf("provedor = %q %q %q", got.ProviderPaymentID, got.ProviderStatus, got.ProviderStatusDetail)
	}

	if _, _, err := s.ConfirmGuestPayment(ctx, b.ID, b.ProviderOrderID, "pay-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateGuestProviderStatus(ctx, b.ID, b.ProviderOrderID, "pay-1", "action_required", "waiting_transfer"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GuestBookingByID(ctx, b.ID)
	if got.ProviderStatus != "processed" || got.ProviderStatusDetail != "accredited" {
		t.Errorf("confirmada voltou para %q %q", got.ProviderStatus, got.ProviderStatusDetail)
	}
}
