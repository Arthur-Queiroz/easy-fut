package guests

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		err  error
	}{
		{"celular com máscara", "(11) 98765-4321", "+5511987654321", nil},
		{"só dígitos", "11987654321", "+5511987654321", nil},
		{"com +55 e espaços", "+55 11 98765-4321", "+5511987654321", nil},
		{"com 55 sem +", "5511987654321", "+5511987654321", nil},
		{"já em E.164", "+5511987654321", "+5511987654321", nil},
		{"pontos e espaços nas pontas", "  21.99876.5432 ", "+5521998765432", nil},
		{"DDD 55 sem país", "(55) 99876-5432", "+5555998765432", nil},
		{"DDD 55 com país", "+55 55 99876-5432", "+5555998765432", nil},
		{"zero de discagem", "011 98765-4321", "+5511987654321", nil},
		{"fixo", "(11) 3333-4444", "", ErrInvalidPhone},
		{"fixo com país", "+55 11 3333-4444", "", ErrInvalidPhone},
		{"sem DDD", "98765-4321", "", ErrInvalidPhone},
		{"curto", "1198765", "", ErrInvalidPhone},
		{"longo", "551198765432100", "", ErrInvalidPhone},
		{"DDD com zero", "(01) 98765-4321", "", ErrInvalidPhone},
		{"DDD terminando em zero", "(10) 98765-4321", "", ErrInvalidPhone},
		{"outro país", "+1 415 555 2671", "", ErrInvalidPhone},
		{"outro país com 13 dígitos", "+351 912 345 6789", "", ErrInvalidPhone},
		{"13 dígitos sem 55", "4411987654321", "", ErrInvalidPhone},
		{"letras", "11 9abc-4321", "", ErrInvalidPhone},
		{"vazio", "   ", "", ErrInvalidPhone},
		{"mais no meio", "11+987654321", "", ErrInvalidPhone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.raw)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("NormalizePhone(%q) = %q, %v; want %q, %v", tc.raw, got, err, tc.want, tc.err)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		err  error
	}{
		{"nome simples", "Ana", "Ana", nil},
		{"espaços extras", "  Ana   Maria  Souza ", "Ana Maria Souza", nil},
		{"acentos contam como uma letra", "Zé", "Zé", nil},
		{"no limite", strings.Repeat("á", MaxNameLength), strings.Repeat("á", MaxNameLength), nil},
		{"vazio", "   ", "", ErrInvalidName},
		{"uma letra", "A", "", ErrInvalidName},
		{"longo demais", strings.Repeat("a", MaxNameLength+1), "", ErrInvalidName},
		{"quebra de linha vira espaço", "Ana\nMaria", "Ana Maria", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeName(tc.raw)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("NormalizeName(%q) = %q, %v; want %q, %v", tc.raw, got, err, tc.want, tc.err)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		err  error
	}{
		{"simples", "ana@example.com", "ana@example.com", nil},
		{"maiúsculas e espaços", "  Ana.Souza@Example.COM ", "ana.souza@example.com", nil},
		{"com mais", "ana+fut@example.com.br", "ana+fut@example.com.br", nil},
		{"vazio", "", "", ErrInvalidEmail},
		{"sem arroba", "ana.example.com", "", ErrInvalidEmail},
		{"sem domínio com ponto", "ana@localhost", "", ErrInvalidEmail},
		{"com nome de exibição", "Ana <ana@example.com>", "", ErrInvalidEmail},
		{"espaço no meio", "ana souza@example.com", "", ErrInvalidEmail},
		{"duas arrobas", "ana@@example.com", "", ErrInvalidEmail},
		{"longo demais", strings.Repeat("a", 250) + "@example.com", "", ErrInvalidEmail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeEmail(tc.raw)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("NormalizeEmail(%q) = %q, %v; want %q, %v", tc.raw, got, err, tc.want, tc.err)
			}
		})
	}
}

func TestParseApplicant(t *testing.T) {
	a, err := ParseApplicant(" Ana  Souza ", "(11) 98765-4321", "ANA@example.com")
	if err != nil {
		t.Fatal(err)
	}
	want := Applicant{Name: "Ana Souza", Phone: "+5511987654321", Email: "ana@example.com"}
	if a != want {
		t.Fatalf("ParseApplicant = %+v, want %+v", a, want)
	}
	// A ordem dos erros segue a do formulário: nome, WhatsApp, e-mail.
	if _, err := ParseApplicant("", "x", "x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("nome inválido primeiro: %v", err)
	}
	if _, err := ParseApplicant("Ana", "x", "x"); !errors.Is(err, ErrInvalidPhone) {
		t.Errorf("telefone antes do e-mail: %v", err)
	}
	if _, err := ParseApplicant("Ana", "11987654321", "x"); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("e-mail: %v", err)
	}
}

