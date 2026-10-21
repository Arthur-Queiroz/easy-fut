<script setup lang="ts">
// Caixa do grupo (admin): saldo de todos os lançamentos, lista do período,
// despesas lançadas à mão. Separado das mensalidades.
import { computed, onMounted, ref, watch } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { CashEntry, CashExpenseInput, CashReport } from '../../api/adminGuests'
import { currentMonth, formatCents, formatDMY, monthBounds, todayISO } from '../../lib/format'
import { CASH_KIND_INFO, cashTotals, parseReais } from '../../lib/guests'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import Modal from '../../components/ui/Modal.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

type Period = 'month' | 'last_month' | 'all' | 'custom'

const report = ref<CashReport | null>(null)
const loadError = ref('')

const period = ref<Period>('month')
const from = ref(monthBounds(currentMonth()).from)
const to = ref(monthBounds(currentMonth()).to)

const showExpense = ref(false)
const expense = ref({ description: '', amount: '', occurred_on: todayISO() })
const expenseError = ref('')
const savingExpense = ref(false)

const removing = ref<CashEntry | null>(null)
const removeError = ref('')
const removingBusy = ref(false)

onMounted(load)

function setPeriod(p: Period) {
  period.value = p
  if (p === 'month' || p === 'last_month') {
    const bounds = monthBounds(currentMonth(p === 'month' ? 0 : -1))
    from.value = bounds.from
    to.value = bounds.to
  } else if (p === 'all') {
    from.value = ''
    to.value = ''
  }
  void load()
}

watch([from, to], () => {
  if (period.value === 'custom') void load()
})

function onDateInput() {
  period.value = 'custom'
}

async function load() {
  loadError.value = ''
  try {
    const params = { from: from.value || undefined, to: to.value || undefined }
    const { data } = await api.get<CashReport>('/admin/cash', { params })
    report.value = { balance_cents: data.balance_cents, entries: data.entries ?? [] }
  } catch (e) {
    loadError.value = errorMessage(e)
  }
}

const entries = computed(() => report.value?.entries ?? [])
const totals = computed(() => cashTotals(entries.value))

const PERIODS: { value: Period; label: string }[] = [
  { value: 'month', label: 'Este mês' },
  { value: 'last_month', label: 'Mês passado' },
  { value: 'all', label: 'Tudo' },
]

function openExpense() {
  expense.value = { description: '', amount: '', occurred_on: todayISO() }
  expenseError.value = ''
  showExpense.value = true
}

async function saveExpense() {
  expenseError.value = ''
  const cents = parseReais(expense.value.amount)
  if (!expense.value.description.trim()) {
    expenseError.value = 'Informe a descrição da despesa.'
    return
  }
  if (cents === null || cents <= 0) {
    expenseError.value = 'Informe um valor em reais maior que zero, como 12,50.'
    return
  }
  if (!expense.value.occurred_on) {
    expenseError.value = 'Informe a data da despesa.'
    return
  }
  savingExpense.value = true
  try {
    const body: CashExpenseInput = {
      description: expense.value.description.trim(),
      amount_cents: cents,
      occurred_on: expense.value.occurred_on,
    }
    await api.post('/admin/cash/expenses', body)
    showExpense.value = false
    await load()
  } catch (e) {
    expenseError.value = errorMessage(e)
  } finally {
    savingExpense.value = false
  }
}

function askRemove(entry: CashEntry) {
  removeError.value = ''
  removing.value = entry
}

async function confirmRemove() {
  if (!removing.value) return
  removingBusy.value = true
  removeError.value = ''
  try {
    await api.delete(`/admin/cash/expenses/${removing.value.id}`)
    removing.value = null
    await load()
  } catch (e) {
    removeError.value = errorMessage(e)
  } finally {
    removingBusy.value = false
  }
}

function entryTitle(entry: CashEntry): string {
  return entry.description || CASH_KIND_INFO[entry.kind].label
}
</script>

