# Meios de pagamento e repasse de taxas — proposta

- **Data:** 2026-10-20
- **Status:** etapa 1 (repasse no Pix) implementada; etapas 2 e 3 em
  proposta. Decidido em 2026-10-20: conta de pessoa física no Mercado Pago,
  e a taxa do Pix é repassada ao pagador e somada automaticamente ao valor
  da mensalidade, só nas cobranças geradas a partir daí.

## Problema

1. **Só há um meio de pagamento automático.** Hoje a cobrança (mensalidade
   ou avulsa de partida) só pode ser paga por Pix dinâmico do Mercado Pago
   (Orders API, `internal/mercadopago`), ou por baixa manual do admin.
2. **A taxa sai do bolso do admin.** O Mercado Pago desconta a taxa do valor
   recebido, então o grupo recebe menos do que rateou.
3. **A taxa real não é registrada.** O app não guarda a taxa cobrada nem o
   líquido recebido, então ninguém sabe quanto foi perdido.

Objetivo:
- oferecer pelo menos mais um meio de pagamento, sem perder a confirmação
  automática;
- fazer o pagador cobrir a taxa do meio que escolheu, mostrando o valor
  antes de pagar;
- registrar o custo real de cada pagamento.

## Quanto custa hoje (valores a confirmar)

As tarifas abaixo vêm de páginas dos provedores e de comparadores de
outubro de 2026. Precisam ser conferidas na conta antes da decisão, porque
variam por conta, prazo de recebimento e negociação.

| Meio | Provedor | Tarifa citada | Recebe em |
| --- | --- | --- | --- |
| Pix dinâmico | Mercado Pago (atual) | **0,99% (confirmado nos extratos)** | na hora |
| Cartão de crédito à vista | Mercado Pago | 4,98% / 4,49% / 3,98% | na hora / 14 d / 30 d |
| Boleto | Mercado Pago | R$ 3,49 | 1–2 dias úteis |
| Pix | Asaas | R$ 1,99 (promoção de R$ 0,99) | na hora |
| Cartão em assinatura | Asaas | 2,99% + R$ 0,49 | 32 dias |
| Pix via API | Cora (conta PJ) | 1%, máximo R$ 0,50 | na hora |
| Pix via API | Inter (conta PJ) | "baixo custo", sem valor público | na hora |

Exemplo com uma mensalidade de **R$ 60,00**, com a taxa repassada de forma
que o grupo receba os R$ 60,00 inteiros (fórmula na seção de repasse):

| Meio | Pagador paga | Taxa |
| --- | --- | --- |
| Pix Mercado Pago (0,99%) | R$ 60,60 | R$ 0,60 |
| Pix Cora (1%, teto R$ 0,50) | R$ 60,50 | R$ 0,50 |
| Pix Asaas (R$ 1,99) | R$ 61,99 | R$ 1,99 |
| Cartão MP, recebe em 30 dias (3,98%) | R$ 62,49 | R$ 2,49 |
| Cartão MP, recebe na hora (4,98%) | R$ 63,15 | R$ 3,15 |
| Boleto MP (R$ 3,49) | R$ 63,49 | R$ 3,49 |

Leitura:
- **Valores pequenos favorecem a taxa percentual.** Para mensalidades na
  casa de R$ 50–100, o Pix do Mercado Pago já é o mais barato entre os que
  aceitam pessoa física. Trocar de provedor de Pix só compensa com conta PJ
  (Cora ou Inter).
- **Hoje o grupo perde 0,99% do total.** Num rateio de R$ 1.200 por mês,
  são cerca de R$ 11,88 que o admin cobre.
- **Cartão custa cerca de 4 vezes o Pix.** Faz sentido como opção para quem
  prefere pagar assim, com a taxa repassada, nunca como padrão.
- **Boleto não compensa** para esses valores.

## Opções de meio de pagamento

### A. Cartão de crédito pelo Mercado Pago (recomendado como 2º meio)

