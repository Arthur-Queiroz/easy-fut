# FUT-002 — Votação acessível pelo jogador na aba Partida

**Status:** implemented (PR fut/002-voting-in-match-tab)
**Tipo:** vertical (API + PWA)
**Origem:** após FUT-001 abrir a votação automaticamente, a home do jogador continua mostrando "Nenhuma partida agendada no momento" e a aba Partida não exibe votação alguma — a votação ficou acessível só via painel admin/deep link. Causa: `NextMatch` filtra `status in ('open','closed','teams_drawn')`, excluindo `voting`.

## Regras

- **R1 — Partida em votação é "a partida atual":** `NextMatch` passa a incluir `voting` (ordenada por data, junto das demais). Quando a partida mais próxima está em `voting`, a aba Partida mostra a votação (mesma UI de `MatchDetailContent`: artilheiro + perna de pau, 2 votos por categoria, desfazer).
- **R2 — Sem partida agendada mas com votação em aberto:** se não há partida futura (`open/closed/teams_drawn` hoje ou depois) mas existe partida em `voting`, é ela que aparece ("votação em andamento" em vez de "nenhuma partida").
- **R3 — Painel admin inalterado:** admin segue com as ações de fechar votação/reabrir confirmações no admin.
- **R4 — Backend:** `GET /api/matches/next` retorna a partida em `voting` com os campos que o MatchDetail precisa via `GET /api/matches/{id}` — o MatchView passa a buscar o detail completo quando `status === 'voting'` e renderizar `MatchDetailContent`, reaproveitando tudo (confirmations, teams, media, my_votes, results, is_participant).

## Contrato

- Nenhuma rota/contrato novo. Uma linha no SQL de `NextMatch`.
- PWA: `MatchView.vue` troca de render: match em `voting` → `MatchDetailContent` (load via `/matches/{id}`); senão comportamento atual.

## Testes / aceite

- API: teste de `NextMatch` (ou handler) com partida em `voting` → retornada; sem partidas elegíveis → `match: null`.
- PWA: `npm run build` (vue-tsc) limpo; screenshot/verificação manual: aba Partida exibindo votação quando em `voting`.
- E2E mental do fluxo: partida 18/10 em votação → aparece na aba Partida do jogador → voto registrado.
