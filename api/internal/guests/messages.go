package guests

import (
	"fmt"
	"strings"
	"time"
)

// ConfirmationMessage é o WhatsApp enviado quando o pagamento confirma a
// vaga. matchDate vem do banco como YYYY-MM-DD e startTime como HH:MM.
func ConfirmationMessage(matchDate, startTime, venue string) string {
	date := matchDate
	if d, err := time.Parse("2006-01-02", matchDate); err == nil {
		date = d.Format("02/01/2006")
	}
	return fmt.Sprintf("Sua presença na partida de %s às %s, em %s, está confirmada. "+
		"Você receberá o link para votar após o encerramento da partida.", date, startTime, venue)
}

// WhatsAppNumber converte o telefone da inscrição (E.164) para o formato da
// fila de WhatsApp, sem "+", o mesmo de notify.NormalizeNumber.
func WhatsAppNumber(phone string) string {
	return strings.TrimPrefix(phone, "+")
}
