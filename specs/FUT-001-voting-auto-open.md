# FUT-001 — Abertura automática da votação

**Status:** draft → implemented (ao fim do PR)
**Tipo:** vertical (API + jobs; sem mudança de contrato de front)
**Origem:** reclamações de usuários que "não conseguiam votar". Investigação em produção (2026-10-18) confirmou: partidas encerradas ficam em `closed`/`teams_drawn` indefinidamente — a votação só abre por clique manual do admin em `POST /matches/{id}/finish`. Partida de 04/10: 8 confirmados, 0 votos; a partida de 18/10 ainda nem abriu. Quem joga não vota porque **a votação nunca é aberta**.

## Problema

1. `handleFinishMatch` é o único caminho que move a partida para `status = 'voting'`, e a rota é admin-only.
2. `CloseExpiredVoting` (cron a cada 10 min) só finaliza votações já abertas — nunca abre uma.
3. Resultado: sem ação do admin, a tela de votação nunca aparece; os usuários chegam na partida e não há onde votar.

## Regras

- **R1 — Abertura automática:** partida com `status in ('closed','teams_drawn')` cujo horário de fim (`match_date` + `end_time`) passou há pelo menos `voting_open_delay_min` (setting numérico, default: `0` — no fim do jogo) entra em `voting`, com `voting_closes_at = fim da partida + voting_close_days` (setting existente, default 2).
- **R2 — Idempotente/estado:** o job não toca partidas em `open`, `voting`, `finished` nem `cancelled`. Executa no mesmo cron das demais tarefas (a cada 10 min, junto de `closeExpiredVoting`).
- **R3 — Fuso:** `end_time` é `time` puro sem fuso; a partida acontece em America/Sao_Paulo. A conversão usa a TZ configurada no processo (`TZ`), e a comparação `fim da partida <= now() - delay` acontece em SQL com a mesma TZ da sessão (`timezone('America/Sao_Paulo', ...)`), evitando drift de horário de verão — data+hora naive → timestamptz na TZ de SP.
- **R4 — Sem admin:** administrador continua podendo encerrar antecipadamente (`POST /matches/{id}/finish`); se já estiver em `voting`, nada muda. `reopen-confirmations` não interage com a votação.
- **R5 — Mascote de auditoria:** quando a votação abre automaticamente, grava activity log `voting_opened` ("Votação aberta automaticamente — partida do dia MM/DD").

## Contrato de API / banco

- Nenhuma rota nova; nenhum contrato de request/response muda.
- Não há migration (nenhuma coluna/constraint nova). `settings` ganha linha `voting_open_delay_min = '0'` via SQL idempotente no migration utility, para ficar visível ao painel admin (`GET/PUT /api/admin/settings` — já lista settings dinamicamente).
- Novo método em `Store`: `OpenExpiredVoting(ctx) ([]string, error)` — mesma assinatura/estilo de `CloseExpiredVoting`.

## Testes / aceite

- Teste no pacote `store` cobrindo `OpenExpiredVoting`: partida em `closed` com fim no passado → vira `voting` com `voting_closes_at` correto; partida em `teams_drawn` idem; partida em `open`/`voting`/`finished` → intocada; partida ainda dentro do delay → intocada.
- Teste no pacote `jobs` para o runner (abre+fecha na mesma execução não duplica).
- `go vet ./... && go test ./...` verde antes do commit (gofmt limpo).
- Smoke local: seed mínimo de partida no passado, esperar/forçar tick e confirmar `status='voting'` + `voting_closes_at` preenchido.
