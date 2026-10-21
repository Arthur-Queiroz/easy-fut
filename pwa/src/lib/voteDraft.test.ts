import { describe, expect, it } from 'vitest'
import { changedVotes, markedVotes, toggleCandidate } from './voteDraft'
import { formatRemaining } from './format'

describe('voteDraft', () => {
  it('marcado usa o rascunho da categoria e cai no gravado nas outras', () => {
    expect(markedVotes({ top_scorer: ['a'] }, { top_scorer: ['b'], worst_player: ['c'] })).toEqual({
      top_scorer: ['a'],
      worst_player: ['c'],
      best_goal: [],
    })
  })

  it('só categorias diferentes do gravado entram no envio, sem ligar para a ordem', () => {
    const persisted = { top_scorer: ['a', 'b'], worst_player: ['c'] }
    expect(changedVotes({ top_scorer: ['b', 'a'], worst_player: [] }, persisted)).toEqual({ worst_player: [] })
    expect(changedVotes({}, persisted)).toEqual({})
  })

  it('toggle respeita o limite de 2', () => {
    expect(toggleCandidate([], 'a')).toEqual(['a'])
    expect(toggleCandidate(['a', 'b'], 'a')).toEqual(['b'])
    expect(toggleCandidate(['a', 'b'], 'c')).toBeNull()
  })
})

describe('formatRemaining', () => {
  const now = Date.parse('2026-10-21T12:00:00Z')
  it('formata mm:ss e h:mm:ss, sem negativo', () => {
    expect(formatRemaining('2026-10-21T12:29:05Z', now)).toBe('29:05')
    expect(formatRemaining('2026-10-21T13:02:03Z', now)).toBe('1:02:03')
    expect(formatRemaining('2026-10-21T11:00:00Z', now)).toBe('00:00')
  })
})
