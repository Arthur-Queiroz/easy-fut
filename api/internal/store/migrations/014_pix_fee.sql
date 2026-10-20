-- Repasse da taxa do Pix ao pagador (.agents/specs/design/2026-10-20-meios-de-pagamento-e-taxas.md).
-- A taxa é congelada no lote e em cada cobrança, como o rateio; lotes e
-- cobranças já existentes ficam com 0 e continuam cobrando só o rateio.
alter table charge_batches
    add column if not exists pix_fee_bps integer not null default 0,
    add column if not exists pix_fee_cents bigint not null default 0;

alter table charges
    add column if not exists pix_fee_cents bigint not null default 0;

-- 0,99%: tarifa do Pix da conta, conferida em extrato. O admin altera em
-- /admin/configuracoes; 0 desliga o repasse.
insert into settings (key, value) values ('pix_fee_bps', '99')
on conflict (key) do nothing;
