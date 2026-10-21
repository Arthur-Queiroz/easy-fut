# Participação avulsa — plano de implementação

- **Data:** 2026-10-21
- **Regras:** `2026-10-19-participacao-avulsa.md`, seção "Decisões". Onde o
  texto do desenho inicial divergir dela, vale a seção Decisões.
- **Depende de:** PR #13 (repasse da taxa do Pix, pacote `billing`).
- **Modo:** desenvolvimento com subagentes, cada um numa worktree própria e
  com TDD conforme `2026-10-19-dominio-e-tdd.md`. A integração, os testes de
  ponta a ponta e os PRs ficam com o agente principal.

## Contratos fixos

Os agentes implementam exatamente isto. Mudar um contrato exige combinar
antes, porque API e PWA são feitas em paralelo.

### Banco (migration `015_guest_participation.sql`, aditiva)

- **`users`:**
  - `kind text not null default 'member' check (kind in ('member','guest'))`;
  - `email` e `password_hash` passam a aceitar nulo **só** para `guest`
    (check), e `guest` nunca é `admin` (check);
  - membros existentes não mudam.
- **`matches`:**
  - `guest_sale_open boolean not null default false`;
  - `guest_slots int not null default 0`;
  - `guest_price_cents bigint not null default 0` (valor base congelado ao
    abrir a venda);
  - `guest_total_cents bigint not null default 0` (com a taxa embutida).
- **`guest_bookings`:**
  - colunas:
    - `id`, `match_id`, `user_id` (o avulso), `name`, `phone` (normalizado
      E.164), `email`;
    - `status`, em `reserved`, `pending_payment`, `confirmed`, `expired`,
      `cancelled`, `refund_pending`, `refunded`;
    - `base_cents`, `total_cents`, `fee_cents`;
    - `expires_at`, `confirmed_at`;
    - `access_token_hash`;
    - campos do Mercado Pago (`provider_order_id`, único quando preenchido;
      `provider_payment_id`, `pix_payload`, `pix_qr_code_base64`,
      `pix_ticket_url`, `provider_status`, `provider_status_detail`);
    - `refund_reason`, `created_at`.
  - Índice único parcial em `(match_id, phone)` onde `status in
    ('reserved','pending_payment','confirmed')`.
- **`guest_vote_tokens`:** `booking_id` (único), `token_hash`, `expires_at`,
  `revoked_at`.
- **`guest_messages`:** `booking_id`, `kind` (`confirmation`, `vote_link`),
  `notification_id`, `created_at`. Único `(booking_id, kind)` para o envio
  automático; reenvio do admin grava com `resend = true`.
- **`cash_entries`:**
  - `id`, `kind` (`guest_payment`, `refund`, `expense`), `amount_cents`
    (positivo; o sinal vem do `kind`), `fee_cents`, `booking_id` (nulo em
    despesa), `description`, `occurred_on date`, `created_by`, `created_at`.
- **Configuração:** `settings.guest_price_cents = '2500'`.

### Domínio (`internal/guests`, Go puro)

- **Telefone:** `NormalizePhone(raw) (string, error)`, Brasil, só celular,
  E.164.
- **Venda:** `CheckSaleOpen(match facts)` valida venda aberta, partida
  `open`, prazo de confirmação no futuro e vagas livres (`reserved +
  pending_payment + confirmed < slots`).
- **Transições:** tabela de transições de `status`, com `ErrInvalidTransition`.
- **Pagamento aprovado:** `OnPaymentApproved(booking, now)`:
  - com a reserva ainda válida, `confirmed`;
  - com a reserva expirada, `refund_pending` (decisão "sempre reembolsa").
- **Partida cancelada:** confirmadas vão para `refund_pending`; reservas
  vão para `cancelled`.
- **Erros com mensagem ao usuário:**
  - `ErrSoldOut` "as vagas avulsas desta partida acabaram";
  - `ErrSaleClosed` "a venda de vagas desta partida está encerrada";
  - `ErrPhoneAlreadyBooked` "este WhatsApp já tem uma vaga nesta partida";
  - `ErrInvalidPhone`, `ErrInvalidEmail`, `ErrInvalidName`.

  Mapeados em `writeStoreError` (409 para os três primeiros, 400 para os
  outros) e em `respond_test.go`.

### HTTP

**Públicos** (sem sessão; limite por IP no mesmo estilo do rate limit de
login):

| Método e caminho | Entrada | Saída |
| --- | --- | --- |
| `GET /api/public/guest-matches` | — | `[{match_id, match_date, start_time, end_time, venue, address, total_cents, slots_left}]`, só partidas com venda aberta |
| `POST /api/public/guest-bookings` | `{match_id, name, phone, email}` | 201 `{booking_id, access_token, status, total_cents, expires_at, pix_payload, pix_qr_code_base64, pix_ticket_url}` |
| `GET /api/public/guest-bookings/{id}` | header `X-Booking-Token` | `{status, total_cents, expires_at, match: {...com endereço}, pix_*}` (pix só enquanto pendente) |
| `GET /api/public/guest-vote/{token}` | — | `{match, participants: [{user_id, name, avatar_color}], my_votes, voting_closes_at}`; 404 token inválido, 410 votação fechada |
| `PUT /api/public/guest-vote/{token}/votes` | `{votes: {categoria: [ids]}}` | igual a `PUT /matches/{id}/votes` (`matches.Submission`) |

