import { afterEach, describe, expect, it, vi } from 'vitest'
vi.mock('../api/client', () => ({ api: { delete: vi.fn() } }))
import { api } from '../api/client'
import { applicationServerKey, disablePush, supportsPush } from './push'

afterEach(() => vi.unstubAllGlobals())
function browser() {
 const unsubscribe = vi.fn().mockResolvedValue(true)
 const sub = { endpoint: 'https://fcm.googleapis.com/test', unsubscribe }
 vi.stubGlobal('isSecureContext', true)
 vi.stubGlobal('PushManager', class {})
 vi.stubGlobal('Notification', class {})
 vi.stubGlobal('navigator', { serviceWorker: { getRegistration: vi.fn().mockResolvedValue({ pushManager: { getSubscription: vi.fn().mockResolvedValue(sub) } }) } })
 return sub
}
describe('Web Push devices', () => {
 it('decodes URL-safe application keys', () => {
   expect([...applicationServerKey('-_8')]).toEqual([251, 255])
 })
 it('requires a secure context and push support', () => {
   browser(); expect(supportsPush()).toBe(true)
   vi.stubGlobal('isSecureContext', false); expect(supportsPush()).toBe(false)
 })
 it('removes the account association before unsubscribing the device', async () => {
   const sub = browser()
   vi.mocked(api.delete).mockResolvedValue({} as never)
   await disablePush()
   expect(api.delete).toHaveBeenCalledWith('/push/subscriptions', { data: { endpoint: sub.endpoint } })
   expect(sub.unsubscribe).toHaveBeenCalledOnce()
 })
 it('does not leave an account subscribed if removing the device fails', async () => {
   const sub = browser()
   vi.mocked(api.delete).mockRejectedValue(new Error('network'))
   await expect(disablePush()).rejects.toThrow('network')
   expect(sub.unsubscribe).not.toHaveBeenCalled()
 })
})
