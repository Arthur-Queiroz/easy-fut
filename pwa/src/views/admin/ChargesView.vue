<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Charge, ChargeBatch, User } from '../../api/types'
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

const batches = ref<ChargeBatch[]>([])
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

// Formulário de cobrança avulsa (pelada/partida extra).
const matchModalOpen = ref(false)
const matchTitle = ref('')
const matchTotalInput = ref('')
const matchUsers = ref<User[]>([])
const matchSearch = ref('')
const matchSelected = ref<string[]>([])
const matchError = ref('')
const matchSaving = ref(false)

// Exclusão de lote.
const batchToDelete = ref<ChargeBatch | null>(null)
const deletingBatch = ref(false)

onMounted(async () => {
  await Promise.all([load(), loadDefaults()])
})

watch(month, load)

async function load() {
  const { data } = await api.get<{ batches: ChargeBatch[]; charges: Charge[] }>('/admin/charges', {
    params: month.value ? { month: month.value } : {},
  })
  batches.value = data.batches ?? []
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

function chargesOf(batchId: string) {
  return charges.value.filter((c) => c.batch_id === batchId)
}

const monthlyBatch = computed(() => batches.value.find((b) => b.kind === 'monthly') ?? null)
// Mensalidade primeiro, depois os lotes avulsos.
const batchSections = computed(() =>
  [...batches.value]
    .sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'monthly' ? -1 : 1))
    .map((batch) => ({ batch, charges: chargesOf(batch.id) })),
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
    await api.post<{ message: string; provider_message_id: string }>(
      `/admin/charges/${charge.id}/whatsapp-send`,
    )
    actionSuccess.value = `Lembrete enviado com sucesso para ${charge.user_name.split(' ')[0]}.`
    menuFor.value = ''
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

// Cobrança avulsa.
const matchTotalCents = computed(() => {
  const parsed = Number(matchTotalInput.value.replace(/\./g, '').replace(',', '.'))
  return Number.isFinite(parsed) ? Math.round(parsed * 100) : 0
})
const filteredMatchUsers = computed(() => {
  const term = matchSearch.value.trim().toLowerCase()
  if (!term) return matchUsers.value
  return matchUsers.value.filter((u) => u.name.toLowerCase().includes(term))
})
const matchIndividualCents = computed(() =>
  matchSelected.value.length > 0 ? Math.floor(matchTotalCents.value / matchSelected.value.length) : 0,
)

async function openMatchModal() {
  matchModalOpen.value = true
  matchError.value = ''
  if (matchUsers.value.length > 0) return
  try {
    // A listagem é paginada de 20 em 20; percorre as páginas até trazer todos os ativos.
    const all: User[] = []
    for (let page = 1; ; page++) {
      const { data } = await api.get<{ users: User[]; total: number }>('/admin/users', {
        params: { status: 'active', page },
      })
      all.push(...data.users)
      if (all.length >= data.total || data.users.length === 0) break
    }
    matchUsers.value = all
  } catch (e) {
    matchError.value = errorMessage(e)
  }
}

function toggleMatchUser(id: string) {
  matchSelected.value = matchSelected.value.includes(id)
    ? matchSelected.value.filter((v) => v !== id)
    : [...matchSelected.value, id]
}

function selectAllMatchUsers() {
  matchSelected.value = filteredMatchUsers.value.map((u) => u.id)
}

async function generateMatchCharge() {
  matchError.value = ''
  if (!matchTitle.value.trim()) {
    matchError.value = 'Informe o título da cobrança.'
    return
  }
  if (matchTotalCents.value <= 0) {
    matchError.value = 'Informe o valor total.'
    return
  }
  if (matchSelected.value.length === 0) {
    matchError.value = 'Selecione ao menos um participante.'
    return
  }
  matchSaving.value = true
  try {
    // Sem "month": o backend usa o mês atual.
    await api.post('/admin/charges/generate-match', {
      title: matchTitle.value.trim(),
      total_amount_cents: matchTotalCents.value,
      user_ids: matchSelected.value,
    })
    matchModalOpen.value = false
    matchTitle.value = ''
    matchTotalInput.value = ''
    matchSearch.value = ''
    matchSelected.value = []
    await load()
  } catch (e) {
    matchError.value = errorMessage(e)
  } finally {
    matchSaving.value = false
  }
}

// Exclusão de lote.
function batchLabel(batch: ChargeBatch) {
  return batch.kind === 'monthly' ? `a mensalidade de ${formatMonth(batch.reference_month)}` : `“${batch.title}”`
}

