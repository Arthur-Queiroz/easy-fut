// Rascunho de votos (FUT-003) da página de voto do avulso: toques alteram o
// rascunho localmente e "Enviar votos" manda só as categorias alteradas.
import type { VoteCategory } from '../api/types'

export const MAX_VOTES_PER_CATEGORY = 2

export const GUEST_VOTE_CATEGORIES: { key: VoteCategory; label: string; icon: string; warnColor: boolean }[] = [
  { key: 'top_scorer', label: 'Vote no artilheiro', icon: 'trophy', warnColor: false },
  { key: 'worst_player', label: 'Vote no perna de pau', icon: '', warnColor: true },
  { key: 'best_goal', label: 'Vote no melhor gol', icon: 'ball', warnColor: false },
]

export type VoteMap = Partial<Record<VoteCategory, string[]>>

const sameSet = (a: string[], b: string[]) => [...a].sort().join(',') === [...b].sort().join(',')

/** O que aparece marcado: o rascunho da categoria, se houver; senão, o que já foi gravado. */
export function markedVotes(draft: VoteMap, persisted: VoteMap): Record<VoteCategory, string[]> {
  const out = {} as Record<VoteCategory, string[]>
  for (const { key } of GUEST_VOTE_CATEGORIES) out[key] = draft[key] ?? persisted[key] ?? []
  return out
}

/** Categorias em que o marcado difere do gravado, com a lista completa que deve valer. */
export function changedVotes(draft: VoteMap, persisted: VoteMap): VoteMap {
  const marks = markedVotes(draft, persisted)
  const changed: VoteMap = {}
  for (const { key } of GUEST_VOTE_CATEGORIES) {
    if (!sameSet(marks[key], persisted[key] ?? [])) changed[key] = marks[key]
  }
  return changed
}

/** Marca ou desmarca um candidato; devolve null se passaria do limite. */
export function toggleCandidate(current: string[], candidateId: string): string[] | null {
  if (current.includes(candidateId)) return current.filter((id) => id !== candidateId)
  if (current.length >= MAX_VOTES_PER_CATEGORY) return null
  return [...current, candidateId]
}
