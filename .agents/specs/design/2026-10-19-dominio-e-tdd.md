# Regras de domínio em pacotes próprios, guiadas por testes

- **Data:** 2026-10-19
- **Status:** aceita; piloto (votação) implementado

## Contexto

A API nasceu como MVP com duas camadas: handlers HTTP e um `Store` com SQL à
mão. As regras de negócio ficaram espalhadas entre handler, SQL e jobs, e o
`Server` depende do `*store.Store` concreto, então testar uma regra exigia
Postgres. Resultado na época da decisão: geração e baixa de cobranças,
inadimplência, transições de partida e votação sem nenhum teste; `api` com
27% de cobertura (só helpers) e `jobs` com 5%.

## Decisão

1. **Sem Clean Architecture completa.** Para ~5,7 mil linhas de Go em um
   deploy, entities/use cases/presenters e um DTO por camada custariam mais do
   que resolvem. Nada de rewrite.
2. **DDD estratégico:** um pacote de domínio por contexto, com nome em inglês
   como o resto do código: `matches` (partidas, confirmação, times,
   votação), `billing` (rateio, vencimento, cobranças, inadimplência),
   `members` (usuários, convites, status) e `notices` (filas de push e
   WhatsApp). Criados sob demanda, quando uma regra for migrada.
3. **DDD tático mínimo:** o domínio é Go puro (tipos, constantes, funções,
   erros), sem importar `store`, `api` nem `net/http`. O handler carrega os
   fatos do banco, o domínio decide, o handler responde. O SQL mantém as
   guardas atômicas (`where status in (...)`, constraints) para concorrência.
   Sem repositórios genéricos; interface só onde um teste precisar.
4. **TDD:** antes de mover uma regra, testes de caracterização do
   comportamento atual (store com `TEST_DATABASE_URL`, contrato HTTP em
   `respond_test.go`); depois testes do domínio escritos antes do código.
   Feature nova começa pelo teste. `scripts/test-db.sh` roda tudo com um
   Postgres descartável.

## Piloto: votação

- `internal/matches/voting.go`: `Category` (incluindo `best_goal`, que
  chegou em paralelo pelo FUT-005), `Ballot.Validate`, `CheckVotingOpen`,
  `ResultsVisible`, `WinnerAnnouncement` e os erros, com 100% de cobertura.
  Substitui `api/internal/api/vote_categories.go`.
- `handleVote`, `handleRemoveVote`, `handleGetMatch`, `logVoteWinners` e o job
  `closeExpiredVoting` passaram a usar o domínio; o texto do vencedor deixou
  de estar duplicado entre handler e job.
- `writeStoreError` mapeia os erros de domínio para os mesmos status de antes;
  `respond_test.go` trava esse contrato.
- Caracterização em `internal/store/votes_test.go` (6 testes) e verificação de
  ponta a ponta pela API real (26 casos) sem mudança de comportamento.

## Próximos passos

1. `billing`: caracterizar geração de lote, baixa, cancelamento/isenção,
   vencimento, inativação e reativação; depois extrair.
2. `matches`: transições de estado da partida (tabela em
   `BUSINESS_RULES.md`) e regras de confirmação; mover `draw` para dentro.
3. PWA: quebrar views grandes (`ChargesView.vue`) em composables e separar
   chamadas da API por domínio.

## Consequências

- Uma regra nova ou alterada tem lugar definido e teste sem banco.
- Erros de domínio precisam ser mapeados em `writeStoreError`; esquecer um
  resulta em 500, e o teste de contrato deve ganhar o caso.
- Enquanto a migração não termina, convivem regras no domínio e regras ainda
  nos handlers; `api/.agents/specs/ARCHITECTURE.md` diz qual é qual.
