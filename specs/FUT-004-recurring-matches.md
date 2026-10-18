# FUT-004 — Partidas recorrentes (criação automática)

**Status:** implemented (PR fut/004-recurring-matches)
**Tipo:** vertical (API + jobs + PWA admin)
**Origem:** as partidas são sempre no mesmo lugar/hora (evidência: últimas 6 aos domingos, 11:00–12:30, Arena Futiba 1) mas hoje precisam ser criadas manualmente — risco de esquecer. Vamos adicionar criação automática via template recorrente, **sem remover** o fluxo manual (os dois coexistem).

## Decisões (com Arthur, 2026-10-18)

- Confirmações seguem o fluxo atual; votação já é automática pelo FUT-001 (abre no fim da partida, delay configurável por setting).
- Antecedência: **4 partidas à frente** (ex.: domingo de manhã já mostra os 4 próximos domingos). Motivo: suporta feature futura a decidir.
- Cancelamento pontual de uma partida não pausa o template.
- **Deadline default de confirmação = 10:30 do próprio dia da partida** (a partida é 11:00) — definido por Arthur. O deadline é configurável por template (`same_day_deadline_time`); partidas geradas com deadline no passado por atraso de materialização são fechadas no tick seguinte do `CloseExpiredConfirmations`.

## Modelo de dados

```sql
create table recurring_matches (
  id uuid pk default gen_random_uuid(),
  weekday smallint not null check (weekday between 0 and 6), -- 0 = domingo
  start_time time not null,
  end_time time not null,
  venue text not null,
  address text not null default '',
  same_day_deadline_time time, -- null = sem deadline automático; senão fecha confirmações no próprio dia da partida à hora indicada (default 10:30)
  created_by uuid not null references users(id),
  active boolean not null default true,
  created_at timestamptz not null default now()
);

-- idempotência da materialização
alter table matches add column recurring_id uuid null references recurring_matches(id);
create unique index uq_matches_recurring_date on matches (recurring_id, match_date) where recurring_id is not null;
```

## Regras

- **R7 — Janela de times:** do fechamento das confirmações (10:30 default) até o início (11:00), o admin sorteia ou ajusta os times manualmente (mecânica existente, com presets da migration 007 e re-sorteio ilimitado). Sorteio é sempre manual — decisão do Arthur: nada de sorteio automático.
- **R8 — Push pro admin:** quando uma partida entre em `closed`, o sistema envia push para os admins: "Confirmações encerradas — sorteie os times da partida de {dia}". Reusa o pipeline de push existente (migrations 008/009). Isso vale para qualquer partida (recorrente ou não).

- **R1 — Materialização:** job diário (cron `15 5 * * *`, 05:15 SP) materializa, para cada template ativo, as partidas **das próximas 4 ocorrências do weekday**, se ainda não existem (idempotente pelo índice único `recurring_id + match_date`).
- **R2 — Partida gerada:** `status='open'`, `confirmation_deadline` = `match_date + same_day_deadline_time` (TZ America/Sao_Paulo, mesma conversão do FUT-001; ex.: domingo 10:30). Template sem deadline → confirmações fecham manualmente.
- **R3 — Exceção:** cancelar/excluir a partida gerada é normal (admin no painel); o template fica ativo e a próxima ocorrência nasce no tick seguinte — para de gerar apenas com o template inativo/excluído.
- **R4 — Edição:** admin pode editar a partida gerada (mudar o dia/hora/local daquela ocorrência) sem afetar o template — a partida deixa de ser regenerada (id existe).
- **R5 — Latência aceitável:** partidas futuras ordenam normalmente; `NextMatch` e histórico não mudam.
- **R6 — Seed inicial:** migration controlada por setting `recurring_seed_done`: cria o template domingo 11:00–12:30 Arena Futiba 1 / Alameda Araguaia 580, deadline 10:30 do próprio domingo, criado pelo admin existente mais antigo.

## Contrato de API

- `GET /api/admin/recurring` — lista templates (com próximas ocorrências geradas)
- `POST /api/admin/recurring` — cria template `{weekday, start_time, end_time, venue, address, same_day_deadline_time?}`
- `PATCH /api/admin/recurring/{id}` — edita template (materializa ocorrências que faltam)
- `DELETE /api/admin/recurring/{id}` — exclui template (partidas geradas ficam)
- PWA: aba admin "Recorrências" — CRUD simples no mesmo padrão de Invites/Charges

## Testes / aceite

- Store: materialização idempotente (cria 4, não duplica em tick seguinte), respeita template inativo, deadline herdado correto (SP → UTC), edição de partida gerada não é sobrescrita.
- Handler: CRUD smoke; auth admin-required.
- Push: fechamento de partida dispara 1 push(admin) — sem duplicar em re-tick.
- E2E: banco de teste vazio + seed → 4 partidas futuras criadas.
