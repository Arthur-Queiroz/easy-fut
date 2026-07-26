<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Charge } from '../../api/types'
import { formatCents, formatDateShort, formatDMY, formatMonth } from '../../lib/format'
import MobileShell from '../../components/layout/MobileShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Badge from '../../components/ui/Badge.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const charges = ref<Charge[]>([])
const copied = ref(false)
const pixError = ref('')

onMounted(async () => {
  await loadCharges()
})

async function loadCharges() {
  const { data } = await api.get<Charge[]>('/charges/me')
  charges.value = data ?? []

  const charge = charges.value.find((item) => item.status === 'pending' || item.status === 'overdue')
  if (!charge || charge.pix_payload) return

  try {
    const { data: pixCharge } = await api.post<Charge>(`/charges/${charge.id}/pix`)
    charges.value = charges.value.map((item) => (item.id === pixCharge.id ? pixCharge : item))
  } catch (error) {
    pixError.value = errorMessage(error)
  }
}

const current = computed(() => charges.value.find((c) => c.status === 'pending' || c.status === 'overdue') ?? null)
const history = computed(() => charges.value.filter((c) => c !== current.value))

async function copyPix() {
  if (!current.value?.pix_payload) return
  await navigator.clipboard.writeText(current.value.pix_payload)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function statusBadge(charge: Charge): { tone: 'success' | 'warn' | 'danger' | 'neutral'; label: string } {
  switch (charge.status) {
    case 'paid':
      return { tone: 'success', label: `Pago${charge.paid_at ? ' · ' + formatDMY(charge.paid_at.slice(0, 10)) : ''}` }
    case 'manual_paid':
      return { tone: 'success', label: 'Pago · manual' }
    case 'pending':
      return { tone: 'warn', label: 'Aguardando' }
    case 'overdue':
      return { tone: 'danger', label: 'Vencida' }
    case 'exempt':
      return { tone: 'neutral', label: 'Isento' }
    default:
      return { tone: 'neutral', label: 'Cancelada' }
  }
}
</script>

<template>
  <MobileShell>
    <template #header>
      <div class="text-[22px] font-bold">Pagamentos</div>
      <div class="text-[13.5px] text-white/65">Mensalidade da quadra · rateio entre os ativos</div>
    </template>

    <div class="flex flex-col gap-3.5 px-4 pt-4">
      <!-- Cobrança atual -->
      <Card v-if="current" class="p-4">
        <div class="flex items-center justify-between">
          <SectionLabel>{{ formatMonth(current.reference_month) }}</SectionLabel>
          <Badge :tone="current.status === 'overdue' ? 'danger' : 'warn'" dot>
            {{ current.status === 'overdue' ? 'Vencida' : 'Aguardando pagamento' }}
          </Badge>
        </div>

        <div class="mt-3.5 flex items-center gap-4">
          <div
            class="flex h-[124px] w-[124px] shrink-0 items-center justify-center rounded-[14px] border border-border bg-white p-2 text-center"
          >
            <img
              v-if="current.pix_qr_code_base64"
              :src="`data:image/jpeg;base64,${current.pix_qr_code_base64}`"
              alt="QR Code para pagamento via Pix"
              class="h-full w-full object-contain"
            />
            <span v-else class="px-2 text-[11px] leading-snug text-[#8CA094]">Gerando QR Code PIX…</span>
          </div>
          <div class="flex flex-1 flex-col gap-1">
            <div class="font-condensed text-[34px] font-bold leading-none">{{ formatCents(current.amount_cents) }}</div>
            <div class="text-[13px] text-ink2">
              Vence <strong :class="current.status === 'overdue' ? 'text-danger' : 'text-warn'">{{ formatDateShort(current.due_date) }}</strong>
              <br />5º dia útil após a geração
            </div>
          </div>
        </div>

        <div
          v-if="current.pix_payload"
          class="mt-3.5 flex items-center gap-2 rounded-xl border border-border bg-surface2 px-3 py-2.5"
        >
          <span class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-ink2">
            {{ current.pix_payload }}
          </span>
          <button type="button" class="inline-flex shrink-0 items-center gap-1.5 text-[12.5px] font-bold text-brand" @click="copyPix">
            <NavIcon name="copy" :size="14" :stroke-width="1.9" />
            {{ copied ? 'Copiado!' : 'Copiar' }}
          </button>
        </div>
        <div v-else class="mt-3.5 rounded-xl bg-infoBg px-3.5 py-3 text-[12.5px] leading-relaxed text-info">
          {{ pixError || 'Gerando o código PIX…' }}
        </div>
        <a
          v-if="current.pix_ticket_url"
          :href="current.pix_ticket_url"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-3.5 inline-flex text-[12.5px] font-bold text-brand"
        >
          Abrir instruções de pagamento
        </a>
      </Card>

      <Card v-else class="p-4">
        <EmptyState>Nenhuma cobrança em aberto. Tudo em dia! 🎉</EmptyState>
      </Card>

      <!-- Histórico -->
      <Card class="p-4">
        <SectionLabel>Histórico</SectionLabel>
        <div class="mt-2.5 flex flex-col">
          <div
            v-for="(charge, i) in history"
            :key="charge.id"
            class="flex items-center gap-3 py-[9px]"
            :class="i < history.length - 1 ? 'border-b border-border' : ''"
          >
            <div
              class="flex h-[30px] w-[30px] items-center justify-center rounded-full"
              :class="charge.status === 'paid' || charge.status === 'manual_paid' ? 'bg-brandSoft text-brand' : 'bg-surface2 text-ink3'"
            >
              <NavIcon name="check" :size="15" :stroke-width="2.4" />
            </div>
            <div class="flex-1">
              <div class="text-sm font-semibold">{{ formatMonth(charge.reference_month) }}</div>
              <div class="text-xs text-ink3">
                <template v-if="charge.paid_at">
                  Pago em {{ formatDMY(charge.paid_at.slice(0, 10)) }}
                  · {{ charge.paid_method === 'pix' ? 'PIX' : `registrado por ${charge.registered_by_name}` }}
                </template>
                <template v-else>{{ statusBadge(charge).label }}</template>
              </div>
            </div>
            <span class="text-sm font-semibold text-ink2">{{ formatCents(charge.amount_cents) }}</span>
          </div>
          <p v-if="history.length === 0" class="pt-2 text-sm text-ink3">Nenhum pagamento anterior.</p>
        </div>
      </Card>
    </div>
  </MobileShell>
</template>
