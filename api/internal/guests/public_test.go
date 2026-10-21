package guests

import (
	"testing"
	"time"
)

// O avulso acompanha a inscrição pela página pública: uma reserva vencida
// aparece como expirada mesmo antes de o cron gravar isso, porque o Pix
// expira no mesmo instante e um pagamento tardio vira reembolso.
func TestPublicStatus(t *testing.T) {
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		status  string
		expires time.Time
		want    string
	}{
		{"pendente no prazo", StatusPendingPayment, now.Add(time.Minute), StatusPendingPayment},
		{"reservada no prazo", StatusReserved, now.Add(time.Minute), StatusReserved},
		{"pendente vencida", StatusPendingPayment, now, StatusExpired},
		{"reservada vencida", StatusReserved, now.Add(-time.Minute), StatusExpired},
		{"confirmada depois do prazo da reserva", StatusConfirmed, now.Add(-time.Hour), StatusConfirmed},
		{"reembolso pendente", StatusRefundPending, now.Add(-time.Hour), StatusRefundPending},
		{"cancelada", StatusCancelled, now.Add(time.Hour), StatusCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PublicStatus(Booking{Status: tt.status, ExpiresAt: tt.expires}, now); got != tt.want {
				t.Errorf("PublicStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// O Pix só é mostrado enquanto pode ser pago.
func TestShowsPix(t *testing.T) {
	for status, want := range map[string]bool{
		StatusReserved: false, StatusPendingPayment: true, StatusConfirmed: false,
		StatusExpired: false, StatusCancelled: false, StatusRefundPending: false, StatusRefunded: false,
	} {
		if got := ShowsPix(status); got != want {
			t.Errorf("ShowsPix(%q) = %v, want %v", status, got, want)
		}
	}
}
