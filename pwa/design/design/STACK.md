# Fut da Rapaziada — Stack e guia de implementação

## Vue puro (Vite) vs Nuxt SPA — decisão

**Recomendo Vue 3 + Vite puro, sem Nuxt.**

Motivo: Nuxt existe para ganhar SSR/SSG e roteamento por arquivos com hidratação no servidor — nenhum dos dois pesa aqui. O app é 100% autenticado/atrás de convite (nada de SEO a otimizar), e como o backend Go já serve a API e vai servir os assets estáticos do build, SSR só adicionaria um processo Node rodando 24/7 na mesma VPS pequena só pra fazer o trabalho que o Go já faz. Rodar `ssr:false` no Nuxt vira "Vite + Vue com convenções a mais" — você paga a complexidade de aprender as convenções do Nuxt (auto-imports, `pages/`, `nuxt.config`) sem usar o que ele foi feito para resolver.

Vite + Vue puro te dá controle total do bundle, zero processo Node adicional em produção (só arquivos estáticos), e é mais fácil de debugar quando algo foge do previsto.

Se no futuro você quiser landing pages públicas (marketing, SEO) fora do app autenticado, aí sim considere Nuxt para *essas* páginas especificamente — não para o app em si.

> Nota: se optar por Nuxt mesmo assim, use `nuxt generate` (não `nuxt build`+`nuxt start`) — com `ssr:false` isso gera só arquivos estáticos, sem processo Node em produção, ficando equivalente em infra ao Vite puro. A diferença vira só DX (auto-imports, rotas por arquivo) vs controle explícito.

## Stack completa

**Frontend**
- Vue 3 (`<script setup>`, Composition API)
- Vite (build/dev server)
- Vue Router (rotas autenticadas com guards)
- Pinia (estado: usuário logado, próxima partida, tema claro/escuro)
- Tailwind CSS (tokens abaixo replicam o design)
- `vite-plugin-pwa` (instalar no celular, cache offline básico)
- Axios ou `fetch` com wrapper simples para a API Go

**Backend**
- Go (`chi` router), PostgreSQL, `sqlc`/`sqlx`
- JWT em cookie httpOnly
- Gateway PIX (Mercado Pago/Efí/Asaas) + webhook de confirmação
- WhatsApp (Twilio ou Meta Cloud API) para lembretes
- `robfig/cron` para jobs (gerar mensalidade, lembrete, inativar inadimplente)

**Infra (VPS KVM Hostinger)**
- Docker Compose: `api` (Go), `postgres`, `caddy` (serve o build do Vite como estático + reverse proxy pra `/api`, TLS automático)
- Deploy: GitHub Actions → build → `docker compose up -d` via SSH
- Backup: `pg_dump` diário → Backblaze B2

## Design tokens (do mockup) → Tailwind

```js
// tailwind.config.js
theme: {
  extend: {
    colors: {
      canvas: { DEFAULT: '#E6EDE7', dark: '#0A120E' },
      surface: { DEFAULT: '#FFFFFF', dark: '#17251D' },
      surface2: { DEFAULT: '#ECF2ED', dark: '#1E3126' },
      border: { DEFAULT: '#DBE4DC', dark: '#2A3E32' },
      ink: { DEFAULT: '#15231B', dark: '#EDF5EF' },
      ink2: { DEFAULT: '#576B5F', dark: '#A6BAAE' },
      ink3: { DEFAULT: '#8CA094', dark: '#71877A' },
      brand: { DEFAULT: '#0E9455', dark: '#34CE81' },  // verde principal
      brandSoft: { DEFAULT: '#DFF2E7', dark: '#17392A' },
      lime: '#C8F14B',        // acento sobre fundo escuro (headers/hero)
      warn: '#B45309', warnBg: '#FCF0DB',
      danger: '#C43030', dangerBg: '#FBE9E9',
      info: '#2563EB', infoBg: '#E5EDFC',
    },
    fontFamily: {
      sans: ['Barlow', 'system-ui', 'sans-serif'],
      condensed: ['"Barlow Condensed"', 'sans-serif'], // números grandes (datas, placar, R$)
    },
  }
}
```
Use a classe `dark:` do Tailwind (modo `class`) para o toggle de tema — o mock já define os pares claro/escuro acima.

