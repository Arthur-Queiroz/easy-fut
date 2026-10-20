# Easy Fut PWA

Siga também o `AGENTS.md` da raiz: contexto, regras de negócio e segurança
valem para o monorepo inteiro. Específico da PWA:

1. `.agents/specs/ARCHITECTURE.md` (nesta pasta).
2. `.agents/specs/UI_RULES.md` antes de criar ou alterar telas.
3. `PUSH_NOTIFICATIONS.md` ao mexer em service worker ou notificações.

## Stack

Vue 3 (Composition API + `<script setup>`) · Vite 7 · TypeScript · Tailwind
CSS 3 · Pinia · Vue Router · Axios · vite-plugin-pwa · Vitest + Vue Test Utils
(jsdom).

## Comandos

```bash
npm ci
npm run dev          # :5173, proxy /api → http://localhost:8080 (suba a API antes)
npm test             # vitest run
npm run build        # vue-tsc -b + vite build
npm run preview -- --host 127.0.0.1   # testa o service worker (dev não registra)
```

## Verificação

Após alterar a PWA, rode `npm test` e `npm run build`.
