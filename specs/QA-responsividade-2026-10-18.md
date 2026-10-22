# Relatório de QA de Responsividade — Easy Fut PWA

**Data:** 2026-10-18 · **Testador:** browser automatizado (CDP, sessão admin legítima)
**Viewports:** 390×844 (primário), 320×568 (iPhone SE), 1024×768 (tablet)
**Método:** auditoria de geometria DOM em 14 rotas (5 app + 9 admin) — overflow horizontal, elementos fora do viewport, overlap com bottom nav, targets de interação, modais; listener global de erros JS.

## Resultado geral: sem bugs críticos de responsividade

- **0 overflow horizontal** não-intencional em 14 rotas × 3 viewports (incluindo iPhone SE 320px)
- **0 erros de console** (window error + unhandledrejection) em todas as rotas
- **Modais:** "Nova partida" full-screen 390×844 exato; "Editar" 358px centralizado; 8 inputs, nenhum estreito (<40px)
- **Bottom nav do app jogador:** nenhum conteúdo por baixo nas 5 rotas app
- **Tabelas admin** (usuários/min-w-880, mensalidades/620, partidas/680): scroll-x **intencional** — parents com `overflow-x-auto` corretamente configurados; página não quebra

## Melhorias recomendadas (polimento, nenhum é bug)

1. **Nav horizontal do admin no mobile** (`AdminLayout aside`): scroll lateral sem indicador visual — usuário não sabe que há mais itens. Adicionar fade/gradiente na borda direita ou `scroll-snap`.
2. **Tabelas admin no celular:** as colunas de ação (Editar/Cancelar) ficam a ~900px — exige rolar. Em <640px, esconder colunas secundárias (email, criado em) ou mover ações pra dropdown "⋮".
3. **Comparativo de votos:** partidas futuras (15/11, 08/11…) aparecem com 0/0 no comparativo — barulho; já priorizamos com-votos no dropdown, aplicar filtro semelhante à lista do comparativo.
4. **Deep-link de detalhe:** histórico usa `?detalhe=<id>` (query) em vez da rota `/historico/:matchId` — funciona, mas links compartilháveis de partida específica dariam melhor UX com a rota real.

## Fora de escopo desta rodada
- Testes de teclado virtual (soft keyboard Android/iOS) e safe-areas com notch dinâmico (requer dispositivo físico)
- Acessibilidade de contraste/touch-targets (WCAG) — sugerida como rodada própria
