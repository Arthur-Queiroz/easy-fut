<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { errorMessage } from '../../api/client'
import { useAuthStore } from '../../stores/auth'
import NavIcon from '../../components/layout/NavIcon.vue'

const router = useRouter()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push(auth.isActive ? '/' : '/bloqueado')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div
    class="flex min-h-dvh w-full flex-col px-7 pb-12 pt-24 text-white md:grid md:grid-cols-2 md:items-center md:gap-20 md:px-[12vw] md:py-16"
    style="background-image: linear-gradient(165deg, #082a1d 0%, #0a3b28 45%, #0f5238 100%)"
  >
    <div class="flex flex-col items-center text-center md:items-start md:text-left">
      <div class="flex h-[92px] w-[92px] items-center justify-center rounded-full border-2 border-lime bg-lime/10 text-lime">
        <NavIcon name="ball" :size="46" :stroke-width="1.6" />
      </div>
      <div class="mt-[18px] font-condensed text-[30px] font-bold leading-none tracking-[.14em] text-lime">
        FUT DA RAPAZIADA
      </div>
      <div class="mt-1 text-[13px] font-semibold tracking-[.22em] text-white/55">GESTÃO DO GRUPO</div>
    </div>

    <form class="mt-12 flex flex-col gap-4 md:mt-0 md:rounded-3xl md:bg-surface md:px-10 md:py-12 md:text-ink md:shadow-2xl" @submit.prevent="submit">
      <label class="flex flex-col gap-1.5">
        <span class="text-[13px] font-semibold text-white/70 md:text-ink2">E-mail</span>
        <input
          v-model="email"
          type="email"
          required
          autocomplete="email"
          class="h-[50px] rounded-xl border border-white/15 bg-white/5 px-4 text-[15px] text-white outline-none placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:placeholder:text-ink3 md:focus:border-brand"
          placeholder="voce@email.com"
        />
      </label>
      <label class="flex flex-col gap-1.5">
        <span class="text-[13px] font-semibold text-white/70 md:text-ink2">Senha</span>
        <input
          v-model="password"
          type="password"
          required
          autocomplete="current-password"
          class="h-[50px] rounded-xl border border-white/15 bg-white/5 px-4 text-[15px] text-white outline-none placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:placeholder:text-ink3 md:focus:border-brand"
          placeholder="••••••••"
        />
      </label>

      <p v-if="error" class="rounded-xl bg-white/10 px-4 py-3 text-[13px] font-medium text-[#ffb4b4] md:bg-dangerBg md:text-danger">{{ error }}</p>

      <button
        type="submit"
        :disabled="loading"
        class="mt-2 flex h-[54px] w-full items-center justify-center rounded-[14px] bg-lime text-base font-bold text-pitch-2 disabled:opacity-60"
      >
        {{ loading ? 'Entrando…' : 'Entrar' }}
      </button>
      <p class="text-center text-[13px] text-white/55">O cadastro é feito apenas por link de convite</p>
    </form>
  </div>
</template>

<style scoped>
@media (min-width: 768px) {
  form > p:last-child {
    color: var(--ink3);
  }
}
</style>