Comportamento da criação de inscrição (`POST /api/public/guest-bookings`):
- **Uma transação:** com lock da partida, checa venda e vagas, cria o
  avulso e cria a inscrição com `expires_at = now + 30 min`.
- **Pix:** gera o Pix com `external_reference = "guest:<booking_id>"` e
  expiração de 30 min, e marca a inscrição como `pending_payment`.
- **Token de acompanhamento:** o `access_token` serve para acompanhar a
  inscrição. É devolvido uma vez e guardado só como hash.

**Admin** (`requireAdmin`):

| Método e caminho | Uso |
| --- | --- |
| `PUT /api/admin/matches/{id}/guest-sale` `{open, slots}` | abre ou fecha a venda; ao abrir, congela o preço e o total |
| `GET /api/admin/guest-bookings?match_id=&status=&q=` | tela Avulsos (busca por nome ou telefone) |
| `POST /api/admin/guest-bookings/{id}/resend` `{kind}` | reenvio de confirmação ou link de voto |
| `POST /api/admin/guest-bookings/{id}/refunded` | marca o estorno feito no Mercado Pago e lança a saída no caixa |
| `GET /api/admin/users?kind=member\|guest` | lista com filtro de tipo; `member` é o padrão, inclusive na contagem e na paginação |
| `GET /api/admin/cash?from=&to=` | `{balance_cents, entries: [...]}` |
| `POST /api/admin/cash/expenses` `{description, amount_cents, occurred_on}` | lança despesa |
| `DELETE /api/admin/cash/expenses/{id}` | remove despesa lançada por engano |

**Integração com o que existe:**
- **Webhook do Mercado Pago:** `external_reference` com prefixo `guest:`
  vai para o fluxo de inscrição; sem prefixo, segue o fluxo atual de
  cobrança.
- **Aprovação do pagamento:** na mesma transação, grava `going` em
  `match_confirmations`, lança a entrada no caixa e agenda o WhatsApp de
  confirmação, uma vez por inscrição.
- **Cron:** a cada minuto, expira as reservas vencidas. Ao abrir a votação,
  pelo admin ou pela abertura automática, agenda o link de voto de cada
  avulso confirmado, uma vez.
- **Cancelamento da partida:** confirmadas vão para `refund_pending` e
  reservas para `cancelled`.
- **Selo "Avulso":** confirmações e times passam a expor `kind` do usuário.

### Isolamento dos membros

Toda consulta que fala do grupo passa a exigir `kind = 'member'`:
- `ActiveUsers` e o rateio da mensalidade;
- confirmações iniciais em `CreateMatch` e na materialização de
  recorrentes;
- push de partida e avisos;
- dashboard;
- inadimplência;
- lista de usuários do admin.

Login, convite, recuperação de senha e `requireAuth` recusam `guest`.

## Fases e agentes

```text
Fase 0 (principal) ─ merge do #13; contratos acima congelados
Fase 1 (1 agente)  ─ F: fundação
Fase 2 (3 agentes, em paralelo, sobre F)
                     A: inscrição pública + Pix + webhook + expiração
                     B: admin (venda, avulsos, reembolso, caixa, filtro de usuários)
                     C: voto por token + mensagens de WhatsApp + selo Avulso na API
Fase 3 (2 agentes, em paralelo, contra os contratos)
                     D: PWA pública (entrada no login, partidas, formulário,
                        acompanhamento do Pix, página de voto por link)
                     E: PWA admin (venda na partida, tela Avulsos, Caixa,
                        filtro Membros/Avulsos, selo Avulso)
Fase 4 (principal) ─ integração, ponta a ponta pela API real com Pix
                     simulado, screenshots, revisão e PRs
```

**F, fundação:**
- migration 015;
- domínio `guests` com testes de tabela;
- store básico de inscrições, caixa e tokens, com testes de caracterização;
- isolamento dos membros, com teste de que avulsos não entram em rateio,
  confirmações iniciais, push, dashboard e inadimplência;
- recusa de `guest` no login e na autenticação.

**A, B e C** dependem só de F e mexem em arquivos diferentes, com handlers
e store em arquivos próprios por área. Os pontos de contato ficam definidos
nos contratos:
- o webhook (A) chama a função de store de F que confirma a inscrição e
  agenda a mensagem;
- C fornece o envio do link de voto chamado pelo job e por
  `handleFinishMatch`.

**D e E** podem começar junto com a Fase 2, usando os contratos e a API
simulada nos testes (`vi.mock` do client). Eles fazem só UI; o
acompanhamento do Pix é feito por polling do endpoint público a cada 5 s.

**Regras para todos os agentes:**
- Ler `AGENTS.md`, `BUSINESS_RULES.md`, este plano e a seção Decisões
  antes de começar.
- Teste antes do código; `gofmt`, `go vet`, `bash scripts/test-db.sh`;
  `npm test` e `npm run build`.
- Não fazer push, PR nem deploy; terminar com commit na própria branch e um
  relatório do que foi feito, testado e deixado de fora.
- Nunca falar com o Mercado Pago real nem mandar WhatsApp: os transportes
  são simulados nos testes.
- Não mudar os contratos; se um contrato não fechar, parar e reportar.

## PRs previstos

1. Fundação e isolamento dos membros. Não muda nada visível, mas o
   isolamento é o ponto de maior risco e fica revisável sozinho.
2. API da inscrição, admin, voto e caixa (A + B + C).
3. PWA (D + E).

Deploy em ordem, esperando cada deploy terminar e conferindo os jobs da API
e da PWA antes do próximo merge.
