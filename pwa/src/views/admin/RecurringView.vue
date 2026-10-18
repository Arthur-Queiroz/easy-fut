<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

interface RecurringMatch {
  id: string
  weekday: number // 0 = domingo
  start_time: string
  end_time: string
  venue: string
  address: string
  same_day_deadline_time?: string
  created_by_name?: string
  active: boolean
}

const WEEKDAYS = ['Domingo', 'Segunda', 'Terça', 'Quarta', 'Quinta', 'Sexta', 'Sábado']

const recurring = ref<RecurringMatch[]>([])
const error = ref('')
const saving = ref(false)
const togglingId = ref('')

// form de criação
const weekday = ref(0)
const startTime = ref('11:00')
const endTime = ref('12:30')
const venue = ref('')
const address = ref('')
const deadline = ref('10:30')

onMounted(load)

async function load() {
  const { data } = await api.get<{ recurring: RecurringMatch[] }>('/admin/recurring')
  recurring.value = data.recurring ?? []
}

async function create() {
  if (!venue.value.trim()) {
    error.value = 'Informe o local.'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await api.post('/admin/recurring', {
      weekday: weekday.value,
      start_time: startTime.value,
      end_time: endTime.value,
      venue: venue.value.trim(),
      address: address.value.trim(),
      same_day_deadline_time: deadline.value.trim(),
    })
    venue.value = ''
    address.value = ''
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function toggle(rec: RecurringMatch) {
  togglingId.value = rec.id
  error.value = ''
  try {
    await api.patch(`/admin/recurring/${rec.id}/activate`, { active: !rec.active })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    togglingId.value = ''
  }
}

async function remove(rec: RecurringMatch) {
  if (!confirm(`Excluir a recorrência de ${WEEKDAYS[rec.weekday]} ${rec.start_time}? As partidas já criadas continuam.`)) return
  error.value = ''
  try {
    await api.delete(`/admin/recurring/${rec.id}`)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <AdminLayout>
    <div class="flex flex-col gap-3 lg:gap-4">
      <header class="flex items-center justify-between">
        <h1 class="text-[19px] font-bold lg:text-[22px]">Recorrências</h1>
        <span class="text-[12px] text-ink3">Partidas criadas automaticamente pelo cron</span>
      </header>

      <Card class="p-4">
        <SectionLabel>Nova recorrência</SectionLabel>
        <div class="mt-2 grid gap-2 lg:grid-cols-6">
          <select v-model="weekday" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm">
            <option v-for="(d, i) in WEEKDAYS" :key="i" :value="i">{{ d }}</option>
          </select>
          <input v-model="startTime" placeholder="11:00" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input v-model="endTime" placeholder="12:30" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input v-model="venue" placeholder="Local (Arena Futiba 1)" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm lg:col-span-2" />
          <input v-model="deadline" placeholder="Deadline 10:30" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm" />
          <input v-model="address" placeholder="Endereço completo" class="rounded-xl border border-border bg-surface px-3 py-2 text-sm lg:col-span-4" />
          <BaseButton :disabled="saving" class="lg:col-span-2" @click="create">
            {{ saving ? 'Salvando…' : 'Criar recorrência' }}
          </BaseButton>
        </div>
        <p v-if="error" class="mt-2 text-[12.5px] font-semibold text-danger">{{ error }}</p>
      </Card>

      <Card v-for="rec in recurring" :key="rec.id" class="p-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <div class="flex items-center gap-2">
              <span class="text-[15px] font-bold">{{ WEEKDAYS[rec.weekday] }} · {{ rec.start_time }}–{{ rec.end_time }}</span>
              <span
                class="rounded-md px-2 py-px text-[10.5px] font-bold"
                :class="rec.active ? 'bg-brandSoft text-brand' : 'bg-dangerBg text-danger'"
              >{{ rec.active ? 'ATIVA' : 'PAUSADA' }}</span>
            </div>
            <p class="mt-[3px] text-[12.5px] text-ink3">
              {{ rec.venue }}<template v-if="rec.address"> · {{ rec.address }}</template>
              <template v-if="rec.same_day_deadline_time"> · confirmações até {{ rec.same_day_deadline_time }}</template>
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="rounded-xl border border-border px-3 py-[7px] text-[12.5px] font-semibold hover:bg-surface"
              :disabled="togglingId === rec.id"
              @click="toggle(rec)"
            >{{ rec.active ? 'Pausar' : 'Ativar' }}</button>
            <button
              type="button"
              class="rounded-xl border border-danger/40 px-3 py-[7px] text-[12.5px] font-semibold text-danger hover:bg-dangerBg"
              @click="remove(rec)"
            >Excluir</button>
          </div>
        </div>
      </Card>

      <p v-if="recurring.length === 0" class="text-sm text-ink3">
        Nenhuma recorrência. As partidas continuam sendo criadas manualmente.
      </p>
    </div>
  </AdminLayout>
</template>
