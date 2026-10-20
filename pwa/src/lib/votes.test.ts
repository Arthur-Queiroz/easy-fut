import { describe, expect, it } from 'vitest'
import type { Match } from '../api/types'
import { categoryBars, filterVotes, matchMatchesQuery, normalizeSearch, playerSummary, type FlatVote, type VoteTally } from './votes'

const vote = (voter: string, category: FlatVote['category'], candidate: string, at = '2026-10-18T12:00:00Z'): FlatVote => ({
  voter_id: voter,
  voter_name: voter.toUpperCase(),
  category,
  candidate_id: candidate,
  candidate_name: candidate.toUpperCase(),
  created_at: at,
})

const tally: VoteTally[] = [
  { category: 'top_scorer', candidate_id: 'julio', name: 'Julio', avatar_color: '#111111', votes: 3 },
  { category: 'top_scorer', candidate_id: 'ana', name: 'Ana', avatar_color: '#222222', votes: 1 },
  { category: 'worst_player', candidate_id: 'caio', name: 'Caio', avatar_color: '#333333', votes: 1 },
  { category: 'worst_player', candidate_id: 'bia', name: 'Bia', avatar_color: '#444444', votes: 1 },
]
const flat: FlatVote[] = [
  vote('ana', 'top_scorer', 'julio'),
  vote('bia', 'top_scorer', 'julio'),
  vote('caio', 'top_scorer', 'julio'),
  vote('julio', 'top_scorer', 'ana'),
  vote('ana', 'worst_player', 'caio'),
  vote('julio', 'worst_player', 'bia'),
]

describe('categoryBars', () => {
  it('ordena por votos, calcula a fatia e lista quem votou', () => {
    const { total, bars } = categoryBars(tally, flat, 'top_scorer')
    expect(total).toBe(4)
    expect(bars.map((b) => [b.id, b.votes, b.share])).toEqual([
      ['julio', 3, 0.75],
      ['ana', 1, 0.25],
    ])
    expect(bars[0].voters).toEqual(['ANA', 'BIA', 'CAIO'])
    expect(bars.map((b) => b.winner)).toEqual([true, false])
  })

  it('empate marca todos os líderes e desempata a ordem pelo nome', () => {
    const { bars } = categoryBars(tally, flat, 'worst_player')
    expect(bars.map((b) => b.name)).toEqual(['Bia', 'Caio'])
    expect(bars.every((b) => b.winner)).toBe(true)
  })

  it('categoria sem votos não tem vencedor', () => {
    expect(categoryBars(tally, flat, 'best_goal')).toEqual({ total: 0, bars: [] })
  })
})

describe('filterVotes', () => {
  it('jogador filtra votos dados e recebidos', () => {
    const got = filterVotes(flat, { playerId: 'julio' })
    expect(got).toHaveLength(5) // 3 recebidos em top_scorer + 2 dados
  })

  it('candidato e categoria vêm do clique na barra', () => {
    const got = filterVotes(flat, { category: 'top_scorer', candidateId: 'julio' })
    expect(got.map((v) => v.voter_id)).toEqual(['ana', 'bia', 'caio'])
  })

  it('busca ignora acento e caixa, no eleitor ou no candidato', () => {
    const accented = [...flat, { ...vote('jose', 'best_goal', 'ana'), voter_name: 'José Antônio' }]
    expect(filterVotes(accented, { query: 'jose antonio' })).toHaveLength(1)
    expect(filterVotes(accented, { query: 'cai' }).map((v) => v.candidate_id)).toEqual(['julio', 'caio'])
  })
})

describe('playerSummary', () => {
  it('separa em quem o jogador votou e de quem recebeu, por categoria', () => {
    const s = playerSummary(flat, 'julio')
    expect(s.given.top_scorer.map((v) => v.candidate_name)).toEqual(['ANA'])
    expect(s.given.worst_player.map((v) => v.candidate_name)).toEqual(['BIA'])
    expect(s.received.top_scorer.map((v) => v.voter_name)).toEqual(['ANA', 'BIA', 'CAIO'])
    expect(s.received.best_goal).toEqual([])
  })
})

describe('busca de partida', () => {
  const match = { match_date: '2026-10-18', venue: 'Arena Futiba 1', status: 'finished' } as Match

  it('acha por data em vários formatos, local e status', () => {
    for (const q of ['18/10', '18/10/2026', '2026-10-18', 'futiba', 'arena 1', 'encerrada', '']) {
      expect(matchMatchesQuery(match, q), q).toBe(true)
    }
    expect(matchMatchesQuery(match, '19/10')).toBe(false)
  })

  it('normalizeSearch tira acento e espaço extra', () => {
    expect(normalizeSearch('  Votação  ABERTA ')).toBe('votacao aberta')
  })
})
