package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"futdarapaziada/api/internal/guests"
)

// Participação avulsa — voto por link e mensagens (fase C). O token do link
// só existe na mensagem de WhatsApp; o banco guarda o hash.

// GuestVoter é o avulso identificado pelo token do link de voto.
type GuestVoter struct {
	BookingID string
	MatchID   string
	UserID    string
	ExpiresAt time.Time
}

// GuestVoterByToken encontra o avulso dono do link. Token desconhecido,
// revogado (ou girado por um reenvio) ou de inscrição que deixou de estar
// confirmada devolve ErrNotFound. O prazo fica para quem chama decidir
// (guests.VoteLinkOpen), para separar "link inválido" de "votação fechada".
func (s *Store) GuestVoterByToken(ctx context.Context, tokenHash string) (GuestVoter, error) {
	var v GuestVoter
	err := s.pool.QueryRow(ctx, `
		select g.id, g.match_id, g.user_id, t.expires_at
		from guest_vote_tokens t
		join guest_bookings g on g.id = t.booking_id
		where t.token_hash = $1 and t.revoked_at is null and g.status = 'confirmed'`, tokenHash).
		Scan(&v.BookingID, &v.MatchID, &v.UserID, &v.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// voteLinkMatch são os dados da partida que entram no link e na mensagem.
type voteLinkMatch struct {
	status         string
	date           string // YYYY-MM-DD
	startTime      string // HH:MM
	venue          string
	votingClosesAt *time.Time
}

func loadVoteLinkMatch(ctx context.Context, tx pgx.Tx, matchID string) (voteLinkMatch, error) {
	var m voteLinkMatch
	err := tx.QueryRow(ctx, `
		select status, match_date::text, to_char(start_time, 'HH24:MI'), venue, voting_closes_at
		from matches where id = $1`, matchID).
		Scan(&m.status, &m.date, &m.startTime, &m.venue, &m.votingClosesAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// ScheduleGuestVoteLinks agenda o link de voto de cada avulso confirmado da
// partida que ainda não recebeu nenhum (automático ou reenviado). Chamado
// quando o admin abre a votação; a varredura do job cobre a abertura
// automática e qualquer falha daqui. Partida fora da votação, ou com o
// prazo vencido, não agenda nada. Devolve quantos links foram agendados.
func (s *Store) ScheduleGuestVoteLinks(ctx context.Context, matchID, appURL string, now time.Time) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	m, err := loadVoteLinkMatch(ctx, tx, matchID)
	if err != nil {
		return 0, err
	}
	if m.votingClosesAt == nil || !guests.VoteLinkOpen(m.status, *m.votingClosesAt, now) {
		return 0, nil
	}

	// O lock nas inscrições serializa com o reenvio do admin.
	rows, err := tx.Query(ctx, `
		select g.id, g.user_id, g.phone from guest_bookings g
		where g.match_id = $1 and g.status = 'confirmed'
		  and not exists (select 1 from guest_messages gm
		                  where gm.booking_id = g.id and gm.kind = 'vote_link')
		order by g.id
		for update of g`, matchID)
	if err != nil {
		return 0, err
	}
	var pending []GuestBooking
	for rows.Next() {
		var b GuestBooking
		if err := rows.Scan(&b.ID, &b.UserID, &b.Phone); err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	scheduled := 0
	for _, b := range pending {
		ok, err := scheduleGuestVoteLink(ctx, tx, b, m, appURL, false, now)
		if err != nil {
			return 0, err
		}
		if ok {
			scheduled++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return scheduled, nil
}

// ScheduleMissingGuestVoteLinks é a varredura do job: agenda os links que
// faltam em toda partida em votação. Cobre a abertura automática e os
// envios que falharam na abertura pelo admin.
func (s *Store) ScheduleMissingGuestVoteLinks(ctx context.Context, appURL string, now time.Time) (int, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct m.id from matches m
		join guest_bookings g on g.match_id = m.id and g.status = 'confirmed'
		where m.status = 'voting' and m.voting_closes_at > $1
		  and not exists (select 1 from guest_messages gm
		                  where gm.booking_id = g.id and gm.kind = 'vote_link')`, now)
	if err != nil {
		return 0, err
	}
	ids, err := collectIDs(rows)
	rows.Close()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, id := range ids {
		n, err := s.ScheduleGuestVoteLinks(ctx, id, appURL, now)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// scheduleGuestVoteLink gera um token novo (o anterior, se houver, deixa de
// valer), guarda o hash com o prazo da votação, põe o WhatsApp na fila e
// registra em guest_messages. O envio automático é único por inscrição
// (índice guest_messages_automatic): se já existir, devolve false sem gerar
// token.
func scheduleGuestVoteLink(ctx context.Context, tx pgx.Tx, b GuestBooking, m voteLinkMatch, appURL string, resend bool, now time.Time) (bool, error) {
	var messageID string
	err := tx.QueryRow(ctx, `
		insert into guest_messages (booking_id, kind, resend) values ($1, 'vote_link', $2)
		on conflict (booking_id, kind) where not resend do nothing
		returning id`, b.ID, resend).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	token, hash, err := guests.NewVoteToken()
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		insert into guest_vote_tokens (booking_id, token_hash, expires_at) values ($1, $2, $3)
		on conflict (booking_id) do update
		set token_hash = excluded.token_hash, expires_at = excluded.expires_at, revoked_at = null`,
		b.ID, hash, *m.votingClosesAt); err != nil {
		return false, err
	}
	message := guests.VoteLinkMessage(m.date, *m.votingClosesAt, guests.VoteLinkURL(appURL, token))
	if err := queueGuestMessage(ctx, tx, messageID, b, message, now); err != nil {
		return false, err
	}
	return true, nil
}

// queueGuestMessage põe o WhatsApp na fila existente (notifications, enviada
// pelo job) e liga a fila ao registro em guest_messages.
func queueGuestMessage(ctx context.Context, tx pgx.Tx, messageID string, b GuestBooking, message string, now time.Time) error {
	var notificationID string
	if err := tx.QueryRow(ctx, `
		insert into notifications (user_id, phone, message, scheduled_at)
		values ($1, $2, $3, $4) returning id`,
		b.UserID, guests.WhatsAppNumber(b.Phone), message, now).Scan(&notificationID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `update guest_messages set notification_id = $2 where id = $1`, messageID, notificationID)
	return err
}

// ResendGuestMessage é o reenvio do admin (guests.Resend decide se vale).
// Grava guest_messages com resend = true. O link de voto reenviado gira o
// token: o antigo para de funcionar.
func (s *Store) ResendGuestMessage(ctx context.Context, bookingID, kind, appURL string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var b GuestBooking
	err = tx.QueryRow(ctx, `
		select id, match_id, user_id, phone, status from guest_bookings where id = $1 for update`, bookingID).
		Scan(&b.ID, &b.MatchID, &b.UserID, &b.Phone, &b.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	m, err := loadVoteLinkMatch(ctx, tx, b.MatchID)
	if err != nil {
		return err
	}
	req := guests.Resend{Kind: kind, BookingStatus: b.Status, MatchStatus: m.status, VotingClosesAt: m.votingClosesAt, Now: now}
	if err := req.Validate(); err != nil {
		return err
	}

	switch kind {
	case guests.MessageVoteLink:
		if _, err := scheduleGuestVoteLink(ctx, tx, b, m, appURL, true, now); err != nil {
			return err
		}
	case guests.MessageConfirmation:
		var messageID string
		if err := tx.QueryRow(ctx, `
			insert into guest_messages (booking_id, kind, resend) values ($1, 'confirmation', true)
			returning id`, b.ID).Scan(&messageID); err != nil {
			return err
		}
		message := guests.ConfirmationMessage(m.date, m.startTime, m.venue)
		if err := queueGuestMessage(ctx, tx, messageID, b, message, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
