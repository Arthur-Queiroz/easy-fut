<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Charge } from '../../api/types'
import { formatCents, formatDateShort, formatDMY, formatMonth } from '../../lib/format'
import { safeUrl } from '../../lib/security'
import MobileShell from '../../components/layout/MobileShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const charges = ref<Charge[]>([])
const copiedFor = ref('') // id da cobrança com o Pix copiado
const pixErrors = ref<Record<string, string>>({})
const generatingPixFor = ref('')

onMounted(async () => {
  await loadCharges()
})

async function loadCharges() {
  const { data } = await api.get<Charge[]>('/charges/me')
  charges.value = data ?? []
}

const openCharges = computed(() => charges.value.filter((c) => c.status === 'pending' || c.status === 'overdue'))
const history = computed(() => charges.value.filter((c) => c.status !== 'pending' && c.status !== 'overdue'))

function chargeTitle(charge: Charge) {
  return charge.batch_kind === 'match' ? charge.batch_title : formatMonth(charge.reference_month)
}

// Pix sob demanda: gera apenas quando o jogador pede, por cobrança.
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
      <div class="text-[13.5px] text-white/65">Mensalidades e partidas avulsas</div>
    </template>

    <div class="flex flex-col gap-3.5 px-4 pt-4 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden lg:block">
        <div class="text-2xl font-extrabold">Pagamentos</div>
        <div class="mt-1 text-[14px] text-ink2">Mensalidades e partidas avulsas</div>
      </div>

      <div class="grid gap-3.5 lg:grid-cols-[1.1fr_1fr] lg:items-start">
        <div class="flex min-w-0 flex-col gap-3.5">
          <!-- Uma cobrança em aberto por card -->
          <Card v-for="charge in openCharges" :key="charge.id" class="p-4 lg:p-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <SectionLabel>{{ chargeTitle(charge) }}</SectionLabel>
                <Badge v-if="charge.batch_kind === 'match'" tone="info">Avulsa</Badge>
              </div>
              <Badge :tone="charge.status === 'overdue' ? 'danger' : 'warn'" dot>
                {{ charge.status === 'overdue' ? 'Vencida' : 'Aguardando pagamento' }}
              </Badge>
            </div>

            <div class="mt-3.5 flex items-center gap-4 lg:mt-5 lg:gap-6">
              <div
                class="flex h-[124px] w-[124px] shrink-0 items-center justify-center rounded-[14px] border border-border bg-white p-2 text-center lg:h-[150px] lg:w-[150px]"
              >
                <img
                  v-if="charge.pix_qr_code_base64"
                  :src="`data:image/jpeg;base64,${charge.pix_qr_code_base64}`"
                  alt="QR Code para pagamento via Pix"
                  class="h-full w-full object-contain"
                />
                <span v-else class="px-2 text-[11px] leading-snug text-[#8CA094]">QR Code disponível após gerar o Pix</span>
              </div>
              <div class="flex flex-1 flex-col gap-1 lg:gap-1.5">
                <div class="font-condensed text-[34px] font-bold leading-none lg:text-[42px]">{{ formatCents(charge.amount_cents) }}</div>
                <div class="text-[13px] text-ink2 lg:text-[13.5px]">
                  Vence <strong :class="charge.status === 'overdue' ? 'text-danger' : 'text-warn'">{{ formatDateShort(charge.due_date) }}</strong>
                  <template v-if="charge.batch_kind === 'monthly'"><br />5º dia útil após a geração</template>
                </div>
                <div class="text-[12px] text-ink3 lg:text-[12.5px]">Lembrete no WhatsApp próximo ao vencimento</div>
              </div>
            </div>

            <div
              v-if="charge.pix_payload"
              class="mt-3.5 flex items-center gap-2 rounded-xl border border-border bg-surface2 px-3 py-2.5"
            >
              <span class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-ink2">
                {{ charge.pix_payload }}
              </span>
              <button type="button" class="inline-flex shrink-0 items-center gap-1.5 text-[12.5px] font-bold text-brand" @click="copyPix(charge)">
                <NavIcon name="copy" :size="14" :stroke-width="1.9" />
                {{ copiedFor === charge.id ? 'Copiado!' : 'Copiar' }}
              </button>
            </div>
            <template v-else>
              <p v-if="pixErrors[charge.id]" class="mt-3.5 rounded-xl bg-dangerBg px-3.5 py-3 text-[12.5px] leading-relaxed text-danger">
                {{ pixErrors[charge.id] }}
              </p>
              <BaseButton
                class="mt-3.5 w-full"
                variant="outline-brand"
                :loading="generatingPixFor === charge.id"
                @click="generatePix(charge)"
              >
                Gerar Pix
              </BaseButton>
            </template>
            <a
              v-if="safeUrl(charge.pix_ticket_url)"
              :href="safeUrl(charge.pix_ticket_url)"
              target="_blank"
              rel="noopener noreferrer"
              class="mt-3.5 inline-flex text-[12.5px] font-bold text-brand"
            >
              Abrir instruções de pagamento
            </a>
          </Card>

          <Card v-if="openCharges.length === 0" class="p-4 lg:p-6">
            <EmptyState>Nenhuma cobrança em aberto. Tudo em dia! 🎉</EmptyState>
          </Card>
        </div>

        <!-- Histórico -->
        <Card class="min-w-0 p-4 lg:p-6">
          <SectionLabel>Histórico de pagamentos</SectionLabel>
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
                <div class="text-sm font-semibold">{{ chargeTitle(charge) }}</div>
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
    </div>
  </MobileShell>
</template>
