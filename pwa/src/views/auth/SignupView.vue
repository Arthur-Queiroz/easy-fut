<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import { validatePassword } from '../../lib/security'
import type { InvitePublic, User } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import BaseButton from '../../components/ui/BaseButton.vue'
import PasswordInput from '../../components/ui/PasswordInput.vue'
import NavIcon from '../../components/layout/NavIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const token = route.params.token as string

const invite = ref<InvitePublic | null>(null)
const name = ref('')
const email = ref('')
const phone = ref('')
const password = ref('')
const error = ref('')
const saving = ref(false)

onMounted(async () => {
  try {
    const { data } = await api.get<InvitePublic>(`/invites/${token}`)
    if (data.status !== 'pending') {
      router.replace(`/convite/${token}`)
      return
    }
    invite.value = data
    name.value = data.invited_name
  } catch {
    router.replace(`/convite/${token}`)
  }
})

async function submit() {
  error.value = ''

  const passwordCheck = validatePassword(password.value)
  if (!passwordCheck.ok) {
    error.value = passwordCheck.error
    return
  }

  saving.value = true
  try {
    const { data } = await api.post<User>(`/invites/${token}/signup`, {
      name: name.value,
      email: email.value,
      phone: phone.value,
      password: password.value,
    })
    auth.user = data
    auth.loaded = true
    router.push('/')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-bg md:items-center md:justify-center md:bg-pitch-2 md:px-[12vw] md:py-16">
    <header
      class="px-6 pb-[22px] pt-16 text-white md:hidden"
      style="background-image: linear-gradient(150deg, #0f3325, #1a6644)"
    >
      <h1 class="text-2xl font-bold">Criar sua conta</h1>
      <span
        class="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-lime/15 px-3 py-1 text-[12.5px] font-semibold text-lime"
      >
        <NavIcon name="check" :size="13" :stroke-width="2.4" />
        Convite validado
      </span>
    </header>

    <form class="flex flex-1 flex-col gap-4 px-6 py-[22px] md:max-w-md md:flex-none md:rounded-3xl md:bg-surface md:px-8 md:py-10 md:shadow-2xl" @submit.prevent="submit">
      <div class="hidden pb-2 md:block">
        <h1 class="text-2xl font-bold text-ink">Criar sua conta</h1>
        <span
          class="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-brandSoft px-3 py-1 text-[12.5px] font-semibold text-brand"
        >
          <NavIcon name="check" :size="13" :stroke-width="2.4" />
          Convite validado
        </span>
      </div>

      <label class="flex flex-col gap-1.5">
        <span class="text-[13px] font-semibold text-ink2">Nome completo</span>
        <input v-model="name" required autocomplete="name" class="input" placeholder="Seu nome" />
      </label>
      <label class="flex flex-col gap-1.5">
        <span class="text-[13px] font-semibold text-ink2">E-mail</span>
        <input v-model="email" required type="email" autocomplete="email" class="input" placeholder="voce@email.com" />
      </label>
      <label class="flex flex-col gap-1.5">
        <span class="text-[13px] font-semibold text-ink2">WhatsApp</span>
        <input v-model="phone" type="tel" autocomplete="tel" class="input" placeholder="(11) 98765-4321" />
        <span class="text-xs text-ink3">Usado para os lembretes de pagamento</span>
      </label>
      <!-- Sem <label> envolvendo: o botão de ocultar entraria no nome acessível do campo. -->
      <div class="flex flex-col gap-1.5">
        <label for="signup-password" class="text-[13px] font-semibold text-ink2">Senha</label>
        <PasswordInput
          id="signup-password"
          v-model="password"
          required
          minlength="8"
          pattern="(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}"
          title="Mínimo 8 caracteres, com letra maiúscula, minúscula e número"
          autocomplete="new-password"
          placeholder="Mínimo 8 caracteres"
        />
      </div>

      <p v-if="error" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ error }}</p>

      <div class="mt-auto flex flex-col gap-2.5 pb-6 md:mt-2 md:pb-0">
        <BaseButton size="lg" :loading="saving" @click="submit">Concluir cadastro</BaseButton>
        <p class="text-center text-[11.5px] text-ink3">Ao continuar você entra como usuário ativo do grupo</p>
      </div>
    </form>
  </div>
</template>

<style scoped>
.input {
  @apply h-[50px] rounded-xl border border-border bg-surface px-4 text-[15px] text-ink outline-none focus:border-brand;
}
</style>
