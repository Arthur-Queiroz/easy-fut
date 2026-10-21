package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
)

// Participação avulsa — admin (fase B): venda por partida, lista de
// inscrições e baixa do reembolso. O caixa fica em cash.go.

// UpdateGuestSale abre, reajusta ou fecha a venda de vagas avulsas, com a
// partida travada para não competir com uma inscrição em andamento.
//
//   - Abrir valida a partida e as vagas (guests.CheckSaleOpening). Se a venda
//     estava fechada, congela priceCents e o total com a taxa embutida; se já
//     estava aberta, só muda as vagas e mantém o preço que o público já viu.
//   - Fechar não valida nada e mantém vagas, preço e total.
func (s *Store) UpdateGuestSale(ctx context.Context, matchID string, open bool, slots int, priceCents int64, rate billing.FeeRate, now time.Time) (Match, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Match{}, err
	}
	defer tx.Rollback(ctx)

	o := guests.SaleOpening{Now: now, Slots: slots}
	var wasOpen bool
	err = tx.QueryRow(ctx, `
		select status, confirmation_deadline, guest_sale_open from matches where id = $1 for update`, matchID).
		Scan(&o.MatchStatus, &o.ConfirmationDeadline, &wasOpen)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrNotFound
	}
	if err != nil {
		return Match{}, err
	}

	if !open {
		if _, err := tx.Exec(ctx, `update matches set guest_sale_open = false where id = $1`, matchID); err != nil {
			return Match{}, err
		}
	} else {
		// Reserva vencida ainda não expirada pelo cron já não ocupa vaga.
		if err := tx.QueryRow(ctx, `
			select count(*) from guest_bookings
			where match_id = $1 and (status = 'confirmed' or
				(status in ('reserved', 'pending_payment') and expires_at > $2))`,
			matchID, now).Scan(&o.ActiveBookings); err != nil {
			return Match{}, err
		}
		if err := guests.CheckSaleOpening(o); err != nil {
			return Match{}, err
		}
		if wasOpen {
			_, err = tx.Exec(ctx, `update matches set guest_slots = $2 where id = $1`, matchID, slots)
		} else {
			fee := rate.PassOn(priceCents)
			_, err = tx.Exec(ctx, `
				update matches set guest_sale_open = true, guest_slots = $2,
					guest_price_cents = $3, guest_total_cents = $4
				where id = $1`, matchID, slots, fee.BaseCents, fee.TotalCents)
		}
		if err != nil {
			return Match{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Match{}, err
	}
	return s.MatchByID(ctx, matchID)
}

// AdminGuestBooking é a inscrição na tela Avulsos do admin, com a partida e
// a situação dos dois WhatsApps (nulos até a primeira entrega).
type AdminGuestBooking struct {
	ID                 string     `json:"id"`
	MatchID            string     `json:"match_id"`
	MatchDate          string     `json:"match_date"` // YYYY-MM-DD
	StartTime          string     `json:"start_time"` // HH:MM
	Venue              string     `json:"venue"`
	Name               string     `json:"name"`
	Phone              string     `json:"phone"` // E.164
	Email              string     `json:"email"`
	Status             string     `json:"status"`
	BaseCents          int64      `json:"base_cents"`
	TotalCents         int64      `json:"total_cents"`
	CreatedAt          time.Time  `json:"created_at"`
	ConfirmedAt        *time.Time `json:"confirmed_at"`
	RefundReason       string     `json:"refund_reason"`
	ConfirmationSentAt *time.Time `json:"confirmation_sent_at"`
	VoteLinkSentAt     *time.Time `json:"vote_link_sent_at"`
}

// GuestBookingFilter são os filtros da tela Avulsos; vazios não filtram.
type GuestBookingFilter struct {
	MatchID string
	Status  string
	Query   string // nome ou telefone, em qualquer formatação
}

// guestMessageSentAt é a última entrega de um tipo de WhatsApp da inscrição,
// automática ou reenviada pelo admin.
func guestMessageSentAt(kind string) string {
	return `(select max(n.sent_at) from guest_messages gm
		join notifications n on n.id = gm.notification_id
		where gm.booking_id = g.id and gm.kind = '` + kind + `' and n.status = 'sent')`
}

var adminGuestBookingSelect = `
	select g.id, g.match_id, m.match_date::text, to_char(m.start_time, 'HH24:MI'), m.venue,
		g.name, g.phone, g.email, g.status, g.base_cents, g.total_cents, g.created_at,
		g.confirmed_at, g.refund_reason,
		` + guestMessageSentAt("confirmation") + `,
		` + guestMessageSentAt("vote_link") + `
	from guest_bookings g join matches m on m.id = g.match_id`

func scanAdminGuestBooking(row pgx.Row) (AdminGuestBooking, error) {
	var b AdminGuestBooking
	err := row.Scan(&b.ID, &b.MatchID, &b.MatchDate, &b.StartTime, &b.Venue,
		&b.Name, &b.Phone, &b.Email, &b.Status, &b.BaseCents, &b.TotalCents, &b.CreatedAt,
		&b.ConfirmedAt, &b.RefundReason, &b.ConfirmationSentAt, &b.VoteLinkSentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// ListGuestBookings devolve as inscrições, mais recentes primeiro. A busca
// casa o nome (sem diferenciar maiúsculas) ou o telefone: um celular
// completo é normalizado como no formulário; um trecho de dígitos procura
// dentro do número.
func (s *Store) ListGuestBookings(ctx context.Context, f GuestBookingFilter) ([]AdminGuestBooking, error) {
	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if f.MatchID != "" {
		where = append(where, "g.match_id = "+arg(f.MatchID)+"::uuid")
	}
	if f.Status != "" {
		where = append(where, "g.status = "+arg(f.Status))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		or := []string{"g.name ilike " + arg("%"+escapeLike(q)+"%")}
		if phone, err := guests.NormalizePhone(q); err == nil {
			or = append(or, "g.phone = "+arg(phone))
		} else if digits := onlyDigits(q); digits != "" && !strings.ContainsFunc(q, unicode.IsLetter) {
			// Trecho do número (ex.: "98765"): só quando a busca não tem
			// letras, para "Ana 2" não casar telefones.
			or = append(or, "g.phone like "+arg("%"+digits+"%"))
		}
		where = append(where, "("+strings.Join(or, " or ")+")")
	}
	query := adminGuestBookingSelect
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by g.created_at desc, g.id desc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminGuestBooking
	for rows.Next() {
		b, err := scanAdminGuestBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AdminGuestBookingByID é uma inscrição no formato da tela Avulsos.
func (s *Store) AdminGuestBookingByID(ctx context.Context, id string) (AdminGuestBooking, error) {
	return scanAdminGuestBooking(s.pool.QueryRow(ctx, adminGuestBookingSelect+` where g.id = $1`, id))
}

// escapeLike protege os curingas do LIKE no texto digitado pelo admin.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MarkGuestBookingRefunded registra o estorno feito pelo admin no Mercado
// Pago (guests.CheckRefund). Numa transação, com a inscrição travada: grava
// refunded e lança no caixa a saída do mesmo valor (e taxa) da entrada da
// inscrição; sem entrada, não há saída. Repetir a ação não muda nada
// (changed = false).
func (s *Store) MarkGuestBookingRefunded(ctx context.Context, id, adminID string, now time.Time) (changed bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var status, name string
	err = tx.QueryRow(ctx, `select status, name from guest_bookings where id = $1 for update`, id).Scan(&status, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if changed, err = guests.CheckRefund(status); err != nil || !changed {
		return false, err
	}

	if _, err := tx.Exec(ctx, `update guest_bookings set status = 'refunded' where id = $1`, id); err != nil {
		return false, err
	}
	// O índice único (booking_id, kind) impede uma segunda saída.
	if _, err := tx.Exec(ctx, `
		insert into cash_entries (kind, amount_cents, fee_cents, booking_id, description, occurred_on, created_by)
		select 'refund', e.amount_cents, e.fee_cents, e.booking_id, $2,
			($3::timestamptz at time zone 'America/Sao_Paulo')::date, $4
		from cash_entries e where e.booking_id = $1 and e.kind = 'guest_payment'
		on conflict do nothing`,
		id, "Estorno de inscrição avulsa: "+name, now, adminID); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
