import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../../api/client', () => ({ api: { get: mocks.get, post: mocks.post }, errorMessage: () => 'Erro de conexão' }))
import NotificationsView from './NotificationsView.vue'

const recipients = [
  { id: 'a', name: 'Ana Ativa', status: 'active', avatar_color: '#123456', devices: 2 },
  { id: 'b', name: 'Bruno Ativo', status: 'active', avatar_color: '#123456', devices: 1 },
  { id: 'c', name: 'Caio Sem Aparelho', status: 'active', avatar_color: '#123456', devices: 0 },
  { id: 'd', name: 'Davi Inativo', status: 'inactive', avatar_color: '#123456', devices: 1 },
]

function mountView() {
  return mount(NotificationsView, {
    global: {
      stubs: { AdminLayout: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) }), teleport: true },
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.get.mockImplementation((url: string) =>
    Promise.resolve({ data: url === '/push/config' ? { enabled: true } : url === '/admin/push/recipients' ? recipients : [] }),
  )
  mocks.post.mockResolvedValue({ data: { users: 2, devices: 3 } })
})

const sendButton = (wrapper: ReturnType<typeof mountView>) => wrapper.findAll('button').find((b) => b.text().includes('Revisar e enviar'))!

describe('admin notifications', () => {
  it('counts only active members with devices for "all"', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Vai para 2 membros (3 aparelhos)')
    expect(sendButton(wrapper).attributes('disabled')).toBeDefined() // sem mensagem ainda
    await wrapper.get('textarea').setValue('Jogo cancelado')
    expect(sendButton(wrapper).attributes('disabled')).toBeUndefined()
  })

  it('sends the typed message to the chosen members after confirmation', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('textarea').setValue('  Pague a mensalidade  ')
    await wrapper.get('input[value="selected"]').setValue(true)
    const boxes = wrapper.findAll('input[type="checkbox"]')
    await boxes[2].setValue(true) // Caio, sem aparelho
    await boxes[3].setValue(true) // Davi, inativo mas com aparelho
    expect(wrapper.text()).toContain('Vai para 1 membro (1 aparelho)')
    expect(wrapper.text()).toContain('1 selecionado não receberá')
    await sendButton(wrapper).trigger('click')
    expect(mocks.post).not.toHaveBeenCalled()
    await wrapper.findAll('button').find((b) => b.text() === 'Enviar agora')!.trigger('click')
    await flushPromises()
    expect(mocks.post).toHaveBeenCalledWith('/admin/push/broadcasts', {
      title: '',
      body: '  Pague a mensalidade  ',
      audience: 'selected',
      user_ids: ['c', 'd'],
    })
    expect(wrapper.text()).toContain('Aviso enviado para 2 membros (3 aparelhos)')
  })

  it('blocks messages longer than the server limit, counting emojis as one character', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('textarea').setValue('⚽'.repeat(240))
    expect(wrapper.text()).toContain('240/240')
    expect(sendButton(wrapper).attributes('disabled')).toBeUndefined()
    await wrapper.get('textarea').setValue('⚽'.repeat(241))
    expect(sendButton(wrapper).attributes('disabled')).toBeDefined()
  })

  it('warns and disables sending when push is not configured', async () => {
    mocks.get.mockImplementation((url: string) =>
      Promise.resolve({ data: url === '/push/config' ? { enabled: false } : url === '/admin/push/recipients' ? recipients : [] }),
    )
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('textarea').setValue('Oi')
    expect(wrapper.text()).toContain('ainda não estão configuradas')
    expect(sendButton(wrapper).attributes('disabled')).toBeDefined()
  })
})
