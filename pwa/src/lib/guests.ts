// Regras de exibição da participação avulsa no admin: rótulos de status,
// sinal dos lançamentos do caixa e conversão de valores digitados em reais.
// A API é a dona das regras; aqui só se escolhe como mostrar.
import type { CashEntry, CashEntryKind, GuestBookingStatus } from '../api/adminGuests'

type Tone = 'success' | 'warn' | 'danger' | 'info' | 'neutral'

export const GUEST_STATUS_INFO: Record<GuestBookingStatus, { label: string; tone: Tone }> = {
  reserved: { label: 'Reservada', tone: 'warn' },
  pending_payment: { label: 'Aguardando Pix', tone: 'warn' },
  confirmed: { label: 'Confirmada', tone: 'success' },
  expired: { label: 'Expirada', tone: 'neutral' },
  cancelled: { label: 'Cancelada', tone: 'neutral' },
  refund_pending: { label: 'Reembolsar', tone: 'danger' },
  refunded: { label: 'Reembolsada', tone: 'info' },
}

/** Ordem dos estados no filtro da tela de avulsos. */
export const GUEST_STATUSES = Object.keys(GUEST_STATUS_INFO) as GuestBookingStatus[]

/** Entrada soma, estorno e despesa subtraem. O texto acompanha a cor. */
export const CASH_KIND_INFO: Record<CashEntryKind, { label: string; sign: 1 | -1 }> = {
  guest_payment: { label: 'Entrada · vaga avulsa', sign: 1 },
  refund: { label: 'Estorno', sign: -1 },
  expense: { label: 'Despesa', sign: -1 },
}

export function signedCashCents(entry: Pick<CashEntry, 'kind' | 'amount_cents'>): number {
  return CASH_KIND_INFO[entry.kind].sign * entry.amount_cents
}

/** Entradas e saídas somadas de uma lista de lançamentos. */
export function cashTotals(entries: Pick<CashEntry, 'kind' | 'amount_cents'>[]): { inCents: number; outCents: number } {
  let inCents = 0
  let outCents = 0
  for (const e of entries) {
    if (CASH_KIND_INFO[e.kind].sign > 0) inCents += e.amount_cents
    else outCents += e.amount_cents
  }
  return { inCents, outCents }
}

/**
 * Valor digitado em reais → centavos. Aceita "12,50", "12.50", "12",
 * "1.234,56" e "R$ 12,50". Devolve null se o texto não for um valor válido.
 */
export function parseReais(input: string): number | null {
  let s = input.replace(/R\$/i, '').replace(/\s/g, '')
  if (s === '') return null
  if (s.includes(',')) {
    s = s.replace(/\./g, '').replace(',', '.')
  } else if (/^\d{1,3}(\.\d{3})+$/.test(s)) {
    s = s.replace(/\./g, '') // "1.234" sem vírgula é milhar
  }
  if (!/^\d+(\.\d{1,2})?$/.test(s)) return null
  return Math.round(Number(s) * 100)
}

/** Centavos (número ou texto da configuração) → texto do campo em reais ("25,00"). */
export function centsToReaisInput(cents: number | string | undefined): string {
  if (cents === undefined || String(cents).trim() === '') return ''
  const n = Number(cents)
  if (!Number.isFinite(n)) return ''
  return (n / 100).toFixed(2).replace('.', ',')
}

/** Campo da configuração em reais → centavos em texto. Texto inválido segue como está para a API recusar. */
export function reaisInputToCents(input: string): string {
  const cents = parseReais(input)
  return cents === null ? input : String(cents)
}
