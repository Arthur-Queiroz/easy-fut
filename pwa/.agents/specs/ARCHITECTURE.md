# Arquitetura da PWA

```text
src/
  main.ts, App.vue
  router/index.ts       rotas e guard global (public, ativo, admin)
  stores/               Pinia: auth (usuário logado), theme (claro/escuro)
  api/client.ts         instância Axios (baseURL /api) + errorMessage()
  api/types.ts          tipos espelhando o JSON da API Go
  components/
    ui/                 Card, Badge, Avatar, BaseButton, Modal, EmptyState, SearchSelect…
    layout/             MobileShell (app), AdminLayout, BottomNav, TopNav, NavIcon
    pwa/                InstallPrompt, PushSettings
    admin/              componentes exclusivos do admin (sorteio, gráficos de votos)
  views/
    auth/               convite, cadastro, login, recuperação de senha
    app/                início, partida, times, pagamentos, histórico, perfil, bloqueado
    admin/              painel, partidas, usuários, mensalidades, convites, avisos, configurações
  lib/
    format.ts           datas e valores no padrão brasileiro
    security.ts         cores, URLs, arquivos de mídia, senha, CSV
    push.ts             inscrição Web Push
    search.ts           normalização de busca (sem acento) e tipo das opções de SearchSelect
    votes.ts            cálculos do dashboard de votos (barras, filtros, participação)
public/push-sw.js       handler de push importado pelo service worker gerado
design/design/          mockups e tokens originais (referência visual)
```

## Rotas e acesso

- As rotas e os caminhos são em português (`/partida`, `/pagamentos`,
  `/admin/mensalidades`). Toda view é carregada com `() => import(...)`.
- `meta.public` libera sem login; `meta.admin` exige admin. O guard manda
  para `/login?redirect=...` sem sessão e para `/bloqueado` se o usuário não
  estiver ativo.
- O interceptor do Axios repete essa regra para respostas da API: `401` vai
  para o login (preservando o destino) e `403` com `code: "inactive"` vai
  para `/bloqueado`. Não trate esses casos de novo em cada view.

## Dados

- Chame a API pelo `api` de `src/api/client.ts`, com o tipo da resposta
  (`api.get<Charge[]>('/charges/me')`). Erros para o usuário passam por
  `errorMessage(error)`, que já devolve a mensagem em português da API.
- Ao mudar um contrato, atualize `src/api/types.ts` no mesmo PR da API.
  Campos opcionais da API (`omitempty`) são opcionais no tipo.
- Estado local da tela fica na própria view (`ref`/`computed`). Pinia só para
  estado global de verdade (sessão e tema).
- A API é a dona das regras. A PWA pode desabilitar botões para orientar o
  usuário, mas não deve ser a única barreira.

## Testes

- Vitest com jsdom; arquivo `*.test.ts` ao lado do componente, ou em
  `tests/` para o service worker.
- Mock da API com `vi.hoisted` + `vi.mock('../../api/client', ...)`; layouts
  são stubados no `mount`. Ver `src/views/admin/NotificationsView.test.ts`.
- Nada de rede real nem push real nos testes.
