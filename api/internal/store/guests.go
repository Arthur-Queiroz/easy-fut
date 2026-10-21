package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
)

// Participação avulsa: venda de vagas por partida, inscrições e confirmação
// do pagamento. Regras em internal/guests; aqui ficam as transações e as
// guardas de concorrência.

const guestBookingColumns = `
	g.id, g.match_id, g.user_id, g.name, g.phone, g.email, g.status,
	g.base_cents, g.total_cents, g.fee_cents, g.expires_at, g.confirmed_at,
	g.access_token_hash, g.provider_order_id, g.provider_payment_id,
	g.pix_payload, g.pix_qr_code_base64, g.pix_ticket_url,
	g.provider_status, g.provider_status_detail, g.refund_reason, g.created_at`

func scanGuestBooking(row pgx.Row) (GuestBooking, error) {
	var b GuestBooking
	err := row.Scan(&b.ID, &b.MatchID, &b.UserID, &b.Name, &b.Phone, &b.Email, &b.Status,
		&b.BaseCents, &b.TotalCents, &b.FeeCents, &b.ExpiresAt, &b.ConfirmedAt,
		&b.AccessTokenHash, &b.ProviderOrderID, &b.ProviderPaymentID,
		&b.PixPayload, &b.PixQRCodeBase64, &b.PixTicketURL,
		&b.ProviderStatus, &b.ProviderStatusDetail, &b.RefundReason, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	return b, err
}

// SetGuestSale abre ou fecha a venda de vagas avulsas da partida. Ao abrir,
// congela o preço base vigente e o total com a taxa do Pix embutida; ao
// fechar, mantém os valores (inscrições já feitas têm os seus próprios).
func (s *Store) SetGuestSale(ctx context.Context, matchID string, open bool, slots int, priceCents int64, rate billing.FeeRate) error {
	var tag pgconn.CommandTag
	var err error
	if open {
		fee := rate.PassOn(priceCents)
		tag, err = s.pool.Exec(ctx, `
			update matches set guest_sale_open = true, guest_slots = $2,
				guest_price_cents = $3, guest_total_cents = $4
			where id = $1`, matchID, slots, fee.BaseCents, fee.TotalCents)
	} else {
		tag, err = s.pool.Exec(ctx, `update matches set guest_sale_open = false where id = $1`, matchID)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GuestBookingInput é o formulário da inscrição, ainda sem normalizar.
type GuestBookingInput struct {
	MatchID         string
	Name            string
	Phone           string
	Email           string
	AccessTokenHash string // hash do token de acompanhamento, gerado pelo handler
	AvatarColor     string // opcional; vazio usa a cor padrão de users
}

// CreateGuestBooking reserva uma vaga avulsa por guests.ReservationTTL. Numa
// transação, com a partida travada: expira reservas vencidas da partida,
// conta as vagas ocupadas, decide pela regra de venda, cria o avulso (sem
// e-mail e sem senha) e a inscrição com o preço congelado na partida.
func (s *Store) CreateGuestBooking(ctx context.Context, in GuestBookingInput, now time.Time) (GuestBooking, error) {
	applicant, err := guests.ParseApplicant(in.Name, in.Phone, in.Email)
	if err != nil {
		return GuestBooking{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GuestBooking{}, err
	}
	defer tx.Rollback(ctx)

	sale := guests.Sale{Now: now}
	var price, total int64
	err = tx.QueryRow(ctx, `
		select guest_sale_open, status, confirmation_deadline, guest_slots, guest_price_cents, guest_total_cents
		from matches where id = $1 for update`, in.MatchID).
		Scan(&sale.SaleOpen, &sale.MatchStatus, &sale.ConfirmationDeadline, &sale.Slots, &price, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return GuestBooking{}, ErrNotFound
	}
	if err != nil {
		return GuestBooking{}, err
	}

	// O cron expira a cada minuto; aqui a reserva vencida já libera a vaga e
	// o telefone, sem esperar por ele.
	if _, err := tx.Exec(ctx, `
		update guest_bookings set status = 'expired'
		where match_id = $1 and status in ('reserved', 'pending_payment') and expires_at <= $2`,
		in.MatchID, now); err != nil {
		return GuestBooking{}, err
	}
	// Quem já tem a vaga (talvez a última) precisa ouvir isso, não "esgotado".
	// O índice único de (partida, telefone) continua sendo a guarda final.
	var phoneBooked bool
	if err := tx.QueryRow(ctx, `
		select count(*), coalesce(bool_or(phone = $3), false)
		from guest_bookings where match_id = $1 and status = any($2)`,
		in.MatchID, guests.ActiveStatuses, applicant.Phone).Scan(&sale.ActiveBookings, &phoneBooked); err != nil {
		return GuestBooking{}, err
	}
	if phoneBooked {
		return GuestBooking{}, guests.ErrPhoneAlreadyBooked
	}
	if err := guests.CheckSaleOpen(sale); err != nil {
		return GuestBooking{}, err
	}

	var userID string
	if err := tx.QueryRow(ctx, `
		insert into users (name, phone, avatar_color, role, kind)
		values ($1, $2, coalesce(nullif($3, ''), '#3B82A0'), 'player', 'guest') returning id`,
		applicant.Name, applicant.Phone, in.AvatarColor).Scan(&userID); err != nil {
		return GuestBooking{}, err
	}

	var id string
	err = tx.QueryRow(ctx, `
		insert into guest_bookings (match_id, user_id, name, phone, email, status,
			base_cents, total_cents, fee_cents, expires_at, access_token_hash)
		values ($1, $2, $3, $4, $5, 'reserved', $6, $7, $8, $9, $10) returning id`,
		in.MatchID, userID, applicant.Name, applicant.Phone, applicant.Email,
		price, total, total-price, now.Add(guests.ReservationTTL), in.AccessTokenHash).Scan(&id)
	if isUniqueViolation(err, "guest_bookings_active_phone") {
		return GuestBooking{}, guests.ErrPhoneAlreadyBooked
	}
	if err != nil {
		return GuestBooking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GuestBooking{}, err
	}
	return s.GuestBookingByID(ctx, id)
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

func (s *Store) GuestBookingByID(ctx context.Context, id string) (GuestBooking, error) {
	return scanGuestBooking(s.pool.QueryRow(ctx,
		`select `+guestBookingColumns+` from guest_bookings g where g.id = $1`, id))
}

// GuestPix é o Pix devolvido pelo Mercado Pago para uma inscrição.
type GuestPix struct {
	OrderID      string
	PaymentID    string
	Status       string
	StatusDetail string
	Payload      string
	TicketURL    string
	QRCodeBase64 string
}

// MarkGuestBookingPendingPayment guarda o Pix e passa a reserva para
// pending_payment. Gravar de novo a mesma order é aceito (idempotência do
// Mercado Pago); outra order, ou uma reserva que já saiu do ciclo, não.
func (s *Store) MarkGuestBookingPendingPayment(ctx context.Context, id string, pix GuestPix) (GuestBooking, error) {
	tag, err := s.pool.Exec(ctx, `
		update guest_bookings
		set status = 'pending_payment', provider_order_id = $2, provider_payment_id = $3,
			provider_status = $4, provider_status_detail = $5, pix_payload = $6,
			pix_ticket_url = $7, pix_qr_code_base64 = $8
		where id = $1 and (
			(status = 'reserved' and provider_order_id in ('', $2)) or
			(status = 'pending_payment' and provider_order_id = $2))`,
		id, pix.OrderID, pix.PaymentID, pix.Status, pix.StatusDetail, pix.Payload, pix.TicketURL, pix.QRCodeBase64)
	if err != nil {
		return GuestBooking{}, err
	}
	if tag.RowsAffected() == 0 {
		return GuestBooking{}, ErrNotFound
	}
	return s.GuestBookingByID(ctx, id)
}

// Motivos gravados quando o pagamento chega e não vira vaga.
const (
	refundReasonLatePayment = "pagamento aprovado depois do fim da reserva"
	refundReasonCancelled   = "partida cancelada"
)

// ConfirmGuestPayment aplica o pagamento aprovado pelo Mercado Pago (usado
// pelo webhook). Numa transação, com a inscrição travada:
//
//   - guests.OnPaymentApproved decide entre confirmed e refund_pending;
//   - o dinheiro entrou nos dois casos, então o caixa recebe a entrada
//     (valor base, com a taxa embutida em fee_cents); um reembolso sai
//     depois como lançamento próprio;
//   - confirmed grava going na presença e agenda, uma única vez, o WhatsApp
//     de confirmação (guest_messages + fila notifications).
//
// O webhook pode chegar repetido: uma inscrição já tratada devolve
// changed = false sem mudar nada. orderID precisa ser o da inscrição.
func (s *Store) ConfirmGuestPayment(ctx context.Context, bookingID, orderID, paymentID string, now time.Time) (GuestBooking, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GuestBooking{}, false, err
	}
	defer tx.Rollback(ctx)

	b, err := scanGuestBooking(tx.QueryRow(ctx,
		`select `+guestBookingColumns+` from guest_bookings g where g.id = $1 for update`, bookingID))
	if err != nil {
		return GuestBooking{}, false, err
	}
	if b.ProviderOrderID != orderID {
		return GuestBooking{}, false, ErrNotFound
	}
	next, err := guests.OnPaymentApproved(guests.Booking{Status: b.Status, ExpiresAt: b.ExpiresAt}, now)
	if errors.Is(err, guests.ErrInvalidTransition) {
		return b, false, nil
	}
	if err != nil {
		return GuestBooking{}, false, err
	}

	reason := ""
	switch {
	case next == guests.StatusRefundPending && b.Status == guests.StatusCancelled:
		reason = refundReasonCancelled
	case next == guests.StatusRefundPending:
		reason = refundReasonLatePayment
	}
	if _, err := tx.Exec(ctx, `
		update guest_bookings
		set status = $2, provider_payment_id = $3, provider_status = 'processed',
			provider_status_detail = 'accredited', refund_reason = $4,
			confirmed_at = case when $2 = 'confirmed' then $5::timestamptz end
		where id = $1`, b.ID, next, paymentID, reason, now); err != nil {
		return GuestBooking{}, false, err
	}

	if _, err := tx.Exec(ctx, `
		insert into cash_entries (kind, amount_cents, fee_cents, booking_id, description, occurred_on)
		values ('guest_payment', $1, $2, $3, $4, ($5::timestamptz at time zone 'America/Sao_Paulo')::date)`,
		b.BaseCents, b.FeeCents, b.ID, "Inscrição avulsa: "+b.Name, now); err != nil {
		return GuestBooking{}, false, err
	}

	if next == guests.StatusConfirmed {
		if _, err := tx.Exec(ctx, `
			insert into match_confirmations (match_id, user_id, response, responded_at)
			values ($1, $2, 'going', $3)
			on conflict (match_id, user_id) do update set response = 'going', responded_at = $3`,
			b.MatchID, b.UserID, now); err != nil {
			return GuestBooking{}, false, err
		}
		if err := scheduleGuestConfirmation(ctx, tx, b, now); err != nil {
			return GuestBooking{}, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return GuestBooking{}, false, err
	}
	updated, err := s.GuestBookingByID(ctx, b.ID)
	return updated, true, err
}

// scheduleGuestConfirmation registra a intenção de enviar o WhatsApp de
// confirmação e o coloca na fila existente (notifications, enviada pelo
// job). O índice único de guest_messages garante um envio automático só.
func scheduleGuestConfirmation(ctx context.Context, tx pgx.Tx, b GuestBooking, now time.Time) error {
	var messageID string
	err := tx.QueryRow(ctx, `
		insert into guest_messages (booking_id, kind) values ($1, 'confirmation')
		on conflict (booking_id, kind) where not resend do nothing
		returning id`, b.ID).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // já agendada
	}
	if err != nil {
		return err
	}

	var date, start, venue string
	if err := tx.QueryRow(ctx, `
		select match_date::text, to_char(start_time, 'HH24:MI'), venue from matches where id = $1`,
		b.MatchID).Scan(&date, &start, &venue); err != nil {
		return err
	}
	var notificationID string
	if err := tx.QueryRow(ctx, `
		insert into notifications (user_id, phone, message, scheduled_at)
		values ($1, $2, $3, $4) returning id`,
		b.UserID, guests.WhatsAppNumber(b.Phone), guests.ConfirmationMessage(date, start, venue), now).
		Scan(&notificationID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update guest_messages set notification_id = $2 where id = $1`, messageID, notificationID)
	return err
}

// ExpireGuestBookings expira as reservas e os Pix vencidos (job a cada
// minuto) e devolve os ids das inscrições expiradas nesta execução.
func (s *Store) ExpireGuestBookings(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		update guest_bookings set status = 'expired'
		where status in ('reserved', 'pending_payment') and expires_at <= $1
		returning id`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectIDs(rows)
}

// settleGuestBookingsOnCancel aplica guests.OnMatchCancelled às inscrições da
// partida cancelada, dentro da transação do cancelamento.
func settleGuestBookingsOnCancel(ctx context.Context, tx pgx.Tx, matchID string) error {
	rows, err := tx.Query(ctx, `
		select id, status from guest_bookings where match_id = $1 order by id for update`, matchID)
	if err != nil {
		return err
	}
	type change struct{ id, status string }
	var changes []change
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			rows.Close()
			return err
		}
		if next, changed := guests.OnMatchCancelled(status); changed {
			changes = append(changes, change{id, next})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range changes {
		reason := ""
		if c.status == guests.StatusRefundPending {
			reason = refundReasonCancelled
		}
		if _, err := tx.Exec(ctx, `update guest_bookings set status = $2, refund_reason = $3 where id = $1`,
			c.id, c.status, reason); err != nil {
			return err
		}
	}
	return nil
}
