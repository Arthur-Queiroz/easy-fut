// Package guests reúne as regras de domínio da participação avulsa: quem
// compra uma vaga numa partida sem ser membro do grupo. Ele não conhece HTTP
// nem banco: recebe os fatos já carregados e devolve a decisão.
//
// Regras: .agents/specs/design/2026-10-19-participacao-avulsa.md (seção
// "Decisões") e o plano 2026-10-21-participacao-avulsa-plano.md.
package guests

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Estados de uma inscrição avulsa (guest_bookings.status).
const (
	StatusReserved       = "reserved"        // dados aceitos, Pix em criação
	StatusPendingPayment = "pending_payment" // Pix criado, aguardando pagamento
	StatusConfirmed      = "confirmed"       // pago: presença confirmada
	StatusExpired        = "expired"         // reserva venceu sem pagamento
	StatusCancelled      = "cancelled"       // partida cancelada antes do pagamento
	StatusRefundPending  = "refund_pending"  // pago, mas sem vaga: estornar
	StatusRefunded       = "refunded"        // estorno feito pelo admin
)

// Statuses lista todos os estados, na ordem do ciclo de vida.
var Statuses = []string{
	StatusReserved, StatusPendingPayment, StatusConfirmed, StatusExpired,
	StatusCancelled, StatusRefundPending, StatusRefunded,
}

// ActiveStatuses são os estados que ocupam vaga. É também o filtro do índice
// único de (partida, telefone): uma inscrição ativa por WhatsApp por partida.
var ActiveStatuses = []string{StatusReserved, StatusPendingPayment, StatusConfirmed}

// ReservationTTL é quanto a vaga fica presa esperando o Pix; o Pix expira no
// mesmo instante.
const ReservationTTL = 30 * time.Minute

// MatchOpen é o único estado de partida em que a venda vale.
const MatchOpen = "open"

var (
	ErrSoldOut            = errors.New("as vagas avulsas desta partida acabaram")
	ErrSaleClosed         = errors.New("a venda de vagas desta partida está encerrada")
	ErrPhoneAlreadyBooked = errors.New("este WhatsApp já tem uma vaga nesta partida")
	ErrInvalidPhone       = errors.New("informe um celular com DDD, como (11) 98765-4321")
	ErrInvalidEmail       = errors.New("informe um e-mail válido")
	ErrInvalidName        = errors.New("informe seu nome, com até 60 caracteres")
	ErrInvalidTransition  = errors.New("a inscrição não pode mudar para este estado")
	ErrInvalidExpense     = errors.New("informe a descrição e um valor maior que zero")
)

// NormalizePhone aceita um celular brasileiro em qualquer formatação comum
// (com ou sem +55, espaços, traços, pontos, parênteses, zero de discagem) e
// devolve o E.164 (+55DDDNNNNNNNNN). Fixo, número curto e outro país são
// recusados: o número precisa receber WhatsApp.
func NormalizePhone(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	hasPlus := strings.HasPrefix(s, "+")
	s = strings.TrimPrefix(s, "+")

	var digits strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", ErrInvalidPhone
		}
	}
	d := digits.String()

	switch {
	case len(d) == 13 && strings.HasPrefix(d, "55"):
		d = d[2:]
	case hasPlus:
		// Com +, só vale o código do Brasil.
		return "", ErrInvalidPhone
	case len(d) == 12 && strings.HasPrefix(d, "0"):
		d = d[1:] // 0 + DDD + número, como se disca de outra cidade
	}

	// DDD sem zero nos dois dígitos, depois o 9 do celular e mais 8 dígitos.
	if len(d) != 11 || d[0] == '0' || d[1] == '0' || d[2] != '9' {
		return "", ErrInvalidPhone
	}
	return "+55" + d, nil
}

// MaxNameLength é o maior nome aceito, em caracteres.
const MaxNameLength = 60

// NormalizeName junta espaços repetidos e exige de 2 a MaxNameLength
// caracteres.
func NormalizeName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if n := utf8.RuneCountInString(name); n < 2 || n > MaxNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}