O mesmo provedor e a mesma conta, então o webhook e a conciliação continuam
num lugar só.
- **Formulário de cartão:** a PWA usa o formulário do Mercado Pago (Card
  Payment Brick), que gera um token. O número do cartão nunca passa pela
  nossa API, o que mantém o projeto fora do escopo PCI. Isso exige carregar
  o SDK do Mercado Pago na PWA e liberar o domínio dele na CSP do Caddy.
- **Pagamento:** a API cria a order com o token e o valor já com a taxa.
- **Prazo de recebimento:** escolher 30 dias deixa a taxa repassada menor
  (3,98% em vez de 4,98%). Para mensalidade, receber em 30 dias não costuma
  ser um problema.
- **Parcelamento:** fica fora. A mensalidade é mensal; parcelar só aumenta
  a taxa e a inadimplência.

### B. Pix Automático (avaliar depois)

Recorrência regulada pelo Banco Central desde 2025: o pagador autoriza uma
vez, a cobrança cai sozinha todo mês, e há até 3 retentativas em 7 dias se
faltar saldo. É gratuito para quem paga.
- **Resolve a inadimplência:** é a maior dor que o cron de vencidas e o
  bloqueio tentam resolver hoje.
- **Valor que muda todo mês:** o rateio varia com o número de ativos. O Pix
  Automático aceita valor variável com limite máximo autorizado pelo
  pagador, mas o fluxo de autorização precisa explicar isso bem.
- **Suporte do provedor:** não confirmei suporte na API do Mercado Pago. O
  Inter tem API de Pix Automático (só conta PJ). Precisa de pesquisa
  própria antes de virar spec.

### C. Trocar de provedor (Asaas, Efí, Cora, Inter) — descartado por ora

Com conta de pessoa física, Cora e Inter (só PJ) ficam fora, e o Asaas sai
mais caro que o Mercado Pago no Pix. O Mercado Pago continua como provedor.

- **Asaas:** reúne Pix, boleto, cartão e assinatura numa API só e aceita
  CPF. Mas a tarifa fixa de R$ 1,99 no Pix encarece mensalidades pequenas.
- **Cora e Inter:** são mais baratos no Pix, mas exigem conta PJ (CNPJ ou
  MEI) e outra integração inteira.

Volta a fazer sentido se o grupo abrir um CNPJ ou MEI, ou se o Pix
Automático pesar na decisão.

### D. Manter a baixa manual como saída sem taxa

Dinheiro, ou Pix direto para a chave pessoal do admin (sem taxa para pessoa
física em uso não comercial), continua existindo com baixa manual. É a
opção sem custo para quem não quer pagar taxa, com o trabalho de
conferência ficando com o admin.

## Repasse da taxa

### Regra

- **Rateio inalterado.** `charges.amount_cents` continua sendo a fotografia
  do rateio (ver `BUSINESS_RULES.md`).
- **Pix: taxa somada ao gerar o lote.** Ao gerar o lote (mensalidade ou
  avulso), cada cobrança ganha `pix_fee_cents`, calculado pela tabela de
  taxas do momento. O valor da cobrança que o jogador vê passa a ser
  `amount_cents + pix_fee_cents`, e é esse total que vai na order Pix do
  Mercado Pago.
- **Taxa congelada.** Mudar a tabela depois não altera cobranças já
  geradas, como acontece com o rateio.
- **Cartão (etapa 2): taxa no momento do pagamento.** É calculada sobre o
  rateio, e não sobre o total do Pix, e mostrada antes de pagar.

O jogador vê só o valor final (**R$ 60,60**), sem detalhamento. O admin
anuncia no grupo o valor já com a taxa (a prévia ao gerar o lote mostra
esse valor), e o detalhamento (rateio, taxa e total) aparece só nas telas do
admin.

Para o grupo receber exatamente o valor do rateio, a taxa incide sobre o
total, e não sobre a base:

```text
total = ceil((base + taxa_fixa) / (1 − taxa_percentual))   em centavos
taxa  = total − base
```

Exemplo: base de 6000 centavos com 0,99% dá total = ceil(6000 / 0,9901) =
6060 centavos.

