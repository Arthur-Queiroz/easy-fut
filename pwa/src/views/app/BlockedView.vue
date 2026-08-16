<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../api/client'
import type { Charge } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { formatCents, formatMonth } from '../../lib/format'
import NavIcon from '../../components/layout/NavIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const charges = ref<Charge[]>([])

onMounted(async () => {
  await auth.fetchMe()
  if (auth.isActive) {
    router.replace('/')
    return
  }
  const { data } = await api.get<Charge[]>('/charges/me')
  charges.value = (data ?? []).filter((c) => c.status === 'overdue' || c.status === 'pending')
})

const totalDue = computed(() => charges.value.reduce((sum, c) => sum + c.amount_cents, 0))

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div
    class="mx-auto flex min-h-dvh w-full max-w-md flex-col items-center justify-center px-7 text-center text-white"
    style="background-image: linear-gradient(165deg, #0b1210 0%, #132a20 45%, #102a20 100%)"
  >
    <div class="flex h-[92px] w-[92px] items-center justify-center rounded-full border-2 border-[#F17070] bg-[#F17070]/10 text-[#F17070]">
      <NavIcon name="x" :size="40" :stroke-width="2" />
    </div>
    <h1 class="mt-6 text-[26px] font-bold">Acesso bloqueado</h1>
    <p class="mt-2.5 max-w-[300px] text-[15px] leading-normal text-white/75">
      {{ auth.user?.inactive_reason || 'Sua conta está inativa. Fale com um administrador.' }}
    </p>

    <div v-if="charges.length" class="mt-7 w-full rounded-2xl border border-white/15 bg-white/5 px-[18px] py-4 text-left">
      <div class="text-[11px] font-bold uppercase tracking-[.1em] text-white/50">Pendências</div>
      <div v-for="charge in charges" :key="charge.id" class="mt-3 flex justify-between text-[13.5px] text-white/70">
        <span>{{ formatMonth(charge.reference_month) }}</span>
        <span class="font-semibold text-white">{{ formatCents(charge.amount_cents) }}</span>
      </div>
      <div class="mt-3 border-t border-white/10 pt-3 text-[13.5px] text-white/70">
        Total: <strong class="text-lime">{{ formatCents(totalDue) }}</strong>
      </div>
      <p class="mt-3 text-xs leading-relaxed text-white/50">
        Após a confirmação do pagamento pelo administrador, seu acesso volta automaticamente.
      </p>
    </div>

    <button type="button" class="mt-8 text-[13.5px] font-semibold text-white/60 underline-offset-4 hover:underline" @click="logout">
      Sair da conta
    </button>
  </div>
</template>
