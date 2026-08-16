// Formatação de datas e valores no padrão brasileiro.

const WEEKDAYS_SHORT = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb']
const WEEKDAYS_LONG = ['Domingo', 'Segunda', 'Terça', 'Quarta', 'Quinta', 'Sexta', 'Sábado']
const MONTHS_SHORT = ['JAN', 'FEV', 'MAR', 'ABR', 'MAI', 'JUN', 'JUL', 'AGO', 'SET', 'OUT', 'NOV', 'DEZ']
const MONTHS_LONG = [
  'Janeiro', 'Fevereiro', 'Março', 'Abril', 'Maio', 'Junho',
  'Julho', 'Agosto', 'Setembro', 'Outubro', 'Novembro', 'Dezembro',
]

/** Interpreta 'YYYY-MM-DD' como data local (sem deslocamento de fuso). */
export function parseISODate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function formatCents(cents: number): string {
  return (cents / 100).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

/** 'qui, 16/07' */
export function formatDateShort(iso: string): string {
  const d = parseISODate(iso)
  return `${WEEKDAYS_SHORT[d.getDay()]}, ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
}

/** 'Quinta, 16 de julho' */
export function formatDateLong(iso: string): string {
  const d = parseISODate(iso)
  return `${WEEKDAYS_LONG[d.getDay()]}, ${d.getDate()} de ${MONTHS_LONG[d.getMonth()].toLowerCase()}`
}

export function formatDMY(iso: string): string {
  const d = parseISODate(iso)
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`
}

export function dayOfMonth(iso: string): string {
  return pad(parseISODate(iso).getDate())
}

export function monthShort(iso: string): string {
  return MONTHS_SHORT[parseISODate(iso).getMonth()]
}

export function weekdayShort(iso: string): string {
  return WEEKDAYS_SHORT[parseISODate(iso).getDay()].toUpperCase()
}

/** '2026-07' → 'Julho / 2026' */
export function formatMonth(referenceMonth: string): string {
  const [y, m] = referenceMonth.split('-').map(Number)
  return `${MONTHS_LONG[m - 1]} / ${y}`
}

/** '20:00' → '20h00' */
export function formatHour(time: string): string {
  return time.replace(':', 'h')
}

/** Timestamp → 'hoje, 09h14' / 'seg, 13/07'. */
export function formatTimestamp(ts: string): string {
  const d = new Date(ts)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return `hoje, ${pad(d.getHours())}h${pad(d.getMinutes())}`
  }
  return `${WEEKDAYS_SHORT[d.getDay()]}, ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
}

/** Idade curta de um evento: '2h', '3d', '1sem'. */
export function timeAgo(ts: string): string {
  const seconds = (Date.now() - new Date(ts).getTime()) / 1000
  if (seconds < 3600) return `${Math.max(1, Math.floor(seconds / 60))}min`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`
  const days = Math.floor(seconds / 86400)
  if (days < 7) return `${days}d`
  return `${Math.floor(days / 7)}sem`
}

/** Contagem regressiva '3h12' até um timestamp; null se já passou. */
export function countdown(ts: string): string | null {
  const ms = new Date(ts).getTime() - Date.now()
  if (ms <= 0) return null
  const totalMinutes = Math.floor(ms / 60000)
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours >= 48) return `${Math.floor(hours / 24)}d`
  return `${hours}h${pad(minutes)}`
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/)
  const first = parts[0]?.[0] ?? ''
  const last = parts.length > 1 ? parts[parts.length - 1][0] : (parts[0]?.[1] ?? '')
  return (first + last).toUpperCase()
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** Data local de hoje no formato 'YYYY-MM-DD'. */
export function todayISO(): string {
  const d = new Date()
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** 'YYYY-MM' do mês atual (ou seguinte com offset). */
export function currentMonth(offset = 0): string {
  const d = new Date()
  d.setMonth(d.getMonth() + offset)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`
}