## Estrutura de pastas sugerida

```
src/
  main.ts
  App.vue
  router/index.ts
  stores/ (auth.ts, match.ts, payments.ts, theme.ts)
  api/ (client.ts, matches.ts, payments.ts, users.ts, invites.ts)
  components/
    layout/ (BottomNav.vue, AdminSidebar.vue, TopBar.vue)
    ui/ (Card.vue, Badge.vue, Avatar.vue, Button.vue)
  views/
    auth/ (InviteView.vue, SignupView.vue, LoginView.vue)
    app/ (HomeView.vue, MatchView.vue, TeamsView.vue, PaymentsView.vue, HistoryView.vue, MatchDetailView.vue, ProfileView.vue)
    admin/ (DashboardView.vue, UsersView.vue, ChargesView.vue, InvitesView.vue)
```

## Mapeamento telas do mockup → rotas

| Tela do mockup | Rota | Guard |
|---|---|---|
| 1a Convite | `/convite/:token` | pública |
| 1b Cadastro | `/cadastro/:token` | pública |
| 1c Início | `/` | auth |
| 1d Próxima partida | `/partida` | auth |
| 1e Times sorteados | `/partida/times` | auth |
| 1f Pagamentos | `/pagamentos` | auth |
| 1g Histórico | `/historico` | auth |
| 1h Detalhe + votação | `/historico/:matchId` | auth |
| 1i Perfil | `/perfil` | auth |
| 1j Dashboard admin | `/admin` | auth + admin |
| 1k Usuários | `/admin/usuarios` | auth + admin |
| 1l Mensalidades | `/admin/mensalidades` | auth + admin |
| 1m Convites | `/admin/convites` | auth + admin |

## Passos de implementação

1. **Scaffold**: `npm create vite@latest frontend -- --template vue-ts`, instalar Tailwind, Vue Router, Pinia, `vite-plugin-pwa`, Axios.
2. **Tokens**: copiar o bloco de cores/fontes acima pro `tailwind.config.js`; importar fonte Barlow via `@fontsource/barlow` (evita depender do Google Fonts CDN em produção).
3. **Componentes base**: extrair do mockup os padrões repetidos — `Card` (fundo `surface`, borda `border`, `rounded-2xl`, `shadow`), `Badge` (pill colorido por status: sucesso/aviso/perigo/info), `Avatar` (círculo com iniciais + cor por hash do nome).
4. **Layout shells**: `BottomNav.vue` (5 itens do mock mobile) e `AdminSidebar.vue` (verde escuro, ícones + label, replicar do admin desktop).
5. **Stores Pinia**: `auth` (token, usuário, papel admin/jogador), `theme` (claro/escuro, persistido em `localStorage`), `match` (próxima partida, confirmações), `payments`.
6. **API client**: wrapper Axios com `baseURL` apontando pro Go (`/api`), interceptor pra token e refresh de sessão expirada.
7. **Views**: implementar tela por tela seguindo o mockup 1:1 (cores, espaçamento, hierarquia tipográfica com Barlow Condensed nos números grandes).
8. **Guards de rota**: `beforeEach` checando `auth.isLoggedIn` e `auth.isAdmin` pras rotas `/admin/*`.
9. **PWA**: configurar `vite-plugin-pwa` com manifest (nome "Fut da Rapaziada", ícone, cor tema `#0A3B28`), `injectRegister: 'auto'`.
10. **Build + deploy**: `vite build` gera `dist/`; Caddy serve esse `dist/` como estático e faz proxy de `/api/*` pro binário Go — um único `docker compose up -d` sobe tudo na VPS.
