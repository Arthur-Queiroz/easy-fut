// Cálculos do dashboard de votos do admin (FUT-005). Funções puras sobre o
// que a API devolve em /admin/matches/{id}/votes, para a view só exibir.
import type { Match, VoteCategory } from '../api/types'
import { formatDMY } from './format'
import { normalizeSearch } from './search'

export { normalizeSearch }

export interface VoteTally {
  category: VoteCategory
  candidate_id: string
  name: string
  avatar_color: string
  votes: number
}

export interface FlatVote {
  voter_id: string
  voter_name: string
  category: VoteCategory
  candidate_id: string
  candidate_name: string
  created_at: string
}

export interface MatchVoteSummary {
  match_id: string
  match_date: string // DD/MM
  status: string
  voters: number
  votes: number
}

// Uma coluna do gráfico de participação entre partidas.
export interface ParticipationItem {
  id: string
  label: string // DD/MM
  venue: string
  voters: number
  participants: number
}

export const VOTE_CATEGORIES: VoteCategory[] = ['top_scorer', 'worst_player', 'best_goal']

export const VOTE_CATEGORY_LABELS: Record<VoteCategory, string> = {
  top_scorer: 'Artilheiro',
  worst_player: 'Perna de pau',
  best_goal: 'Melhor gol',
}

// Mesmo limite de matches.MaxVotesPerCategory na API.
export const MAX_VOTES_PER_CATEGORY = 2

export interface CandidateBar {
  id: string
  name: string
  avatarColor: string
  votes: number
  share: number // fração dos votos da categoria, 0–1
  voters: string[] // nomes de quem votou
  winner: boolean // empatado ou sozinho no topo
}

const byName = (a: string, b: string) => a.localeCompare(b, 'pt-BR')

export function categoryBars(tally: VoteTally[], flat: FlatVote[], category: VoteCategory): { total: number; bars: CandidateBar[] } {
  const rows = tally.filter((t) => t.category === category)
  const total = rows.reduce((sum, r) => sum + r.votes, 0)
  const max = Math.max(0, ...rows.map((r) => r.votes))
  const bars = rows
    .map((r) => ({
      id: r.candidate_id,
      name: r.name,
      avatarColor: r.avatar_color,
      votes: r.votes,
      share: total ? r.votes / total : 0,
      voters: flat
        .filter((v) => v.category === category && v.candidate_id === r.candidate_id)
        .map((v) => v.voter_name)
        .sort(byName),
      winner: max > 0 && r.votes === max,
    }))
    .sort((a, b) => b.votes - a.votes || byName(a.name, b.name))
  return { total, bars }
}

export interface VoteFilter {
  query?: string
  category?: VoteCategory | 'all'
  playerId?: string // votos dados ou recebidos
  candidateId?: string // só recebidos (clique na barra)
}

export function filterVotes(flat: FlatVote[], f: VoteFilter): FlatVote[] {
  const q = normalizeSearch(f.query ?? '')
  return flat.filter(
    (v) =>
      (!f.category || f.category === 'all' || v.category === f.category) &&
      (!f.playerId || v.voter_id === f.playerId || v.candidate_id === f.playerId) &&
      (!f.candidateId || v.candidate_id === f.candidateId) &&
      (!q || normalizeSearch(`${v.voter_name} ${v.candidate_name}`).includes(q)),
  )
}

export interface PlayerSummary {
  given: Record<VoteCategory, FlatVote[]>
  received: Record<VoteCategory, FlatVote[]>
}

export function playerSummary(flat: FlatVote[], playerId: string): PlayerSummary {
  const empty = () => ({ top_scorer: [], worst_player: [], best_goal: [] }) as Record<VoteCategory, FlatVote[]>
  const summary = { given: empty(), received: empty() }
  for (const v of flat) {
    if (v.voter_id === playerId) summary.given[v.category]?.push(v)
    if (v.candidate_id === playerId) summary.received[v.category]?.push(v)
  }
  return summary
}

export const MATCH_STATUS_LABELS: Record<string, string> = {
  open: 'Confirmações abertas',
  closed: 'Confirmações fechadas',
  teams_drawn: 'Times sorteados',
  voting: 'Votação aberta',
  finished: 'Encerrada',
  cancelled: 'Cancelada',
}

// A busca de partida aceita 18/10, 18/10/2026, 2026-10-18, local e status.
export function matchMatchesQuery(m: Pick<Match, 'match_date' | 'venue' | 'status'>, query: string): boolean {
  const q = normalizeSearch(query)
  if (!q) return true
  const dmy = formatDMY(m.match_date)
  const text = normalizeSearch(`${dmy} ${dmy.slice(0, 5)} ${m.match_date} ${m.venue} ${MATCH_STATUS_LABELS[m.status] ?? ''}`)
  return q.split(' ').every((word) => text.includes(word))
}
