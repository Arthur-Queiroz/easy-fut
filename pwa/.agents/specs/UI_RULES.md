# Regras de interface

## Público e idioma

- Mobile first: a maioria usa o app instalado no celular. Desktop usa o
  mesmo componente com variantes `lg:`.
- Todo texto visível em português do Brasil, em tom informal de grupo de
  futebol ("Fala, Bruno!"). Datas, horas e valores sempre por
  `src/lib/format.ts` (`formatCents`, `formatDateShort`, `formatMonth`…).
  Falta um formato? Acrescente a função lá em vez de chamar
  `toLocale*String` na view.

## Layout

- Telas do jogador usam `MobileShell` (header verde em degradê e
  `BottomNav` no mobile, `TopNav` no desktop). Telas do admin usam
  `AdminLayout`. Respeite `env(safe-area-inset-*)` em elementos fixos.
- Monte telas com os componentes de `components/ui/` antes de escrever
  marcação nova: `Card`, `SectionLabel`, `Badge`, `BaseButton`, `Avatar`,
  `Modal`, `EmptyState`, `PasswordInput`.
- Escolha entre muitas opções (partida, jogador) usa `SearchSelect`: dropdown
  com busca sem acento e navegação por teclado.
- Lista vazia usa `EmptyState`; ação em andamento usa `BaseButton` com
  `loading`; erro de API aparece perto da ação, com `errorMessage(error)`.

## Cores e tema

- Use os tokens do Tailwind (`bg-surface`, `text-ink2`, `border-border`,
  `bg-brand`, `text-danger`, `bg-warnBg`…). Eles apontam para variáveis CSS
  em `src/style.css`, que trocam sob a classe `.dark` (store `theme`).
- Não use `dark:` e evite hex solto (restam poucos casos antigos): um token
  novo entra em `style.css` (claro e escuro) e em `tailwind.config.js`.
- `Badge` tem os tons `success`, `warn`, `danger`, `info` e `neutral`;
  `BaseButton` tem `primary`, `outline`, `outline-brand`, `danger`, `ghost` e
  `lime`. Reaproveite-os para estados (pendente, vencida, paga…).
- Fontes: Manrope (`font-sans`) no texto, Barlow Condensed
  (`font-condensed`) em números e títulos de destaque.

## Gráficos

- Uma série por gráfico, sem legenda: destaque em `bg-brand`, o resto em
  `bg-chartMuted` (par validado para daltonismo nos dois temas). O valor fica
  escrito na barra; tooltip só complementa e também abre por foco.
- Destaque nunca só por cor: o líder ganha ícone, e há sempre uma tabela ou
  lista com os mesmos números. Exemplos em `components/admin/`.

## Segurança na interface

- Cor vinda da API (ex.: `avatar_color`, cor de time) passa por `safeColor`;
  URL vinda da API (mídia, link de Pix) passa por `safeUrl`.
- Upload de mídia é validado com `validateMediaFile` antes do envio; senha
  nova, com `validatePassword` (mesmas regras da API).
- Exportação CSV passa cada célula por `sanitizeCSVCell`.
- Não use `v-html` com dado da API.

## Referência visual

`design/design/` guarda os mockups originais (`*.dc.html`) e `STACK.md`
com os tokens. Os valores atuais estão em `src/style.css`; em caso de
diferença, vale o código.
