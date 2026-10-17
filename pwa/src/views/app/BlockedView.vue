<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import type { Charge } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { formatCents, formatMonth } from '../../lib/format'
import { safeUrl } from '../../lib/security'
import NavIcon from '../../components/layout/NavIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const charges = ref<Charge[]>([])
const copiedFor = ref('') // id da cobrança com o Pix copiado
const pixErrors = ref<Record<string, string>>({})
const generatingPixFor = ref('')
const checking = ref(false)
const checkMessage = ref('')

onMounted(async () => {
  await auth.fetchMe()
  if (auth.isActive) {
    router.replace('/')
    return
  }
  await loadCharges()
})

async function loadCharges() {
  const { data } = await api.get<Charge[]>('/charges/me')
  charges.value = (data ?? []).filter((c) => c.status === 'overdue' || c.status === 'pending')
}

const totalDue = computed(() => charges.value.reduce((sum, c) => sum + c.amount_cents, 0))
const hasPix = computed(() => charges.value.some((c) => c.pix_payload))

function chargeTitle(charge: Charge) {
  return charge.batch_kind === 'match' ? charge.batch_title : formatMonth(charge.reference_month)
}

// Pix sob demanda: a API aceita usuários inativos nesta rota justamente para quitar o bloqueio.
async function generatePix(charge: Charge) {
  const { [charge.id]: _ignored, ...rest } = pixErrors.value
  pixErrors.value = rest
  generatingPixFor.value = charge.id
  try {
    const { data: pixCharge } = await api.post<Charge>(`/charges/${charge.id}/pix`)
    charges.value = charges.value.map((item) => (item.id === pixCharge.id ? pixCharge : item))
  } catch (error) {
    pixErrors.value = { ...pixErrors.value, [charge.id]: errorMessage(error) }
  } finally {
    generatingPixFor.value = ''
  }
}

async function copyPix(charge: Charge) {
  if (!charge.pix_payload) return
  await navigator.clipboard.writeText(charge.pix_payload)
  copiedFor.value = charge.id
  setTimeout(() => (copiedFor.value = ''), 2000)
}

// Reconsulta o Mercado Pago (o POST reconcilia Pix já gerado) e libera o acesso se tudo foi quitado.
async function checkPayment() {
  checking.value = true
  checkMessage.value = ''
  try {
    await Promise.allSettled(
      charges.value.filter((c) => c.pix_payload).map((c) => api.post<Charge>(`/charges/${c.id}/pix`)),
    )
    await auth.fetchMe()
    if (auth.isActive) {
      router.replace('/')
      return
    }
    await loadCharges()
    checkMessage.value = charges.value.length
      ? 'Pagamento ainda não confirmado. Pode levar alguns instantes — tente de novo.'
      : 'Pagamento confirmado. Aguardando liberação do acesso.'
  } catch (error) {
    checkMessage.value = errorMessage(error)
  } finally {
    checking.value = false
  }
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div
    class="mx-auto flex min-h-dvh w-full max-w-md flex-col items-center justify-center px-7 py-10 text-center text-white"
    style="background-image: linear-gradient(165deg, #0c100f 0%, #13251f 45%, #0f3325 100%)"
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

      <div v-for="charge in charges" :key="charge.id" class="mt-3 border-b border-white/10 pb-3.5 last:border-b-0">
        <div class="flex justify-between text-[13.5px] text-white/70">
          <span>{{ chargeTitle(charge) }}</span>
          <span class="font-semibold text-white">{{ formatCents(charge.amount_cents) }}</span>
        </div>

        <template v-if="charge.pix_payload">
          <div
            v-if="charge.pix_qr_code_base64"
            class="mx-auto mt-3 flex h-[168px] w-[168px] items-center justify-center rounded-[14px] bg-white p-2"
          >
            <img
              :src="`data:image/jpeg;base64,${charge.pix_qr_code_base64}`"
              alt="QR Code para pagamento via Pix"
              class="h-full w-full object-contain"
            />
          </div>
          <div class="mt-3 flex items-center gap-2 rounded-xl border border-white/15 bg-black/20 px-3 py-2.5">
            <span class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-white/60">
              {{ charge.pix_payload }}
            </span>
            <button type="button" class="inline-flex shrink-0 items-center gap-1.5 text-[12.5px] font-bold text-lime" @click="copyPix(charge)">
              <NavIcon name="copy" :size="14" :stroke-width="1.9" />
              {{ copiedFor === charge.id ? 'Copiado!' : 'Copiar' }}
            </button>
          </div>
          <a
            v-if="safeUrl(charge.pix_ticket_url)"
            :href="safeUrl(charge.pix_ticket_url)"
            target="_blank"
            rel="noopener noreferrer"
            class="mt-2.5 inline-flex text-[12.5px] font-bold text-lime"
          >
            Abrir instruções de pagamento
          </a>
        </template>
        <template v-else>
          <p v-if="pixErrors[charge.id]" class="mt-3 rounded-xl bg-[#F17070]/15 px-3.5 py-3 text-[12.5px] leading-relaxed text-[#F17070]">
            {{ pixErrors[charge.id] }}
          </p>
          <button
            type="button"
            class="mt-3 w-full rounded-xl border border-lime py-2.5 text-[13.5px] font-bold text-lime disabled:opacity-60"
            :disabled="generatingPixFor === charge.id"
            @click="generatePix(charge)"
          >
            {{ generatingPixFor === charge.id ? 'Gerando Pix…' : 'Pagar com Pix' }}
          </button>
        </template>
      </div>

      <div class="border-t border-white/10 pt-3 text-[13.5px] text-white/70">
        Total: <strong class="text-lime">{{ formatCents(totalDue) }}</strong>
      </div>
      <p class="mt-3 text-xs leading-relaxed text-white/50">
        Pagando via Pix, seu acesso volta automaticamente assim que o pagamento for confirmado.
      </p>
    </div>

    <button
      v-if="hasPix"
      type="button"
      class="mt-6 w-full rounded-xl bg-lime py-3 text-[14px] font-bold text-[#0c100f] disabled:opacity-60"
      :disabled="checking"
      @click="checkPayment"
    >
      {{ checking ? 'Verificando…' : 'Já paguei' }}
    </button>
    <p v-if="checkMessage" class="mt-3 max-w-[300px] text-[12.5px] leading-relaxed text-white/60">{{ checkMessage }}</p>

    <button type="button" class="mt-8 text-[13.5px] font-semibold text-white/60 underline-offset-4 hover:underline" @click="logout">
      Sair da conta
    </button>
  </div>
</template>
