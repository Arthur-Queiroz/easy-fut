# Easy Fut API

Siga também o `AGENTS.md` da raiz: contexto, regras de negócio e segurança
valem para o monorepo inteiro. Específico da API:

1. `.agents/specs/ARCHITECTURE.md` (nesta pasta).
2. `PUSH_NOTIFICATIONS.md` ao mexer em push, jobs ou na fila de envios.
3. `OPERATIONS.md` para backup, restauração e monitoramento (ver
   `../.agents/PENDENCIAS.md` antes de usar os scripts).

## Stack

Go 1.26+ · chi v5 · PostgreSQL via `pgx/v5` (SQL escrito à mão) · JWT em cookie
httpOnly · `robfig/cron` · `webpush-go` · Mercado Pago SDK. Módulo Go:
`futdarapaziada/api`.

## Comandos

```bash
gofmt -l .                # precisa sair vazio (o CI falha caso contrário)
go vet ./...
go test ./...             # testes de banco são pulados sem TEST_DATABASE_URL
bash scripts/test-db.sh   # todos os testes com um PostgreSQL descartável (Docker)
bash scripts/test-db.sh ./internal/matches -run Ballot   # aceita args do go test
staticcheck ./...         # roda no CI; local se instalado
go build ./cmd/api
```

## Executar localmente

A API lê variáveis do processo; `go run` não carrega `.env`.

```bash
docker network create fut-app     # uma vez
docker compose up -d postgres
set -a; source .env; set +a       # a partir de .env.example
go run ./cmd/api                  # :8080, aplica migrations e cria o admin inicial
```

Ao apontar para o banco da VPS (túnel SSH), use `DISABLE_JOBS=true`.

## Verificação

Regra de negócio nova ou alterada segue o ciclo de TDD de
`.agents/specs/ARCHITECTURE.md`: teste do domínio primeiro.

Após alterar a API, rode `gofmt -l .`, `go vet ./...` e
`bash scripts/test-db.sh` (ou `go test ./...` sem Docker, sabendo que os
testes de banco serão pulados).
