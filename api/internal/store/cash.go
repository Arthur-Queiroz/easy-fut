package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"futdarapaziada/api/internal/guests"
)

// Caixa do grupo: entradas das inscrições avulsas, estornos e despesas
// lançadas pelo admin. Não se mistura com as mensalidades.

const cashColumns = `
	e.id, e.kind, e.amount_cents, e.fee_cents, e.booking_id, e.description,
	e.occurred_on::text, e.created_by, coalesce(u.name, ''), e.created_at`

const cashJoins = `
	from cash_entries e left join users u on u.id = e.created_by`

func scanCashEntry(row pgx.Row) (CashEntry, error) {
	var e CashEntry
	err := row.Scan(&e.ID, &e.Kind, &e.AmountCents, &e.FeeCents, &e.BookingID, &e.Description,
		&e.OccurredOn, &e.CreatedBy, &e.CreatedByName, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// AddCashExpense lança uma despesa do grupo. occurredOn é YYYY-MM-DD,
// validado pelo handler.
func (s *Store) AddCashExpense(ctx context.Context, description string, amountCents int64, occurredOn, createdBy string) (CashEntry, error) {
	description, err := guests.CheckExpense(description, amountCents)
	if err != nil {
		return CashEntry{}, err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		insert into cash_entries (kind, amount_cents, description, occurred_on, created_by)
		values ('expense', $1, $2, $3::date, $4) returning id`,
		amountCents, description, occurredOn, createdBy).Scan(&id); err != nil {
		return CashEntry{}, err
	}
	return scanCashEntry(s.pool.QueryRow(ctx, `select `+cashColumns+cashJoins+` where e.id = $1`, id))
}

// DeleteCashExpense remove uma despesa lançada por engano. Entradas e
// estornos de inscrições não são removíveis.
func (s *Store) DeleteCashExpense(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from cash_entries where id = $1 and kind = 'expense'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CashReport devolve o saldo do caixa inteiro e os lançamentos do período
// (from e to opcionais, YYYY-MM-DD, inclusivos), mais recentes primeiro.
func (s *Store) CashReport(ctx context.Context, from, to string) (CashReport, error) {
	var report CashReport
	if err := s.pool.QueryRow(ctx, `
		select coalesce(sum(case kind when $1 then amount_cents else -amount_cents end), 0)
		from cash_entries`, guests.CashGuestPayment).Scan(&report.BalanceCents); err != nil {
		return report, err
	}

	rows, err := s.pool.Query(ctx, `select `+cashColumns+cashJoins+`
		where e.occurred_on >= coalesce(nullif($1, '')::date, '-infinity'::date)
		  and e.occurred_on <= coalesce(nullif($2, '')::date, 'infinity'::date)
		order by e.occurred_on desc, e.created_at desc`, from, to)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanCashEntry(rows)
		if err != nil {
			return report, err
		}
		report.Entries = append(report.Entries, e)
	}
	return report, rows.Err()
}
