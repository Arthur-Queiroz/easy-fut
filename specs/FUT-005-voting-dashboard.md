# FUT-005 — Votação ampliada: categoria "melhor gol" + dashboard de votos

**Status:** implemented (PR fut/005-voting-dashboard)
**Tipo:** vertical (API + PWA admin + PWA app)

**Origem:** dois pedidos do Arthur em sequência — (a) terceira categoria de voto "melhor gol" além de artilheiro e perna de pau; (b) dashboard de votação no admin: ver votos por partida, pesquisar por usuário, comparar partidas.

## Parte A — Categoria "best_goal" (melhor gol)

### Banco / API

1. **Migration:** sem constraint enum na tabela `votes` (categoria é `text` Livreetrada hoje pelos handlers); NADA a migrar no banco. Categoria é validada em código.
2. **Handlers:** aceitar `best_goal` em `handleVote`/`handleRemoveVote`/`logVoteWinners` — troca a comparação binária por `slices.Contains([]string{"top_scorer","worst_player","best_goal"}, category)`.
3. **Regras por categoria:** `best_goal` herda as regras de `top_scorer` — pode votar em si mesmo (foi você que fez o gol), até **2 votos** em jogadores distintos.
4. **Jobs:** `logVoteWinners` e fechamento automático mapeiam `best_goal → "melhor gol"` nos activity logs.
5. **PWA:** bloco de votação e resultados iguais aos das outras; badge/ícone próprio (⚽️ ou 🥅). front espelha `my_votes.best_goal` no diff de rascunho (FUT-003).

## Parte B — Dashboard de votação (admin)

### Queries (Store)

1. `VoteTallyByMatch(ctx, matchID)` — tally completo (não só vencedores): `[{candidate_id, name, avatar_color, votes}]` por categoria, ordenado desc.
2. `VotesByMatchFlat(ctx, matchID, userID?)` — lista auditável `{voter_id, voter_name, category, candidate_id, candidate_name, created_at}`; filtro por votante opcional (é o "pesquisar por usuário").
3. `UserVoteHistory(ctx, userID, since?)` — tudo que aquele usuário votou em qualquer partida (histórico individual).

### Endpoints admin-only

- `GET /api/admin/matches/{id}/votes` — tally completo + flat list (com `?user=` para filtrar)
- `GET /api/admin/users/{id}/votes` — histórico de votos de um usuário
- `GET /api/admin/stats/votes?from=&to=` — agregado entre partidas: participação (% de going que votou), distribuição de vencedores por categoria ao longo do tempo

### PWA admin — seção "Votos"

- **Por partida:** escolher partida → tally por categoria (barras simples com nome, foto e nº de votos), toggle "ver votos individuais" (flat), busca por votante
- **Por usuário:** buscar usuário → tudo que ele votou (por partida), detectar padrões (vota sempre no mesmo? nunca vota?)
- **Comparativo:** última vista agregada (tabela simples por partida × categoria × vencedor), sem gráfico complexo no MVP

## Regras de alcance (fora do MVP)

- Sem export CSV, sem heatmap, sem análise de "coalizões" — fica pra depois se houver apetite.

## Testes / aceite

- Handlers: vote/remove/tally com `best_goal` (2 votos, sem repetição, voto em si mesmo permitido).
- Dashboard: filtros por usuário, aggregates corretos, admin-required sem sessão.
- PWA: componente de votação com 3 categorias funciona; dashboard renderiza tally e busca.
- E2E: votar em 3 categorias, ver no admin tudo sensato.
