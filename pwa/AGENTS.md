# AGENTS.md — fut-app-pwa

Frontend (PWA) do sistema **Fut da Rapaziada** — gestão de partidas, mensalidades e votação.

## Stack

Vue 3 (Composition API + `<script setup>`) · Vite · TypeScript · Tailwind CSS · Pinia · Vue Router · vite-plugin-pwa

## Estrutura

```
src/
  main.ts
  App.vue
  router/             rotas e guards (auth, ativo, admin)
  stores/             Pinia: auth, theme
  api/                client Axios + tipos espelhando o JSON da API
  components/
    ui/               Card, Badge, Avatar, Button…
    layout/           BottomNav, AdminLayout
  views/
    auth/             Invite, Signup, Login
    app/              Home, Match, Teams, Payments, History, MatchDetail, Profile
    admin/            Dashboard, Users, Charges, Invites
  lib/format.ts       datas e valores no padrão brasileiro
public/               ícones PWA, favicon
```

## Comandos

```bash
npm install
npm run build        # type-check (vue-tsc) + build Vite
npm run dev          # dev server (proxy /api → :8080)
```

## Dev local

O `vite.config.ts` já proxy de `/api/*` para `http://localhost:8080` — suba a API Go antes.

## Repo backend

A API (Go + chi + Postgres) vive em `fut-app-org/fut-app-api`. Este repo e a API são versionados e deployados independentemente.

## Documentação completa

Ver `../README.md`, `../STACK.md` (design tokens, rotas) e `../requisitos.md` no workspace raiz (`fut-app/`).
