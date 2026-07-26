<script setup lang="ts">
import { ref } from 'vue'
import { api, errorMessage } from '../../api/client'

const email = ref('')
const loading = ref(false)
const sent = ref(false)
const error = ref('')

async function submit() {
  loading.value = true
  error.value = ''
  try {
    await api.post('/password-reset/request', { email: email.value })
    sent.value = true
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
      <h1 class="font-condensed text-3xl font-bold">Recuperar senha</h1>
      <p class="mt-2 text-sm text-white/70">Informe seu e-mail para receber um link de redefini&ccedil;&atilde;o.</p>
      <label v-if="!sent" class="mt-6 flex flex-col gap-1.5">
        <span class="text-sm font-semibold">E-mail</span>
        <input v-model="email" required type="email" autocomplete="email" class="h-[50px] rounded-xl border border-white/15 bg-white/5 px-4 text-[15px] text-white outline-none placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:placeholder:text-ink3 md:focus:border-brand" placeholder="voce@email.com" />
      </label>
      <p v-if="sent" class="mt-6 rounded-xl bg-white/10 p-4 text-sm text-white/80">
        Se existir uma conta com esse e-mail, voc&ecirc; receber&aacute; um link em alguns minutos.
      </p>
      <p v-if="error" class="mt-4 text-sm text-danger">{{ error }}</p>
      <button v-if="!sent" :disabled="loading" class="mt-6 h-12 w-full rounded-xl bg-lime font-bold text-pitch-2 disabled:opacity-60">
        {{ loading ? 'Enviando...' : 'Enviar link' }}
      </button>
      <router-link :to="{ name: 'login' }" class="mt-5 block text-center text-sm font-semibold text-lime">Voltar ao login</router-link>
    </form>
  </main>
</template>
