<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Match, Team } from '../../api/types'
import { dayOfMonth, formatDateShort, formatHour, formatTimestamp, monthShort } from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import Modal from '../../components/ui/Modal.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const matches = ref<Match[]>([])
const teams = ref<Record<string, Team[]>>({})
const error = ref('')
const saving = ref(false)

const showForm = ref(false)
const editingId = ref<string | null>(null)
const form = ref({
  match_date: '',
  start_time: '20:00',
  end_time: '22:00',
  venue: '',
  address: '',
  deadline_date: '',
  deadline_time: '18:00',
  notes: '',
})

const cancelling = ref<Match | null>(null)
const cancelReason = ref('')
const drawing = ref<Match | null>(null)
const teamCount = ref(2)

onMounted(load)

async function load() {
  const { data } = await api.get<Match[]>('/matches')
  matches.value = data ?? []
  for (const match of matches.value) {
    if (match.status === 'teams_drawn') {
      const res = await api.get<Team[]>(`/matches/${match.id}/teams`)
      teams.value[match.id] = res.data ?? []
    }
  }
}

const upcoming = computed(() =>
  matches.value.filter((m) => ['open', 'closed', 'teams_drawn'].includes(m.status)),
)
const past = computed(() => matches.value.filter((m) => ['voting', 'finished', 'cancelled'].includes(m.status)))

function openCreate() {
  editingId.value = null
  // Sugere a próxima quinta-feira às 20h, com prazo de confirmação no mesmo dia às 18h.
  const next = new Date()
  next.setDate(next.getDate() + ((4 - next.getDay() + 7) % 7 || 7))
  const iso = next.toISOString().slice(0, 10)
  form.value = {
    match_date: iso,
    start_time: '20:00',
    end_time: '22:00',
    venue: matches.value[0]?.venue ?? '',
    address: matches.value[0]?.address ?? '',
    deadline_date: iso,
    deadline_time: '18:00',
    notes: '',
  }
  error.value = ''
  showForm.value = true
}

function openEdit(match: Match) {
  editingId.value = match.id
  const deadline = new Date(match.confirmation_deadline)
  form.value = {
    match_date: match.match_date,
    start_time: match.start_time,
    end_time: match.end_time,
    venue: match.venue,
    address: match.address,
    deadline_date: `${deadline.getFullYear()}-${pad(deadline.getMonth() + 1)}-${pad(deadline.getDate())}`,
    deadline_time: `${pad(deadline.getHours())}:${pad(deadline.getMinutes())}`,
    notes: match.notes,
  }
  error.value = ''
  showForm.value = true
}

function pad(n: number) {
  return String(n).padStart(2, '0')
}

