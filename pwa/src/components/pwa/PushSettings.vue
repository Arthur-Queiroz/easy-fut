<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import { applicationServerKey, disablePush, pushRegistration, supportsPush } from '../../lib/push'
import BaseButton from '../ui/BaseButton.vue'
import Card from '../ui/Card.vue'

const supported = supportsPush()
const loading = ref(true)
const busy = ref(false)
const subscribed = ref(false)
const enabled = ref(false)
const publicKey = ref('')
const error = ref('')
const blocked = ref(supported && Notification.permission === 'denied')

onMounted(async () => {
  try {
    const { data } = await api.get<{ enabled: boolean; public_key: string }>('/push/config')
    enabled.value = data.enabled
    publicKey.value = data.public_key
    if (supported) {
      const registration = await navigator.serviceWorker.getRegistration('/')
      const subscription = await registration?.pushManager.getSubscription()
      if (subscription) {
        const { data: status } = await api.post<{ subscribed: boolean }>('/push/subscriptions/status', { endpoint: subscription.endpoint })
        subscribed.value = status.subscribed && Notification.permission === 'granted'
      }
    }
  } catch (e) { error.value = errorMessage(e) }
  finally { loading.value = false }
})

async function toggle() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    if (subscribed.value) {
      await disablePush()
      subscribed.value = false
      return
    }
    // Request before awaiting anything else: iOS requires a direct user gesture.
    const permission = await Notification.requestPermission()
    blocked.value = permission === 'denied'
    if (permission !== 'granted') {
      error.value = 'Permita as notificações para receber os avisos de partida.'
      return
    }
    const registration = await pushRegistration()
    let subscription = await registration.pushManager.getSubscription()
    const existingKey = subscription?.options.applicationServerKey
    const desiredKey = applicationServerKey(publicKey.value)
    if (subscription && existingKey && !new Uint8Array(existingKey).every((value, i) => value === desiredKey[i])) {
      await subscription.unsubscribe()
      subscription = null
    }
    subscription ??= await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: desiredKey })
    await api.post('/push/subscriptions', subscription.toJSON())
    subscribed.value = true
  } catch (e) { error.value = e instanceof Error && !('response' in e) ? e.message : errorMessage(e) }
  finally { busy.value = false }
}
</script>

<template>
  <Card class="p-4">
    <h2 class="text-sm font-semibold">Notificações de partidas</h2>
    <p class="mt-1 text-sm text-ink3">Receba avisos de novas partidas e lembretes para confirmar sua presença neste aparelho.</p>
    <p v-if="!supported" class="mt-2 text-sm text-ink2">No iPhone, adicione o app à tela inicial pelo Safari e abra pelo ícone. Em outros aparelhos, use um navegador compatível com notificações.</p>
    <p v-else-if="blocked" class="mt-2 text-sm text-ink2">As notificações estão bloqueadas. Permita os avisos nas configurações do navegador ou do app.</p>
    <p v-else-if="!loading && !enabled" class="mt-2 text-sm text-ink2">Os avisos estarão disponíveis em breve.</p>
    <p v-if="subscribed" class="mt-2 text-sm font-medium text-brand">Notificações ativadas neste aparelho.</p>
    <BaseButton v-if="supported && !blocked && enabled" class="mt-3" size="sm" :loading="busy" :disabled="loading" @click="toggle">
      {{ subscribed ? 'Desativar notificações' : 'Ativar notificações' }}
    </BaseButton>
    <p v-if="error" role="alert" class="mt-2 text-sm text-danger">{{ error }}</p>
  </Card>
</template>
