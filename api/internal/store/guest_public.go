package store

import (
	"context"
	"time"

	"futdarapaziada/api/internal/guests"
)

// Participação avulsa, parte pública (fase A): a vitrine de partidas com
// venda aberta e o ciclo do Pix da inscrição.

// GuestSaleMatch é uma partida à venda na página pública. Só dados para a
// escolha: nunca presença, nomes ou telefones.
type GuestSaleMatch struct {
	MatchID        string
	MatchDate      string // YYYY-MM-DD
	StartTime      string // HH:MM
	EndTime        string
	Venue          string
	Address        string
	TotalCents     int64
	Slots          int
	ActiveBookings int // inscrições que ainda prendem vaga
}

// OpenGuestSales lista as partidas em que a venda vale agora: aberta pelo
// admin, partida open e prazo de confirmação no futuro (as condições de
// guests.CheckSaleOpen, menos a vaga: esgotada aparece com zero vagas).
// Reserva vencida não conta como ocupada, mesmo antes de o cron passar.
func (s *Store) OpenGuestSales(ctx context.Context, now time.Time) ([]GuestSaleMatch, error) {
	rows, err := s.pool.Query(ctx, `
		select m.id, m.match_date::text, to_char(m.start_time, 'HH24:MI'), to_char(m.end_time, 'HH24:MI'),
			m.venue, m.address, m.guest_total_cents, m.guest_slots,
			(select count(*) from guest_bookings g
			 where g.match_id = m.id and g.status = any($2)
			   and (g.status = 'confirmed' or g.expires_at > $1))
		from matches m
		where m.guest_sale_open and m.status = $3 and m.confirmation_deadline > $1
		order by m.match_date, m.start_time`, now, guests.ActiveStatuses, guests.MatchOpen)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sales []GuestSaleMatch
	for rows.Next() {
		var m GuestSaleMatch
		if err := rows.Scan(&m.MatchID, &m.MatchDate, &m.StartTime, &m.EndTime, &m.Venue, &m.Address,
			&m.TotalCents, &m.Slots, &m.ActiveBookings); err != nil {
			return nil, err
		}
		sales = append(sales, m)
	}
	return sales, rows.Err()
}

// ReleaseGuestBooking expira na hora uma reserva cujo Pix não pôde ser
// gerado, para não prender a vaga nem o telefone até o fim dos 30 minutos.
// Só vale para reserved: com o Pix já criado, quem decide é o pagamento ou o
// cron.
func (s *Store) ReleaseGuestBooking(ctx context.Context, id string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update guest_bookings set status = 'expired', expires_at = least(expires_at, $2)
		where id = $1 and status = 'reserved'`, id, now)
	return err
}

// UpdateGuestProviderStatus registra o último estado informado pelo Mercado
// Pago para a order da inscrição. Depois do pagamento tratado (confirmed,
// refund_pending...) não mexe: uma notificação fora de ordem não pode apagar
// o estado gravado na confirmação.
func (s *Store) UpdateGuestProviderStatus(ctx context.Context, id, orderID, paymentID, status, statusDetail string) error {
	_, err := s.pool.Exec(ctx, `
		update guest_bookings
		set provider_payment_id = $3, provider_status = $4, provider_status_detail = $5
		where id = $1 and provider_order_id = $2 and provider_order_id <> ''
		  and status in ('reserved', 'pending_payment', 'expired')`,
		id, orderID, paymentID, status, statusDetail)
	return err
}
