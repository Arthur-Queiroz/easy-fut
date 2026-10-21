import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ put: vi.fn() }))
vi.mock('../../api/client', () => ({
  api: { put: mocks.put },
  errorMessage: (e: { message?: string }) => e?.message ?? 'Erro de conexão',
}))
import type { Match } from '../../api/types'
import GuestSaleControl from './GuestSaleControl.vue'

const match = (over: Partial<Match> = {}): Match =>
  ({
    id: 'm1',
    status: 'open',
    match_date: '2026-10-29',
    start_time: '20:00',
    end_time: '22:00',
    venue: 'Arena',
    guest_sale_open: false,
    guest_slots: 0,
    guest_price_cents: 0,
    guest_total_cents: 0,
    guest_confirmed_count: 0,
    guest_pending_count: 0,
    ...over,
  }) as Match

const button = (w: ReturnType<typeof mount>, text: string) => w.findAll('button').find((b) => b.text().includes(text))!

beforeEach(() => {
  vi.clearAllMocks()
  mocks.put.mockResolvedValue({ data: {} })
})

describe('venda de vagas avulsas na partida', () => {
  it('abre a venda com o número de vagas', async () => {
    const w = mount(GuestSaleControl, { props: { match: match() } })
    expect(w.text()).toContain('Venda fechada')
    expect(button(w, 'Abrir venda').attributes('disabled')).toBeDefined() // sem vagas ainda
    await w.get('input[type="number"]').setValue('4')
    await button(w, 'Abrir venda').trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledWith('/admin/matches/m1/guest-sale', { open: true, slots: 4 })
    expect(w.emitted('changed')).toHaveLength(1)
  })

  it('mostra valor congelado, confirmados e pendentes; muda vagas e fecha a venda', async () => {
    const w = mount(GuestSaleControl, {
      props: {
        match: match({
          guest_sale_open: true, guest_slots: 6, guest_price_cents: 2500, guest_total_cents: 2525,
          guest_confirmed_count: 2, guest_pending_count: 1,
        }),
      },
    })
    const text = w.text().replace(/\s+/g, ' ')
    expect(text).toContain('Venda aberta')
    expect(text).toContain('R$ 25,25 por vaga')
    expect(text).toContain('R$ 25,00 para o caixa')
    expect(text).toContain('2 confirmados')
    expect(text).toContain('1 aguardando Pix')
    expect(text).toContain('3 de 6 vagas livres')

    await w.get('input[type="number"]').setValue('8')
    await button(w, 'Salvar vagas').trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenLastCalledWith('/admin/matches/m1/guest-sale', { open: true, slots: 8 })

    await button(w, 'Fechar venda').trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenLastCalledWith('/admin/matches/m1/guest-sale', { open: false, slots: 6 })
  })

  it('mostra o erro da API perto da ação', async () => {
    mocks.put.mockRejectedValue({ message: 'a partida não está aberta' })
    const w = mount(GuestSaleControl, { props: { match: match({ guest_slots: 3 }) } })
    await button(w, 'Abrir venda').trigger('click')
    await flushPromises()
    expect(w.get('[data-test="guest-sale"]').text()).toContain('a partida não está aberta')
    expect(w.emitted('changed')).toBeUndefined()
  })

  it('fora de open, só mostra o resumo dos avulsos', () => {
    const w = mount(GuestSaleControl, {
      props: { match: match({ status: 'closed', guest_slots: 4, guest_confirmed_count: 3, guest_pending_count: 0 }) },
    })
    expect(w.find('[data-test="guest-sale"]').exists()).toBe(false)
    expect(w.get('[data-test="guest-summary"]').text()).toContain('3 confirmados')
    expect(w.find('button').exists()).toBe(false)
  })
})
