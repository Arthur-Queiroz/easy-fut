package guests

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Regras das ações do admin sobre a participação avulsa: abrir a venda de
// uma partida, preço configurado e baixa do reembolso.

// MaxSlots é o maior número de vagas avulsas por partida.
const MaxSlots = 100

// Faixa aceita para o preço base da vaga (configuração guest_price_cents).
const (
	DefaultPriceCents int64 = 2500
	MinPriceCents     int64 = 100
	MaxPriceCents     int64 = 100000
)

var (
	ErrInvalidSlots     = errors.New("informe de 0 a 100 vagas avulsas")
	ErrMatchNotOpen     = errors.New("a venda de vagas só abre com a partida aberta para confirmações")
	ErrSlotsBelowBooked = errors.New("há mais inscrições avulsas ativas do que esse número de vagas")
	ErrInvalidPrice     = errors.New("preço do avulso inválido: use de R$ 1,00 a R$ 1.000,00")
)

// SaleOpening são os fatos da partida quando o admin abre (ou reajusta) a
// venda, lidos com a partida travada.
type SaleOpening struct {
	MatchStatus          string
	ConfirmationDeadline time.Time
	Now                  time.Time
	Slots                int // vagas pedidas pelo admin
	ActiveBookings       int // inscrições que ocupam vaga agora
}

// CheckSaleOpening decide se a venda pode ficar aberta com as vagas pedidas:
// número de vagas na faixa, partida aberta com prazo no futuro e nenhuma
// inscrição ativa sem vaga (reduzir abaixo disso deixaria gente paga de fora).
func CheckSaleOpening(o SaleOpening) error {
	if o.Slots < 0 || o.Slots > MaxSlots {
		return ErrInvalidSlots
	}
	if o.MatchStatus != MatchOpen || !o.Now.Before(o.ConfirmationDeadline) {
		return ErrMatchNotOpen
	}
	if o.Slots < o.ActiveBookings {
		return ErrSlotsBelowBooked
	}
	return nil
}

// ParsePrice valida o preço base editado pelo admin nas configurações, em
// centavos inteiros.
func ParsePrice(raw string) (int64, error) {
	cents, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || cents < MinPriceCents || cents > MaxPriceCents {
		return 0, ErrInvalidPrice
	}
	return cents, nil
}

// PriceOrDefault é o preço vigente ao abrir a venda. Um valor ausente ou
// inválido no banco cai no padrão em vez de travar a venda.
func PriceOrDefault(raw string) int64 {
	cents, err := ParsePrice(raw)
	if err != nil {
		return DefaultPriceCents
	}
	return cents
}

// CheckRefund decide a baixa do estorno feito pelo admin no Mercado Pago. Só
// uma inscrição em refund_pending muda; uma já reembolsada é repetição da
// mesma ação (changed = false, sem erro).
func CheckRefund(status string) (changed bool, err error) {
	if status == StatusRefunded {
		return false, nil
	}
	if err := CheckTransition(status, StatusRefunded); err != nil {
		return false, err
	}
	return true, nil
}

// ValidStatus diz se o texto é um estado de inscrição (filtro do admin).
func ValidStatus(status string) bool {
	return slices.Contains(Statuses, status)
}
