import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), registration: vi.fn(), disable: vi.fn() }))
vi.mock('../../api/client', () => ({ api: { get: mocks.get, post: mocks.post }, errorMessage: () => 'Erro de conexão' }))
vi.mock('../../lib/push', () => ({ supportsPush: () => true, applicationServerKey: () => new Uint8Array([1, 2]), pushRegistration: mocks.registration, disablePush: mocks.disable }))
import PushSettings from './PushSettings.vue'

beforeEach(() => {
  vi.clearAllMocks()
  mocks.get.mockResolvedValue({ data: { enabled: true, public_key: 'key' } })
  mocks.post.mockResolvedValue({ data: { subscribed: false } })
  vi.stubGlobal('Notification', { permission: 'default', requestPermission: vi.fn().mockResolvedValue('granted') })
  vi.stubGlobal('navigator', { serviceWorker: { getRegistration: vi.fn().mockResolvedValue(undefined) } })
})
afterEach(() => vi.unstubAllGlobals())
describe('push settings', () => {
  it('does not request permission on page load', async () => {
    const wrapper = mount(PushSettings)
    await flushPromises()
    expect(Notification.requestPermission).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Ativar notificações')
    wrapper.unmount()
  })
  it('saves the subscription after an explicit user action', async () => {
    const serialized = { endpoint: 'https://fcm.googleapis.com/test', keys: { auth: 'a', p256dh: 'p' } }
    const subscribe = vi.fn().mockResolvedValue({ toJSON: () => serialized })
    mocks.registration.mockResolvedValue({ pushManager: { getSubscription: vi.fn().mockResolvedValue(null), subscribe } })
    const wrapper = mount(PushSettings)
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(Notification.requestPermission).toHaveBeenCalledOnce()
    expect(subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: new Uint8Array([1, 2]) })
    expect(mocks.post).toHaveBeenCalledWith('/push/subscriptions', serialized)
    expect(wrapper.text()).toContain('Notificações ativadas neste aparelho.')
    wrapper.unmount()
  })
  it('does not subscribe when permission is denied', async () => {
    vi.mocked(Notification.requestPermission).mockResolvedValue('denied')
    const wrapper = mount(PushSettings)
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(mocks.registration).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('notificações estão bloqueadas')
    wrapper.unmount()
  })
  it('explains unavailable server configuration', async () => {
    mocks.get.mockResolvedValue({ data: { enabled: false, public_key: '' } })
    const wrapper = mount(PushSettings)
    await flushPromises()
    expect(wrapper.text()).toContain('disponíveis em breve')
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })
})
