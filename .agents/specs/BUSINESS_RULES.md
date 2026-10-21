# Regras de negócio

Fonte da verdade: o código em `api/internal/`. Ao mudar uma regra, atualize
este arquivo no mesmo PR.

## Acesso

- Sessão por JWT em cookie httpOnly. `requireAuth` recarrega o usuário do
  banco a cada requisição, então inativações valem na hora. Trocar a senha
  incrementa `session_version` e derruba as sessões antigas.
- Rotas operacionais (partidas, votos, mídia, atividade) exigem
  `requireActive`. Inativos só acessam perfil, cobranças, Pix e push; a API
  responde `403` com `code: "inactive"` e a PWA leva para `/bloqueado`.
- Rotas `/admin/*` e as ações administrativas de partida exigem
  `requireAdmin`.
- Convite vale uma vez: inválido se usado, revogado ou expirado.
- Usuários têm um tipo (`users.kind`): `member` (membro do grupo) ou
  `guest` (avulso, quem comprou uma vaga numa partida). **Avulso não tem
  conta:** fica sem e-mail e sem senha em `users`, nunca é admin (check no
  banco), e login, cadastro por convite, recuperação de senha,
  `requireAuth` e `requireActive` o recusam. O admin não muda perfil nem
  status de um avulso (`409`); corrigir nome e telefone continua possível.

### Isolamento dos membros

Tudo que fala do grupo considera só `kind = 'member'`: `ActiveUsers` e o
rateio da mensalidade, os participantes de cobrança de partida, as
confirmações iniciais da partida criada e das recorrentes, o push de partida
(criação, lembrete e envio), os avisos manuais (destinatários, fila e envio),
os números do painel, a inadimplência (inativação e reativação) e a lista
`/admin/users`, que aceita `?kind=member|guest` com `member` como padrão,
inclusive na contagem e na paginação.

## Partidas

Estados (`matches.status`):

| De                      | Para          | Quem                                |
| ----------------------- | ------------- | ----------------------------------- |
| `open`                  | `closed`      | cron no prazo de confirmação, ou admin |
| `closed`, `teams_drawn` | `open`        | admin reabre as confirmações        |
| `closed`, `teams_drawn` | `teams_drawn` | admin sorteia ou refaz os times     |
| `closed`, `teams_drawn` | `voting`      | admin encerra a partida, ou cron (FUT-001) após o fim previsto + `voting_open_delay_min` |
| `voting`                | `finished`    | cron ao fim de `voting_closes_at`, ou admin |
| qualquer, exceto `finished`/`cancelled` | `cancelled` | admin, com motivo  |

- Ao criar a partida, todos os membros ativos recebem uma confirmação
  `no_response`. Avulsos entram na presença (`going`) só ao pagar a vaga.
- Confirmação só é aceita com a partida `open`; o cron fecha as vencidas a
  cada minuto.
- Sorteio só depois do fechamento: embaralha os confirmados e distribui em
  round-robin (diferença máxima de um jogador). Também há escalação manual,
  em que cada confirmado aparece exatamente uma vez, e presets de times.
- Encerrar a partida abre a votação por `voting_close_days` dias (padrão 2).
  Na abertura automática, o prazo conta a partir do fim previsto da partida
  (`match_date + end_time`, horário de São Paulo).
- A "próxima partida" da tela inicial prioriza uma partida em `voting`.

### Partidas recorrentes (FUT-004)

- Templates em `recurring_matches` (dia da semana, horário, local, prazo de
  confirmação opcional; sem prazo, 30 min antes do início). Admin em
  `/admin/recorrencias`.
- Job diário (`15 8 * * *` no fuso do processo, que é São Paulo: 08h15)
  materializa as 4 próximas ocorrências de cada template ativo (janela de
  amanhã a hoje + 28 dias), sem recriar datas que já existem
  (`recurring_id + match_date`). Dia e horários do template são de São
  Paulo. Uma partida gerada pode ser editada sem afetar o template; excluir
  o template mantém as partidas.
- No mesmo job, todo membro ativo ganha confirmação `no_response` nas
  partidas recorrentes abertas (inclusive quem virou ativo depois da
  geração), o que habilita o lembrete por push. Não há push de "partida
  criada" para recorrentes.

## Votação

Código: `api/internal/matches/voting.go` (`Ballot.Validate`).

Categorias: `top_scorer` (artilheiro), `worst_player` (perna de pau) e
`best_goal` (melhor gol, FUT-005, mesmas regras do artilheiro).

- Só vota e só recebe voto quem participou da partida (confirmou `going`).
- Até 2 votos por categoria, em jogadores distintos; o voto pode ser
  desfeito enquanto a votação estiver aberta. O limite vale também com
  envios simultâneos: as gravações de um eleitor numa partida são
  serializadas por lock (`lockVoter` em `store/votes.go`).
- Não vale votar em si mesmo como `worst_player`.
- Empate na apuração devolve mais de um vencedor. A apuração só é exposta
  depois de `finished`.
