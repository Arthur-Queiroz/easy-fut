import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), delete: vi.fn() }))
vi.mock('../../api/client', () => ({
  api: { get: mocks.get, post: mocks.post, delete: mocks.delete },
  errorMessage: (e: { message?: string }) => e?.message ?? 'Erro de conexão',
}))
import CashView from './CashView.vue'

const report = {
  balance_cents: 3750,
  entries: [
    { id: 'e1', kind: 'guest_payment', amount_cents: 2500, fee_cents: 25, booking_id: 'b1', description: 'Vaga avulsa · Pedro', occurred_on: '2026-10-20', created_at: '2026-10-20T12:00:00Z' },
    { id: 'e2', kind: 'refund', amount_cents: 2500, fee_cents: 0, booking_id: 'b2', description: 'Estorno · Rafa', occurred_on: '2026-10-19', created_at: '2026-10-19T12:00:00Z' },
    { id: 'e3', kind: 'expense', amount_cents: 1250, fee_cents: 0, booking_id: null, description: 'Bola nova', occurred_on: '2026-10-18', created_at: '2026-10-18T12:00:00Z' },
  ],
}

function mountView() {
  return mount(CashView, {
    attachTo: document.body,
    global: { stubs: { AdminLayout: defineComponent({ setup: (_, { slots }) => () => h('div', [slots.actions?.(), slots.default?.()]) }), teleport: true } },
  })
}

const button = (w: { findAll: (s: string) => { text: () => string }[] }, text: string) =>
  w.findAll('button').find((b) => b.text().includes(text)) as ReturnType<ReturnType<typeof mountView>['get']>

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 21, 12, 0, 0))
  mocks.get.mockResolvedValue({ data: report })
  mocks.post.mockResolvedValue({ data: {} })
  mocks.delete.mockResolvedValue({ data: {} })
})
afterEach(() => {
  vi.useRealTimers()
})

describe('caixa do admin', () => {
  it('mostra saldo, totais do período e o sinal de cada lançamento em texto', async () => {
    const w = mountView()
    await flushPromises()
    expect(mocks.get).toHaveBeenCalledWith('/admin/cash', { params: { from: '2026-10-01', to: '2026-10-31' } })
    expect(w.get('[data-test="balance"]').text().replace(/\s+/g, ' ')).toContain('R$ 37,50')
    const text = w.text().replace(/\s+/g, ' ')
    expect(text).toContain('+ R$ 25,00') // entradas
    expect(text).toContain('− R$ 37,50') // saídas
    const rows = w.findAll('[data-test="cash-entry"]').map((r) => r.text().replace(/\s+/g, ' '))
    expect(rows[0]).toContain('Entrada · vaga avulsa')
    expect(rows[0]).toContain('+ R$ 25,00')
    expect(rows[0]).toContain('taxa estimada R$ 0,25')
    expect(rows[1]).toContain('Estorno')
    expect(rows[1]).toContain('− R$ 25,00')
    expect(rows[2]).toContain('Despesa')
    expect(rows[2]).toContain('18/10/2026')
    // só despesa pode ser removida
    expect(w.findAll('button[aria-label^="Remover despesa"]')).toHaveLength(1)
    w.unmount()
  })

  it('troca o período', async () => {
    const w = mountView()
    await flushPromises()
    await button(w, 'Mês passado').trigger('click')
    await flushPromises()
    expect(mocks.get).toHaveBeenLastCalledWith('/admin/cash', { params: { from: '2026-09-01', to: '2026-09-30' } })
    await button(w, 'Tudo').trigger('click')
    await flushPromises()
    expect(mocks.get).toHaveBeenLastCalledWith('/admin/cash', { params: { from: undefined, to: undefined } })
    w.unmount()
  })

  it('lança despesa convertendo "12,50" em 1250 centavos', async () => {
    const w = mountView()
    await flushPromises()
    await button(w, 'Despesa').trigger('click')
    await w.get('input[placeholder="Ex.: bola nova"]').setValue('  Coletes  ')
    await w.get('input[aria-label="Valor da despesa"]').setValue('12,50')
    await w.get('input[aria-label="Data da despesa"]').setValue('2026-10-20')
    await button(w, 'Lançar despesa').trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/admin/cash/expenses', {
      description: 'Coletes', amount_cents: 1250, occurred_on: '2026-10-20',
    })
    expect(mocks.get).toHaveBeenCalledTimes(2) // recarrega depois de lançar
    w.unmount()
  })

  it('recusa valor inválido sem chamar a API', async () => {
    const w = mountView()
    await flushPromises()
    await button(w, 'Despesa').trigger('click')
    await w.get('input[placeholder="Ex.: bola nova"]').setValue('Coletes')
    await w.get('input[aria-label="Valor da despesa"]').setValue('doze')
    await button(w, 'Lançar despesa').trigger('click')
    await flushPromises()
    expect(mocks.post).not.toHaveBeenCalled()
    expect(w.text()).toContain('Informe um valor em reais maior que zero')
    w.unmount()
  })

  it('remove despesa depois de confirmar', async () => {
    const w = mountView()
    await flushPromises()
    await w.get('button[aria-label="Remover despesa Bola nova"]').trigger('click')
    expect(mocks.delete).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('lançada por engano')
    const confirm = w.findAll('button').filter((b) => b.text() === 'Remover despesa')
    await confirm[confirm.length - 1].trigger('click')
    await flushPromises()
    expect(mocks.delete).toHaveBeenCalledWith('/admin/cash/expenses/e3')
    w.unmount()
  })
})
