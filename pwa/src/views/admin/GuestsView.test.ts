import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../../api/client', () => ({
  api: { get: mocks.get, post: mocks.post },
  errorMessage: (e: { message?: string }) => e?.message ?? 'Erro de conexão',
}))
const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
import GuestsView from './GuestsView.vue'

const booking = (id: string, name: string, status: string, over: Record<string, unknown> = {}) => ({
  id, match_id: 'm1', match_date: '2026-10-29', start_time: '20:00', venue: 'Arena Central', name,
  phone: '+5511987654321', email: `${id}@mail.com`, status, base_cents: 2500, total_cents: 2525,
  created_at: '2026-10-21T12:00:00Z', confirmed_at: null, refund_reason: '',
  confirmation_sent_at: null, vote_link_sent_at: null, ...over,
})
const bookings = [
  booking('b1', 'Pedro Avulso', 'confirmed', { confirmation_sent_at: '2026-10-21T12:05:00Z' }),
  booking('b2', 'Rafa Reembolso', 'refund_pending', { refund_reason: 'partida cancelada' }),
  booking('b3', 'Lia Pendente', 'pending_payment'),
]
const matches = [
  { id: 'm1', match_date: '2026-10-29', venue: 'Arena Central', status: 'open', guest_confirmed_count: 1 },
  { id: 'm2', match_date: '2026-10-22', venue: 'Quadra Norte', status: 'finished', guest_confirmed_count: 0 },
]

function mountView() {
  return mount(GuestsView, {
    attachTo: document.body,
    global: { stubs: { AdminLayout: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }), teleport: true } },
  })
}

const rows = (w: ReturnType<typeof mountView>) => w.findAll('[data-test="booking"]')
const button = (el: { findAll: (s: string) => { text: () => string }[] }, text: string) =>
  el.findAll('button').find((b) => b.text().includes(text)) as ReturnType<ReturnType<typeof mountView>['get']> | undefined
const lastListParams = () => mocks.get.mock.calls.filter(([url]) => url === '/admin/guest-bookings').at(-1)![1].params

beforeEach(() => {
  vi.clearAllMocks()
  route.query = {}
  mocks.get.mockImplementation((url: string) =>
    Promise.resolve({ data: url === '/matches' ? matches : url === '/admin/guest-bookings' ? bookings : null }),
  )
  mocks.post.mockResolvedValue({ data: {} })
})

describe('admin avulsos', () => {
  it('lista inscrições com partida, contato, status, total e mensagens', async () => {
    const w = mountView()
    await flushPromises()
    expect(rows(w)).toHaveLength(3)
    const first = rows(w)[0].text().replace(/\s+/g, ' ')
    expect(first).toContain('Pedro Avulso')
    expect(first).toContain('Arena Central')
    expect(first).toContain('(11) 98765-4321')
    expect(first).toContain('b1@mail.com')
    expect(first).toContain('Confirmada')
    expect(first).toContain('R$ 25,25')
    expect(first).toMatch(/Confirmação\s*enviada/)
    expect(first).toMatch(/Link de voto\s*não enviado/)
    expect(rows(w)[1].text()).toContain('partida cancelada')
    expect(lastListParams()).toEqual({ match_id: undefined, status: undefined, q: undefined })
    w.unmount()
  })

  it('filtra por partida, estado e busca', async () => {
    const w = mountView()
    await flushPromises()

    await w.get('button[aria-label="Partida"]').trigger('click')
    const option = w.findAll('li[role="option"]').find((li) => li.text().includes('Quadra Norte'))!
    await option.trigger('click')
    await flushPromises()
    expect(lastListParams()).toMatchObject({ match_id: 'm2' })

    await w.get('select[aria-label="Estado da inscrição"]').setValue('refund_pending')
    await flushPromises()
    expect(lastListParams()).toMatchObject({ match_id: 'm2', status: 'refund_pending' })

    await w.get('input[type="search"]').setValue(' 98765 ')
    await new Promise((r) => setTimeout(r, 350)) // debounce da busca
    await flushPromises()
    expect(lastListParams()).toEqual({ match_id: 'm2', status: 'refund_pending', q: '98765' })
    w.unmount()
  })

  it('usa a busca da URL (vinda de Usuários)', async () => {
    route.query = { q: '+5511987654321' }
    const w = mountView()
    await flushPromises()
    expect(lastListParams()).toMatchObject({ q: '+5511987654321' })
    w.unmount()
  })

  it('reenvia a confirmação direto e o link de voto só depois do aviso', async () => {
    const w = mountView()
    await flushPromises()
    const row = rows(w)[0]
    expect(button(rows(w)[2], 'Reenviar')).toBeUndefined() // pendente não tem ações

    await button(row, 'Reenviar confirmação')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/admin/guest-bookings/b1/resend', { kind: 'confirmation' })
    expect(rows(w)[0].text()).toContain('Confirmação reenviada')

    await button(rows(w)[0], 'Reenviar link de voto')!.trigger('click')
    expect(mocks.post).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).toContain('O link enviado antes deixa de valer')
    await button(w, 'Enviar link novo')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenLastCalledWith('/admin/guest-bookings/b1/resend', { kind: 'vote_link' })
    w.unmount()
  })

  it('mostra o erro do reenvio na linha', async () => {
    mocks.post.mockRejectedValue({ message: 'a votação desta partida não está aberta' })
    const w = mountView()
    await flushPromises()
    await button(rows(w)[0], 'Reenviar link de voto')!.trigger('click')
    await button(w, 'Enviar link novo')!.trigger('click')
    await flushPromises()
    expect(rows(w)[0].text()).toContain('a votação desta partida não está aberta')
    w.unmount()
  })

  it('marca reembolso só em refund_pending e com confirmação', async () => {
    const w = mountView()
    await flushPromises()
    expect(button(rows(w)[0], 'Marcar reembolso')).toBeUndefined()
    expect(button(rows(w)[2], 'Marcar reembolso')).toBeUndefined()

    await button(rows(w)[1], 'Marcar reembolso feito')!.trigger('click')
    expect(mocks.post).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Confirme que você já estornou')
    await button(w, 'Já estornei')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/admin/guest-bookings/b2/refunded')
    w.unmount()
  })

  it('lista vazia usa EmptyState', async () => {
    mocks.get.mockImplementation((url: string) => Promise.resolve({ data: url === '/matches' ? matches : [] }))
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('Nenhuma inscrição avulsa com esses filtros.')
    w.unmount()
  })
})