**Arredondamento do Mercado Pago:** a tarifa é arredondada ao centavo.
Evidência de um extrato real: pago R$ 59,52, creditado R$ 58,93, tarifa de
R$ 0,59 (0,99% × 59,52 = 0,589).
- O extrato não distingue "centavo mais próximo" de "para cima". O cálculo
  (`billing.FeeRate`) assume para cima, o pior caso para o grupo.
- Com essa hipótese, o líquido é `floor(total × (1 − p)) − fixo`, e o `ceil`
  acima dá exatamente o menor total. Exemplo: base 5952 dá total 6012; o
  total 6011 deixaria líquido de 5951.
- Se o arredondamento for para o centavo mais próximo, o grupo nunca recebe
  menos que o rateio, só um centavo a mais às vezes.

### Configuração

- **Tabela de taxas** em `settings`, editável em `/admin/configuracoes`:
  - `pix_fee_bps` (padrão 99, ou seja, 0,99%);
  - `pix_fee_fixed_cents` (padrão 0);
  - na etapa 2, as do cartão.
- **Arredondamento** sempre para cima no centavo, para o grupo nunca
  receber menos que a base.
- **Tela do admin:** ao gerar o lote, mostra o rateio por pessoa e o total
  com taxa. Por exemplo: "R$ 60,00 por pessoa · R$ 60,60 no Pix".
- **Baixa manual (dinheiro ou Pix direto):** não passa pelo Mercado Pago e
  não tem taxa. A recomendação é registrar o valor do rateio (decisão
  pendente 3).

### Conciliação

Etapa 1: a Orders API não devolve tarifa nem líquido (só `paid_amount` e
`taxes_amount`), e não está documentado se o id de pagamento dela vale em
`/v1/payments/{id}`. O app guarda a taxa repassada e o total cobrado; a
tarifa efetiva fica para quando houver como consultá-la e testar no sandbox.

A taxa exibida é uma estimativa; a cobrada pelo provedor pode diferir por
centavos ou por mudança de tabela. Em cada pagamento aprovado, a API grava:
- a taxa exibida;
- o total cobrado;
- a taxa efetiva e o líquido recebido, quando o provedor informar.
  Confirmar quais campos a Orders API devolve.

O admin vê por mês quanto foi rateado, cobrado em taxa e recebido líquido.
Uma diferença nunca é cobrada de volta do pagador.

### Aspecto legal

- A Lei 13.455/2017 permite preço diferente por meio de pagamento, desde
  que informado de forma clara. Órgãos de defesa do consumidor discutem se
  repassar a taxa do cartão é prática abusiva pelo CDC.
- Aqui o caso é um rateio de despesa entre membros de um grupo fechado, e
  não venda ao consumidor, mas vale manter a prática transparente:
  - o valor final é anunciado e mostrado antes do pagamento (por decisão
    do grupo, sem a taxa separada);
  - existe sempre uma opção sem taxa (baixa manual).
- Não é parecer jurídico. Se o grupo virar PJ, revisar.

## Modelo de dados (rascunho)

Hoje cada cobrança tem uma única order do Mercado Pago, em colunas de
`charges` (`provider_order_id`, `pix_*`). Com mais de um meio e o valor
dependendo do meio, uma cobrança pode ter várias tentativas.

- Nova tabela `charge_payments`, uma linha por tentativa:
  - `charge_id`, `method` (`pix`, `credit_card`), `provider`,
    `provider_order_id` (único);
  - `base_cents`, `fee_cents`, `total_cents` (congelados ao criar);
  - `status`, `provider_fee_cents` e `net_cents` (preenchidos na aprovação);
  - datas.
- A aprovação de uma tentativa marca a cobrança como `paid`, na mesma
  transação, com idempotência pelo `provider_order_id`, como já é hoje no
  webhook. Uma segunda tentativa aprovada para a mesma cobrança vira caso
  de estorno ou atendimento.
- **Migração aditiva:** as colunas atuais de `charges` continuam sendo lidas
  enquanto a PWA anterior estiver no ar. Pagamentos novos gravam nas duas
  estruturas até a PWA nova estar publicada.
