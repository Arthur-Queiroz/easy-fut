// Valores de cobrança mostrados na PWA. A taxa do Pix é calculada e
// congelada pela API ao gerar o lote; aqui só se escolhe qual valor exibir.
// O jogador vê só o valor final: a taxa não é detalhada para ele, só para o admin.
import type { Charge } from '../api/types'

type ChargeValues = Pick<Charge, 'status' | 'amount_cents' | 'pix_fee_cents' | 'pix_total_cents'>

/** O que o jogador paga pelo app: o rateio mais a taxa do Pix repassada. */
export function payableCents(c: ChargeValues): number {
  return c.pix_total_cents
}

/** Valor pago: Pix pelo Mercado Pago inclui a taxa; baixa manual registra só o rateio. */
export function paidCents(c: ChargeValues): number {
  return c.status === 'paid' ? c.pix_total_cents : c.amount_cents
}

/** Configuração pix_fee_bps (99) → texto da tela (0,99). */
export function formatBasisPoints(raw: string | undefined): string {
  const bps = Number(raw)
  if (raw === undefined || raw.trim() === '' || !Number.isFinite(bps)) return ''
  return (bps / 100).toFixed(2).replace('.', ',')
}

/** Porcentagem digitada (0,99) → pix_fee_bps (99). Texto inválido segue como está para a API recusar. */
export function feePercentToBasisPoints(input: string): string {
  const percent = Number(input.trim().replace(',', '.'))
  if (input.trim() === '' || !Number.isFinite(percent)) return input
  return String(Math.round(percent * 100))
}