- A PWA monta um rascunho e, ao tocar em "Enviar votos" (FUT-003), manda
  `PUT /matches/{id}/votes` com a lista completa de cada categoria alterada
  (`matches.Submission`); a troca é atômica. Sair da tela ou fechar o app
  com rascunho pendente faz o mesmo envio com `keepalive`. `POST` e `DELETE`
  de um voto continuam aceitos (PWA anterior).
- A apuração cobre as 3 categorias, e o feed anuncia o vencedor de cada uma.
- Admin vê apuração completa, votos individuais e comparativo em
  `/admin/votos` (FUT-005).

## Cobranças

Lotes (`charge_batches.kind`):

- `monthly`: um por mês (índice único parcial). Valor total dividido pelos
  usuários ativos no momento da geração.
- `match`: avulso, ilimitado por mês, valor total dividido entre os
  participantes ativos escolhidos pelo admin.

O rateio é uma fotografia: total, quantidade e valor individual ficam gravados
e não mudam se o número de ativos mudar depois. A divisão é inteira em
centavos. Vencimento: 5 dias úteis após a geração (seg–sex, sem feriados).

Taxa do Pix repassada (`api/internal/billing`):
- **Na geração:** cada cobrança ganha `pix_fee_cents`, calculado sobre o
  valor individual com a taxa de `pix_fee_bps` (padrão 99 = 0,99%, a
  tarifa do Mercado Pago na conta).
- **Valor cobrado:** o jogador vê e paga `pix_total_cents` (rateio + taxa),
  e é esse total que vai no Pix. A taxa não é detalhada para o jogador, que
  vê só o valor final; o detalhamento aparece só para o admin, que anuncia
  no grupo o valor com a taxa (a prévia ao gerar o lote mostra esse valor).
- **Cálculo:** o total é o menor valor que, descontada a tarifa (arredondada
  para cima no centavo), deixa o rateio inteiro para o grupo. Exemplo:
  R$ 60,00 → R$ 60,60.
- **Congelamento:** a taxa fica gravada no lote (`pix_fee_bps`,
  `pix_fee_cents`) e na cobrança; mudar a configuração só afeta lotes
  novos. Cobranças anteriores ao repasse têm taxa 0.
- **Baixa manual** (dinheiro ou Pix direto) registra só o rateio.
- Lembretes de WhatsApp (`{{valor}}`) informam o total no Pix.

Estados (`charges.status`):

- `pending` → `overdue`: cron diário às 00h15, que também agenda o lembrete
  de WhatsApp com o template `overdue_template` (`{{nome}}`,
  `{{mes_referencia}}`, `{{valor}}`, `{{data_vencimento}}`).
- `pending`/`overdue` → `paid`: webhook do Mercado Pago (idempotente).
- `pending`/`overdue` → `manual_paid`: baixa pelo admin.
- `pending`/`overdue` → `cancelled` ou `exempt`: ação do admin.
- Um lote só pode ser excluído se nenhuma cobrança dele estiver paga.

Inadimplência: membro ativo com cobrança `overdue` há mais de
`overdue_inactivate_days` dias (padrão 0) é inativado pelo cron. Ao pagar, é
reativado automaticamente se não restar nada vencido.

## Participação avulsa

Regras completas em `.agents/specs/design/2026-10-19-participacao-avulsa.md`
(seção "Decisões"). Código: `api/internal/guests` (domínio) e
`api/internal/store/guests.go` e `cash.go`.

- **Venda por partida** (`PUT /admin/matches/{id}/guest-sale`
  `{open, slots}`): o admin abre a venda com 0 a 100 vagas, só com a
  partida `open` e o prazo de confirmação no futuro (senão `409`), e nunca
  com menos vagas que as inscrições que ocupam vaga (`confirmed` e reservas
  ainda válidas; `409`). Ao abrir uma venda fechada, congela
  `guest_price_cents` (configuração, padrão 2500) e o total com a taxa do
  Pix vigente embutida (`pix_fee_bps`, a mesma do repasse das
  mensalidades: 2500 → 2525 com 0,99%). Com a venda já aberta, o admin só
  reajusta as vagas e o preço não muda. Fechar mantém vagas, preço e total;
  reabrir congela o preço vigente de novo. A partida expõe
  `guest_sale_open`, `guest_slots`, `guest_price_cents`,
  `guest_total_cents`, `guest_confirmed_count` e `guest_pending_count`
  (`reserved` + `pending_payment`).
- **Tela Avulsos** (`GET /admin/guest-bookings?match_id=&status=&q=`):
  inscrições mais recentes primeiro, com a partida e a última entrega de
  cada WhatsApp (confirmação e link de voto). `q` busca no nome (sem
  diferenciar maiúsculas) ou no telefone: celular completo em qualquer
  formatação é normalizado; só dígitos procuram um trecho do número.
