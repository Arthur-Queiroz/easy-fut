// Tipos das telas de admin da participação avulsa (inscrições e caixa),
// espelhando o JSON da API Go. Os tipos públicos ficam em publicGuests.ts.

export type GuestBookingStatus =
  | 'reserved'
  | 'pending_payment'
  | 'confirmed'
  | 'expired'
  | 'cancelled'
  | 'refund_pending'
  | 'refunded'

/** Inscrição na lista do admin (`GET /admin/guest-bookings`), da mais nova para a mais antiga. */
export interface AdminGuestBooking {
  id: string
  match_id: string
  match_date: string
  start_time: string
  venue: string
  name: string
  phone: string // E.164
  email: string
  status: GuestBookingStatus
  base_cents: number
  total_cents: number
  created_at: string
  confirmed_at?: string | null
  refund_reason: string
  confirmation_sent_at: string | null // nulo até o primeiro envio
  vote_link_sent_at: string | null
}

/** Mensagem que o admin pode reenviar (`POST /admin/guest-bookings/{id}/resend`). */
export type GuestMessageKind = 'confirmation' | 'vote_link'

/** Corpo de `PUT /admin/matches/{id}/guest-sale`. */
export interface GuestSaleInput {
  open: boolean
  slots: number
}

export type CashEntryKind = 'guest_payment' | 'refund' | 'expense'

/** Lançamento do caixa. `amount_cents` é sempre positivo; o sinal vem de `kind`. */
export interface CashEntry {
  id: string
  kind: CashEntryKind
  amount_cents: number
  fee_cents: number
  booking_id?: string | null
  description: string
  occurred_on: string // YYYY-MM-DD
  created_at: string
  created_by_name?: string
}

/** `GET /admin/cash?from=&to=`: saldo de todos os lançamentos e a lista do período. */
export interface CashReport {
  balance_cents: number
  entries: CashEntry[]
}

/** Corpo de `POST /admin/cash/expenses`. */
export interface CashExpenseInput {
  description: string
  amount_cents: number
  occurred_on: string
}
