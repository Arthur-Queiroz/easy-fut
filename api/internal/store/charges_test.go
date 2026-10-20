package store

import (
	"context"
	"testing"
	"time"

	"futdarapaziada/api/internal/billing"
)

func chargesTestDue() time.Time { return time.Date(2099, 1, 10, 0, 0, 0, 0, time.UTC) }

// A taxa do Pix é calculada sobre o rateio e congelada no lote e em cada
// cobrança, como o próprio rateio. Inativos não entram.
func TestGenerateBatchFreezesPixFee(t *testing.T) {
	s := newTestStore(t, "charges_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")
	votingTestUser(t, s, "active")
	votingTestUser(t, s, "active")
	votingTestUser(t, s, "inactive")

	batch, err := s.GenerateBatch(ctx, "2099-01", 18000, chargesTestDue(), admin, billing.DefaultPixRate)
	if err != nil {
		t.Fatal(err)
	}
	if batch.IndividualAmountCents != 6000 || batch.UserCount != 3 {
		t.Fatalf("rateio = %d × %d, want 6000 × 3", batch.IndividualAmountCents, batch.UserCount)
	}
	if batch.PixFeeBasisPoints != 99 || batch.PixFeeCents != 60 {
		t.Errorf("lote: taxa %d bps / %d centavos, want 99 / 60", batch.PixFeeBasisPoints, batch.PixFeeCents)
	}
	charges, err := s.ChargesByMonth(ctx, "2099-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(charges) != 3 {
		t.Fatalf("%d cobranças, want 3", len(charges))
	}
	for _, c := range charges {
		if c.AmountCents != 6000 || c.PixFeeCents != 60 || c.PixTotalCents != 6060 {
			t.Errorf("cobrança %s: rateio %d, taxa %d, total %d; want 6000, 60, 6060", c.UserName, c.AmountCents, c.PixFeeCents, c.PixTotalCents)
		}
	}
}

func TestGenerateMatchBatchFreezesPixFee(t *testing.T) {
	s := newTestStore(t, "charges_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")
	other := votingTestUser(t, s, "active")

	batch, err := s.GenerateMatchBatch(ctx, "2099-01", "Churrasco", 10000, chargesTestDue(), admin, []string{admin, other}, billing.DefaultPixRate)
	if err != nil {
		t.Fatal(err)
	}
	if batch.PixFeeCents != 50 {
		t.Errorf("taxa do lote = %d, want 50", batch.PixFeeCents)
	}
	c, err := s.ChargeByID(ctx, mustChargeOf(t, s, batch.ID, other))
	if err != nil {
		t.Fatal(err)
	}
	if c.PixTotalCents != 5050 {
		t.Errorf("total no Pix = %d, want 5050", c.PixTotalCents)
	}
}

// Sem taxa configurada (0), o total é o próprio rateio, como antes.
func TestGenerateBatchWithoutFee(t *testing.T) {
	s := newTestStore(t, "charges_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")

	batch, err := s.GenerateBatch(ctx, "2099-02", 6000, chargesTestDue(), admin, billing.FeeRate{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ChargeByID(ctx, mustChargeOf(t, s, batch.ID, admin))
	if err != nil {
		t.Fatal(err)
	}
	if batch.PixFeeCents != 0 || c.PixFeeCents != 0 || c.PixTotalCents != 6000 {
		t.Errorf("sem taxa: lote %d, cobrança %d/%d", batch.PixFeeCents, c.PixFeeCents, c.PixTotalCents)
	}
}

// O lembrete de vencida informa o valor que o jogador paga no Pix.
func TestMarkOverdueChargesReportsPixTotal(t *testing.T) {
	s := newTestStore(t, "charges_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")
	batch, err := s.GenerateBatch(ctx, "2099-03", 6000, chargesTestDue(), admin, billing.DefaultPixRate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update charges set due_date = current_date - 1 where batch_id = $1`, batch.ID); err != nil {
		t.Fatal(err)
	}
	overdue, err := s.MarkOverdueCharges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(overdue) != 1 || overdue[0].AmountCents != 6000 || overdue[0].PixTotalCents != 6060 {
		t.Fatalf("vencidas = %+v", overdue)
	}
}

func mustChargeOf(t *testing.T, s *Store, batchID, userID string) string {
	t.Helper()
	var id string
	if err := s.pool.QueryRow(context.Background(),
		`select id from charges where batch_id = $1 and user_id = $2`, batchID, userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
