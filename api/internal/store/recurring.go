package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RecurringMatch struct {
	ID                  string     `json:"id"`
	Weekday             int        `json:"weekday"`    // 0 = domingo
	StartTime           string     `json:"start_time"` // HH:MM
	EndTime             string     `json:"end_time"`   // HH:MM
	Venue               string     `json:"venue"`
	Address             string     `json:"address"`
	SameDayDeadlineTime *string    `json:"same_day_deadline_time,omitempty"` // HH:MM
	CreatedBy           string     `json:"created_by"`
	CreatedByName       string     `json:"created_by_name,omitempty"`
	Active              bool       `json:"active"`
	CreatedAt           *time.Time `json:"created_at,omitempty"`
}

type CreateRecurringInput struct {
	Weekday             int    `json:"weekday"`
	StartTime           string `json:"start_time"`
	EndTime             string `json:"end_time"`
	Venue               string `json:"venue"`
	Address             string `json:"address"`
	SameDayDeadlineTime string `json:"same_day_deadline_time"` // "" = sem deadline
}

func (s *Store) CreateRecurringMatch(ctx context.Context, in CreateRecurringInput, createdBy string) (RecurringMatch, error) {
	var deadline any
	if in.SameDayDeadlineTime != "" {
		deadline = in.SameDayDeadlineTime
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into recurring_matches (weekday, start_time, end_time, venue, address, same_day_deadline_time, created_by)
		values ($1, $2, $3, $4, $5, $6, $7) returning id`,
		in.Weekday, in.StartTime, in.EndTime, in.Venue, in.Address, deadline, createdBy).Scan(&id)
	if err != nil {
		return RecurringMatch{}, err
	}
	return s.RecurringByID(ctx, id)
}

func (s *Store) RecurringByID(ctx context.Context, id string) (RecurringMatch, error) {
	var r RecurringMatch
	var deadline string
	err := s.pool.QueryRow(ctx, `
		select r.id, r.weekday, to_char(r.start_time,'HH24:MI'), to_char(r.end_time,'HH24:MI'),
			r.venue, r.address, coalesce(to_char(r.same_day_deadline_time,'HH24:MI'),''),
			r.created_by, coalesce(u.name,''), r.active, r.created_at
		from recurring_matches r left join users u on u.id = r.created_by
		where r.id = $1`, id).Scan(
		&r.ID, &r.Weekday, &r.StartTime, &r.EndTime, &r.Venue, &r.Address,
		&deadline, &r.CreatedBy, &r.CreatedByName, &r.Active, &r.CreatedAt)
	if err != nil {
		return RecurringMatch{}, err
	}
	if deadline != "" {
		d := deadline
		r.SameDayDeadlineTime = &d
	}
	return r, nil
}

func (s *Store) ListRecurringMatches(ctx context.Context, onlyActive bool) ([]RecurringMatch, error) {
	q := `
		select r.id, r.weekday, to_char(r.start_time,'HH24:MI'), to_char(r.end_time,'HH24:MI'),
			r.venue, r.address, coalesce(to_char(r.same_day_deadline_time,'HH24:MI'),''),
			r.created_by, coalesce(u.name,''), r.active, r.created_at
		from recurring_matches r left join users u on u.id = r.created_by`
	if onlyActive {
		q += ` where r.active`
	}
	q += ` order by r.weekday, r.start_time`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecurringMatch
	for rows.Next() {
		var r RecurringMatch
		var deadline string
		if err := rows.Scan(&r.ID, &r.Weekday, &r.StartTime, &r.EndTime, &r.Venue, &r.Address,
			&deadline, &r.CreatedBy, &r.CreatedByName, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		if deadline != "" {
			d := deadline
			r.SameDayDeadlineTime = &d
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRecurringMatch(ctx context.Context, id string, in CreateRecurringInput) error {
	var deadline any
	if in.SameDayDeadlineTime != "" {
		deadline = in.SameDayDeadlineTime
	}
	tag, err := s.pool.Exec(ctx, `
		update recurring_matches set weekday=$2, start_time=$3, end_time=$4, venue=$5, address=$6, same_day_deadline_time=$7
		where id=$1`, id, in.Weekday, in.StartTime, in.EndTime, in.Venue, in.Address, deadline)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ActivateRecurringMatch liga/desliga o template (toggle).
func (s *Store) ActivateRecurringMatch(ctx context.Context, id string, active bool) error {
	tag, err := s.pool.Exec(ctx, `update recurring_matches set active=$2 where id=$1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRecurringMatch exclui o template; partidas já geradas ficam (recurring_id vira pendente de SET NULL? sem FK cascade — limpa manualmente).
func (s *Store) DeleteRecurringMatch(ctx context.Context, id string) error {
	// as partidas geradas continuam existindo, mas perdem a referência:
	if _, err := s.pool.Exec(ctx, `update matches set recurring_id = null where recurring_id = $1`, id); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `delete from recurring_matches where id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MaterializeRecurringMatches cria as próximas `ahead` ocorrências de cada
// template ativo ainda não materializadas. Idempotente (uq recurring+date).
// Dia e horário do template são de São Paulo; a sessão do PostgreSQL está em
// UTC, então toda conversão de data/hora diz o fuso explicitamente.
//
// Na mesma transação, os usuários ativos ganham a confirmação no_response em
// toda partida recorrente aberta, como em CreateMatch: é dela que saem o
// lembrete por push e a lista "sem resposta". Rodando todo dia, isso também
// alcança quem virou ativo depois que a partida foi gerada. O push de
// "partida criada" não é enfileirado: a partida nasce semanas antes do jogo.
func (s *Store) MaterializeRecurringMatches(ctx context.Context, ahead int) (int, error) {
	if ahead < 1 {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// Janela de amanhã até hoje + `ahead` semanas: 7×ahead dias, então cada
	// dia da semana aparece exatamente `ahead` vezes.
	tag, err := tx.Exec(ctx, `
		with today as (
			select (now() at time zone 'America/Sao_Paulo')::date as d
		), next_dates as (
			select r.id as recurring_id, g.d::date as match_date
			from recurring_matches r, today,
			lateral generate_series(today.d + 1, today.d + 7 * $1::int, interval '1 day') g(d)
			where r.active and extract(dow from g.d) = r.weekday
		)
		insert into matches (match_date, start_time, end_time, venue, address,
			confirmation_deadline, notes, created_by, recurring_id)
		select n.match_date, r.start_time, r.end_time, r.venue, r.address,
			-- sem prazo no template: 30 min antes do início
			case
				when r.same_day_deadline_time is not null
					then (n.match_date + r.same_day_deadline_time) at time zone 'America/Sao_Paulo'
				else ((n.match_date + r.start_time) at time zone 'America/Sao_Paulo') - interval '30 minutes'
			end,
			'Partida recorrente', r.created_by, n.recurring_id
		from next_dates n
		join recurring_matches r on r.id = n.recurring_id
		where not exists (
			select 1 from matches m
			where m.recurring_id = n.recurring_id and m.match_date = n.match_date
		)`, ahead)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		insert into match_confirmations (match_id, user_id)
		select m.id, u.id
		from matches m join users u on u.status = 'active'
		where m.recurring_id is not null and m.status = 'open'
		on conflict (match_id, user_id) do nothing`); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// RecurringSeed cria o template do grupo, uma única vez (setting recurring_seed_done).
// Smart: se o template já existe (dia-hora-local idênticos), não duplica.
func (s *Store) RecurringSeed(ctx context.Context) error {
	var done string
	if err := s.pool.QueryRow(ctx, `select value from settings where key='recurring_seed_done'`).Scan(&done); err == nil && done == "1" {
		return nil
	}
	if err := s.pool.QueryRow(ctx, `select value from settings where key='recurring_seed_done'`).Scan(&done); err == nil && done == "1" {
		return nil
	}
	// admin mais antigo
	var adminID string
	err := s.pool.QueryRow(ctx, `
		select id from users where role='admin' order by created_at limit 1`).Scan(&adminID)
	if err != nil {
		return err
	}
	// não duplica se já existe template idêntico
	var existing uuid.UUID
	err = s.pool.QueryRow(ctx, `
		select id from recurring_matches where weekday=0 and start_time='11:00' and venue=$1 and active`, "Arena Futiba 1").Scan(&existing)
	if err == nil {
		s.setRecurringSeedDone(ctx)
		return nil
	}
	_, err = s.CreateRecurringMatch(ctx, CreateRecurringInput{
		Weekday:             0,
		StartTime:           "11:00",
		EndTime:             "12:30",
		Venue:               "Arena Futiba 1",
		Address:             "Alameda Araguaia 580",
		SameDayDeadlineTime: "10:30",
	}, adminID)
	if err != nil {
		return err
	}
	s.setRecurringSeedDone(ctx)
	return nil
}

func (s *Store) setRecurringSeedDone(ctx context.Context) {
	_, _ = s.pool.Exec(ctx, `
		insert into settings (key, value) values ('recurring_seed_done','1')
		on conflict (key) do update set value='1'`)
}
