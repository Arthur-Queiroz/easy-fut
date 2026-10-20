# Pendências

Itens levantados e ainda não resolvidos. Remova o item no PR que o resolver.

## Documentação

- `api/OPERATIONS.md`, `api/README.md` e `api/.env.example` têm acentos
  corrompidos (UTF-8 lido como Latin-1, ex.: `OperaÃ§Ãµes`).
- `api/OPERATIONS.md` e `api/scripts/{backup,restore}-postgres.sh` assumem o
  layout antigo (`/opt/fut-app-api` + `docker compose exec postgres`). Em
  produção o deploy é por `deployctl` e o PostgreSQL roda em outro container;
  confira os scripts antes de usá-los na VPS.
- `api/BACKEND_SPEC.md` e `pwa/design/design/BACKEND_SPEC.md` divergem entre
  si e do código (votação, cobranças avulsas, Pix). Ver
  `.agents/specs/PROJECT_CONTEXT.md`, seção Histórico.

## Produto

- Dias úteis não consideram feriados (`api/internal/busdays`).
- **Horário do job de recorrentes.** `jobs.go` agenda `15 8 * * *` com o
  comentário "05:15 SP" (o pedido do FUT-004 era `15 5`). A imagem usa
  `TZ=America/Sao_Paulo` e o cron usa o fuso local, então roda às 08h15.
  Decidir o horário e alinhar código, comentário e spec.
- **Tarifa efetiva do Mercado Pago não é conciliada.** O repasse grava a
  taxa calculada, mas não a tarifa e o líquido reais de cada Pix (a Orders
  API não os devolve). Ver a seção Conciliação de
  `.agents/specs/design/2026-10-20-meios-de-pagamento-e-taxas.md`.
- `SettingsView.vue`, no card "Pendentes de integração", ainda diz que a
  geração automática do Pix aguarda a definição do provedor; o Pix do
  Mercado Pago já está em produção.
- Lembrete D-1 de cobrança: o agendamento automático foi removido em
  `c65e007` (virou envio manual), mas `SettingsView.vue` ainda diz "O envio é
  agendado para um dia útil antes do vencimento" e o comentário em
  `handleGenerateCharges` também. Decidir se volta o automático ou se corrige
  o texto.
