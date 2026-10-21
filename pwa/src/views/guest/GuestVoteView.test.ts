import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('../../api/client', () => ({
  api: { get: mocks.get, put: mocks.put },
  errorMessage: (e: { response?: { data?: { error?: string } } }) => e?.response?.data?.error ?? 'Erro de conexão',
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useRoute: () => ({ params: { token: 'vt-1' } }),
}))
import GuestVoteView from './GuestVoteView.vue'

const vote = (myVotes: Record<string, string[]> = {}) => ({
  match: { match_date: '2026-10-24', venue: 'Arena Central' },
  participants: [
    { user_id: 'u-a', name: 'Ana', avatar_color: '#123456' },
    { user_id: 'u-b', name: 'Bruno', avatar_color: '#234567' },
    { user_id: 'u-c', name: 'Caio', avatar_color: '#345678' },
  ],
  my_votes: myVotes,
  voting_closes_at: '2026-10-26T12:00:00Z',
})

function mountView() {
  return mount(GuestVoteView, { global: { stubs: { RouterLink: true } } })
}

const pick = (w: ReturnType<typeof mountView>, category: string, name: string) =>
  w.get(`[data-test="cat-${category}"]`).findAll('button').find((b) => b.text().includes(name))!
const sendButton = (w: ReturnType<typeof mountView>) => w.findAll('button').find((b) => b.text().includes('Enviar votos'))

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  mocks.get.mockResolvedValue({ data: vote() })
  mocks.put.mockResolvedValue({ data: { ok: true } })
})

describe('/votar/:token', () => {
  it('mostra as 3 categorias com os participantes', async () => {
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/public/guest-vote/vt-1')
    for (const cat of ['top_scorer', 'worst_player', 'best_goal']) {
      const card = w.get(`[data-test="cat-${cat}"]`)
      expect(card.findAll('button')).toHaveLength(3)
    }
    expect(w.text()).toContain('Vote no artilheiro')
    expect(w.text()).toContain('Vote no perna de pau')
    expect(w.text()).toContain('Vote no melhor gol')
  })

  it('envia só as categorias alteradas', async () => {
    mocks.get.mockResolvedValue({ data: vote({ worst_player: ['u-c'] }) })
    const w = mountView()
    await flushPromises()
    await pick(w, 'top_scorer', 'Ana').trigger('click')
    await pick(w, 'top_scorer', 'Bruno').trigger('click')
    await pick(w, 'best_goal', 'Caio').trigger('click')
    // marca e desmarca no perna de pau: volta ao gravado, não vai no envio
    await pick(w, 'worst_player', 'Ana').trigger('click')
    await pick(w, 'worst_player', 'Ana').trigger('click')
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledTimes(1)
    expect(mocks.put).toHaveBeenCalledWith('/public/guest-vote/vt-1/votes', {
      votes: { top_scorer: ['u-a', 'u-b'], best_goal: ['u-c'] },
    })
    expect(localStorage.getItem('fut-guest-vote-draft-vt-1')).toBeNull()
  })

  it('não deixa marcar mais de 2 por categoria', async () => {
    const w = mountView()
    await flushPromises()
    await pick(w, 'top_scorer', 'Ana').trigger('click')
    await pick(w, 'top_scorer', 'Bruno').trigger('click')
    await pick(w, 'top_scorer', 'Caio').trigger('click')
    expect(w.text()).toContain('Escolha no máximo 2 jogadores')
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledWith('/public/guest-vote/vt-1/votes', { votes: { top_scorer: ['u-a', 'u-b'] } })
  })

  it('rascunho sobrevive a recarregar a página', async () => {
    const w = mountView()
    await flushPromises()
    await pick(w, 'best_goal', 'Bruno').trigger('click')
    w.unmount()
    const again = mountView()
    await flushPromises()
    expect(pick(again, 'best_goal', 'Bruno').attributes('aria-pressed')).toBe('true')
    expect(sendButton(again)).toBeDefined()
  })

  it('400 da API (voto em si mesmo) aparece perto do botão', async () => {
    mocks.put.mockRejectedValue({ response: { status: 400, data: { error: 'não vale votar em si mesmo como perna de pau' } } })
    const w = mountView()
    await flushPromises()
    await pick(w, 'worst_player', 'Ana').trigger('click')
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('não vale votar em si mesmo como perna de pau')
    expect(sendButton(w)).toBeDefined() // rascunho continua para corrigir
  })

  it('404 mostra link inválido', async () => {
    mocks.get.mockRejectedValue({ response: { status: 404, data: {} } })
    const w = mountView()
    await flushPromises()
    expect(w.find('[data-test="invalid"]').text()).toContain('Link inválido')
  })

  it('410 mostra votação encerrada', async () => {
    mocks.get.mockRejectedValue({ response: { status: 410, data: {} } })
    const w = mountView()
    await flushPromises()
    expect(w.find('[data-test="closed"]').text()).toContain('Votação encerrada')
  })

  it('410 ao enviar troca a tela para votação encerrada', async () => {
    mocks.put.mockRejectedValue({ response: { status: 410, data: {} } })
    const w = mountView()
    await flushPromises()
    await pick(w, 'top_scorer', 'Ana').trigger('click')
    await sendButton(w)!.trigger('click')
    await flushPromises()
    expect(w.find('[data-test="closed"]').exists()).toBe(true)
  })
})