var now = time.Date(2099, 1, 10, 12, 0, 0, 0, time.UTC)

// validSale é uma venda que passa em todas as regras; cada caso altera um campo.
func validSale() Sale {
	return Sale{
		SaleOpen:             true,
		MatchStatus:          MatchOpen,
		ConfirmationDeadline: now.Add(time.Hour),
		Slots:                4,
		ActiveBookings:       3,
		Now:                  now,
	}
}

func TestCheckSaleOpen(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Sale)
		want   error
	}{
		{"venda válida com a última vaga", func(*Sale) {}, nil},
		{"venda fechada pelo admin", func(s *Sale) { s.SaleOpen = false }, ErrSaleClosed},
		{"partida fechada", func(s *Sale) { s.MatchStatus = "closed" }, ErrSaleClosed},
		{"partida com times", func(s *Sale) { s.MatchStatus = "teams_drawn" }, ErrSaleClosed},
		{"partida cancelada", func(s *Sale) { s.MatchStatus = "cancelled" }, ErrSaleClosed},
		{"prazo vencendo agora", func(s *Sale) { s.ConfirmationDeadline = now }, ErrSaleClosed},
		{"prazo vencido", func(s *Sale) { s.ConfirmationDeadline = now.Add(-time.Minute) }, ErrSaleClosed},
		{"vagas esgotadas", func(s *Sale) { s.ActiveBookings = 4 }, ErrSoldOut},
		{"mais inscrições que vagas", func(s *Sale) { s.Slots = 2 }, ErrSoldOut},
		{"sem vagas definidas", func(s *Sale) { s.Slots, s.ActiveBookings = 0, 0 }, ErrSoldOut},
		// Venda encerrada é a resposta mais útil: não adianta esperar vaga.
		{"fechada vence esgotada", func(s *Sale) { s.SaleOpen, s.ActiveBookings = false, 4 }, ErrSaleClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSale()
			tc.change(&s)
			if err := CheckSaleOpen(s); !errors.Is(err, tc.want) {
				t.Fatalf("CheckSaleOpen() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSlotsLeft(t *testing.T) {
	cases := []struct{ slots, active, want int }{{4, 1, 3}, {4, 4, 0}, {2, 5, 0}, {0, 0, 0}}
	for _, tc := range cases {
		if got := SlotsLeft(tc.slots, tc.active); got != tc.want {
			t.Errorf("SlotsLeft(%d, %d) = %d, want %d", tc.slots, tc.active, got, tc.want)
		}
	}
}

func TestCheckTransition(t *testing.T) {
	allowed := map[string][]string{
		StatusReserved:       {StatusPendingPayment, StatusConfirmed, StatusExpired, StatusCancelled, StatusRefundPending},
		StatusPendingPayment: {StatusConfirmed, StatusExpired, StatusCancelled, StatusRefundPending},
		StatusConfirmed:      {StatusRefundPending},
		StatusExpired:        {StatusRefundPending},
		StatusCancelled:      {StatusRefundPending},
		StatusRefundPending:  {StatusRefunded},
		StatusRefunded:       {},
	}
	if len(allowed) != len(Statuses) {
		t.Fatalf("a tabela do teste cobre %d estados, o domínio tem %d", len(allowed), len(Statuses))
	}
	for _, from := range Statuses {
		for _, to := range Statuses {
			want := false
			for _, ok := range allowed[from] {
				want = want || ok == to
			}
			err := CheckTransition(from, to)
			if want && err != nil {
				t.Errorf("%s → %s deveria valer: %v", from, to, err)
			}
			if !want && !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s → %s: err = %v, want ErrInvalidTransition", from, to, err)
			}
		}
	}
	if err := CheckTransition("desconhecido", StatusConfirmed); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("estado desconhecido: %v", err)
	}
}

func TestHoldsSlot(t *testing.T) {
	for _, status := range Statuses {
		want := status == StatusReserved || status == StatusPendingPayment || status == StatusConfirmed
		if got := HoldsSlot(status); got != want {
			t.Errorf("HoldsSlot(%s) = %v, want %v", status, got, want)
		}
	}
	if len(ActiveStatuses) != 3 {
		t.Errorf("ActiveStatuses = %v", ActiveStatuses)
	}
}

func TestOnPaymentApproved(t *testing.T) {
	expires := now.Add(10 * time.Minute)
	cases := []struct {
		name      string
		status    string
		expiresAt time.Time
		want      string
		err       error
	}{
		{"pendente dentro do prazo", StatusPendingPayment, expires, StatusConfirmed, nil},
		{"reservada dentro do prazo", StatusReserved, expires, StatusConfirmed, nil},
		// Decisão "sempre reembolsa": pagamento tardio nunca vira vaga.
		{"pendente com reserva vencida", StatusPendingPayment, now.Add(-time.Second), StatusRefundPending, nil},
		{"vencendo no instante do pagamento", StatusPendingPayment, now, StatusRefundPending, nil},
		{"já expirada pelo cron", StatusExpired, expires, StatusRefundPending, nil},
		{"partida cancelada antes do pagamento", StatusCancelled, expires, StatusRefundPending, nil},
		// Webhook repetido: nada muda.
		{"já confirmada", StatusConfirmed, expires, "", ErrInvalidTransition},
		{"já aguardando reembolso", StatusRefundPending, expires, "", ErrInvalidTransition},
		{"já reembolsada", StatusRefunded, expires, "", ErrInvalidTransition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := OnPaymentApproved(Booking{Status: tc.status, ExpiresAt: tc.expiresAt}, now)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("OnPaymentApproved = %q, %v; want %q, %v", got, err, tc.want, tc.err)
			}
			if err == nil {
				if terr := CheckTransition(tc.status, got); terr != nil {
					t.Errorf("resultado fora da tabela de transições: %v", terr)
				}
			}
		})
	}
}

