import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), push: vi.fn(), params: { id: 'b1' } as Record<string, string> }))
vi.mock('../../api/client', () => ({
  api: { get: mocks.get },
  errorMessage: (e: { response?: { data?: { error?: string } } }) => e?.response?.data?.error ?? 'Erro de conexão',
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useRouter: () => ({ push: mocks.push }),
  useRoute: () => ({ params: mocks.params }),
}))
import GuestBookingView from './GuestBookingView.vue'

const NOW = new Date('2026-10-21T12:00:00Z')

const booking = (status: string, extra: Record<string, unknown> = {}) => ({
  status,
  total_cents: 2525,
  expires_at: '2026-10-21T12:29:30Z',
  match: { match_date: '2026-10-24', start_time: '08:00', end_time: '10:00', venue: 'Arena Central', address: 'Rua das Flores, 100' },
  ...(status === 'pending_payment'
    ? { pix_payload: '00020126PIXCOPIAECOLA', pix_qr_code_base64: 'iVBORw0KGgo', pix_ticket_url: 'https://mp.example/ticket' }
    : {}),
  ...extra,
})

function mountView() {
  return mount(GuestBookingView, { global: { stubs: { RouterLink: true } } })
}

function storeToken(map: Record<string, string> = { b1: 'tok-123' }) {
  localStorage.setItem('easyfut-guest-bookings', JSON.stringify(map))
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'Date'] })
  vi.setSystemTime(NOW)
  localStorage.clear()
  mocks.params = { id: 'b1' }
})

afterEach(() => {
  vi.useRealTimers()
})

describe('/avulso/inscricao/:id', () => {
  it('sem token neste aparelho mostra mensagem e não chama a API', async () => {
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('Não achamos sua inscrição aqui')
    expect(mocks.get).not.toHaveBeenCalled()
  })

  it('pendente: QR, copia-e-cola, total e contagem regressiva, com o header do token', async () => {
    storeToken()
    mocks.get.mockResolvedValue({ data: booking('pending_payment') })
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/public/guest-bookings/b1', { headers: { 'X-Booking-Token': 'tok-123' } })
    expect(w.get('img').attributes('src')).toBe('data:image/png;base64,iVBORw0KGgo')
    expect(w.text()).toContain('00020126PIXCOPIAECOLA')
    expect(w.text()).toMatch(/R\$\s25,25/)
    expect(w.get('[data-test="countdown"]').text()).toBe('29:30')
    await vi.advanceTimersByTimeAsync(1000)
    expect(w.get('[data-test="countdown"]').text()).toBe('29:29')
  })

  it('botão copiar manda o Pix para a área de transferência', async () => {
    storeToken()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    mocks.get.mockResolvedValue({ data: booking('pending_payment') })
    const w = mountView()
    await flushPromises()
    await w.findAll('button').find((b) => b.text().includes('Copiar'))!.trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith('00020126PIXCOPIAECOLA')
    expect(w.text()).toContain('Copiado!')
  })

  it('consulta a cada 5 s enquanto pendente e para ao confirmar', async () => {
    storeToken()
    mocks.get
      .mockResolvedValueOnce({ data: booking('pending_payment') })
      .mockResolvedValueOnce({ data: booking('pending_payment') })
      .mockResolvedValue({ data: booking('confirmed') })
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(4999)
    expect(mocks.get).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(mocks.get).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(5000)
    expect(mocks.get).toHaveBeenCalledTimes(3)
    expect(w.find('[data-test="confirmed"]').exists()).toBe(true)

    await vi.advanceTimersByTimeAsync(30000)
    expect(mocks.get).toHaveBeenCalledTimes(3)
  })

  it('para de consultar ao sair da tela', async () => {
    storeToken()
    mocks.get.mockResolvedValue({ data: booking('pending_payment') })
    const w = mountView()
    await flushPromises()
    w.unmount()
    await vi.advanceTimersByTimeAsync(20000)
    expect(mocks.get).toHaveBeenCalledTimes(1)
  })

  it('confirmada mostra partida, horário, endereço e o aviso do WhatsApp', async () => {
    storeToken()
    mocks.get.mockResolvedValue({ data: booking('confirmed') })
    const w = mountView()
    await flushPromises()
    const text = w.get('[data-test="confirmed"]').text()
    expect(text).toContain('Sábado, 24 de outubro')
    expect(text).toContain('08h00 às 10h00')
    expect(text).toContain('Arena Central')
    expect(text).toContain('Rua das Flores, 100')
    expect(text).toContain('Você vai receber a confirmação e, depois do jogo, o link para votar pelo WhatsApp.')
    expect(w.find('img').exists()).toBe(false)
  })

  it('expirada oferece tentar de novo em /avulso', async () => {
    storeToken()
    mocks.get.mockResolvedValue({ data: booking('expired') })
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('Sua reserva expirou')
    await w.findAll('button').find((b) => b.text().includes('Tentar de novo'))!.trigger('click')
    expect(mocks.push).toHaveBeenCalledWith({ name: 'guest-matches' })
    await vi.advanceTimersByTimeAsync(10000)
    expect(mocks.get).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['refund_pending', 'Vamos devolver seu dinheiro'],
    ['refunded', 'Dinheiro devolvido'],
    ['cancelled', 'Inscrição cancelada'],
  ])('status %s tem mensagem própria', async (status, message) => {
    storeToken()
    mocks.get.mockResolvedValue({ data: booking(status) })
    const w = mountView()
    await flushPromises()
    expect(w.get(`[data-test="${status}"]`).text()).toContain(message)
    expect(w.text()).not.toContain('Aguardando pagamento')
  })

  it('token errado (404) mostra inscrição não encontrada e não insiste', async () => {
    storeToken()
    mocks.get.mockRejectedValue({ response: { status: 404, data: { error: 'não encontrado' } } })
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('Inscrição não encontrada')
    await vi.advanceTimersByTimeAsync(10000)
    expect(mocks.get).toHaveBeenCalledTimes(1)
  })

  it('falha de rede no meio do polling mantém a tela e tenta de novo', async () => {
    storeToken()
    mocks.get
      .mockResolvedValueOnce({ data: booking('pending_payment') })
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValue({ data: booking('confirmed') })
    const w = mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    expect(w.find('[data-test="pending"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(5000)
    expect(w.find('[data-test="confirmed"]').exists()).toBe(true)
  })
})
