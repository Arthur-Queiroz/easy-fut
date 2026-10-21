package guests

import (
	"strings"
	"testing"
)

func TestConfirmationMessage(t *testing.T) {
	got := ConfirmationMessage("2099-01-10", "20:00", "Arena Futiba 1")
	want := "Sua presença na partida de 10/01/2099 às 20:00, em Arena Futiba 1, está confirmada. " +
		"Você receberá o link para votar após o encerramento da partida."
	if got != want {
		t.Errorf("ConfirmationMessage() = %q, want %q", got, want)
	}
	// Data fora do formato do banco aparece como veio, sem quebrar a mensagem.
	if got := ConfirmationMessage("amanhã", "20:00", "Arena"); !strings.HasPrefix(got, "Sua presença na partida de amanhã às 20:00, em Arena") {
		t.Errorf("data inválida: %q", got)
	}
}

// A fila de WhatsApp usa o número sem "+", como notify.NormalizeNumber.
func TestWhatsAppNumber(t *testing.T) {
	if got := WhatsAppNumber("+5511987654321"); got != "5511987654321" {
		t.Errorf("WhatsAppNumber = %q", got)
	}
}
