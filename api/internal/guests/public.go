package guests

import "time"

// PublicStatus é o estado que o avulso vê ao acompanhar a inscrição. Uma
// reserva ou Pix vencido aparece como expirado mesmo antes de o cron gravar
// isso (ele roda a cada minuto): o Pix expira no mesmo instante da reserva e
// um pagamento depois disso vira reembolso, nunca vaga.
func PublicStatus(b Booking, now time.Time) string {
	if (b.Status == StatusReserved || b.Status == StatusPendingPayment) && !now.Before(b.ExpiresAt) {
		return StatusExpired
	}
	return b.Status
}

// ShowsPix diz se o Pix (copia e cola, QR e link) ainda deve ser mostrado:
// só enquanto a inscrição aguarda o pagamento.
func ShowsPix(status string) bool {
	return status == StatusPendingPayment
}