async function confirmDeleteBatch() {
  const target = batchToDelete.value
  if (!target) return
  actionError.value = ''
  actionSuccess.value = ''
  deletingBatch.value = true
  try {
    await api.delete(`/admin/charges/batches/${target.id}`)
    batchToDelete.value = null
    await load()
  } catch (e) {
    // 409: há pagamentos registrados no lote — a mensagem vem do backend.
    actionError.value = errorMessage(e)
    batchToDelete.value = null
  } finally {
    deletingBatch.value = false
  }
}

/** Exporta as cobranças do mês como CSV, sem depender do backend. */
function exportCSV() {
  const rows = [
    ['Jogador', 'E-mail interno', 'Mês', 'Lote', 'Valor', 'Status', 'Pago em', 'Método'],
    ...charges.value.map((c) => [
      c.user_name,
      c.user_id,
      c.reference_month,
      c.batch_kind === 'match' ? c.batch_title : 'Mensalidade',
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
  link.download = `mensalidades-${monthlyBatch.value?.reference_month ?? 'atual'}.csv`
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
    <template #subtitle>Rateio do aluguel da quadra e de partidas avulsas</template>
    <template #actions>
      <BaseButton variant="outline" size="sm" class="bg-surface" :disabled="charges.length === 0" @click="exportCSV">
        <NavIcon name="download" :size="15" :stroke-width="1.9" />
        Exportar relatório
      </BaseButton>
    </template>

    <div class="grid gap-4 xl:grid-cols-[360px_1fr]">
      <div class="flex flex-col gap-3.5">
        <!-- Fotografia do rateio gerado -->
        <div v-if="monthlyBatch" class="rounded-2xl p-5 text-white" style="background-image: linear-gradient(150deg, #0c100f, #13251f)">
          <div class="text-[11px] font-bold tracking-[.1em] text-white/55">
            {{ formatMonth(monthlyBatch.reference_month).toUpperCase() }} · GERADA
          </div>
          <div class="mt-2.5 flex items-baseline gap-2.5">
            <span class="font-condensed text-[40px] font-bold leading-none text-lime">
              {{ formatCents(monthlyBatch.individual_amount_cents) }}
            </span>
            <span class="text-[13px] font-medium text-white/60">por jogador</span>
          </div>
          <div class="mt-4 flex flex-col gap-2 text-[13px] text-white/75">
            <div class="flex justify-between">
              <span>Valor total da quadra</span><strong class="text-white">{{ formatCents(monthlyBatch.total_amount_cents) }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Usuários no rateio</span><strong class="text-white">{{ monthlyBatch.user_count }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Gerada em</span>
              <strong class="text-white">{{ formatTimestamp(monthlyBatch.created_at) }} · por {{ monthlyBatch.generated_by_name.split(' ')[0] }}</strong>
            </div>
            <div class="flex justify-between">
              <span>Vencimento (5º dia útil)</span><strong class="text-white">{{ formatDMY(monthlyBatch.due_date) }}</strong>
            </div>
          </div>
          <p class="mt-3.5 rounded-[10px] bg-white/10 px-3 py-2.5 text-[11.5px] leading-[1.45] text-white/65">
            Fotografia do rateio registrada na geração — mudanças na quantidade de usuários não alteram cobranças já geradas.
          </p>
          <button
            type="button"
            class="mt-3 text-[12px] font-semibold text-white/60 underline underline-offset-2 hover:text-white"
            @click="batchToDelete = monthlyBatch"
          >
            Excluir lote
          </button>
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

        <!-- Cobrança avulsa -->
        <Card class="px-5 py-[18px]">
          <SectionLabel>Cobrança avulsa</SectionLabel>
          <p class="mt-2.5 text-[12.5px] leading-relaxed text-ink2">
            Rateie o valor de uma pelada ou partida extra entre os participantes selecionados.
          </p>
          <BaseButton class="mt-3.5 w-full" variant="outline-brand" @click="openMatchModal">
            Nova cobrança avulsa
          </BaseButton>
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
            <tbody v-for="section in batchSections" :key="section.batch.id">
              <tr v-if="section.batch.kind === 'match'" class="border-t border-border bg-surface2">
                <td colspan="5" class="px-5 py-2.5">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-[13px] font-bold">{{ section.batch.title }}</span>
                    <Badge tone="info">Avulsa</Badge>
                    <span class="text-[12px] text-ink2">
                      {{ formatCents(section.batch.individual_amount_cents) }} por jogador ·
                      {{ section.batch.user_count }} participantes
                    </span>
                    <button
                      type="button"
                      class="ml-auto text-[12px] font-semibold text-danger"
                      @click="batchToDelete = section.batch"
                    >
                      Excluir lote
                    </button>
                  </div>
                </td>
              </tr>
              <tr
                v-for="charge in section.charges"
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
            </tbody>
            <tbody v-if="charges.length === 0">
              <tr>
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
          O WhatsApp abre em uma nova aba com a mensagem pronta — confirme o envio por lá.
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

    <!-- Nova cobrança avulsa -->
    <Modal :open="matchModalOpen" title="Nova cobrança avulsa" @close="matchModalOpen = false">
      <label class="flex flex-col gap-1.5">
        <span class="text-[12.5px] font-semibold text-ink2">Título</span>
        <input v-model="matchTitle" class="field" placeholder="Pelada de quarta 17/09" />
      </label>
      <label class="mt-3 flex flex-col gap-1.5">
        <span class="text-[12.5px] font-semibold text-ink2">Valor total</span>
        <div class="flex h-11 items-center gap-1.5 rounded-[11px] border border-border bg-bg px-3.5 focus-within:border-brand">
          <span class="text-[15px] font-semibold text-ink2">R$</span>
          <input
            v-model="matchTotalInput"
            inputmode="decimal"
            placeholder="200,00"
            class="w-full bg-transparent text-[15px] font-semibold text-ink outline-none"
          />
        </div>
      </label>
      <div class="mt-3 flex flex-col gap-1.5">
        <span class="text-[12.5px] font-semibold text-ink2">Participantes</span>
        <div class="flex h-10 items-center gap-2.5 rounded-[11px] border border-border bg-surface px-3.5">
          <NavIcon name="search" :size="15" :stroke-width="1.9" class="text-ink3" />
          <input
            v-model="matchSearch"
            type="search"
            placeholder="Buscar por nome…"
            class="w-full bg-transparent text-[13.5px] text-ink outline-none placeholder:text-ink3"
          />
        </div>
        <div class="mt-1.5 flex items-center justify-between">
          <span class="text-[12px] text-ink3">{{ matchSelected.length }} selecionados</span>
          <button type="button" class="text-[12.5px] font-semibold text-brand" @click="selectAllMatchUsers">
            Selecionar todos
          </button>
        </div>
        <div class="max-h-56 overflow-y-auto rounded-xl border border-border">
          <label
            v-for="user in filteredMatchUsers"
            :key="user.id"
            class="flex cursor-pointer items-center gap-2.5 border-b border-border px-3 py-2 last:border-b-0 hover:bg-surface2"
          >
            <input
              type="checkbox"
              :checked="matchSelected.includes(user.id)"
              class="h-4 w-4 shrink-0 accent-brand"
              @change="toggleMatchUser(user.id)"
            />
            <Avatar :name="user.name" :color="user.avatar_color" size="xs" />
            <span class="text-[13px] font-semibold">{{ user.name }}</span>
          </label>
          <p v-if="filteredMatchUsers.length === 0" class="px-3 py-4 text-center text-[13px] text-ink3">
            Nenhum usuário ativo encontrado.
          </p>
        </div>
      </div>
      <div class="mt-3 flex justify-between text-[13px] text-ink2">
        <span>{{ matchSelected.length }} participantes no rateio</span>
        <span>= <strong class="text-ink">{{ formatCents(matchIndividualCents) }}</strong> cada</span>
      </div>
      <p v-if="matchError" class="mt-3 text-[13px] font-medium text-danger">{{ matchError }}</p>
      <BaseButton class="mt-4 w-full" :loading="matchSaving" @click="generateMatchCharge">
        Gerar {{ matchSelected.length }} cobranças
      </BaseButton>
    </Modal>

    <!-- Confirmação de exclusão de lote -->
    <Modal :open="batchToDelete !== null" title="Excluir lote" @close="batchToDelete = null">
      <p v-if="batchToDelete" class="text-sm leading-relaxed text-ink2">
        Excluir {{ batchLabel(batchToDelete) }}? Todas as cobranças do lote serão removidas.
      </p>
      <div class="mt-5 flex gap-2">
        <BaseButton variant="danger" class="flex-1" :loading="deletingBatch" @click="confirmDeleteBatch">
          Excluir lote
        </BaseButton>
        <BaseButton variant="outline" class="flex-1" @click="batchToDelete = null">Cancelar</BaseButton>
      </div>
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
