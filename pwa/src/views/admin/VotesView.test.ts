import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), replace: vi.fn() }))
vi.mock('../../api/client', () => ({ api: { get: mocks.get }, errorMessage: () => 'Erro de conexão' }))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: mocks.replace }),
}))
import VotesView from './VotesView.vue'

const match = (id: string, date: string, venue: string, status: string, going: number) => ({
  id, match_date: date, venue, status, going_count: going, start_time: '11:00', end_time: '12:30',
  voting_closes_at: '2026-10-20T14:00:00Z',
})
const matches = [
  match('m1', '2026-10-18', 'Arena Futiba 1', 'finished', 4),
  match('m2', '2026-10-25', 'Quadra Norte', 'voting', 3),
  match('m3', '2026-11-01', 'Arena Futiba 1', 'open', 0),
]
const going = (id: string, name: string) => ({ user_id: id, name, avatar_color: '#123456', role: 'player', response: 'going' })
const v = (voter: string, category: string, candidate: string) => ({
  voter_id: voter, voter_name: voter[0].toUpperCase() + voter.slice(1), category,
  candidate_id: candidate, candidate_name: candidate[0].toUpperCase() + candidate.slice(1), created_at: '2026-10-18T15:00:00Z',
})
const m1Votes = {
  tally: [
    { category: 'top_scorer', candidate_id: 'julio', name: 'Julio', avatar_color: '#123456', votes: 3 },
    { category: 'top_scorer', candidate_id: 'ana', name: 'Ana', avatar_color: '#123456', votes: 1 },
    { category: 'worst_player', candidate_id: 'caio', name: 'Caio', avatar_color: '#123456', votes: 2 },
  ],
  flat: [
    v('ana', 'top_scorer', 'julio'), v('bia', 'top_scorer', 'julio'), v('caio', 'top_scorer', 'julio'),
    v('bia', 'top_scorer', 'ana'), v('ana', 'worst_player', 'caio'), v('bia', 'worst_player', 'caio'),
  ],
}

function respond(url: string) {
  const data: Record<string, unknown> = {
    '/matches': matches,
    '/admin/stats/votes': { matches: [
      { match_id: 'm2', match_date: '25/10', status: 'voting', voters: 0, votes: 0 },
      { match_id: 'm1', match_date: '18/10', status: 'finished', voters: 3, votes: 6 },
    ] },
    '/admin/matches/m1/votes': m1Votes,
    '/admin/matches/m2/votes': { tally: [], flat: [] },
    '/matches/m1': { confirmations: [going('ana', 'Ana'), going('bia', 'Bia'), going('caio', 'Caio'), going('julio', 'Julio'), { ...going('davi', 'Davi'), response: 'not_going' }] },
    '/matches/m2': { confirmations: [going('ana', 'Ana'), going('bia', 'Bia'), going('caio', 'Caio')] },
  }
  return Promise.resolve({ data: data[url] })
}

function mountView() {
  return mount(VotesView, {
    attachTo: document.body,
    global: { stubs: { AdminLayout: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }) } },
  })
}
const button = (w: ReturnType<typeof mountView>, text: string) => w.findAll('button').find((b) => b.text().includes(text))
const picker = (w: ReturnType<typeof mountView>, label: string) => w.find(`button[aria-label="${label}"]`)
const tableRows = (w: ReturnType<typeof mountView>) => w.findAll('[data-test="vote-row"]')

beforeEach(() => {
  vi.clearAllMocks()
  route.query = {}
  mocks.get.mockImplementation(respond)
})

describe('dashboard de votos', () => {
  it('abre na partida mais recente com votação e mostra a participação', async () => {
    route.query = {}
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/admin/matches/m2/votes')
    expect(w.text()).toContain('0 de 3 participantes votaram')
    w.unmount()
  })

  it('KPIs, quem falta votar e gráficos da partida escolhida pela URL', async () => {
    route.query = { partida: 'm1' }
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('75%')
    expect(w.text()).toContain('3 de 4 participantes votaram')
    expect(w.find('[data-test="not-voted"]').text()).toContain('Julio')
    expect(w.find('[data-test="not-voted"]').text()).not.toContain('Davi') // não jogou
    // líder do artilheiro com o valor escrito na barra
    const top = w.find('[data-test="chart-top_scorer"]')
    expect(top.text()).toContain('Julio')
    expect(top.text()).toContain('Vencedor')
    expect(tableRows(w)).toHaveLength(6)
    w.unmount()
  })

  it('a busca de partida filtra por local e data e não lista partidas sem votação', async () => {
    route.query = { partida: 'm1' }
    const w = mountView()
    await flushPromises()
    await picker(w, 'Partida').trigger('click')
    const options = () => w.findAll('[role="option"]').map((o) => o.text())
    expect(options()).toHaveLength(2) // m3 ainda está em confirmação
    await w.find('input[aria-label="Buscar partida"]').setValue('norte')
    expect(options()).toHaveLength(1)
    expect(options()[0]).toContain('Quadra Norte')
    await w.find('input[aria-label="Buscar partida"]').setValue('18/10')
    expect(options()[0]).toContain('Arena Futiba 1')
    await w.findAll('[role="option"]')[0].trigger('click')
    await flushPromises()
    expect(mocks.replace).toHaveBeenCalledWith({ query: { partida: 'm1' } })
    w.unmount()
  })

  it('escolher um jogador filtra os votos dele naquela partida', async () => {
    route.query = { partida: 'm1' }
    const w = mountView()
    await flushPromises()
    await picker(w, 'Jogador').trigger('click')
    await w.find('input[aria-label="Buscar jogador"]').setValue('bia')
    await w.findAll('[role="option"]')[0].trigger('click')
    await flushPromises()
    const rows = tableRows(w).map((r) => r.text())
    expect(rows).toHaveLength(3) // Bia votou 3 vezes e não recebeu votos
    expect(rows.every((r) => r.includes('Bia'))).toBe(true)
    const panel = w.find('[data-test="player-panel"]')
    expect(panel.text()).toContain('Votou em')
    expect(panel.text()).toContain('Julio')
    expect(mocks.replace).toHaveBeenLastCalledWith({ query: { partida: 'm1', jogador: 'bia' } })
    w.unmount()
  })

  it('clicar na barra mostra quem votou naquele jogador', async () => {
    route.query = { partida: 'm1' }
    const w = mountView()
    await flushPromises()
    await w.find('[data-test="chart-worst_player"] button').trigger('click')
    expect(tableRows(w)).toHaveLength(2)
    expect(w.text()).toContain('Perna de pau: votos em Caio')
    await button(w, 'Perna de pau: votos em Caio')!.trigger('click') // limpa o filtro
    expect(tableRows(w)).toHaveLength(6)
    w.unmount()
  })
})
