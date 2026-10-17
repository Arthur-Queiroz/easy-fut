<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import { validatePassword } from '../../lib/security'
import PasswordInput from '../../components/ui/PasswordInput.vue'

const route = useRoute()
const router = useRouter()
const password = ref('')
const confirmation = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  if (password.value !== confirmation.value) {
    error.value = 'As senhas não coincidem.'
    return
  }
  const passwordCheck = validatePassword(password.value)
  if (!passwordCheck.ok) {
    error.value = passwordCheck.error
    return
  }
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (!token) {
    error.value = 'Link inválido ou expirado.'
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
  <main class="flex min-h-dvh items-center justify-center px-6 md:bg-pitch-2" style="background-image: linear-gradient(165deg, #0c100f 0%, #13251f 100%)">
    <form class="w-full max-w-md rounded-3xl bg-white/5 p-7 md:bg-surface md:text-ink md:shadow-2xl" @submit.prevent="submit">
      <h1 class="font-condensed text-3xl font-bold text-white md:text-ink">Definir nova senha</h1>
      <p class="mt-2 text-sm text-white/70 md:text-ink2">Use ao menos 8 caracteres, uma letra maiúscula e um número.</p>

      <div class="mt-6 space-y-1.5">
        <label for="new-password" class="block text-sm font-semibold text-white/70 md:text-ink2">Nova senha</label>
        <PasswordInput
          id="new-password"
          v-model="password"
          variant="on-dark"
          required
          minlength="8"
          pattern="(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}"
          title="Mínimo 8 caracteres, com letra maiúscula, minúscula e número"
          autocomplete="new-password"
        />
      </div>

      <div class="mt-4 space-y-1.5">
        <label for="confirm-password" class="block text-sm font-semibold text-white/70 md:text-ink2">Confirmar senha</label>
        <PasswordInput
          id="confirm-password"
          v-model="confirmation"
          variant="on-dark"
          required
          minlength="8"
          pattern="(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}"
          title="Mínimo 8 caracteres, com letra maiúscula, minúscula e número"
          autocomplete="new-password"
        />
      </div>

      <p v-if="error" class="mt-4 text-sm text-danger">{{ error }}</p>
      <button :disabled="loading" class="mt-6 h-12 w-full rounded-xl bg-lime font-bold text-pitch-1 disabled:opacity-60">{{ loading ? 'Salvando...' : 'Salvar nova senha' }}</button>
    </form>
  </main>
</template>
