---
name: deploy-cicd
description: Publicar ou diagnosticar o CI/CD do Easy Fut (API e PWA) via GitHub Actions, GHCR e o workflow reutilizável vps-deploy. Use ao alterar o workflow, Dockerfiles, Compose, Caddyfile, variáveis de ambiente, falhas de Actions ou releases na VPS; nunca para orientar deploy manual por SSH.
---

# Deploy CI/CD do Easy Fut

1. Leia `references/pipeline.md` e `.agents/specs/DEPLOYMENT.md` antes de
   alterar o pipeline.
2. Publique exclusivamente pelo GitHub Actions (`.github/workflows/ci.yml`).
   Não execute Git, Docker ou restart manual na VPS como forma de publicar.
3. Variáveis de ambiente e segredos de produção vivem no manifesto
   `apps/fut-app.yaml` do `vps-infra` e em
   `/var/lib/vps-apps/fut-app/secrets.env`. Nunca os escreva em workflow,
   Dockerfile, imagem, logs ou neste repositório. Mudanças nesses arquivos
   ficam com o usuário: entregue os comandos para ele rodar.
4. A PWA não recebe segredos no build. A chave VAPID pública vem de
   `/api/push/config`.
5. Não renomeie `fut-app`, os serviços `api`/`web` nem as imagens
   `arthur-queiroz/easy-fut-{api,web}` sem alterar o `vps-infra` junto.
6. Migrations precisam ser aditivas: a API nova sobe antes da PWA nova e o
   rollback não desfaz o banco.
7. Valide localmente o lado alterado: `gofmt -l .`, `go vet ./...` e
   `go test ./...` em `api/`; `npm test` e `npm run build` em `pwa/`;
   `docker compose config` quando mexer no Compose.
