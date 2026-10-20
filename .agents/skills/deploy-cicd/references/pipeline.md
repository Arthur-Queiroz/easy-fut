# Pipeline

`.github/workflows/ci.yml` roda em PRs, em push para `main` e manualmente.

- `changes` decide, por `dorny/paths-filter`, se `api/**` e/ou `pwa/**`
  mudaram. Execução manual trata os dois como alterados.
- `api-backend`, `api-compose`, `pwa-frontend` e `pwa-compose` rodam só para
  o lado alterado. Jobs pulados contam como sucesso em checks obrigatórios.
- `release-api` e `release-web` rodam em `main` com `DEPLOY_ENABLED=true`.
  `release-web` espera `release-api` quando a API mudou no mesmo push e não
  roda se ela falhar.
- PRs da mesma branch cancelam a execução anterior; na `main` nada é
  cancelado, para não interromper um deploy.

Cada release chama `Arthur-Queiroz/vps-deploy/.github/workflows/app-deploy.yml@main`
com `app-id: fut-app`, `service: api|web` e `context: api|pwa`. O workflow
publica a imagem no GHCR e executa, como `deploy-fut-app`,
`deployctl release fut-app <serviço> <digest>` seguido de `deployctl status`.

Diagnóstico de falhas:

- 403 no push da imagem: o pacote GHCR não dá acesso de escrita a este
  repositório (*Package settings → Manage Actions access*).
- Falha no deploy/healthcheck: o log de `deployctl status fut-app` no próprio
  job mostra o estado dos serviços. A API só fica saudável com o PostgreSQL
  acessível (`/api/healthz`); a web precisa responder em `:80` para qualquer
  Host.
- Variável nova sem efeito: ela foi posta no `compose.yaml` ou só no
  `secrets.env`, e não no manifesto `apps/fut-app.yaml` do `vps-infra`.
- Release não disparou: confira `DEPLOY_ENABLED` e se o job `changes` marcou o
  lado como alterado.