async function save() {
  error.value = ''
  saving.value = true
  try {
    const payload = {
      match_date: form.value.match_date,
      start_time: form.value.start_time,
      end_time: form.value.end_time,
      venue: form.value.venue,
      address: form.value.address,
      // O input datetime é local; o backend recebe o instante em ISO com fuso.
      confirmation_deadline: new Date(`${form.value.deadline_date}T${form.value.deadline_time}`).toISOString(),
      notes: form.value.notes,
    }
    if (editingId.value) {
      await api.patch(`/admin/matches/${editingId.value}`, payload)
    } else {
      await api.post('/admin/matches', payload)
    }
    showForm.value = false
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function act(match: Match, action: string, body: object = {}) {
  error.value = ''
  try {
    await api.post(`/matches/${match.id}/${action}`, body)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function confirmDraw() {
  if (!drawing.value) return
  saving.value = true
  try {
    await api.post(`/matches/${drawing.value.id}/draw-teams`, { team_count: teamCount.value })
    drawing.value = null
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function confirmCancel() {
  if (!cancelling.value) return
  saving.value = true
  try {
    await api.post(`/matches/${cancelling.value.id}/cancel`, { reason: cancelReason.value })
    cancelling.value = null
    cancelReason.value = ''
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

function statusBadge(match: Match): { tone: 'success' | 'warn' | 'danger' | 'info' | 'neutral'; label: string } {
  switch (match.status) {
    case 'open':
      return { tone: 'success', label: 'Confirmação aberta' }
    case 'closed':
      return { tone: 'warn', label: 'Confirmação encerrada' }
    case 'teams_drawn':
      return { tone: 'info', label: 'Times sorteados' }
    case 'voting':
      return { tone: 'info', label: 'Aguardando votação' }
    case 'cancelled':
      return { tone: 'danger', label: match.cancel_reason ? `Cancelada · ${match.cancel_reason}` : 'Cancelada' }
    default:
      return { tone: 'neutral', label: 'Finalizada' }
  }
}
</script>

<template>
  <AdminLayout>
    <template #title>Partidas</template>
    <template #subtitle>Criar, encerrar confirmações, sortear times e finalizar</template>
    <template #actions>
      <BaseButton size="sm" @click="openCreate">
        <NavIcon name="plus" :size="15" :stroke-width="2.2" />
        Nova partida
      </BaseButton>
    </template>

    <p v-if="error" class="mb-3 rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ error }}</p>

    <div class="flex flex-col gap-4">
      <!-- Em andamento -->
      <div>
        <SectionLabel>Em andamento</SectionLabel>
        <div class="mt-2.5 flex flex-col gap-3">
          <Card v-for="match in upcoming" :key="match.id" class="px-5 py-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="w-[60px] shrink-0 rounded-xl border border-border bg-surface2 py-2 text-center">
                <div class="font-condensed text-[26px] font-bold leading-none">{{ dayOfMonth(match.match_date) }}</div>
                <div class="text-[10px] font-medium text-ink3">{{ monthShort(match.match_date) }}</div>
              </div>
              <div class="min-w-[200px] flex-1">
                <div class="text-base font-bold">
                  {{ formatHour(match.start_time) }} – {{ formatHour(match.end_time) }} · {{ match.venue }}
                </div>
                <div class="text-[13px] text-ink2">{{ match.address || 'Sem endereço' }}</div>
                <div class="mt-1 text-[12.5px] font-medium text-ink2">
                  <strong class="text-ink">{{ match.going_count }} confirmados</strong>
                  · {{ match.not_going_count }} não vão · {{ match.no_response_count }} sem resposta
                  <span class="text-ink3">· prazo {{ formatTimestamp(match.confirmation_deadline) }}</span>
                </div>
              </div>
              <Badge :tone="statusBadge(match).tone" dot>{{ statusBadge(match).label }}</Badge>
            </div>

            <div v-if="teams[match.id]?.length" class="mt-3 flex flex-wrap gap-4 rounded-xl bg-bg p-3">
              <div v-for="team in teams[match.id]" :key="team.id" class="min-w-[200px] flex-1">
                <div class="mb-1.5 flex items-center gap-2">
                  <span class="h-3 w-3 rounded-full" :style="{ backgroundColor: team.team_color }" />
                  <span class="font-condensed text-[13px] font-bold tracking-[.06em]">{{ team.team_name.toUpperCase() }}</span>
                  <span class="text-[11.5px] text-ink3">{{ team.members.length }} jogadores</span>
                </div>
                <div class="flex flex-wrap gap-1.5">
                  <div v-for="member in team.members" :key="member.user_id" class="flex items-center gap-1.5">
                    <Avatar :name="member.name" :color="member.avatar_color" size="xs" />
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-3.5 flex flex-wrap gap-2">
              <BaseButton v-if="match.status === 'open'" size="sm" @click="act(match, 'close-confirmations')">
                Encerrar confirmações
              </BaseButton>
              <BaseButton v-else-if="match.status !== 'teams_drawn'" variant="outline" size="sm" @click="act(match, 'reopen-confirmations')">
                Reabrir confirmações
              </BaseButton>
              <BaseButton
                v-if="match.status !== 'open'"
                variant="outline"
                size="sm"
                @click="((drawing = match), (teamCount = teams[match.id]?.length || 2))"
              >
                <NavIcon name="redo" :size="14" :stroke-width="1.9" />
                {{ match.status === 'teams_drawn' ? 'Refazer sorteio' : 'Sortear times' }}
              </BaseButton>
              <BaseButton v-if="match.status !== 'open'" variant="outline-brand" size="sm" @click="act(match, 'finish')">
                Finalizar e abrir votação
              </BaseButton>
              <BaseButton variant="ghost" size="sm" @click="openEdit(match)">Editar</BaseButton>
              <BaseButton variant="ghost" size="sm" class="text-danger" @click="cancelling = match">Cancelar partida</BaseButton>
            </div>
          </Card>

          <Card v-if="upcoming.length === 0" class="px-5 py-8 text-center text-sm text-ink3">
            Nenhuma partida em andamento. Crie a próxima para abrir as confirmações.
          </Card>
        </div>
      </div>

      <!-- Encerradas -->
      <div>
        <SectionLabel>Encerradas</SectionLabel>
        <Card class="mt-2.5 overflow-hidden">
          <div class="overflow-x-auto">
            <table class="w-full min-w-[680px] text-left">
              <thead>
                <tr class="bg-surface2 text-[10.5px] font-bold uppercase tracking-[.08em] text-ink3">
                  <th class="px-5 py-2.5 font-bold">Data</th>
                  <th class="px-3 py-2.5 font-bold">Local</th>
                  <th class="px-3 py-2.5 font-bold">Jogadores</th>
                  <th class="px-3 py-2.5 font-bold">Status</th>
                  <th class="px-5 py-2.5 text-right font-bold">Ações</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="match in past" :key="match.id" class="border-t border-border">
                  <td class="px-5 py-2.5 text-[13px] font-semibold">{{ formatDateShort(match.match_date) }}</td>
                  <td class="px-3 py-2.5 text-[13px] text-ink2">{{ match.venue }}</td>
                  <td class="px-3 py-2.5 text-[13px] text-ink2">
                    {{ match.going_count }}<span v-if="match.media_count" class="text-ink3"> · {{ match.media_count }} mídias</span>
                  </td>
                  <td class="px-3 py-2.5">
                    <Badge :tone="statusBadge(match).tone">{{ statusBadge(match).label }}</Badge>
                  </td>
                  <td class="px-5 py-2.5 text-right">
                    <div class="flex justify-end gap-2">
                      <button
                        v-if="match.status === 'voting'"
                        type="button"
                        class="text-[12.5px] font-semibold text-brand"
                        @click="act(match, 'close-voting')"
                      >
                        Encerrar votação
                      </button>
                      <router-link :to="`/historico/${match.id}`" class="text-[12.5px] font-medium text-ink3 hover:text-ink">
                        Ver detalhes
                      </router-link>
                    </div>
                  </td>
                </tr>
                <tr v-if="past.length === 0">
                  <td colspan="5" class="px-5 py-8 text-center text-sm text-ink3">Nenhuma partida encerrada ainda.</td>
                </tr>
              </tbody>
            </table>
          </div>
        </Card>
      </div>
    </div>

    <!-- Criar / editar -->
    <Modal :open="showForm" :title="editingId ? 'Editar partida' : 'Nova partida'" @close="showForm = false">
      <form class="flex flex-col gap-3" @submit.prevent="save">
        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Data</span>
            <input v-model="form.match_date" type="date" required class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Local</span>
            <input v-model="form.venue" required placeholder="Arena Society Central" class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Início</span>
            <input v-model="form.start_time" type="time" required class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Fim</span>
            <input v-model="form.end_time" type="time" class="field" />
          </label>
        </div>
        <label class="flex flex-col gap-1.5">
          <span class="text-[12.5px] font-semibold text-ink2">Endereço</span>
          <input v-model="form.address" placeholder="R. das Palmeiras, 480" class="field" />
        </label>
        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Confirmação até (data)</span>
            <input v-model="form.deadline_date" type="date" required class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Hora limite</span>
            <input v-model="form.deadline_time" type="time" required class="field" />
          </label>
        </div>
        <label class="flex flex-col gap-1.5">
          <span class="text-[12.5px] font-semibold text-ink2">Observações</span>
          <input v-model="form.notes" placeholder="Opcional" class="field" />
        </label>

        <p v-if="error" class="text-[13px] font-medium text-danger">{{ error }}</p>
        <BaseButton class="mt-1" :loading="saving" @click="save">
          {{ editingId ? 'Salvar alterações' : 'Criar partida e abrir confirmações' }}
        </BaseButton>
      </form>
    </Modal>

    <!-- Sorteio -->
    <Modal :open="drawing !== null" title="Sortear times" @close="drawing = null">
      <div class="flex flex-col gap-4">
        <p class="text-sm text-ink2">
          {{ drawing?.going_count }} jogadores confirmados serão distribuídos aleatoriamente, com diferença máxima de um
          jogador entre os times.
        </p>
        <label class="flex flex-col gap-1.5">
          <span class="text-[12.5px] font-semibold text-ink2">Quantidade de times</span>
          <select v-model.number="teamCount" class="field">
            <option :value="2">2 times</option>
            <option :value="3">3 times</option>
            <option :value="4">4 times</option>
          </select>
        </label>
        <p v-if="drawing?.status === 'teams_drawn'" class="rounded-xl bg-warnBg px-3.5 py-2.5 text-[12.5px] text-warn">
          Refazer o sorteio descarta a distribuição atual e grava uma nova versão.
        </p>
        <BaseButton :loading="saving" @click="confirmDraw">Sortear</BaseButton>
      </div>
    </Modal>

    <!-- Cancelamento -->
    <Modal :open="cancelling !== null" title="Cancelar partida" @close="cancelling = null">
      <form class="flex flex-col gap-4" @submit.prevent="confirmCancel">
        <label class="flex flex-col gap-1.5">
          <span class="text-[12.5px] font-semibold text-ink2">Motivo</span>
          <input v-model="cancelReason" required placeholder="Ex.: chuva" class="field" />
        </label>
        <BaseButton variant="danger" :loading="saving" @click="confirmCancel">Cancelar partida</BaseButton>
      </form>
    </Modal>
  </AdminLayout>
</template>

<style scoped>
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
</style>
