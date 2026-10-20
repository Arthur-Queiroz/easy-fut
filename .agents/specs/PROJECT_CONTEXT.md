# Contexto do projeto

O Easy Fut (nome antigo: Fut da Rapaziada / Fut App) organiza um grupo fechado
de futebol. Não há cadastro aberto: todo jogador entra por convite. A
interface é uma PWA pensada para celular, instalável na tela inicial.

## Perfis

- **Jogador (`player`):** confirma presença, vê times, paga cobranças por Pix,
  vota após o jogo, envia fotos e vídeos, ativa notificações no aparelho.
- **Administrador (`admin`):** tudo do jogador, mais o painel `/admin`:
  partidas, sorteio, usuários, convites, cobranças, avisos por push e
  configurações. O primeiro admin é criado no startup quando o banco está
  vazio (`SEED_ADMIN_*`).

Status do usuário: `active`, `inactive` (bloqueado, em geral por
inadimplência) e `archived` (saiu do grupo). Inativos continuam logando para
ver o motivo e pagar o que devem.

## Fluxos principais

1. **Convite → cadastro:** admin cria convite (opcionalmente com telefone e
   envio por WhatsApp); o link `/convite/:token` leva ao cadastro.
2. **Partida:** admin cria; jogadores ativos confirmam `going`/`not_going`
   até o prazo; o sistema fecha as confirmações; admin sorteia ou monta os
   times; admin encerra a partida e abre a votação.
3. **Votação:** artilheiro (`top_scorer`) e perna de pau (`worst_player`).
4. **Cobranças:** mensalidade mensal rateada entre ativos ou cobrança avulsa
   de partida rateada entre participantes escolhidos. Pagamento por Pix
   dinâmico do Mercado Pago com webhook, ou baixa manual pelo admin.
5. **Avisos:** Web Push de partida criada, lembrete antes do prazo e avisos
   manuais do admin. WhatsApp para convites e lembretes de cobrança vencida.

## Integrações externas

| Integração      | Uso                                        | Sem configuração            |
| --------------- | ------------------------------------------ | --------------------------- |
| Mercado Pago    | Pix dinâmico + webhook de pagamento        | geração de Pix indisponível |
| Resend          | e-mail de recuperação de senha             | recuperação indisponível    |
| Evolution Go    | WhatsApp (instância própria na VPS)        | mensagens só no log         |
| Web Push (VAPID)| notificações nos aparelhos                 | PWA informa indisponível    |

`MERCADO_PAGO_TEST_MODE=true` só no sandbox.

## Ambientes

- **Produção:** VPS Hostinger, containers `fut-app-api` e `fut-app-web`
  gerenciados por `deployctl`, atrás de Cloudflare Tunnel em
  `https://fut.devarthur.com.br`. PostgreSQL 17 em container próprio,
  só no loopback da VPS.
- **Local:** API em `:8080`, Vite em `:5173` com proxy de `/api`. Uma
  instância local pode apontar para o banco da VPS por túnel SSH; nesse caso
  use `DISABLE_JOBS=true` para não duplicar o cron de produção.

## Histórico

O monorepo `Arthur-Queiroz/easy-fut` nasceu em 2026-10-17 juntando
`fut-app-org/fut-app-api` e `fut-app-org/fut-app-pwa` (arquivados). Referências
`fut-app-org/<repo>#N` nos commits apontam para PRs dos repositórios antigos.
O módulo Go ainda se chama `futdarapaziada/api`.

`api/BACKEND_SPEC.md` e `pwa/design/design/` são o desenho original do
produto. Servem de referência de intenção e de visual, mas o código é a fonte
da verdade: o Pix é do Mercado Pago (não o BR Code estático da seção 7), o
acesso ao banco é `pgx` puro (sem `sqlc`) e há estados e endpoints que o spec
não lista.
