package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"futdarapaziada/api/internal/guests"
)

// O saldo soma entradas e desconta estornos e despesas; o período filtra só
// a lista, o saldo é sempre o do caixa inteiro.
func TestCashReportAndExpenses(t *testing.T) {
	s := newTestStore(t, "cash_test")
	ctx := context.Background()
	m, admin := guestTestMatch(t, s, 3)
	b := mustPending(t, s, mustBook(t, s, m.ID, "11987654321"))
	if _, _, err := s.ConfirmGuestPayment(ctx, b.ID, b.ProviderOrderID, "pay", time.Now()); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddCashExpense(ctx, "  ", 100, "2099-01-05", admin); !errors.Is(err, guests.ErrInvalidExpense) {
		t.Errorf("despesa sem descrição: %v", err)
	}
	if _, err := s.AddCashExpense(ctx, "Bola", 0, "2099-01-05", admin); !errors.Is(err, guests.ErrInvalidExpense) {
		t.Errorf("despesa zerada: %v", err)
	}
	ball, err := s.AddCashExpense(ctx, " Bola nova ", 900, "2099-01-05", admin)
	if err != nil {
		t.Fatal(err)
	}
	if ball.Kind != guests.CashExpense || ball.Description != "Bola nova" || ball.AmountCents != 900 ||
		ball.OccurredOn != "2099-01-05" || ball.CreatedBy == nil || *ball.CreatedBy != admin || ball.CreatedByName == "" {
		t.Fatalf("despesa = %+v", ball)
	}
	old, err := s.AddCashExpense(ctx, "Coletes", 400, "2000-01-01", admin)
	if err != nil {
		t.Fatal(err)
	}

	report, err := s.CashReport(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.BalanceCents != 2500-900-400 || len(report.Entries) != 3 {
		t.Fatalf("caixa = %d com %d lançamentos, want %d com 3", report.BalanceCents, len(report.Entries), 2500-900-400)
	}
	// Mais recente primeiro.
	if report.Entries[len(report.Entries)-1].ID != old.ID {
		t.Errorf("ordem: %+v", report.Entries)
	}

	period, err := s.CashReport(ctx, "2099-01-01", "2099-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if period.BalanceCents != report.BalanceCents || len(period.Entries) != 1 || period.Entries[0].ID != ball.ID {
		t.Errorf("período = %+v", period)
	}

	// Só despesas podem ser removidas; a entrada de uma inscrição, não.
	var paymentID string
	if err := s.pool.QueryRow(ctx, `select id from cash_entries where kind = 'guest_payment'`).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCashExpense(ctx, paymentID); !errors.Is(err, ErrNotFound) {
		t.Errorf("remover entrada de inscrição: %v", err)
	}
	if err := s.DeleteCashExpense(ctx, ball.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCashExpense(ctx, ball.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("remover duas vezes: %v", err)
	}
	report, err = s.CashReport(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.BalanceCents != 2500-400 {
		t.Errorf("saldo depois de remover = %d", report.BalanceCents)
	}
}