<template>
  <AdminLayout>
    <template #title>Caixa</template>
    <template #subtitle>Entradas das vagas avulsas, estornos e despesas do grupo</template>
    <template #actions>
      <BaseButton size="sm" @click="openExpense">
        <NavIcon name="plus" :size="15" :stroke-width="2.2" />
        Despesa
      </BaseButton>
    </template>

    <div class="flex flex-col gap-4">
      <!-- Saldo -->
      <Card class="px-5 py-4">
        <SectionLabel>Saldo do caixa</SectionLabel>
        <div
          class="mt-1 font-condensed text-[40px] font-bold leading-none"
          :class="(report?.balance_cents ?? 0) < 0 ? 'text-danger' : 'text-ink'"
          data-test="balance"
        >
          {{ report ? formatCents(report.balance_cents) : '—' }}
        </div>
        <p class="mt-1 text-[12px] text-ink3">Soma de todos os lançamentos, sem as mensalidades.</p>
        <div class="mt-3 grid grid-cols-2 gap-2 border-t border-border pt-3 text-[12.5px]">
          <div>
            <div class="text-ink3">Entradas no período</div>
            <div class="font-semibold text-brand">+ {{ formatCents(totals.inCents) }}</div>
          </div>
          <div>
            <div class="text-ink3">Saídas no período</div>
            <div class="font-semibold text-danger">− {{ formatCents(totals.outCents) }}</div>
          </div>
        </div>
      </Card>

      <!-- Período -->
      <div class="flex flex-col gap-2.5 sm:flex-row sm:flex-wrap sm:items-end">
        <div class="grid grid-cols-3 gap-1 rounded-[11px] bg-surface2 p-1 sm:w-auto">
          <button
            v-for="p in PERIODS"
            :key="p.value"
            type="button"
            class="h-9 rounded-[9px] px-3 text-[13px] font-semibold"
            :class="period === p.value ? 'bg-surface text-ink shadow-sm' : 'text-ink3'"
            :aria-pressed="period === p.value"
            @click="setPeriod(p.value)"
          >
            {{ p.label }}
          </button>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <label class="flex flex-col gap-1">
            <span class="text-[11.5px] font-semibold text-ink2">De</span>
            <input v-model="from" type="date" aria-label="Data inicial" class="field" @input="onDateInput" />
          </label>
          <label class="flex flex-col gap-1">
            <span class="text-[11.5px] font-semibold text-ink2">Até</span>
            <input v-model="to" type="date" aria-label="Data final" class="field" @input="onDateInput" />
          </label>
        </div>
      </div>

      <p v-if="loadError" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ loadError }}</p>

      <!-- Lançamentos -->
      <Card class="overflow-hidden">
        <ul v-if="entries.length" class="divide-y divide-border">
          <li v-for="entry in entries" :key="entry.id" class="flex items-center gap-3 px-4 py-3" data-test="cash-entry">
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full font-condensed text-[18px] font-bold"
              :class="CASH_KIND_INFO[entry.kind].sign > 0 ? 'bg-brandSoft text-brand' : 'bg-dangerBg text-danger'"
              aria-hidden="true"
            >
              {{ CASH_KIND_INFO[entry.kind].sign > 0 ? '+' : '−' }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-[13.5px] font-semibold">{{ entryTitle(entry) }}</div>
              <div class="text-[12px] text-ink3">
                {{ CASH_KIND_INFO[entry.kind].label }} · {{ formatDMY(entry.occurred_on) }}
                <template v-if="entry.kind === 'guest_payment' && entry.fee_cents">
                  · taxa estimada {{ formatCents(entry.fee_cents) }}
                </template>
              </div>
            </div>
            <div
              class="shrink-0 text-right text-[14px] font-bold"
              :class="CASH_KIND_INFO[entry.kind].sign > 0 ? 'text-brand' : 'text-danger'"
            >
              {{ CASH_KIND_INFO[entry.kind].sign > 0 ? '+' : '−' }} {{ formatCents(entry.amount_cents) }}
            </div>
            <button
              v-if="entry.kind === 'expense'"
              type="button"
              class="shrink-0 rounded-md p-1.5 text-ink3 hover:bg-surface2 hover:text-danger"
              :aria-label="`Remover despesa ${entryTitle(entry)}`"
              @click="askRemove(entry)"
            >
              <NavIcon name="x" :size="14" :stroke-width="2.2" />
            </button>
          </li>
        </ul>
        <EmptyState v-else-if="report">Nenhum lançamento no período.</EmptyState>
      </Card>
    </div>

    <!-- Nova despesa -->
    <Modal :open="showExpense" title="Lançar despesa" @close="showExpense = false">
      <form class="flex flex-col gap-3" @submit.prevent="saveExpense">
        <label class="flex flex-col gap-1.5">
          <span class="text-[12.5px] font-semibold text-ink2">Descrição</span>
          <input v-model="expense.description" required maxlength="120" placeholder="Ex.: bola nova" class="field" />
        </label>
        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Valor (R$)</span>
            <input v-model="expense.amount" inputmode="decimal" placeholder="12,50" aria-label="Valor da despesa" class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Data</span>
            <input v-model="expense.occurred_on" type="date" required aria-label="Data da despesa" class="field" />
          </label>
        </div>
        <p v-if="expenseError" class="text-[13px] font-medium text-danger">{{ expenseError }}</p>
        <BaseButton class="mt-1" :loading="savingExpense" @click="saveExpense">Lançar despesa</BaseButton>
      </form>
    </Modal>

    <!-- Remoção de despesa -->
    <Modal :open="removing !== null" title="Remover despesa" @close="removing = null">
      <div v-if="removing" class="flex flex-col gap-4">
        <p class="text-sm leading-relaxed text-ink2">
          Remover <strong class="text-ink">{{ entryTitle(removing) }}</strong> ({{ formatCents(removing.amount_cents) }},
          {{ formatDMY(removing.occurred_on) }})? Use só para despesa lançada por engano.
        </p>
        <p v-if="removeError" class="rounded-lg bg-dangerBg px-3 py-2 text-[12.5px] font-medium text-danger">{{ removeError }}</p>
        <BaseButton variant="danger" :loading="removingBusy" @click="confirmRemove">Remover despesa</BaseButton>
      </div>
    </Modal>
  </AdminLayout>
</template>

<style scoped>
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
</style>
