import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../../api/client', () => ({ api: { get: mocks.get, post: mocks.post }, errorMessage: () => 'Erro de conexão' }))
import PaymentsView from './PaymentsView.vue'

const charge = (id: string, status: string, fee: number, extra: Record<string, unknown> = {}) => ({
  id, status, batch_kind: 'monthly', batch_title: '', reference_month: '2099-01', due_date: '2099-01-10',
  amount_cents: 6000, pix_fee_cents: fee, pix_total_cents: 6000 + fee, paid_method: '', pix_payload: '',
  pix_ticket_url: '', pix_qr_code_base64: '', ...extra,
})

function mountView() {
  return mount(PaymentsView, {
    global: { stubs: { MobileShell: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }) } },
  })
}

// formatCents usa espaço não separável depois de "R$".
const text = (el: { text(): string }) => el.text().replace(/\u00a0/g, ' ')

beforeEach(() => vi.clearAllMocks())

describe('pagamentos do jogador', () => {
  // A taxa do Pix é repassada sem aparecer: o jogador vê só o valor final.
  it('mostra só o total no Pix, sem detalhar a taxa', async () => {
    mocks.get.mockResolvedValue({ data: [charge('c1', 'pending', 60)] })
    const w = mountView()
    await flushPromises()
    expect(text(w)).toContain('R$ 60,60')
    expect(text(w)).not.toContain('R$ 60,00')
    expect(text(w).toLowerCase()).not.toContain('taxa')
  })

  it('cobrança antiga, sem taxa, mostra o rateio', async () => {
    mocks.get.mockResolvedValue({ data: [charge('c1', 'pending', 0)] })
    const w = mountView()
    await flushPromises()
    expect(text(w)).toContain('R$ 60,00')
  })

  it('no histórico, Pix pago mostra o total e baixa manual só o rateio', async () => {
    mocks.get.mockResolvedValue({ data: [
      charge('c1', 'paid', 60, { paid_at: '2099-01-05T12:00:00Z', paid_method: 'pix' }),
      charge('c2', 'manual_paid', 60, { paid_at: '2098-12-05T12:00:00Z', paid_method: 'manual', registered_by_name: 'Admin', reference_month: '2098-12' }),
    ] })
    const w = mountView()
    await flushPromises()
    const amounts = w.findAll('[data-test="history-amount"]').map(text)
    expect(amounts).toEqual(['R$ 60,60', 'R$ 60,00'])
  })
})
