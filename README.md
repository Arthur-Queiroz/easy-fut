# Easy Fut

Monorepo do Easy Fut: gestão de partidas, mensalidades e votação.

| Pasta  | O que é                                   | Serviço no deploy |
| ------ | ----------------------------------------- | ----------------- |
| `api/` | API Go (chi, PostgreSQL, cron)            | `api`             |
| `pwa/` | PWA Vue 3 + Vite, servida por Caddy       | `web`             |

Os históricos de `fut-app-org/fut-app-api` e `fut-app-org/fut-app-pwa` foram preservados. Os repositórios antigos estão arquivados; referências como `fut-app-org/fut-app-pwa#25` nos commits apontam para os PRs originais.

## Desenvolvimento

```bash
# API (porta 8080). A API lê variáveis do processo; go run não carrega .env.
cd api
set -a; source .env; set +a
go run ./cmd/api

# PWA (porta 5173, proxy /api -> :8080)
cd pwa
npm ci
npm run dev
```

Detalhes de cada lado em [`api/README.md`](api/README.md) e [`pwa/README.md`](pwa/README.md).

## CI/CD

Um único workflow, [`.github/workflows/ci.yml`](.github/workflows/ci.yml):

- Cada lado só é testado quando seus arquivos mudam (`api/**`, `pwa/**`); alterações no workflow testam os dois. `workflow_dispatch` roda tudo.
- Em push na `main`, com `DEPLOY_ENABLED=true`, cada imagem é publicada no GHCR (`arthur-queiroz/easy-fut-api` e `arthur-queiroz/easy-fut-web`) e liberada na VPS pelo workflow reutilizável `Arthur-Queiroz/vps-deploy`.
- Quando API e PWA mudam no mesmo push, a PWA só é publicada depois do release da API.

As variáveis de ambiente de produção não ficam no GitHub: estão no manifesto `apps/fut-app.yaml` do `vps-infra` e em `/var/lib/vps-apps/fut-app/secrets.env` na VPS. O GitHub guarda apenas `DEPLOY_SSH_KEY`, `VPS_HOST` e a variável `DEPLOY_ENABLED`.