func TestOnMatchCancelled(t *testing.T) {
	want := map[string]string{
		StatusReserved:       StatusCancelled,
		StatusPendingPayment: StatusCancelled,
		StatusConfirmed:      StatusRefundPending,
	}
	for _, status := range Statuses {
		got, changed := OnMatchCancelled(status)
		next, ok := want[status]
		if changed != ok || (ok && got != next) || (!ok && got != status) {
			t.Errorf("OnMatchCancelled(%s) = %q, %v; want %q, %v", status, got, changed, next, ok)
		}
		if changed {
			if err := CheckTransition(status, got); err != nil {
				t.Errorf("resultado fora da tabela de transições: %v", err)
			}
		}
	}
}

func TestCheckExpense(t *testing.T) {
	cases := []struct {
		name        string
		description string
		amount      int64
		want        string
		err         error
	}{
		{"válida", "  Bolas novas ", 15000, "Bolas novas", nil},
		{"sem descrição", "  ", 15000, "", ErrInvalidExpense},
		{"valor zero", "Bolas", 0, "", ErrInvalidExpense},
		{"valor negativo", "Bolas", -1, "", ErrInvalidExpense},
		{"descrição longa", strings.Repeat("a", MaxExpenseDescription+1), 100, "", ErrInvalidExpense},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CheckExpense(tc.description, tc.amount)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("CheckExpense = %q, %v; want %q, %v", got, err, tc.want, tc.err)
			}
		})
	}
}

func TestCashSign(t *testing.T) {
	cases := map[string]int64{CashGuestPayment: 1, CashRefund: -1, CashExpense: -1, "outro": 0}
	for kind, want := range cases {
		if got := CashSign(kind); got != want {
			t.Errorf("CashSign(%s) = %d, want %d", kind, got, want)
		}
	}
}

// As mensagens são o contrato com a PWA (plano da participação avulsa).
func TestErrorMessages(t *testing.T) {
	cases := map[error]string{
		ErrSoldOut:            "as vagas avulsas desta partida acabaram",
		ErrSaleClosed:         "a venda de vagas desta partida está encerrada",
		ErrPhoneAlreadyBooked: "este WhatsApp já tem uma vaga nesta partida",
	}
	for err, want := range cases {
		if err.Error() != want {
			t.Errorf("mensagem = %q, want %q", err.Error(), want)
		}
	}
	for _, err := range []error{ErrInvalidPhone, ErrInvalidEmail, ErrInvalidName, ErrInvalidTransition, ErrInvalidExpense} {
		if err.Error() == "" {
			t.Errorf("erro sem mensagem: %#v", err)
		}
	}
}
