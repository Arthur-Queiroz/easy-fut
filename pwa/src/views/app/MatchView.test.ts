import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import MatchView from './MatchView.vue'

const get = vi.hoisted(() => vi.fn())
vi.mock('../../api/client', () => ({ api: { get }, errorMessage: () => 'Erro de conexão' }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))

describe('perfil na próxima partida', () => {
  it('abre o perfil ao clicar em um jogador confirmado', async () => {
    get.mockImplementation((path: string) => Promise.resolve({ data: path === '/matches/next' ? {
      match: { id: 'match-1', status: 'open', match_date: '2026-11-01', start_time: '11:00', end_time: '12:30', venue: 'Arena', confirmation_deadline: '2026-10-31T10:00:00Z' },
      my_response: 'no_response',
      confirmations: [{ user_id: 'user-1', name: 'Ana', avatar_color: '#3B82A0', kind: 'member', role: 'player', response: 'going' }],
    } : {
      id: 'user-1', name: 'Ana', avatar_color: '#3B82A0', role: 'player', created_at: '2026-07-10T12:00:00Z',
      stats: { matches_played: 5, top_scorer_count: 1, worst_count: 0 },
    } }))
    const wrapper = mount(MatchView, {
      global: { stubs: { MobileShell: { template: '<div><slot name="header" /><slot /></div>' }, Teleport: true } },
    })
    await flushPromises()
    await wrapper.get('button[aria-label="Ver perfil de Ana"]').trigger('click')
    await flushPromises()
    expect(get).toHaveBeenCalledWith('/users/user-1/profile')
    expect(wrapper.text()).toContain('No grupo desde jul/2026')
    wrapper.unmount()
  })
})
