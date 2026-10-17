import { api } from '../api/client'

export function supportsPush() {
  return window.isSecureContext && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
}

export function applicationServerKey(key: string): Uint8Array<ArrayBuffer> {
  const decoded = atob(key.replace(/-/g, '+').replace(/_/g, '/'))
  return Uint8Array.from(decoded, (c) => c.charCodeAt(0))
}

export async function pushRegistration(): Promise<ServiceWorkerRegistration> {
  const registration = await navigator.serviceWorker.getRegistration('/')
  if (!registration) throw new Error('Reabra o app para concluir a atualização e tente novamente.')
  return await Promise.race([
    navigator.serviceWorker.ready,
    new Promise<never>((_, reject) => setTimeout(() => reject(new Error('O app está atualizando. Reabra e tente novamente.')), 10000)),
  ])
}

export async function disablePush() {
  if (!supportsPush()) return
  const registration = await navigator.serviceWorker.getRegistration('/')
  const subscription = await registration?.pushManager.getSubscription()
  if (!subscription) return
  await api.delete('/push/subscriptions', { data: { endpoint: subscription.endpoint } })
  if (!await subscription.unsubscribe()) throw new Error('Não foi possível desativar as notificações neste aparelho.')
}
