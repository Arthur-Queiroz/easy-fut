import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), push: vi.fn() }))
vi.mock('../../api/client', () => ({
  api: { get: mocks.get, post: mocks.post },
  errorMessage: (e: { response?: { data?: { error?: string } } }) => e?.response?.data?.error ?? 'Erro de conexão',
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useRouter: () => ({ push: mocks.push }),
}))
import GuestMatchesView from './GuestMatchesView.vue'

const guestMatches = [
  {
    match_id: 'm1',
    match_date: '2026-10-24',
    start_time: '08:00',
    end_time: '10:00',
    venue: 'Arena Central',
    address: 'Rua das Flores, 100',
    total_cents: 2525,
    slots_left: 3,
  },
  {
    match_id: 'm2',
    match_date: '2026-10-31',
    start_time: '20:00',
    end_time: '22:00',
    venue: 'Quadra do Zé',
    address: 'Av. Brasil, 9',
    total_cents: 2525,
    slots_left: 1,
  },
]

const created = {
  booking_id: 'b1',
  access_token: 'tok-123',
  status: 'pending_payment',
  total_cents: 2525,
  expires_at: '2026-10-21T12:30:00Z',
  pix_payload: '000201...',
  pix_qr_code_base64: 'iVBOR',
  pix_ticket_url: 'https://mp/ticket',
}

const apiError = (status: number, error?: string) => ({ response: { status, data: error ? { error } : {} } })

function mountView() {
  return mount(GuestMatchesView, { global: { stubs: { RouterLink: true } } })
}

async function fillForm(w: ReturnType<typeof mountView>, phone = '(11) 91234-5678') {
  await w.findAll('[data-test="guest-match"]')[1].trigger('click')
  await w.get('input[name="name"]').setValue('  Zé Avulso ')
  await w.get('input[name="phone"]').setValue(phone)
  await w.get('input[name="email"]').setValue('ze@email.com')
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  mocks.get.mockResolvedValue({ data: guestMatches })
  mocks.post.mockResolvedValue({ data: created })
})

describe('/avulso', () => {
  it('lista as partidas com data, horário, local, endereço, vagas e total', async () => {
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/public/guest-matches')
    const text = w.text()
    expect(text).toContain('Arena Central')
    expect(text).toContain('Rua das Flores, 100')
    expect(text).toContain('08h00 às 10h00')
    expect(text).toContain('3 vagas')
    expect(text).toContain('Última vaga!')
    expect(text).toMatch(/R\$\s25,25/)
    expect(text).not.toMatch(/taxa/i)
  })

  it('lista vazia mostra estado vazio', async () => {
    mocks.get.mockResolvedValue({ data: [] })
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('Nenhuma partida com vaga avulsa aberta agora')
    expect(w.find('form').exists()).toBe(false)
  })

  it('envia o formulário, guarda o token e vai para a inscrição', async () => {
    const w = mountView()
    await flushPromises()
    await fillForm(w)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/public/guest-bookings', {
      match_id: 'm2',
      name: 'Zé Avulso',
      phone: '(11) 91234-5678',
      email: 'ze@email.com',
    })
    expect(JSON.parse(localStorage.getItem('easyfut-guest-bookings')!)).toEqual({ b1: 'tok-123' })
    expect(mocks.push).toHaveBeenCalledWith({ name: 'guest-booking', params: { id: 'b1' } })
  })

  it('valida o WhatsApp antes de chamar a API', async () => {
    const w = mountView()
    await flushPromises()
    await fillForm(w, '1234')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.post).not.toHaveBeenCalled()
    expect(w.text()).toContain('Confere o WhatsApp')
  })

  it('409 mostra a mensagem da API e atualiza a lista', async () => {
    mocks.post.mockRejectedValue(apiError(409, 'as vagas avulsas desta partida acabaram'))
    const w = mountView()
    await flushPromises()
    await fillForm(w)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('as vagas avulsas desta partida acabaram')
    expect(mocks.get).toHaveBeenCalledTimes(2)
    expect(mocks.push).not.toHaveBeenCalled()
    expect(localStorage.getItem('easyfut-guest-bookings')).toBeNull()
  })

  it('409 de WhatsApp repetido aparece perto do botão', async () => {
    mocks.post.mockRejectedValue(apiError(409, 'este WhatsApp já tem uma vaga nesta partida'))
    const w = mountView()
    await flushPromises()
    await fillForm(w)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('este WhatsApp já tem uma vaga')
  })

  it('429 mostra "muitas tentativas"', async () => {
    mocks.post.mockRejectedValue(apiError(429))
    const w = mountView()
    await flushPromises()
    await fillForm(w)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('Muitas tentativas')
  })
})