- **Compartilhamento com a participação avulsa:** a proposta
  `2026-10-19-participacao-avulsa.md` também mostra taxa separada e registra
  tentativas de pagamento. As duas devem usar a mesma tabela de tentativas e
  o mesmo cálculo de taxa.

## Plano sugerido

1. **Repasse no Pix atual.** É a menor mudança e resolve o custo que hoje
   sai do bolso do admin:
   - `pix_fee_cents` em `charges` (migration aditiva, padrão 0);
   - cálculo no domínio `billing` ao gerar o lote, com testes;
   - order Pix com o total;
   - detalhamento na PWA;
   - registro de taxa e líquido.

   Cobranças já geradas ficam com taxa 0; o repasse vale a partir do
   próximo lote.
2. **Cartão de crédito pelo Mercado Pago**, com repasse e recebimento em
   30 dias, sobre `charge_payments`.
3. **Pix Automático:** só se o Mercado Pago oferecer pela API para conta de
   pessoa física; o Inter exige PJ. Precisa de pesquisa própria.

Cada etapa segue o ADR de domínio e TDD: o cálculo de taxa e o detalhamento
nascem num pacote `billing` com testes de tabela antes do código.

## Decisões

Tomadas em 2026-10-20:

- O dinheiro cai na conta de **pessoa física** do admin no Mercado Pago.
- A tarifa do Pix da conta é **0,99%**, conferida nos extratos de
  pagamentos anteriores. Esse é o padrão de `pix_fee_bps` (99).
- A **taxa do Pix é repassada** ao pagador e somada automaticamente ao
  valor da mensalidade.
- (2026-10-21) O jogador **não vê a taxa separada**, só o valor final; o
  admin anuncia o valor com a taxa no grupo.

Pendentes:

1. **Cartão:** entra como 2º meio? Se sim, recebendo em 30 dias (taxa
   menor) ou na hora?
2. **Valores reais:** qual é o total mensal e quantos ativos pagam em média,
   para refazer a tabela de custo com números reais?
3. **Baixa manual:** registra só o rateio, sem taxa (recomendado), ou o
   total com a taxa do Pix?
4. **Taxa efetiva:** confirmar se a Orders API devolve a taxa e o líquido
   de cada pagamento; senão, consultar `/v1/payments/{id}` (`fee_details`,
   `net_received_amount`) após a aprovação. A tarifa do cartão na conta
   também precisa ser conferida antes da etapa 2.
5. **Cobrança avulsa de partida:** segue a mesma regra de repasse da
   mensalidade? A recomendação é que sim.
6. **Cobranças pendentes no deploy:** ficam com taxa 0 (recomendado) ou
   ganham a taxa?

## Fontes consultadas (out/2026)

- Mercado Pago, aceitar Pix:
  https://www.mercadopago.com.br/ferramentas-para-vender/aceitar-pix
- Mercado Pago, quanto custa Pix e QR:
  https://www.mercadopago.com.br/blog/quanto-custa-receber-pagamentos-via-pix-e-codigo-qr
- Taxas do checkout Mercado Pago (terceiros):
  https://www.negociosfinanceiros.com/taxas-mercado-pago-checkout-transparente-como-funcionam/
  e https://nacao.digital/blog/checkout-transparente/
- Asaas, cobrança recorrente: https://materiais.asaas.com/cobranca-recorrente
  e comparativo https://quasa.io/pt/media/asaas-ou-iugu-tarifa-publicada-vence-ate-o-volume-pedir-proposta
- Cora, termos de API: https://www.cora.com.br/termos-e-condicoes-de-apis/
- Inter, API Pix: https://inter.co/empresas/api-pix/ e
  https://developers.inter.co/references/pix-automatico
- Pix Automático, regras: https://agenciagov.ebc.com.br/noticias/202609/ajustes-pontuais-organizam-regras-operacionais-do-pix-automatico
- Lei 13.455/2017: https://www.contabeis.com.br/legislacao/1911257/lei-13455-2017/
  e Idec https://idec.org.br/consultas/dicas-e-direitos/preco-diferenciado-no-pagamento-com-carto-pode
