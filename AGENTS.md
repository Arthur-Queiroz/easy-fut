# AGENTS.md — easy-fut

Monorepo do **Easy Fut**. Cada pasta tem seu próprio `AGENTS.md` com stack, estrutura e comandos:

- [`api/AGENTS.md`](api/AGENTS.md) — API Go
- [`pwa/AGENTS.md`](pwa/AGENTS.md) — PWA Vue

## Regras do monorepo

- Rode os comandos de cada lado dentro da respectiva pasta (`cd api`, `cd pwa`); não há tooling na raiz.
- O CI fica só em `.github/workflows/ci.yml`. Não crie `.github/` dentro de `api/` ou `pwa/`: o GitHub ignora workflows fora da raiz.
- Os nomes das imagens (`ghcr.io/arthur-queiroz/easy-fut-api`, `ghcr.io/arthur-queiroz/easy-fut-web`) e dos serviços (`api`, `web`) estão fixados no manifesto da VPS; não os renomeie sem alterar o `vps-infra`.
- Mudanças que tocam API e PWA podem ir no mesmo PR; o deploy publica a API antes da PWA.
