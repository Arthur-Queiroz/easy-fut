<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'

const route = useRoute()
const router = useRouter()
const password = ref('')
const confirmation = ref('')
const showPassword = ref(false)
const showConfirmation = ref(false)
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  if (password.value !== confirmation.value) {
    error.value = 'As senhas n\u00e3o coincidem.'
    return
  }
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (!token) {
    error.value = 'Link inv\u00e1lido ou expirado.'
    return
  }
  loading.value = true
  try {
    await api.post('/password-reset/confirm', { token, password: password.value })
    router.replace({ name: 'login' })
  } catch (cause) {
    error.value = errorMessage(cause)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="flex min-h-dvh items-center justify-center bg-pitch-2 px-6 text-white">
    <form class="w-full max-w-md rounded-3xl bg-white/5 p-7" @submit.prevent="submit">
      <h1 class="font-condensed text-3xl font-bold">Definir nova senha</h1>
      <p class="mt-2 text-sm text-white/70">Use ao menos 8 caracteres, uma letra mai&uacute;scula e um n&uacute;mero.</p>

      <label class="mt-6 flex flex-col gap-1.5">
        <span class="text-sm font-semibold">Nova senha</span>
        <span class="relative block">
          <input v-model="password" required :type="showPassword ? 'text' : 'password'" minlength="8" autocomplete="new-password" class="input pr-20" />
          <button type="button" class="absolute inset-y-0 right-3 text-sm font-semibold text-lime" :aria-label="showPassword ? 'Ocultar senha' : 'Mostrar senha'" @click="showPassword = !showPassword">
            {{ showPassword ? 'Ocultar' : 'Mostrar' }}
          </button>
        </span>
      </label>

      <label class="mt-4 flex flex-col gap-1.5">
        <span class="text-sm font-semibold">Confirmar senha</span>
        <span class="relative block">
          <input v-model="confirmation" required :type="showConfirmation ? 'text' : 'password'" minlength="8" autocomplete="new-password" class="input pr-20" />
          <button type="button" class="absolute inset-y-0 right-3 text-sm font-semibold text-lime" :aria-label="showConfirmation ? 'Ocultar senha' : 'Mostrar senha'" @click="showConfirmation = !showConfirmation">
            {{ showConfirmation ? 'Ocultar' : 'Mostrar' }}
          </button>
        </span>
      </label>

      <p v-if="error" class="mt-4 text-sm text-danger">{{ error }}</p>
      <button :disabled="loading" class="mt-6 h-12 w-full rounded-xl bg-lime font-bold text-pitch-2 disabled:opacity-60">{{ loading ? 'Salvando...' : 'Salvar nova senha' }}</button>
    </form>
  </main>
</template>
