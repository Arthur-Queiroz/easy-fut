// Endpoints públicos da participação avulsa (sem sessão). Contratos em
// .agents/specs/design/2026-10-21-participacao-avulsa-plano.md, seção HTTP.
import { api, errorMessage } from './client'
import type { VoteCategory } from './types'

/** Partida com venda avulsa aberta (`GET /public/guest-matches`). */
export interface GuestMatch {
  match_id: string
  match_date: string
  start_time: string
  end_time: string
  venue: string
  address: string
  total_cents: number // já com a taxa do Pix embutida
  slots_left: number
}

export interface GuestBookingInput {
  match_id: string
  name: string
  phone: string
  email: string
}

export type GuestBookingStatus =
  | 'reserved'
  | 'pending_payment'
  | 'confirmed'
  | 'expired'
  | 'cancelled'
  | 'refund_pending'
  | 'refunded'

/** Resposta 201 de `POST /public/guest-bookings`. O token vem só aqui. */
export interface GuestBookingCreated {
  booking_id: string
  access_token: string
  status: GuestBookingStatus
  total_cents: number
  expires_at: string
  pix_payload: string
  pix_qr_code_base64: string
  pix_ticket_url: string
}

/** Partida da inscrição: o contrato só fixa "com endereço". */
export interface GuestBookingMatch {
  match_date: string
  start_time?: string
  end_time?: string
  venue?: string
  address?: string
}

/** `GET /public/guest-bookings/{id}`; Pix só enquanto o pagamento está pendente. */
export interface GuestBooking {
  status: GuestBookingStatus
  total_cents: number
  expires_at: string
  match: GuestBookingMatch
  pix_payload?: string
  pix_qr_code_base64?: string
  pix_ticket_url?: string
}

export interface GuestVoteParticipant {
  user_id: string
  name: string
  avatar_color: string
}

export interface GuestVoteMatch {
  match_date: string
  start_time?: string
  venue?: string
}

/** `GET /public/guest-vote/{token}`. */
export interface GuestVote {
  match: GuestVoteMatch
  participants: GuestVoteParticipant[]
  my_votes: Partial<Record<VoteCategory, string[]>> | null
  voting_closes_at?: string
}

export type GuestVotes = Partial<Record<VoteCategory, string[]>>

export async function listGuestMatches(): Promise<GuestMatch[]> {
  const { data } = await api.get<GuestMatch[] | null>('/public/guest-matches')
  return data ?? []
}

export async function createGuestBooking(input: GuestBookingInput): Promise<GuestBookingCreated> {
  const { data } = await api.post<GuestBookingCreated>('/public/guest-bookings', input)
  return data
}

export async function getGuestBooking(id: string, token: string): Promise<GuestBooking> {
  const { data } = await api.get<GuestBooking>(`/public/guest-bookings/${encodeURIComponent(id)}`, {
    headers: { 'X-Booking-Token': token },
  })
  return data
}

export async function getGuestVote(token: string): Promise<GuestVote> {
  const { data } = await api.get<GuestVote>(`/public/guest-vote/${encodeURIComponent(token)}`)
  return data
}

export async function submitGuestVotes(token: string, votes: GuestVotes): Promise<void> {
  await api.put(`/public/guest-vote/${encodeURIComponent(token)}/votes`, { votes })
}

/** Status HTTP de um erro do Axios (ou undefined se não houve resposta). */
export function httpStatus(error: unknown): number | undefined {
  const status = (error as { response?: { status?: unknown } } | null)?.response?.status
  return typeof status === 'number' ? status : undefined
}

/** Mensagem de erro das telas públicas: 429 ganha texto próprio. */
export function publicErrorMessage(error: unknown): string {
  if (httpStatus(error) === 429) return 'Muitas tentativas. Espera uns minutinhos e tenta de novo.'
  return errorMessage(error)
}

// Token de acompanhamento das inscrições feitas neste aparelho:
// { [booking_id]: access_token }. Ele só é devolvido na criação, então se
// perder não dá pra consultar a inscrição de novo.
const BOOKINGS_KEY = 'easyfut-guest-bookings'

function readTokens(): Record<string, string> {
  try {
    const raw = localStorage.getItem(BOOKINGS_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : {}
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Record<string, string>) : {}
  } catch {
    return {}
  }
}

export function saveBookingToken(bookingId: string, token: string): void {
  try {
    localStorage.setItem(BOOKINGS_KEY, JSON.stringify({ ...readTokens(), [bookingId]: token }))
  } catch {
    /* storage indisponível: o acompanhamento só funciona nesta navegação */
  }
}

export function readBookingToken(bookingId: string): string | null {
  const token = readTokens()[bookingId]
  return typeof token === 'string' && token ? token : null
}
