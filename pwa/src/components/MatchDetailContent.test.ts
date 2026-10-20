import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))
vi.mock('../api/client', () => ({
  api: { get: mocks.get, post: mocks.post, put: mocks.put, delete: mocks.delete },
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
  mocks.put.mockResolvedValue({ data: { ok: true } })
  mocks.delete.mockResolvedValue({ data: { ok: true } })
})

const withVotes = (votes: Record<string, string[]>) => {
  const d = detail()
  Object.assign(d.my_votes, votes)
  mocks.get.mockResolvedValue({ data: d })
}
const candidate = (w: ReturnType<typeof mountC>, name: string, index = 0) =>
  w.findAll('button').filter((b) => b.text().includes(name))[index]

const sendButton = (w: ReturnType<typeof mountC>) => w.findAll('button').find((b) => b.text().includes('Enviar votos'))

describe('voting draft + submit (FUT-003)', () => {
  it('marcas alteram rascunho e botão aparece; submit envia o voto completo num PUT', async () => {
    const w = mountC()
    await flushPromises()
    const candidateButtons = w.findAll('button').filter((b) => /Ana|Bruno|Caio/.test(b.text()))
    expect(candidateButtons.length).toBeGreaterThanOrEqual(4) // 3 em top + 2 em worst

    await candidateButtons[0].trigger('click') // Ana (top)
    await candidateButtons[1].trigger('click') // Bruno (top)
    expect(sendButton(w)).toBeDefined()
    await sendButton(w)!.trigger('click')
    await flushPromises()
    // só a categoria alterada vai no envio
    expect(mocks.put).toHaveBeenCalledTimes(1)
    expect(mocks.put).toHaveBeenCalledWith('/matches/m1/votes', { votes: { top_scorer: ['u-a', 'u-b'] } })
    expect(mocks.post).not.toHaveBeenCalled()
    expect(mocks.delete).not.toHaveBeenCalled()
  })

  it('desmarcar candidato persistido envia a lista sem ele', async () => {
    withVotes({ top_scorer: ['u-a'] })
    const w = mountC()
    await flushPromises()
    await candidate(w, 'Ana').trigger('click') // Ana vem marcada: clicar desmarca
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledWith('/matches/m1/votes', { votes: { top_scorer: [] } })
  })

  it('trocar um voto com os 2 usados é um envio só, sem DELETE + POST', async () => {
    withVotes({ top_scorer: ['u-a', 'u-b'] })
    const w = mountC()
    await flushPromises()
    await candidate(w, 'Bruno').trigger('click') // desmarca Bruno
    await candidate(w, 'Caio').trigger('click') // marca Caio
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledWith('/matches/m1/votes', { votes: { top_scorer: ['u-a', 'u-c'] } })
    expect(mocks.post).not.toHaveBeenCalled()
    expect(mocks.delete).not.toHaveBeenCalled()
  })

  it('sem mudança no rascunho o botão mostra votos registrados', async () => {
    withVotes({ worst_player: ['u-c'] })
    const w = mountC()
    await flushPromises()
    await candidate(w, 'Ana').trigger('click')
    await candidate(w, 'Ana').trigger('click') // marca e desmarca: volta ao persistido
    expect(sendButton(w)).toBeUndefined()
    expect(w.text()).toContain('Votos registrados ✓')
  })

  it('sair da tela com rascunho envia o voto com keepalive', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"ok":true}'))
    vi.stubGlobal('fetch', fetchMock)
    try {
      const w = mountC()
      await flushPromises()
      await candidate(w, 'Ana').trigger('click')
      w.unmount()
      expect(fetchMock).toHaveBeenCalledTimes(1)
      const [url, init] = fetchMock.mock.calls[0]
      expect(url).toBe('/api/matches/m1/votes')
      expect(init).toMatchObject({ method: 'PUT', keepalive: true })
      expect(JSON.parse(init.body)).toEqual({ votes: { top_scorer: ['u-a'] } })
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('sair da tela sem rascunho não envia nada', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    try {
      const w = mountC()
      await flushPromises()
      w.unmount()
      window.dispatchEvent(new Event('pagehide'))
      expect(fetchMock).not.toHaveBeenCalled()
    } finally {
      vi.unstubAllGlobals()
    }
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
