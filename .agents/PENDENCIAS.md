# Pendências

Itens levantados e ainda não resolvidos. Remova o item no PR que o resolver.

## Documentação

- `api/OPERATIONS.md`, `api/README.md` e `api/.env.example` têm acentos
  corrompidos (UTF-8 lido como Latin-1, ex.: `OperaÃ§Ãµes`).
- `api/OPERATIONS.md` e `api/scripts/{backup,restore}-postgres.sh` assumem o
  layout antigo (`/opt/fut-app-api` + `docker compose exec postgres`). Em
  produção o deploy é por `deployctl` e o PostgreSQL roda em outro container;
  confira os scripts antes de usá-los na VPS.
- `api/BACKEND_SPEC.md` e `pwa/design/design/BACKEND_SPEC.md` divergem entre
  si e do código (votação, cobranças avulsas, Pix). Ver
  `.agents/specs/PROJECT_CONTEXT.md`, seção Histórico.

## Produto

- Dias úteis não consideram feriados (`api/internal/busdays`).
- **Troca de voto falha (FUT-003 × limite de 2).** O "Enviar votos" manda
  DELETE e POST em paralelo (`Promise.all` em `MatchDetailContent.vue`). Com
  2 votos gravados, trocar um deles faz o POST ler os 2 antigos antes do
  DELETE e responder 409 "você já usou seus 2 votos": 18 de 20 trocas
  recusadas no teste de 2026-10-19. No auto-envio ao sair da tela a falha é
  silenciosa. O inverso também existe: `handleVote` lê, valida e só depois
  insere, então envios concorrentes podem gravar 3 votos
  (`TestVoteLimitIsNotEnforcedByStore`). Corrigir nos dois lados: PWA
  aplica DELETEs antes dos POSTs; `CastVote` valida o limite dentro de uma
  transação com lock do eleitor, com teste de concorrência.
- **"Melhor gol" fora da apuração (FUT-005).** `VoteResults` só apura
  `top_scorer` e `worst_player`: o jogador não vê o vencedor de melhor gol e
  o feed não o anuncia, embora o FUT-005 previsse resultados iguais aos das
  outras. `TestVoteResultsTieReturnsEveryWinnerByName` registra o
  comportamento atual. Corrigir usando `matches.Categories`.
- **Recorrentes geram 5 partidas na véspera (FUT-004).** A série de
  `MaterializeRecurringMatches` cobre 29 dias (amanhã até hoje + 4 semanas +
  1 dia); quando o job roda na véspera do dia do template, o dia aparece 5
  vezes. `TestMaterializeRespectsInactiveAndDelete` falha nesses dias (o CI
  passou no domingo; falha na segunda para template de terça).
- **Horário do job de recorrentes.** `jobs.go` agenda `15 8 * * *` com o
  comentário "05:15 SP" (o pedido do FUT-004 era `15 5`). A imagem usa
  `TZ=America/Sao_Paulo` e o cron usa o fuso local, então roda às 08h15.
  Decidir o horário e alinhar código, comentário e spec.
- **Recorrentes sem confirmações e sem push (FUT-004).** A materialização
  insere direto em `matches`, sem as linhas `no_response` em
  `match_confirmations` e sem a fila `match_push_deliveries` que o
  `CreateMatch` cria. Consequência (pela leitura do código): sem push de
  "partida criada", sem lembrete antes do prazo e lista "sem resposta"
  vazia. Reaproveitar a mesma lógica do `CreateMatch`.
- Lembrete D-1 de cobrança: o agendamento automático foi removido em
  `c65e007` (virou envio manual), mas `SettingsView.vue` ainda diz "O envio é
  agendado para um dia útil antes do vencimento" e o comentário em
  `handleGenerateCharges` também. Decidir se volta o automático ou se corrige
  o texto.
