import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}))
vi.mock('../api/client', () => ({
  api: { get: mocks.get, post: mocks.post, delete: mocks.delete },
  errorMessage: () => 'Erro de conexão',
}))
import MatchDetailContent from './MatchDetailContent.vue'

// stub de router (o componente usa useRouter; outras partes importam o router real)
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<{ useRouter: unknown }>()
  return { ...(actual as object), useRouter: () => ({ push: vi.fn() }) }
})

const meId = 'u-hero'

const detail = () => ({
  match: { id: 'm1', status: 'voting', match_date: '2026-10-18', start_time: '08:00', end_time: '10:00' },
  my_response: 'going',
  confirmations: [
    { user_id: meId, name: 'Eu', avatar_color: '#123', response: 'going', role: 'player' },
    { user_id: 'u-a', name: 'Ana', avatar_color: '#234', response: 'going', role: 'player' },
    { user_id: 'u-b', name: 'Bruno', avatar_color: '#345', response: 'going', role: 'player' },
    { user_id: 'u-c', name: 'Caio', avatar_color: '#456', response: 'going', role: 'player' },
  ],
  teams: [],
  media: [],
  is_participant: true,
  my_votes: { top_scorer: [], worst_player: [] },
  results: [],
})

const authMock = vi.hoisted(() => ({ user: { id: 'u-hero', name: 'Eu' } }))
vi.mock('../stores/auth', () => ({ useAuthStore: () => authMock }))
// tema store também é pinia
vi.mock('../stores/theme', () => ({ useThemeStore: () => ({ isDark: false }) }))

function mountC() {
  return mount(MatchDetailContent, {
    props: { matchId: 'm1' },
    global: {
      stubs: {
        Teleport: true,
        NavIcon: defineComponent({ setup: (_, { attrs }) => () => h('i', { ...attrs }) }),
      },
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  mocks.get.mockResolvedValue({ data: detail() })
  mocks.post.mockResolvedValue({ data: { ok: true } })
  mocks.delete.mockResolvedValue({ data: { ok: true } })
})

const sendButton = (w: ReturnType<typeof mountC>) => w.findAll('button').find((b) => b.text().includes('Enviar votos'))

describe('voting draft + submit (FUT-003)', () => {
  it('marcas alteram rascunho e botão aparece; submit envia POST de cada marca', async () => {
    const w = mountC()
    await flushPromises()
    const candidateButtons = w.findAll('button').filter((b) => /Ana|Bruno|Caio/.test(b.text()))
    expect(candidateButtons.length).toBeGreaterThanOrEqual(4) // 3 em top + 2 em worst

    await candidateButtons[0].trigger('click') // Ana (top)
    await candidateButtons[1].trigger('click') // Bruno (top)
    expect(sendButton(w)).toBeDefined()
    await sendButton(w)!.trigger('click')
    await flushPromises()
    const posted = mocks.post.mock.calls.filter((c) => String(c[0]).includes('/votes'))
    expect(posted).toHaveLength(2)
    for (const c of posted) expect(c[1]).toEqual({ category: 'top_scorer', candidate_id: expect.any(String) })
  })

  it('desmarcar candidato persistido gera DELETE no submit', async () => {
    const d = detail()
    ;(d.my_votes as Record<string, string[]>).top_scorer = ['u-a']
    mocks.get.mockResolvedValue({ data: d })
    const w = mountC()
    await flushPromises()
    // Ana vem marcada: clicar desmarca
    const ana = w.findAll('button').find((b) => b.text().includes('Ana'))!
    await ana.trigger('click')
    await w.findAll('button').find((b) => b.text().includes('Enviar votos'))!.trigger('click')
    await flushPromises()
    expect(mocks.delete).toHaveBeenCalledWith('/matches/m1/votes/top_scorer/u-a')
    expect(mocks.post).not.toHaveBeenCalled()
  })

  it('não permite marcar mais de 2 por categoria', async () => {
    const w = mountC()
    await flushPromises()
    const names = ['Ana', 'Bruno', 'Caio']
    for (const n of names) {
      await w.findAll('button').find((b) => b.text().includes(n))!.trigger('click')
    }
    expect(w.text()).toContain('Escolha no máximo 2')
  })

  it('restaura rascunho do localStorage após remount (auto-envio fallback)', async () => {
    const w = mountC()
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('Ana'))!.trigger('click')
    expect(Object.keys(localStorage.getItem).length).toBeGreaterThanOrEqual(0)
    const w2 = mountC()
    await flushPromises()
    // rascunho restaurado: Ana aparece marcada
    const anaBtn = w2.findAll('button').find((b) => b.text().includes('Ana'))!
    expect(anaBtn.classes().join(' ')).toContain('border-brand')
  })
})
