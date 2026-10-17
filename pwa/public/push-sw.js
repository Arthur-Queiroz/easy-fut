/* Loaded by the generated Workbox service worker. No API responses are cached. */
self.addEventListener('push', (event) => {
  let message = {}
  try { message = event.data ? event.data.json() : {} } catch { /* Show a useful fallback. */ }
  const candidate = new URL(message.url || '/partida', self.location.origin)
  const url = candidate.origin === self.location.origin ? candidate.href : new URL('/partida', self.location.origin).href
  event.waitUntil(self.registration.showNotification(message.title || 'Easy Fut', {
    body: message.body || 'Abra o app para conferir sua próxima partida.',
    icon: '/icons/pwa-192x192.png',
    badge: '/icons/pwa-64x64.png',
    tag: message.tag || 'easy-fut-match',
    data: { url },
  }))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil((async () => {
    const target = new URL(event.notification.data?.url || '/partida', self.location.origin)
    if (target.origin !== self.location.origin) return
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    const existing = windows.find((client) => new URL(client.url).origin === self.location.origin)
    if (existing) { await existing.navigate(target.href); await existing.focus() }
    else await self.clients.openWindow(target.href)
  })())
})
