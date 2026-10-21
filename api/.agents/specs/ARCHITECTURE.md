# Arquitetura da API

```text
cmd/api/            servidor: config → store.New → Migrate → seedAdmin → jobs → HTTP
cmd/vapid/          gera o par de chaves VAPID
internal/
  api/              rotas (server.go), middlewares e handlers HTTP
  billing/          domínio das cobranças (hoje: taxa do Pix repassada)
  auth/             hash de senha, JWT de sessão, rate limit de login
  busdays/          prazos em dias úteis (seg–sex, sem feriados)
  config/           configuração por variáveis de ambiente
  draw/             sorteio e escalação manual dos times
  jobs/             cron: confirmações, votação, vencimento, filas de WhatsApp e push
  mailer/           e-mail transacional (Resend)
  matches/          domínio das partidas (hoje: regras da votação)
  mercadopago/      cliente do Pix do Mercado Pago
  notify/           WhatsApp via Evolution Go (LogSender sem configuração)
  push/             envio Web Push (VAPID)
  store/            PostgreSQL: migrations embutidas, queries e modelos
```

## Camadas

```text
handler (internal/api) ─┬─▶ domínio (internal/matches, …)   regras puras
jobs (internal/jobs)  ──┘   Store (internal/store) ──▶ PostgreSQL
                            Store ─▶ domínio (só tipos e erros)
```

Decisão e motivos: `.agents/specs/design/2026-10-19-dominio-e-tdd.md` (na
raiz do monorepo).
Não é Clean Architecture completa: não há use cases, presenters nem DTO por
camada.

- **Domínio** (`internal/<contexto>`): Go puro com as regras de negócio
  (tipos, constantes de estado, funções de decisão, erros `Err*` com a
  mensagem para o usuário). Não importa `store`, `api`, `net/http` nem
  `pgx`. Recebe os fatos já carregados (ex.: `matches.Ballot`) e devolve
  `nil` ou um erro de domínio. Contextos previstos: `matches`, `billing`,
  `members`, `notices`; crie o pacote quando migrar a primeira regra dele.
- **Handlers** decodificam com `decodeJSON`, validam o formato da entrada
  (JSON, UUID), carregam os fatos pelo Store, chamam o domínio e respondem com
  `writeJSON` / `writeError` / `writeStoreError`. Depois de uma mutação
  relevante, registram `s.store.LogActivity(...)`.
- **Store** concentra o SQL. Guardas atômicas continuam no SQL (transições
  com `where status in (...)`, constraints únicas, transações) porque o
  domínio não protege contra concorrência. Pode importar o domínio para usar
  constantes e devolver os erros dele.
- **Erros → HTTP** num lugar só: `writeStoreError` em `respond.go`. Erro de
  domínio novo entra lá e em `respond_test.go`.
- **Jobs** chamam o Store, o domínio e os senders; nunca o HTTP.
- Integrações externas ficam atrás de um tipo próprio (`notify.Sender`,
  `push`, `mercadopago`, `mailer`) para os testes não falarem com a rede.

Migração em andamento: a votação já está em `matches` e a taxa do Pix em
`billing`. As demais regras (rateio, inadimplência, transições de partida,
convites) ainda estão nos handlers e no SQL. Ao mexer numa delas, prefira migrá-la seguindo o ADR.

Não introduza ORM, `sqlc`, injeção de dependência ou repositórios genéricos
sem aprovação.

## Rotas

Registradas em `internal/api/server.go`, em quatro grupos:

1. públicas (`/healthz`, convite, login, recuperação de senha, webhook);
2. `requireAuth` (perfil, cobranças, Pix, push, arquivos de mídia);
3. `requireAuth` + `requireActive` (partidas, votos, mídia, atividade);
4. `requireAuth` + `requireActive` + `requireAdmin` (`/admin/*` e ações de
   partida).

Coloque a rota nova no grupo de menor privilégio que a regra permitir.
Parâmetros de caminho são lidos com `r.PathValue("id")`.

## Banco

- Migrations em `internal/store/migrations/NNN_nome.sql`, aplicadas em ordem
  alfabética no startup e registradas em `schema_migrations`. Nunca edite uma
  migration já publicada; crie a próxima. O prefixo `004` está duplicado por
  razões históricas: não renomeie.
- Migrations precisam ser aditivas (a PWA anterior continua no ar durante o
  deploy e o rollback não desfaz o banco).
- Estados são `text` com `check (... in (...))`. Ao acrescentar um estado,
  altere a constraint, `models.go` e `pwa/src/api/types.ts`.
- Valores monetários em centavos (`int64`, sufixo `_cents`). Datas de
  calendário como `date` e trafegadas como `YYYY-MM-DD`; mês de referência
  como `YYYY-MM`.

## Testes (TDD)

Ciclo para regra nova ou alterada:

1. Escreva o teste do domínio (tabela de casos) e veja falhar.
2. Implemente o mínimo no pacote de domínio até passar.
3. Ligue o handler/job ao domínio; se houver erro novo, mapeie em
   `writeStoreError` e no teste de contrato.
4. Rode `bash scripts/test-db.sh` (todos os testes, com banco).

Para migrar regra existente, comece por testes de caracterização do
comportamento atual (no Store, com banco) e só então mova o código, sem mudar
status nem mensagens.

- Testes ao lado do código (`*_test.go`); casos e comentários em português
  quando descrevem regra de negócio. Domínio: table-driven a partir de um
  caso válido que cada linha altera (ver `matches/voting_test.go`).
- Testes de `store` usam `newTestStore(t, prefixo)` (`testdb_test.go`):
  schema isolado com migrations, removido no fim. Sem `TEST_DATABASE_URL`,
  são pulados; o CI e `scripts/test-db.sh` sempre os executam.
- Transportes externos (push, WhatsApp, Mercado Pago, Resend) são simulados.

## Convenções

- Mensagens de erro para o usuário em português, curtas, sem detalhes
  internos; detalhes vão para `log.Printf`.
- Comentários em português explicando o porquê.
- Configuração nova: campo em `config.Config`, leitura em `config.Load()`,
  documentação em `.env.example` e no manifesto do `vps-infra`
  (`.agents/specs/DEPLOYMENT.md` na raiz).
