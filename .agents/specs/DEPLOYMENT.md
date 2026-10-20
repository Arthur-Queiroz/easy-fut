# Deploy

## Pipeline

Um único workflow, `.github/workflows/ci.yml`, roda em PRs, em push na `main`
e manualmente (`workflow_dispatch` roda tudo).

- `changes` (`dorny/paths-filter`) marca `api` e/ou `pwa` como alterados;
  mudar o próprio workflow marca os dois.
- `api-backend`: `gofmt -l`, `bash -n scripts/*.sh`, `go vet`, `go test`
  (com PostgreSQL 17 e `TEST_DATABASE_URL`) e `staticcheck`.
- `api-compose` / `pwa-compose`: `docker compose config` + `build`.
- `pwa-frontend`: `npm ci`, `npm run test`, `npm run build`.
- `release-api` e `release-web`: só na `main` com a variável
  `DEPLOY_ENABLED=true`. `release-web` espera `release-api` quando a API
  também mudou e não roda se ela falhar.

Cada release chama
`Arthur-Queiroz/vps-deploy/.github/workflows/app-deploy.yml@main` com
`app-id: fut-app`, `service: api|web`, `context: api|pwa` e a imagem
`arthur-queiroz/easy-fut-{api,web}`. O workflow publica no GHCR e libera o
digest na VPS via `deployctl`, usando a chave confinada `deploy-fut-app`.

## Variáveis e segredos

- Produção: manifesto `apps/fut-app.yaml` do repositório `vps-infra`
  (bloco `env` de cada serviço, com `fromSecret` para os segredos) e
  `/var/lib/vps-apps/fut-app/secrets.env` na VPS.
- O `compose.yaml` de `api/` e `pwa/` serve para desenvolvimento e para o
  build no CI; **não** é usado em produção. Acrescentar uma variável só nele
  não a injeta no container.
- GitHub guarda apenas `DEPLOY_SSH_KEY`, `VPS_HOST` e a variável
  `DEPLOY_ENABLED`.
- Uma variável nova da API precisa ser documentada em `api/.env.example` e
  adicionada ao manifesto do `vps-infra` antes do merge.
- Não rotacione o par VAPID: trocar a chave pública invalida as inscrições
  de todos os aparelhos.

## Runtime

- A PWA é servida pelo Caddy (`pwa/Caddyfile`) em `:80`, sem TLS (o
  Cloudflare Tunnel termina o TLS). `/api/*` vai para `fut-app-api:8080`. O
  site responde a qualquer Host porque o healthcheck do `deployctl` acessa o
  IP do container.
- `index.html`, `sw.js` e `push-sw.js` não têm cache; `/assets/*` é imutável.
- A API aplica as migrations embutidas no startup. Elas precisam ser
  aditivas: o rollback é publicar a imagem anterior, sem desfazer o banco.
- Healthcheck: `GET /api/healthz` responde `200` só com o PostgreSQL de pé.
