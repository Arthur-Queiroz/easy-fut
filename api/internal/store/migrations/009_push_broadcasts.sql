-- Avisos manuais do admin, enviados por Web Push a todos os ativos ou a usuários escolhidos.
create table push_broadcasts (
    id uuid primary key default gen_random_uuid(),
    title text not null,
    body text not null,
    audience text not null check (audience in ('all', 'selected')),
    created_by uuid references users(id) on delete set null,
    created_at timestamptz not null default now()
);

create table broadcast_push_deliveries (
    id uuid primary key default gen_random_uuid(),
    subscription_id uuid not null references push_subscriptions(id) on delete cascade,
    broadcast_id uuid not null references push_broadcasts(id) on delete cascade,
    status text not null default 'pending' check (status in ('pending', 'sent', 'failed')),
    attempts int not null default 0,
    next_attempt_at timestamptz not null default now(),
    sent_at timestamptz,
    unique(subscription_id, broadcast_id)
);
create index broadcast_push_due on broadcast_push_deliveries(next_attempt_at) where status = 'pending';
