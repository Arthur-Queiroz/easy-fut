-- Lembretes manuais por partida usam a fila de WhatsApp existente.
alter table notifications
    add column match_id uuid references matches (id),
    add column reminder_kind text check (reminder_kind in ('confirmation', 'vote')),
    add constraint match_reminder_fields_together check (
        (match_id is null and reminder_kind is null) or
        (match_id is not null and reminder_kind is not null)
    );

create unique index idx_notifications_match_reminder
    on notifications (match_id, user_id, reminder_kind)
    where match_id is not null;
