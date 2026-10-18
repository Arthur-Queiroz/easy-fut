-- FUT-004: partidas recorrentes (templates materializados pelo cron diário).
create table if not exists recurring_matches (
  id uuid primary key default gen_random_uuid(),
  weekday smallint not null check (weekday between 0 and 6),
  start_time time not null,
  end_time time not null,
  venue text not null,
  address text not null default '',
  same_day_deadline_time time,
  created_by uuid not null references users (id),
  active boolean not null default true,
  created_at timestamptz not null default now()
);

alter table matches add column if not exists recurring_id uuid null references recurring_matches (id);

-- idempotência da materialização: uma partida por template por data.
alter table matches drop constraint if exists uq_matches_recurring_date;
create unique index if not exists uq_matches_recurring_date
  on matches (recurring_id, match_date) where recurring_id is not null;
-- ON CONFLICT exige constraint (não índice parcial); usamos exclusão do conflito por WHERE NOT EXISTS no código.

-- seed do template do grupo (domingo 11:00–12:30 Arena Futiba 1).
-- deadline de confirmação 10:30 do próprio domingo (same_day_deadline_time).
insert into settings (key, value) values ('recurring_seed_done', '0')
on conflict (key) do nothing;
