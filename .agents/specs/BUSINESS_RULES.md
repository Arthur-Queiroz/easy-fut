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

- Ao criar a partida, todos os usuários ativos recebem uma confirmação
  `no_response`.
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
- No mesmo job, todo usuário ativo ganha confirmação `no_response` nas
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

Estados (`charges.status`):

- `pending` → `overdue`: cron diário às 00h15, que também agenda o lembrete
  de WhatsApp com o template `overdue_template` (`{{nome}}`,
  `{{mes_referencia}}`, `{{valor}}`, `{{data_vencimento}}`).
- `pending`/`overdue` → `paid`: webhook do Mercado Pago (idempotente).
- `pending`/`overdue` → `manual_paid`: baixa pelo admin.
- `pending`/`overdue` → `cancelled` ou `exempt`: ação do admin.
- Um lote só pode ser excluído se nenhuma cobrança dele estiver paga.

Inadimplência: usuário ativo com cobrança `overdue` há mais de
`overdue_inactivate_days` dias (padrão 0) é inativado pelo cron. Ao pagar, é
reativado automaticamente se não restar nada vencido.

## Notificações

- **Push de partida:** aviso na criação (fila gravada na mesma transação) e
  lembrete na janela de `MATCH_REMINDER_HOURS` (padrão 2; `0` desliga) antes
  do prazo, só para ativos que não responderam. Worker a cada minuto, um
  envio por aparelho/partida/tipo, até cinco tentativas com backoff, lease de
  dois minutos. HTTP 404/410 remove a inscrição.
- **Avisos manuais:** até 240 caracteres (título opcional até 60), para todos
  os ativos ou membros escolhidos (inativos permitidos, arquivados nunca).
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
  manualmente por WhatsApp.

Fora da tela: `voting_open_delay_min` (padrão 0, minutos após o fim para a
abertura automática da votação), `overdue_template` (lembrete automático de
vencida, só no banco) e `monthly_total_cents` (último total de mensalidade, gravado ao gerar
o lote e usado como sugestão no próximo mês).
