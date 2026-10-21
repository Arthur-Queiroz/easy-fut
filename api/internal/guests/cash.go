package guests

import (
	"strings"
	"unicode/utf8"
)

// Tipos de lançamento do caixa do grupo (cash_entries.kind). O valor é sempre
// positivo; o sinal vem do tipo.
const (
	CashGuestPayment = "guest_payment" // entrada: inscrição avulsa paga
	CashRefund       = "refund"        // saída: estorno de uma inscrição
	CashExpense      = "expense"       // saída: compra do grupo lançada pelo admin
)

// MaxExpenseDescription é o maior texto aceito na descrição de uma despesa.
const MaxExpenseDescription = 120

// CashSign é o sinal do lançamento no saldo do caixa.
func CashSign(kind string) int64 {
	switch kind {
	case CashGuestPayment:
		return 1
	case CashRefund, CashExpense:
		return -1
	}
	return 0
}

// CheckExpense valida a despesa lançada pelo admin e devolve a descrição
// sem espaços nas pontas.
func CheckExpense(description string, amountCents int64) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" || utf8.RuneCountInString(description) > MaxExpenseDescription || amountCents <= 0 {
		return "", ErrInvalidExpense
	}
	return description, nil
}
