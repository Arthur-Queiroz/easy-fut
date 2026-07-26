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

      <div class="mt-6">
        <label for="new-password" class="block text-sm font-semibold">Nova senha</label>
        <div class="relative mt-1.5">
          <input id="new-password" v-model="password" required :type="showPassword ? 'text' : 'password'" minlength="8" autocomplete="new-password" class="h-[50px] w-full rounded-xl border border-white/15 bg-white/5 px-4 pr-14 text-[15px] text-white outline-none placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:focus:border-brand" />
          <button type="button" class="absolute right-1.5 top-1/2 flex size-9 -translate-y-1/2 items-center justify-center rounded-lg text-white/70 transition hover:bg-white/10 hover:text-lime focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lime md:text-ink2 md:hover:bg-brandSoft md:hover:text-brand" :aria-label="showPassword ? 'Ocultar senha' : 'Mostrar senha'" @pointerdown.prevent @click.stop="showPassword = !showPassword">
            <svg v-if="showPassword" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M3 3l18 18M10.6 10.6a2 2 0 002.8 2.8M9.9 4.2A10.7 10.7 0 0112 4c6 0 9.5 8 9.5 8a17.3 17.3 0 01-3 3.8M6.2 6.2C3.8 8.1 2.5 12 2.5 12s3.5 8 9.5 8a10.8 10.8 0 004-0.8" />
            </svg>
            <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M2.5 12s3.5-8 9.5-8 9.5 8 9.5 8-3.5 8-9.5 8-9.5-8-9.5-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          </button>
        </div>
      </div>

      <div class="mt-4">
        <label for="confirm-password" class="block text-sm font-semibold">Confirmar senha</label>
        <div class="relative mt-1.5">
          <input id="confirm-password" v-model="confirmation" required :type="showConfirmation ? 'text' : 'password'" minlength="8" autocomplete="new-password" class="h-[50px] w-full rounded-xl border border-white/15 bg-white/5 px-4 pr-14 text-[15px] text-white outline-none placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:focus:border-brand" />
          <button type="button" class="absolute right-1.5 top-1/2 flex size-9 -translate-y-1/2 items-center justify-center rounded-lg text-white/70 transition hover:bg-white/10 hover:text-lime focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lime md:text-ink2 md:hover:bg-brandSoft md:hover:text-brand" :aria-label="showConfirmation ? 'Ocultar senha' : 'Mostrar senha'" @pointerdown.prevent @click.stop="showConfirmation = !showConfirmation">
            <svg v-if="showConfirmation" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M3 3l18 18M10.6 10.6a2 2 0 002.8 2.8M9.9 4.2A10.7 10.7 0 0112 4c6 0 9.5 8 9.5 8a17.3 17.3 0 01-3 3.8M6.2 6.2C3.8 8.1 2.5 12 2.5 12s3.5 8 9.5 8a10.8 10.8 0 004-0.8" />
            </svg>
            <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M2.5 12s3.5-8 9.5-8 9.5 8 9.5 8-3.5 8-9.5 8-9.5-8-9.5-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          </button>
        </div>
      </div>

      <p v-if="error" class="mt-4 text-sm text-danger">{{ error }}</p>
      <button :disabled="loading" class="mt-6 h-12 w-full rounded-xl bg-lime font-bold text-pitch-2 disabled:opacity-60">{{ loading ? 'Salvando...' : 'Salvar nova senha' }}</button>
    </form>
  </main>
</template>
