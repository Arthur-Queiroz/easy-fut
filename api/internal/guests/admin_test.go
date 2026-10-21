package guests

import (
	"errors"
	"testing"
	"time"
)

// validOpening é uma abertura de venda que passa em todas as regras; cada
// caso altera um campo.
func validOpening() SaleOpening {
	return SaleOpening{
		MatchStatus:          MatchOpen,
		ConfirmationDeadline: now.Add(time.Hour),
		Now:                  now,
		Slots:                4,
		ActiveBookings:       2,
	}
}

func TestCheckSaleOpening(t *testing.T) {
	cases := []struct {
		name   string
		change func(*SaleOpening)
		want   error
	}{
		{"abertura válida", func(*SaleOpening) {}, nil},
		{"vagas iguais às inscrições ativas", func(o *SaleOpening) { o.Slots = 2 }, nil},
		{"zero vagas sem inscrições", func(o *SaleOpening) { o.Slots, o.ActiveBookings = 0, 0 }, nil},
		{"no limite de vagas", func(o *SaleOpening) { o.Slots = MaxSlots }, nil},
		{"vagas negativas", func(o *SaleOpening) { o.Slots = -1 }, ErrInvalidSlots},
		{"vagas acima do limite", func(o *SaleOpening) { o.Slots = MaxSlots + 1 }, ErrInvalidSlots},
		{"partida fechada", func(o *SaleOpening) { o.MatchStatus = "closed" }, ErrMatchNotOpen},
		{"partida em votação", func(o *SaleOpening) { o.MatchStatus = "voting" }, ErrMatchNotOpen},
		{"partida cancelada", func(o *SaleOpening) { o.MatchStatus = "cancelled" }, ErrMatchNotOpen},
		{"prazo de confirmação vencido", func(o *SaleOpening) { o.ConfirmationDeadline = now }, ErrMatchNotOpen},
		{"vagas abaixo das inscrições ativas", func(o *SaleOpening) { o.Slots = 1 }, ErrSlotsBelowBooked},
		// Número fora da faixa é erro de digitação: vale antes do estado.
		{"vagas inválidas em partida fechada", func(o *SaleOpening) { o.Slots, o.MatchStatus = -1, "closed" }, ErrInvalidSlots},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := validOpening()
			tc.change(&o)
			if err := CheckSaleOpening(o); !errors.Is(err, tc.want) {
				t.Fatalf("CheckSaleOpening() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
		err  error
	}{
		{"2500", 2500, nil},
		{" 100 ", 100, nil},
		{"100000", 100000, nil},
		{"99", 0, ErrInvalidPrice},
		{"100001", 0, ErrInvalidPrice},
		{"0", 0, ErrInvalidPrice},
		{"-2500", 0, ErrInvalidPrice},
		{"25,00", 0, ErrInvalidPrice},
		{"25.5", 0, ErrInvalidPrice},
		{"", 0, ErrInvalidPrice},
	}
	for _, tc := range cases {
		got, err := ParsePrice(tc.raw)
		if !errors.Is(err, tc.err) || got != tc.want {
			t.Errorf("ParsePrice(%q) = %d, %v; want %d, %v", tc.raw, got, err, tc.want, tc.err)
		}
	}
}

// Configuração ausente ou corrompida no banco não impede abrir a venda: vale
// o preço padrão.
func TestPriceOrDefault(t *testing.T) {
	cases := map[string]int64{"3000": 3000, "": DefaultPriceCents, "abc": DefaultPriceCents, "50": DefaultPriceCents}
	for raw, want := range cases {
		if got := PriceOrDefault(raw); got != want {
			t.Errorf("PriceOrDefault(%q) = %d, want %d", raw, got, want)
		}
	}
	if DefaultPriceCents != 2500 {
		t.Errorf("preço padrão = %d, want 2500", DefaultPriceCents)
	}
}

func TestCheckRefund(t *testing.T) {
	cases := []struct {
		status  string
		changed bool
		err     error
	}{
		{StatusRefundPending, true, nil},
		// Repetir a marcação não muda nada nem lança outra saída.
		{StatusRefunded, false, nil},
		{StatusConfirmed, false, ErrInvalidTransition},
		{StatusPendingPayment, false, ErrInvalidTransition},
		{StatusReserved, false, ErrInvalidTransition},
		{StatusExpired, false, ErrInvalidTransition},
		{StatusCancelled, false, ErrInvalidTransition},
	}
	for _, tc := range cases {
		changed, err := CheckRefund(tc.status)
		if changed != tc.changed || !errors.Is(err, tc.err) {
			t.Errorf("CheckRefund(%s) = %v, %v; want %v, %v", tc.status, changed, err, tc.changed, tc.err)
		}
	}
}

func TestValidStatus(t *testing.T) {
	for _, s := range Statuses {
		if !ValidStatus(s) {
			t.Errorf("ValidStatus(%s) = false", s)
		}
	}
	for _, s := range []string{"", "paid", "CONFIRMED"} {
		if ValidStatus(s) {
			t.Errorf("ValidStatus(%q) = true", s)
		}
	}
}
