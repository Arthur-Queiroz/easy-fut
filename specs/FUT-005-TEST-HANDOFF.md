# Handoff de teste — FUT-005 (best_goal + dashboard de votação)

**Para:** agente testador
**Branch:** `fut/005-voting-dashboard` | **Spec:** `specs/FUT-005-voting-dashboard.md`
**O que é:** terceira categoria de voto (melhor gol) + dashboard admin de votos (tally por partida, votos individuais com filtro por usuário, histórico por usuário, comparativo de participação).

## Estado do ambiente (na VPS, não container)

- Postgres de teste: `postgres://cSDvbNU2hXq0S1gM:<senha do env POSTGRES_PASSWORD do container postgresql-uii7-postgresql-1>@127.0.0.1:32798/futapp_test?sslmode=disable` (schemas descartáveis criados pela suíte; banco vive após os testes)
- Commando suíte Go: `TEST_DATABASE_URL=... go test ./...` em `api/`
- PWA: `npm run test` (vitest, 40 testes) e `npm run build` (vue-tsc + vite) em `pwa/`

## Como rodar

```bash
cd ~/workspaces/easy-fut/api
# senha: docker exec postgresql-uii7-postgresql-1 env | grep POSTGRES_PASSWORD
TEST_DATABASE_URL="postgres://cSDvbNU2hXq0S1gM:<SENHA>@127.0.0.1:32798/futapp_test?sslmode=disable" go test ./... -v
cd ../pwa && npm run test && npm run build
```

**Resultado esperado: 10 pacotes Go ok; PWA 40/40; build limpo.**

## O que foi implementado (verificar cada)

### A) Categoria best_goal

| # | Item | Onde |
|---|------|------|
| A1 | Migration 012 recria `votes_category_check` com `best_goal` | `api/internal/store/migrations/012_best_goal.sql` |
| A2 | Handler aceita e valida as 3 categorias (POST voto, DELETE voto) | `api/internal/api/match_handlers.go` → `validVoteCategory()` |
| A3 | best_goal pode votar em si mesmo (afirmado em `vote_categories.go`) | regra no handler: self-vote bloqueado só para `worst_player` |
| A4 | Limite 2 votos/categoria válido para best_goal | mesmo handler (`myVotes[cat] >= 2`) |
| A5 | Fecha votação/log com label "melhor gol" | `api/internal/jobs/jobs.go` (closeExpiredVoting + logVoteWinners) e `match_handlers.go` (logVoteWinners) |
| A6 | PWA: 3 blocos de votação (artilheiro / perna de pau / melhor gol) | `pwa/src/components/MatchDetailContent.vue` — `VOTE_CATEGORIES` |
| A7 | PWA: resultados com 3 cards | idem, `RESULT_CATEGORIES` |
| A8 | PWA: rascunho/diff do FUT-003 funciona nas 3 categorias | idem, `CATEGORY_KEYS` |

### B) Dashboard admin

| # | Item | Onde |
|---|------|------|
| B1 | `VoteTallyByMatch` — tally completo (não só vencedores), 3 categorias, ordenado | `api/internal/store/votes.go` |
| B2 | `MatchesVotesFlat` — voto individual com filtro opcional por votante | idem |
| B3 | `UserVoteHistory` — histórico de um usuário entre partidas | idem |
| B4 | `MatchesVoteSummary` — votantes/votos por partida | idem |
| B5 | Rotas admin-only: `GET /api/admin/matches/{id}/votes`, `GET /api/admin/users/{id}/votes`, `GET /api/admin/stats/votes?from=&to=` | `api/internal/api/server.go` + `votes_dashboard_handlers.go` |
| B6 | Validação uuid nos paths/query (400 sem tocar o banco) | handlers |
| B7 | PWA: aba "Votos" no admin (comparativo + tally + individuais + por usuário) | `pwa/src/views/admin/VotesView.vue`, rota `/admin/votos`, nav "Votos" |

## Casos de teste (executar e evidenciar)

### Go (já cobertos pela suíte — só confirmar que passam)

1. `TestBestGoalCategoryAccepted` — best_goal aceito no store; duplicado → `ErrDuplicateVote`
2. `TestVoteTallyByMatchFull` — tally agregado por candidato/categoria correto
3. `TestVotesFlatAndUserHistory` — flat completo, filtro por votante, histórico desc entre partidas
4. `TestMatchesVoteSummary` — votantes/votos por partida
5. `TestVotesDashboardRoutesRequireAuth` — 401 sem sessão admin
6. `TestVoteCategoriesValidation` — rejeita "mvp"/""; label certo

### Teste manual E2E (com API + PWA rodando localmente)

7. Subir API local (`cd api && set -a; source .env; set +a; go run ./cmd/api`) + PWA (`cd pwa && npm run dev`)
8. Login como jogador na partida em `voting` → votar em best_goal (2 jogadores) → "Enviar votos" → toast ✓ → recarregar: votos persistidos
9. MODELO FORMS: marcar 2, fechar aba sem enviar → reabrir: rascunho restaurado (localStorage `fut-vote-draft-<match>-<user>`)
10. Worst player: seu botão não aparece; limiar 2; desmarcar → troca
11. Login admin → `/admin/votos`:
    - comparativo lista partidas com votantes/votos (barras)
    - escolher partida → tally nas 3 categorias com barras
    - abrir "Ver votos individuais" → lista voter→candidate
    - "Histórico por usuário": buscar, selecionar → histórico ordenado desc
12. Admin required: `curl -o /dev/null -w "%{http_code}" http://localhost:8080/api/admin/stats/votes` → **401**

### Teste de exceção

13. POST `/api/matches/{id}/votes` com `category: "mvp"` → 400 "categoria inválida"
14. best_goal em si mesmo → deve ACEITAR (diferente de worst_player)
15. 2 votos em best_goal no mesmo candidato → 409 duplicado
16. `GET /api/admin/matches/xxx/votes?user=yyy` com não-uuid → 400

## Riscos/pontos de atenção ao testar

- **Migration 012 em produção:** o check antigo derruba votos best_goal existentes? Não há (feature nova).Confirmar que nenhum voto existente viola.
- **TZ:** confirmation_deadline nas partidas geradas: ver FUT-004 (não desta spec).
- Config de jobs local: sem DATABASE_URL de prod. Use .env de dev.
- localStorage de rascunho persiste entre testes no browser: **limpar antes do caso 9** (`localStorage.clear()`).

## Critério de aprovação

- Suíte Go 10/10 ok com TEST_DATABASE_URL
- PWA: teste 40/40 + build sem erro TS
- E2E manual: casos 8–12 sem erro; exceções 13–16 com respostas HTTP esperadas
- LOGS da API sem stack traces em fluxo normal

## Contato/pontof de dúvida

Spec original: `specs/FUT-005-voting-dashboard.md` (regras R1–R8, Partes A e B). Divergência código × spec: anotar no PR e aplicar `project-sync` skill.
