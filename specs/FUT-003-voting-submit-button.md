# FUT-003 — Votação em rascunho com botão de envio

**Status:** implemented (PR fut/003-voting-submit)
**Tipo:** vertical (PWA)
**Origem:** pedido do Arthur: tela de votação com "cara de Google Forms" — marcar candidatos e clicar em "Enviar votos" para confirmar.

## Regras

- **R1 — Rascunho local:** tocar num jogador marca/desmarca **localmente** (sem chamada de API). Seleções limitam a 2 por categoria; perna de pau não permite marcar a si mesmo (a opção nem aparece, como hoje).
- **R2 — Botão "Enviar votos":** aparecefixo (sticky) no fim da seção de votação quando há rascunho; habilita quando **pelo menos 1 categoria** tem ≥1 marcado. Envia em sequência: votos a adicionar via `POST /matches/{id}/votes`, os removidos via `DELETE` (novo rascunho parte das marcas persistidas → di f).
- **R3 — Auto-envio:** se o usuário sair da tela com rascunho sujo (route leave / visibilitychange - app fechado), tenta enviar o rascunho restante automaticamente (promises em paralelo). Best-effort: falha não bloqueia; persiste em localStorage como fallback para restaurar na próxima abertura.
- **R4 — Confirmação visual:** sucesso no envio mostra toast "Votos registrados ✓"; resumo "2/2 votos" em cada categoria permanece.
- **R5 — Sem mudança de backend:** todos os endpoints já existem; o diff não toca API, migrations nem contrato.

## Contrato / UI

- `MatchDetailContent.vue`: estado `draftVotes = { top_scorer: [], worst_player: [] }` carregado de `detail.my_votes`; toques alteram o rascunho; botão calcula diff (toAdd / toRemove) e o aplica seq.

## Testes / aceite

- Novo teste de componente (`MatchDetailContent.test.ts`): marcar 2, botão habilita, enviar chama POST 2x; desmarcar 1 → DELETE 1x; trocar 1 → algum par POST/DELETE correto.
- Suíte PWA (`npm run test` + build) verde.
- Manual: votar, fechar o app antes de enviar, reabrir → rascunho restaurado do localStorage.
