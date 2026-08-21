<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Charge, ChargeBatch } from '../../api/types'
import { currentMonth, formatCents, formatDMY, formatMonth, formatTimestamp } from '../../lib/format'
import { sanitizeCSVCell } from '../../lib/security'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import Modal from '../../components/ui/Modal.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const batch = ref<ChargeBatch | null>(null)
const charges = ref<Charge[]>([])
const month = ref('')
const activeUsers = ref(0)
const menuFor = ref('') // id da cobrança com o menu "⋯" aberto

// Formulário de geração do próximo mês.
const nextMonth = ref(currentMonth(1))
const totalInput = ref('')
const error = ref('')
const generating = ref(false)
const actionError = ref('')
const actionSuccess = ref('')
const remindingChargeID = ref('')
const reminderQueue = ref<Charge[]>([])
const reminderIndex = ref(0)

onMounted(async () => {
  await Promise.all([load(), loadDefaults()])
})

watch(month, load)

async function load() {
  const { data } = await api.get<{ batch: ChargeBatch | null; charges: Charge[] }>('/admin/charges', {
    params: month.value ? { month: month.value } : {},
  })
  batch.value = data.batch
  charges.value = data.charges ?? []
}

async function loadDefaults() {
  const [dash, settings] = await Promise.all([
    api.get<{ stats: { active_users: number } }>('/admin/dashboard'),
    api.get<Record<string, string>>('/admin/settings'),
  ])
  activeUsers.value = dash.data.stats.active_users
  const saved = Number(settings.data.monthly_total_cents ?? 0)
  if (saved > 0) totalInput.value = (saved / 100).toFixed(2).replace('.', ',')
}

const totalCents = computed(() => {
  const parsed = Number(totalInput.value.replace(/\./g, '').replace(',', '.'))
  return Number.isFinite(parsed) ? Math.round(parsed * 100) : 0
})
const individualCents = computed(() =>
  activeUsers.value > 0 ? Math.floor(totalCents.value / activeUsers.value) : 0,
)

const paidCount = computed(() => charges.value.filter((c) => c.status === 'paid' || c.status === 'manual_paid').length)
const pendingCount = computed(() => charges.value.filter((c) => c.status === 'pending' || c.status === 'overdue').length)
const chargesAwaitingPayment = computed(() =>
  charges.value.filter((charge) => charge.status === 'pending' || charge.status === 'overdue'),
)
const currentReminder = computed(() => reminderQueue.value[reminderIndex.value] ?? null)
const collected = computed(() =>
  charges.value
    .filter((c) => c.status === 'paid' || c.status === 'manual_paid')
    .reduce((sum, c) => sum + c.amount_cents, 0),
)
const expected = computed(() =>
  charges.value.filter((c) => c.status !== 'cancelled' && c.status !== 'exempt').reduce((sum, c) => sum + c.amount_cents, 0),
)

async function generate() {
  error.value = ''
  if (totalCents.value <= 0) {
    error.value = 'Informe o valor total do mês.'
    return
  }
  generating.value = true
  try {
    await api.post('/admin/charges/generate', {
      month: nextMonth.value,
      total_amount_cents: totalCents.value,
    })
    month.value = nextMonth.value
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    generating.value = false
  }
}

async function markPaid(charge: Charge) {
  await api.post(`/admin/charges/${charge.id}/mark-paid`, { method: 'manual' })
  menuFor.value = ''
  await load()
}

async function sendWhatsAppReminder(charge: Charge) {
  actionError.value = ''
  actionSuccess.value = ''
  remindingChargeID.value = charge.id
  try {
    const { data } = await api.post<{ message: string; provider_message_id: string }>(
      `/admin/charges/${charge.id}/whatsapp-send`,
    )
    actionSuccess.value = `Lembrete enviado com sucesso para ${charge.user_name.split(' ')[0]}.`
    menuFor.value = ''
    // eslint-disable-next-line no-console
    console.log('WhatsApp message sent:', data.message, data.provider_message_id)
  } catch (e) {
    actionError.value = errorMessage(e)
  } finally {
    remindingChargeID.value = ''
  }
}

function startWhatsAppReminders() {
  actionError.value = ''
  actionSuccess.value = ''
  reminderQueue.value = chargesAwaitingPayment.value
  reminderIndex.value = 0
}

function closeWhatsAppReminders() {
  reminderQueue.value = []
  reminderIndex.value = 0
}

async function openNextWhatsAppReminder() {
  const charge = currentReminder.value
  if (!charge) return

  await sendWhatsAppReminder(charge)
  if (!actionError.value) reminderIndex.value += 1
}

