-- Participação avulsa (.agents/specs/design/2026-10-21-participacao-avulsa-plano.md).
-- Aditiva: membros existentes não mudam e a API anterior continua funcionando.

-- Identidade: o avulso fica em users para reaproveitar as chaves de presença,
-- times e votos, mas com kind = 'guest', sem e-mail, sem senha e nunca admin.
alter table users
    add column if not exists kind text not null default 'member';
alter table users
    add constraint users_kind_check check (kind in ('member', 'guest'));
alter table users alter column email drop not null;
alter table users alter column password_hash drop not null;
alter table users
    add constraint users_member_credentials
        check (kind = 'guest' or (email is not null and password_hash is not null));
alter table users
    add constraint users_guest_not_admin check (kind = 'member' or role = 'player');

-- Venda de vagas por partida. Preço e total (com a taxa do Pix embutida) são
-- congelados quando o admin abre a venda.
alter table matches
    add column if not exists guest_sale_open boolean not null default false,
    add column if not exists guest_slots int not null default 0,
    add column if not exists guest_price_cents bigint not null default 0,
    add column if not exists guest_total_cents bigint not null default 0;

-- Cada compra é uma inscrição independente, com o próprio avulso em users.
-- O e-mail fica aqui (users.email é único e o avulso o repete entre compras).
create table guest_bookings (
    id                     uuid primary key default gen_random_uuid(),
    match_id               uuid not null references matches (id),
    user_id                uuid not null references users (id),
    name                   text not null,
    phone                  text not null, -- E.164
    email                  text not null,
    status                 text not null default 'reserved'
        check (status in ('reserved', 'pending_payment', 'confirmed', 'expired',
                          'cancelled', 'refund_pending', 'refunded')),
    base_cents             bigint not null,
    total_cents            bigint not null,
    fee_cents              bigint not null,
    expires_at             timestamptz not null,
    confirmed_at           timestamptz,
    access_token_hash      text not null default '',
    provider_order_id      text not null default '',
    provider_payment_id    text not null default '',
    pix_payload            text not null default '',
    pix_qr_code_base64     text not null default '',
    pix_ticket_url         text not null default '',
    provider_status        text not null default '',
    provider_status_detail text not null default '',
    refund_reason          text not null default '',
    created_at             timestamptz not null default now()
);

-- Uma vaga por WhatsApp por partida; depois de expirar, o número tenta de novo.
create unique index guest_bookings_active_phone
    on guest_bookings (match_id, phone)
    where status in ('reserved', 'pending_payment', 'confirmed');
create unique index guest_bookings_provider_order
    on guest_bookings (provider_order_id)
    where provider_order_id <> '';
create index guest_bookings_match on guest_bookings (match_id, status);
create index guest_bookings_due on guest_bookings (expires_at)
    where status in ('reserved', 'pending_payment');

-- Link privado de voto: só o hash do token é guardado.
create table guest_vote_tokens (
    booking_id uuid primary key references guest_bookings (id),
    token_hash text not null unique,
    expires_at timestamptz not null,
    revoked_at timestamptz
);

-- Mensagens de WhatsApp por inscrição. O envio automático acontece uma vez
-- por tipo; o reenvio do admin grava outra linha com resend = true.
create table guest_messages (
    id              uuid primary key default gen_random_uuid(),
    booking_id      uuid not null references guest_bookings (id),
    kind            text not null check (kind in ('confirmation', 'vote_link')),
    notification_id uuid references notifications (id),
    resend          boolean not null default false,
    created_at      timestamptz not null default now()
);
create unique index guest_messages_automatic
    on guest_messages (booking_id, kind)
    where not resend;

-- Caixa do grupo, separado das mensalidades. O valor é sempre positivo; o
-- sinal vem do tipo. Entrada e estorno pertencem a uma inscrição (uma de
-- cada, no máximo); despesa não.
create table cash_entries (
    id           uuid primary key default gen_random_uuid(),
    kind         text not null check (kind in ('guest_payment', 'refund', 'expense')),
    amount_cents bigint not null check (amount_cents > 0),
    fee_cents    bigint not null default 0 check (fee_cents >= 0),
    booking_id   uuid references guest_bookings (id),
    description  text not null default '',
    occurred_on  date not null,
    created_by   uuid references users (id),
    created_at   timestamptz not null default now(),
    check ((kind = 'expense') = (booking_id is null))
);
create unique index cash_entries_booking_kind
    on cash_entries (booking_id, kind)
    where booking_id is not null;
create index cash_entries_occurred on cash_entries (occurred_on desc, created_at desc);

insert into settings (key, value) values ('guest_price_cents', '2500')
on conflict (key) do nothing;
