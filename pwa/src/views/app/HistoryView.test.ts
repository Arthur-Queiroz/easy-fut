import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import HistoryView from './HistoryView.vue'

const get = vi.hoisted(() => vi.fn())
vi.mock('../../api/client', () => ({ api: { get } }))
afterEach(() => vi.unstubAllGlobals())

async function mountHistory() {
  get.mockResolvedValue({ data: [{ id: 'm1', match_date: '2026-10-25', venue: 'Arena Futiba 1', going_count: 15, media_count: 2, status: 'finished', results: [] }] })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/historico', component: HistoryView },
      { path: '/historico/:matchId', name: 'match-detail', component: { template: '<div>Detalhes da partida</div>' } },
    ],
  })
  await router.push('/historico')
  await router.isReady()
  const wrapper = mount(HistoryView, {
    global: {
      plugins: [router],
      stubs: {
        MobileShell: { template: '<div><slot name="header" /><slot /></div>' },
        MatchDetailContent: { props: ['matchId'], template: '<div>Detalhe {{ matchId }}</div>' },
      },
    },
  })
  await flushPromises()
  return { router, wrapper }
}

describe('histórico de partidas', () => {
  it('abre a página de detalhes e fotos quando o painel está oculto', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query === '(min-width: 1024px)' }))
    const { router, wrapper } = await mountHistory()
    const link = wrapper.get('a[href="/historico/m1"]')
    expect(link.text()).toContain('Ver detalhes e fotos')
    await link.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/historico/m1')
  })

  it('mostra o detalhe ao lado em tela larga', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query === '(min-width: 1280px)' }))
    const { router, wrapper } = await mountHistory()
    await wrapper.get('a[href="/historico/m1"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/historico?detalhe=m1')
    expect(wrapper.text()).toContain('Detalhe m1')
  })
})