async function exempt(charge: Charge) {
  await api.post(`/admin/charges/${charge.id}/exempt`)
  menuFor.value = ''
  await load()
}

async function cancel(charge: Charge) {
  await api.post(`/admin/charges/${charge.id}/cancel`)
  menuFor.value = ''
  await load()
}

/** Exporta as cobranças do mês como CSV, sem depender do backend. */
function exportCSV() {
  const rows = [
    ['Jogador', 'E-mail interno', 'Mês', 'Valor', 'Status', 'Pago em', 'Método'],
    ...charges.value.map((c) => [
      c.user_name,
      c.user_id,
      c.reference_month,
      (c.amount_cents / 100).toFixed(2).replace('.', ','),
      statusInfo(c).label,
      c.paid_at ? formatDMY(c.paid_at.slice(0, 10)) : '',
      c.paid_method,
    ]),
  ]
  const csv = rows
    .map((row) => row.map((cell) => `"${sanitizeCSVCell(cell).replace(/"/g, '""')}"`).join(';'))
    .join('\n')
  const url = URL.createObjectURL(new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `mensalidades-${batch.value?.reference_month ?? 'atual'}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

// O vencimento fica na coluna de data, não no badge: assim a coluna de status
// não infla e a ação principal cabe na tela sem rolagem horizontal.
function statusInfo(charge: Charge): { tone: 'success' | 'warn' | 'danger' | 'neutral'; label: string; solid?: boolean } {
  switch (charge.status) {
    case 'paid':
      return { tone: 'success', label: 'Pago · PIX' }
    case 'manual_paid':
      return { tone: 'success', label: 'Pago · manual' }
    case 'overdue':
      return { tone: 'danger', label: 'Vencida', solid: true }
    case 'exempt':
      return { tone: 'neutral', label: 'Isento' }
    case 'cancelled':
      return { tone: 'neutral', label: 'Cancelada' }
    default:
      return { tone: 'warn', label: 'Aguardando', solid: true }
  }
}

// Últimos 6 meses como opções do seletor.
const monthOptions = computed(() => {
  const options: { value: string; label: string }[] = []
  for (let i = 0; i < 6; i++) options.push({ value: currentMonth(-i), label: formatMonth(currentMonth(-i)) })
  return options
})
</script>

<template>
  <AdminLayout>
    <template #title>Mensalidades</template>
    <template #subtitle>Rateio do aluguel da quadra entre os usuários ativos</template>
    <template #actions>
      <BaseButton variant="outline" size="sm" class="bg-surface" :disabled="charges.length === 0" @click="exportCSV">
        <NavIcon name="download" :size="15" :stroke-width="1.9" />
        Exportar relatório
      </BaseButton>
    </template>

    <div class="grid gap-4 xl:grid-cols-[360px_1fr]">
      <div class="flex flex-col gap-3.5">
        <!-- Fotografia do rateio gerado -->
        <div v-if="batch" class="rounded-2xl p-5 text-white" style="background-image: linear-gradient(150deg, #0c100f, #13251f)">
          <div class="text-[11px] font-bold tracking-[.1em] text-white/55">
            {{ formatMonth(batch.reference_month).toUpperCase() }} · GERADA
          </div>
          <div class="mt-2.5 flex items-baseline gap-2.5">
            <span class="font-condensed text-[40px] font-bold leading-none text-lime">
              {{ formatCents(batch.individual_amount_cents) }}
            </span>
            <span class="text-[13px] font-medium text-white/60">por jogador</span>
          </div>
          <div class="mt-4 flex flex-col gap-2 text-[13px] text-white/75">
            <div class="flex justify-between">
              <span>Valor total da quadra</span><strong class="text-white">{{ formatCents(batch.total_amount_cents) }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Usuários no rateio</span><strong class="text-white">{{ batch.user_count }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Gerada em</span>
              <strong class="text-white">{{ formatTimestamp(batch.created_at) }} · por {{ batch.generated_by_name.split(' ')[0] }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Vencimento (5º dia útil)</span><strong class="text-white">{{ formatDMY(batch.due_date) }}</strong>
            </div>
          </div>
          <p class="mt-3.5 rounded-[10px] bg-white/10 px-3 py-2.5 text-[11.5px] leading-[1.45] text-white/65">
            Fotografia do rateio registrada na geração — mudanças na quantidade de usuários não alteram cobranças já geradas.
          </p>
        </div>

        <!-- Gerar próximo mês -->
        <Card class="px-5 py-[18px]">
          <SectionLabel>Gerar {{ formatMonth(nextMonth) }}</SectionLabel>
          <label class="mt-3 flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Mês de referência</span>
            <input v-model="nextMonth" type="month" class="field" />
          </label>
          <label class="mt-3 flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Valor total do mês</span>
            <div class="flex h-11 items-center gap-1.5 rounded-[11px] border border-border bg-bg px-3.5 focus-within:border-brand">
              <span class="text-[15px] font-semibold text-ink2">R$</span>
              <input
                v-model="totalInput"
                inputmode="decimal"
                placeholder="1.440,00"
                class="w-full bg-transparent text-[15px] font-semibold text-ink outline-none"
              />
            </div>
          </label>
          <div class="mt-3 flex justify-between text-[13px] text-ink2">
            <span>{{ activeUsers }} ativos no rateio</span>
            <span>= <strong class="text-ink">{{ formatCents(individualCents) }}</strong> cada</span>
          </div>
          <p v-if="error" class="mt-3 text-[13px] font-medium text-danger">{{ error }}</p>
          <BaseButton class="mt-3.5 w-full" :loading="generating" @click="generate">
            Gerar {{ activeUsers }} cobranças
          </BaseButton>
          <p class="mt-2.5 text-[11.5px] leading-relaxed text-ink3">
            O vencimento é calculado automaticamente no 5º dia útil. Os lembretes são enviados pelo WhatsApp automaticamente pelo sistema.
          </p>
        </Card>
      </div>

      <!-- Cobranças do mês -->
      <Card class="flex flex-col overflow-hidden">
        <div class="flex flex-wrap items-center gap-2.5 border-b border-border px-5 py-3.5">
          <span class="text-sm font-bold">Cobranças de</span>
          <select v-model="month" class="filter">
            <option value="">Mês mais recente</option>
            <option v-for="opt in monthOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
          <Badge tone="success">{{ paidCount }} pagas</Badge>
          <Badge tone="warn">{{ pendingCount }} aguardando</Badge>
          <BaseButton
            v-if="chargesAwaitingPayment.length"
            variant="outline"
            size="sm"
            class="ml-auto"
            @click="startWhatsAppReminders"
          >
            Enviar {{ chargesAwaitingPayment.length }} lembretes
          </BaseButton>
        </div>
        <p v-if="actionError" class="border-b border-danger/20 bg-dangerBg px-5 py-3 text-[13px] font-medium text-danger">
          {{ actionError }}
        </p>
        <p v-if="actionSuccess" class="border-b border-brand/20 bg-brandSoft px-5 py-3 text-[13px] font-medium text-brandInk">
          {{ actionSuccess }}
        </p>

        <div class="overflow-x-auto">
          <table class="w-full min-w-[620px] text-left">
            <thead>
              <tr class="bg-surface2 text-[10.5px] font-bold uppercase tracking-[.08em] text-ink3">
                <th class="whitespace-nowrap px-5 py-2.5 font-bold">Jogador</th>
                <th class="whitespace-nowrap px-3 py-2.5 font-bold">Valor</th>
                <th class="whitespace-nowrap px-3 py-2.5 font-bold">Status</th>
                <th class="whitespace-nowrap px-3 py-2.5 font-bold">Pagamento</th>
                <th class="whitespace-nowrap px-5 py-2.5 text-right font-bold">Ações</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="charge in charges"
                :key="charge.id"
                class="border-t border-border"
                :class="charge.status === 'overdue' ? 'bg-dangerBg' : charge.status === 'pending' ? 'bg-warnBg' : ''"
              >
                <td class="px-5 py-2.5">
                  <div class="flex items-center gap-2.5">
                    <Avatar :name="charge.user_name" :color="charge.avatar_color" size="xs" />
                    <span class="text-[13px] font-semibold">
                      {{ charge.user_name }}
                      <span
                        v-if="charge.user_role === 'admin'"
                        class="ml-1 rounded-md bg-infoBg px-1.5 py-px text-[10px] font-semibold text-info"
                      >ADMIN</span>
                    </span>
                  </div>
                </td>
                <td class="px-3 py-2.5 text-[13px] font-medium">{{ formatCents(charge.amount_cents) }}</td>
                <td class="px-3 py-2.5">
                  <Badge :tone="statusInfo(charge).tone" :solid="statusInfo(charge).solid" dot>
                    {{ statusInfo(charge).label }}
                  </Badge>
                </td>
                <td class="whitespace-nowrap px-3 py-2.5 text-[12.5px] text-ink2">
                  <template v-if="charge.paid_at">
                    {{ formatTimestamp(charge.paid_at) }}
                    <span v-if="charge.registered_by_name" class="text-ink3">· por {{ charge.registered_by_name.split(' ')[0] }}</span>
                  </template>
                  <span v-else-if="charge.status === 'pending' || charge.status === 'overdue'" class="text-ink3">
                    vence {{ formatDMY(charge.due_date).slice(0, 5) }}
                  </span>
                  <span v-else class="text-ink3">—</span>
                </td>
                <td class="whitespace-nowrap px-5 py-2.5 text-right">
                  <div v-if="charge.status === 'pending' || charge.status === 'overdue'" class="flex items-center justify-end gap-2">
                    <button
                      type="button"
                      class="text-[12.5px] font-semibold text-info disabled:opacity-50"
                      :disabled="remindingChargeID === charge.id"
                      @click="sendWhatsAppReminder(charge)"
                    >
                      {{ remindingChargeID === charge.id ? 'Enviando...' : 'Enviar lembrete' }}
                    </button>
                    <button type="button" class="text-[12.5px] font-semibold text-brand" @click="markPaid(charge)">
                      Registrar pagamento
                    </button>
                    <!-- Ações secundárias ficam no menu, como no padrão ⋯ da tabela de usuários. -->
                    <div class="relative">
                      <button
                        type="button"
                        class="px-1.5 text-base font-bold leading-none tracking-[2px] text-ink3 hover:text-ink"
                        :aria-label="`Mais ações para ${charge.user_name}`"
                        :aria-expanded="menuFor === charge.id"
                        @click="menuFor = menuFor === charge.id ? '' : charge.id"
                      >
                        ⋯
                      </button>
                      <div
                        v-if="menuFor === charge.id"
                        class="absolute right-0 top-7 z-10 flex w-36 flex-col overflow-hidden rounded-xl border border-border bg-surface py-1 text-left shadow-card"
                      >
                        <button type="button" class="px-3 py-2 text-left text-[12.5px] font-medium text-ink2 hover:bg-surface2" @click="exempt(charge)">
                          Marcar isento
                        </button>
                        <button type="button" class="px-3 py-2 text-left text-[12.5px] font-medium text-danger hover:bg-surface2" @click="cancel(charge)">
                          Cancelar cobrança
                        </button>
                      </div>
                    </div>
                  </div>
                  <span v-else class="text-[12.5px] text-ink3">—</span>
                </td>
              </tr>
              <tr v-if="charges.length === 0">
                <td colspan="5" class="px-5 py-10 text-center text-sm text-ink3">
                  Nenhuma cobrança gerada para este mês.
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="mt-auto flex items-center justify-between border-t border-border px-5 py-3">
          <span class="text-[12.5px] text-ink3">{{ charges.length }} cobranças</span>
          <span class="text-[13px] font-semibold text-ink">
            Total: <span class="text-brand">{{ formatCents(collected) }}</span> / {{ formatCents(expected) }}
          </span>
        </div>
      </Card>
    </div>

    <Modal
      :open="reminderQueue.length > 0"
      title="Lembretes de pagamento"
      @close="closeWhatsAppReminders"
    >
      <template v-if="currentReminder">
        <p class="text-sm leading-relaxed text-ink2">
          {{ reminderIndex + 1 }} de {{ reminderQueue.length }}: enviar lembrete para
          <strong class="text-ink">{{ currentReminder.user_name }}</strong>.
        </p>
        <p class="mt-2 text-xs leading-relaxed text-ink3">
          A mensagem é disparada automaticamente pelo sistema via WhatsApp.
        </p>
        <p v-if="actionError" class="mt-3 rounded-xl bg-dangerBg px-3 py-2.5 text-[13px] font-medium text-danger">
          {{ actionError }}
        </p>
        <p v-if="actionSuccess" class="mt-3 rounded-xl bg-brandSoft px-3 py-2.5 text-[13px] font-medium text-brandInk">
          {{ actionSuccess }}
        </p>
        <BaseButton
          class="mt-5 w-full"
          :loading="remindingChargeID === currentReminder.id"
          @click="openNextWhatsAppReminder"
        >
          Enviar lembrete de {{ currentReminder.user_name.split(' ')[0] }}
        </BaseButton>
      </template>
      <template v-else>
        <p class="text-sm leading-relaxed text-ink2">
          Todos os {{ reminderQueue.length }} lembretes foram enviados.
        </p>
        <BaseButton class="mt-5 w-full" @click="closeWhatsAppReminders">Concluir</BaseButton>
      </template>
    </Modal>
  </AdminLayout>
</template>

<style scoped>
.filter {
  @apply h-8 rounded-lg border border-border bg-surface px-2.5 text-[12.5px] font-medium text-ink2 outline-none;
}
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-[15px] font-semibold text-ink outline-none focus:border-brand;
}
</style>
