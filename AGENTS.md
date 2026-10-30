# Easy Fut

Monorepo do sistema de um grupo fechado de futebol: partidas, confirmações,
sorteio de times, mensalidades por Pix, votação pós-jogo e avisos por push.

```text
api/      API Go (chi, pgx/PostgreSQL, cron): regras de negócio, banco e integrações
pwa/      PWA Vue 3 + Vite + Tailwind, servida por Caddy: interface dos jogadores e do admin
.agents/  contexto do produto, regras de negócio, deploy, pendências e skills
```

Leia antes de alterar código:

1. `.agents/specs/PROJECT_CONTEXT.md`
2. `.agents/specs/BUSINESS_RULES.md`
3. `api/AGENTS.md` ou `pwa/AGENTS.md`, conforme o lado alterado.
4. `.agents/specs/DEPLOYMENT.md` quando a alteração afetar Dockerfiles,
   Compose, Caddy, GitHub Actions ou variáveis de ambiente.
5. `.agents/PENDENCIAS.md` para saber o que já está levantado antes de abrir
   trabalho novo.

## Regras do monorepo

- Rode os comandos de cada lado dentro da respectiva pasta (`cd api`,
  `cd pwa`); não há tooling na raiz.
- O CI fica só em `.github/workflows/ci.yml`. Não crie `.github/` dentro de
  `api/` ou `pwa/`: o GitHub ignora workflows fora da raiz.
- Os nomes das imagens (`ghcr.io/arthur-queiroz/easy-fut-api`,
  `ghcr.io/arthur-queiroz/easy-fut-web`), do app (`fut-app`) e dos serviços
  (`api`, `web`) estão fixados no manifesto da VPS; não os renomeie sem
  alterar o `vps-infra`.
- O contexto para agentes fica em `.agents/`. `.claude/skills` é um link para
  `.agents/skills`; crie skills novos só em `.agents/skills/`.

## Mudanças que atravessam API e PWA

A API é dona das regras de negócio; a PWA só as exibe e respeita. Uma feature
que altera um contrato HTTP muda os dois lados no mesmo PR:

- handlers em `api/internal/api/` e modelos com tags JSON em
  `api/internal/store/models.go`;
- tipos em `pwa/src/api/types.ts` e as chamadas nas views.

O deploy publica a API antes da PWA, então a API nova precisa continuar
compatível com a PWA anterior (e as migrations precisam ser aditivas).

## Segurança

- Nunca versionar `.env`, tokens (Mercado Pago, Resend, Evolution), chaves
  VAPID privadas, `JWT_SECRET`, connection strings reais ou chaves SSH.
- O browser nunca recebe segredos: a PWA não tem variáveis de build com
  credenciais e lê a chave VAPID pública em `/api/push/config`.
- Não dispare push, WhatsApp ou e-mail para usuários reais em testes. Os
  testes automatizados simulam esses transportes.
- O banco de produção é de usuários reais. Leitura para diagnóstico só com
  autorização explícita; escrita, nunca fora da aplicação.

## Verificação

- API (`cd api`): `gofmt -l .` vazio, `go vet ./...` e
  `bash scripts/test-db.sh` (testes com PostgreSQL descartável).
- Regras de negócio da API ficam em pacotes de domínio e nascem por TDD:
  ver `.agents/specs/design/2026-10-19-dominio-e-tdd.md`.
- PWA (`cd pwa`): `npm test` e `npm run build` (type-check + build).

## Publicação

- Para mudanças concluídas solicitadas pelo usuário, valide o lado alterado,
  abra um PR, aguarde os checks e publique por merge na `main` como padrão,
  sem pedir nova autorização. Respeite pedidos explícitos para manter uma
  mudança local ou em revisão.
- Acompanhe o GitHub Actions até confirmar o deploy e o status pós-deploy.
  Não inclua alterações locais de outras tarefas na publicação.
- Para CI/CD, use o skill `deploy-cicd`; não publique por SSH manualmente.
