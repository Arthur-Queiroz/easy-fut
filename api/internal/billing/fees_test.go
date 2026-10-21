package billing

import (
	"errors"
	"testing"
)

// Extrato real do Mercado Pago: Pix de R$ 59,52, creditado R$ 58,93.
func TestProviderFeeMatchesMercadoPagoStatement(t *testing.T) {
	if fee := DefaultPixRate.ProviderFee(5952); fee != 59 {
		t.Fatalf("ProviderFee(5952) = %d, want 59", fee)
	}
}

func TestPassOnKeepsTheFullSplitForTheGroup(t *testing.T) {
	cases := []struct {
		name      string
		rate      FeeRate
		base      int64
		wantTotal int64
	}{
		{"mensalidade de R$ 60", DefaultPixRate, 6000, 6060},
		{"valor do extrato", DefaultPixRate, 5952, 6012},
		{"centavo", DefaultPixRate, 1, 2},
		{"sem taxa", FeeRate{}, 6000, 6000},
		{"só valor fixo", FeeRate{FixedCents: 199}, 6000, 6199},
		{"cartão 3,98%", FeeRate{BasisPoints: 398}, 6000, 6249},
		{"percentual e fixo", FeeRate{BasisPoints: 199, FixedCents: 49}, 6000, 6172},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fee := tc.rate.PassOn(tc.base)
			if fee.TotalCents != tc.wantTotal || fee.FeeCents != tc.wantTotal-tc.base || fee.BaseCents != tc.base {
				t.Fatalf("PassOn(%d) = %+v, want total %d", tc.base, fee, tc.wantTotal)
			}
			// O grupo recebe ao menos a base, e um centavo a menos já não bastaria.
			if net := fee.TotalCents - tc.rate.ProviderFee(fee.TotalCents); net < tc.base {
				t.Errorf("líquido %d < base %d", net, tc.base)
			}
			if tc.wantTotal > tc.base {
				if net := tc.wantTotal - 1 - tc.rate.ProviderFee(tc.wantTotal-1); net >= tc.base {
					t.Errorf("total %d não é o menor: %d já deixa líquido %d", tc.wantTotal, tc.wantTotal-1, net)
				}
			}
		})
	}
}

// Para qualquer rateio, o líquido nunca fica abaixo da base.
func TestPassOnNeverLosesMoney(t *testing.T) {
	for _, rate := range []FeeRate{DefaultPixRate, {BasisPoints: 498}, {BasisPoints: 1999, FixedCents: 349}} {
		for base := int64(1); base <= 30000; base += 7 {
			fee := rate.PassOn(base)
			if net := fee.TotalCents - rate.ProviderFee(fee.TotalCents); net < base {
				t.Fatalf("%+v base %d: total %d deixa líquido %d", rate, base, fee.TotalCents, net)
			}
		}
	}
}

func TestParseBasisPoints(t *testing.T) {
	for raw, want := range map[string]int64{"0": 0, "99": 99, " 150 ": 150, "1999": 1999} {
		got, err := ParseBasisPoints(raw)
		if err != nil || got != want {
			t.Errorf("ParseBasisPoints(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "-1", "2000", "0.99", "abc"} {
		if _, err := ParseBasisPoints(raw); !errors.Is(err, ErrInvalidFeeRate) {
			t.Errorf("ParseBasisPoints(%q) err = %v, want ErrInvalidFeeRate", raw, err)
		}
	}
}