// NormalizeEmail devolve o e-mail em minúsculas. A verificação é de formato:
// o Mercado Pago exige um e-mail do pagador, e um erro de digitação aqui só
// atrapalharia o próprio avulso.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 || strings.ContainsFunc(email, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`<>()[],;:"\`, r)
	}) {
		return "", ErrInvalidEmail
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || len(local) > 64 || strings.Contains(domain, "@") {
		return "", ErrInvalidEmail
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 || slices.Contains(labels, "") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// Applicant são os dados do formulário de inscrição já normalizados.
type Applicant struct {
	Name  string
	Phone string // E.164
	Email string
}

// ParseApplicant valida o formulário na ordem em que ele aparece na tela.
func ParseApplicant(name, phone, email string) (Applicant, error) {
	var a Applicant
	var err error
	if a.Name, err = NormalizeName(name); err != nil {
		return Applicant{}, err
	}
	if a.Phone, err = NormalizePhone(phone); err != nil {
		return Applicant{}, err
	}
	if a.Email, err = NormalizeEmail(email); err != nil {
		return Applicant{}, err
	}
	return a, nil
}

// Sale são os fatos da partida no instante da inscrição, lidos com a partida
// travada para que duas compras simultâneas não levem a mesma vaga.
type Sale struct {
	SaleOpen             bool
	MatchStatus          string
	ConfirmationDeadline time.Time
	Slots                int
	ActiveBookings       int // inscrições em ActiveStatuses
	Now                  time.Time
}

// CheckSaleOpen decide se ainda dá para reservar uma vaga: venda aberta pelo
// admin, partida aberta, prazo de confirmação no futuro e vaga livre.
func CheckSaleOpen(s Sale) error {
	if !s.SaleOpen || s.MatchStatus != MatchOpen || !s.Now.Before(s.ConfirmationDeadline) {
		return ErrSaleClosed
	}
	if SlotsLeft(s.Slots, s.ActiveBookings) == 0 {
		return ErrSoldOut
	}
	return nil
}

// SlotsLeft é o número de vagas avulsas livres, nunca negativo (o admin pode
// reduzir as vagas abaixo das já vendidas).
func SlotsLeft(slots, activeBookings int) int {
	return max(slots-activeBookings, 0)
}

// HoldsSlot diz se a inscrição ocupa vaga.
func HoldsSlot(status string) bool {
	return slices.Contains(ActiveStatuses, status)
}

// transitions é a tabela de mudanças de estado permitidas. Pagamento pode
// chegar em qualquer estado anterior ao pagamento; o que não vira vaga vira
// reembolso.
var transitions = map[string][]string{
	StatusReserved:       {StatusPendingPayment, StatusConfirmed, StatusExpired, StatusCancelled, StatusRefundPending},
	StatusPendingPayment: {StatusConfirmed, StatusExpired, StatusCancelled, StatusRefundPending},
	StatusConfirmed:      {StatusRefundPending},
	StatusExpired:        {StatusRefundPending},
	StatusCancelled:      {StatusRefundPending},
	StatusRefundPending:  {StatusRefunded},
}

// CheckTransition valida a mudança de estado da inscrição.
func CheckTransition(from, to string) error {
	if !slices.Contains(transitions[from], to) {
		return ErrInvalidTransition
	}
	return nil
}

// Booking são os fatos da inscrição usados nas decisões de estado.
type Booking struct {
	Status    string
	ExpiresAt time.Time
}

// OnPaymentApproved decide o destino da inscrição quando o Mercado Pago
// aprova o pagamento. Com a reserva ainda válida, a vaga é confirmada. Depois
// do prazo, ou se a inscrição já saiu do ciclo (expirada pelo cron, partida
// cancelada), a decisão do grupo é "sempre reembolsa": nunca vira vaga.
// Um pagamento já tratado (webhook repetido) devolve ErrInvalidTransition.
func OnPaymentApproved(b Booking, now time.Time) (string, error) {
	switch b.Status {
	case StatusReserved, StatusPendingPayment:
		if now.Before(b.ExpiresAt) {
			return StatusConfirmed, nil
		}
		return StatusRefundPending, nil
	case StatusExpired, StatusCancelled:
		return StatusRefundPending, nil
	}
	return "", ErrInvalidTransition
}

// OnMatchCancelled diz o que acontece com a inscrição quando a partida é
// cancelada: quem pagou vai para reembolso; quem ainda não pagou tem a
// reserva cancelada. Os demais estados não mudam (changed = false).
func OnMatchCancelled(status string) (next string, changed bool) {
	switch status {
	case StatusConfirmed:
		return StatusRefundPending, true
	case StatusReserved, StatusPendingPayment:
		return StatusCancelled, true
	}
	return status, false
}