- **Inscrição:** nome, WhatsApp (celular brasileiro, guardado em E.164) e
  e-mail (fica na inscrição, não em `users`). Cada compra cria um avulso
  novo. Só com a venda aberta, partida `open`, prazo de confirmação no
  futuro e vaga livre (`reserved + pending_payment + confirmed < vagas`),
  com a partida travada durante a contagem. Uma inscrição ativa por
  WhatsApp por partida. A reserva dura 30 minutos.
- **Fluxo público:** `/public/guest-matches` mostra as vendas abertas;
  `/public/guest-bookings` reserva a vaga e cria um Pix com o mesmo prazo.
  O token de acompanhamento é devolvido uma vez, guardado só como hash e
  exigido em `X-Booking-Token` na consulta da inscrição. O job expira as
  reservas vencidas a cada minuto.
- **Estados:** `reserved` → `pending_payment` → `confirmed`; `expired`
  (reserva vencida), `cancelled` (partida cancelada antes do pagamento),
  `refund_pending` → `refunded`. Transições fora da tabela do domínio são
  recusadas.
- **Pagamento aprovado:** com a reserva válida, `confirmed`: grava `going`
  na presença, lança a entrada no caixa e agenda uma única vez o WhatsApp de
  confirmação. Depois do prazo (ou com a inscrição já expirada ou
  cancelada), sempre reembolsa: `refund_pending`, sem vaga e sem mensagem,
  mas com a entrada no caixa, porque o dinheiro entrou. Webhook repetido não
  muda nada.
- **Votação:** ao abrir a votação, o admin ou o job agenda um link privado
  para cada avulso confirmado. A varredura do job cobre links que faltarem.
  O token fica guardado só como hash, dá acesso apenas à partida da
  inscrição e perde validade ao encerrar a votação. O reenvio pelo admin
  revoga o link anterior.
- **Partida cancelada:** confirmadas vão para `refund_pending`; reservas e
  pendentes, para `cancelled`.
- **Reembolso** (`POST /admin/guest-bookings/{id}/refunded`): o admin
  estorna o Pix no Mercado Pago e marca aqui. Só de `refund_pending`
  (`409` nos demais); grava `refunded` e, na mesma transação, lança no
  caixa a saída `refund` com o valor e a taxa da entrada da inscrição (sem
  entrada, sem saída). Repetir a marcação não muda nada.
- **Caixa** (`cash_entries`; `GET /admin/cash?from=&to=`): entradas de
  inscrição (valor base, com a taxa em `fee_cents`), estornos e despesas
  lançadas pelo admin (`POST /admin/cash/expenses` com descrição de até 120
  caracteres, valor positivo e data `YYYY-MM-DD`). Valor sempre positivo, o
  sinal vem do tipo; o saldo soma o caixa inteiro e o período (datas
  inclusivas) filtra só a lista. Só despesas podem ser removidas
  (`DELETE /admin/cash/expenses/{id}`). Não se mistura com as mensalidades.

## Notificações

- **Push de partida:** aviso na criação (fila gravada na mesma transação) e
  lembrete na janela de `MATCH_REMINDER_HOURS` (padrão 2; `0` desliga) antes
  do prazo, só para membros ativos que não responderam. Worker a cada minuto, um
  envio por aparelho/partida/tipo, até cinco tentativas com backoff, lease de
  dois minutos. HTTP 404/410 remove a inscrição.
- **Avisos manuais:** até 240 caracteres (título opcional até 60), para todos
  os membros ativos ou membros escolhidos (inativos permitidos, arquivados
  e avulsos nunca).
  Aviso sem nenhum aparelho de destino é recusado; não entregue em 24 h é
  descartado.
- Um aparelho fica inscrito em uma conta por vez; logout remove a inscrição.
- **WhatsApp:** fila `notifications` enviada a cada 5 minutos; envio manual
  de convite e de lembrete de cobrança pelo admin.

Detalhes operacionais em `api/PUSH_NOTIFICATIONS.md` e
`pwa/PUSH_NOTIFICATIONS.md`.

## Configurações do admin

Tabela `settings` (chave/valor). Editadas em `/admin/configuracoes`:

- `voting_close_days` (padrão 2), `overdue_inactivate_days` (padrão 0),
  `invite_valid_days` (padrão 7);
- `reminder_template`: mensagem do lembrete de cobrança que o admin envia
  manualmente por WhatsApp;
- `pix_fee_bps`: taxa do Pix repassada, editada em porcentagem (0,99) e
  validada de 0 a 19,99%; 0 desliga o repasse;
- `guest_price_cents`: preço base da vaga avulsa em centavos (padrão 2500),
  validado de 100 a 100000 e congelado na partida ao abrir a venda. Valor
  inválido no banco vale como 2500.

Fora da tela: `voting_open_delay_min` (padrão 0, minutos após o fim para a
abertura automática da votação), `overdue_template` (lembrete automático de
vencida, só no banco) e `monthly_total_cents` (último total de mensalidade, gravado ao gerar
o lote e usado como sugestão no próximo mês).
