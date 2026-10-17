import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import { describe, expect, it, vi } from 'vitest'

function worker() {
 const listeners: Record<string, (event: any) => void> = {}
 const showNotification = vi.fn().mockResolvedValue(undefined)
 const openWindow = vi.fn().mockResolvedValue(undefined)
 const self = { location: { origin: 'https://fut.example.com' }, registration: { showNotification }, clients: { matchAll: vi.fn().mockResolvedValue([]), openWindow }, addEventListener: (name: string, handler: (event: any) => void) => { listeners[name] = handler } }
 runInNewContext(readFileSync('public/push-sw.js', 'utf8'), { self, URL })
 return { listeners, showNotification, openWindow }
}
describe('push service worker', () => {
 it('shows the supplied notification and exact match link', async () => {
   const w = worker(); let pending: Promise<unknown> | undefined
   w.listeners.push({ data: { json: () => ({ title: 'Nova partida', body: 'Confirme', url: '/partida?match=abc' }) }, waitUntil: (p: Promise<unknown>) => { pending = p } })
   await pending
   expect(w.showNotification).toHaveBeenCalledWith('Nova partida', expect.objectContaining({ data: { url: 'https://fut.example.com/partida?match=abc' } }))
 })
 it('restricts push links to the app origin', () => {
   const w = worker()
   w.listeners.push({ data: { json: () => ({ url: 'https://evil.example.com' }) }, waitUntil: () => {} })
   expect(w.showNotification).toHaveBeenCalledWith('Easy Fut', expect.objectContaining({ data: { url: 'https://fut.example.com/partida' } }))
 })
 it('opens the exact match after tapping a notification', async () => {
   const w = worker(); let pending: Promise<unknown> | undefined
   const close = vi.fn()
   w.listeners.notificationclick({ notification: { data: { url: 'https://fut.example.com/partida?match=abc' }, close }, waitUntil: (p: Promise<unknown>) => { pending = p } })
   await pending
   expect(close).toHaveBeenCalledOnce()
   expect(w.openWindow).toHaveBeenCalledWith('https://fut.example.com/partida?match=abc')
 })
})
