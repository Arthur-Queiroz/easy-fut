// Package billing reúne as regras de domínio das cobranças. Ele não conhece
// HTTP nem banco: recebe os valores já carregados e devolve a decisão.
package billing

import (
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidFeeRate: a configuração guarda pontos-base (99 = 0,99%), mas o
// admin edita em porcentagem.
var ErrInvalidFeeRate = errors.New("taxa do Pix inválida: use de 0% a 19,99%")

// FeeRate é a tarifa de um meio de pagamento: um percentual em pontos-base
// (99 = 0,99%) mais um valor fixo por pagamento.
type FeeRate struct {
	BasisPoints int64
	FixedCents  int64
}

// DefaultPixRate é a tarifa do Pix pelo Mercado Pago na conta do grupo,
// conferida em extrato (pago R$ 59,52, creditado R$ 58,93).
var DefaultPixRate = FeeRate{BasisPoints: 99}

// ProviderFee é quanto o provedor desconta de um pagamento de total
// centavos. O Mercado Pago arredonda a tarifa ao centavo; o extrato não
// distingue "mais próximo" de "para cima", então o cálculo assume para cima,
// o pior caso para o grupo.
func (r FeeRate) ProviderFee(totalCents int64) int64 {
	return (totalCents*r.BasisPoints+9999)/10000 + r.FixedCents
}

// Fee é o detalhamento mostrado ao pagador: o rateio, a taxa repassada e o
// total cobrado.
type Fee struct {
	BaseCents  int64
	FeeCents   int64
	TotalCents int64
}

// PassOn devolve o menor total que, descontada a tarifa do provedor, ainda
// deixa ao menos baseCents para o grupo. Com a tarifa arredondada para cima,
// o líquido é floor(total × (1 − p)) − fixo, então o menor total é
// ceil((base + fixo) / (1 − p)), sem ajuste fino.
func (r FeeRate) PassOn(baseCents int64) Fee {
	divisor := 10000 - r.BasisPoints
	total := ((baseCents+r.FixedCents)*10000 + divisor - 1) / divisor
	return Fee{BaseCents: baseCents, FeeCents: total - baseCents, TotalCents: total}
}

// ParseBasisPoints valida a taxa editada pelo admin nas configurações.
func ParseBasisPoints(raw string) (int64, error) {
	bps, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || bps < 0 || bps >= 2000 {
		return 0, ErrInvalidFeeRate
	}
	return bps, nil
}
