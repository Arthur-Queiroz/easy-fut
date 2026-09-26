<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { ConfirmationEntry, Match, Team, TeamPreset } from '../../api/types'
import { safeColor } from '../../lib/security'
import Avatar from '../ui/Avatar.vue'
import BaseButton from '../ui/BaseButton.vue'
import Modal from '../ui/Modal.vue'
import SectionLabel from '../ui/SectionLabel.vue'

const props = defineProps<{ match: Match }>()
const emit = defineEmits<{ drawn: []; close: [] }>();

// Espelha os presets visuais de api/internal/draw/draw.go (ordem define o time).
const teamLooks = [
  { name: 'Colete Verde', color: '#C8F14B' },
  { name: 'Colete Laranja', color: '#F59E0B' },
  { name: 'Colete Azul', color: '#3B82F6' },
  { name: 'Colete Preto', color: '#1F2937' },
]

const mode = ref<'random' | 'manual'>('random')
const teamCount = ref(2)
const going = ref<ConfirmationEntry[]>([])
const presets = ref<TeamPreset[]>([])
// time de cada jogador no modo manual: índice 0..3, -1 = sem time
const assignment = ref<Record<string, number>>({})
const result = ref<Team[] | null>(null)
const presetTarget = ref<Record<string, number>>({})
const presetName = ref('')
const presetSourceTeam = ref(0)
const saving = ref(false)
const error = ref('')

onMounted(async () => {
  const [confirmationsRes, presetsRes] = await Promise.all([
    api.get<ConfirmationEntry[]>(`/matches/${props.match.id}/confirmations`),
    api.get<TeamPreset[]>('/admin/team-presets'),
  ])
  going.value = (confirmationsRes.data ?? []).filter((c) => c.response === 'going')
  presets.value = presetsRes.data ?? []
  for (const p of presets.value) {
    presetTarget.value[p.id] = 0
  }

  // Se já houve sorteio, a escalação atual é o ponto de partida do modo manual.
  if (props.match.status === 'teams_drawn') {
    const { data: current } = await api.get<Team[]>(`/matches/${props.match.id}/teams`)
    for (const team of current ?? []) {
      for (const member of team.members) {
        assignment.value[member.user_id] = team.position
      }
    }
  }
  for (const p of going.value) {
    assignment.value[p.user_id] ??= -1
  }
})

const unassigned = computed(() => going.value.filter((p) => (assignment.value[p.user_id] ?? -1) < 0))

function teamLabel(index: number) {
  return `Time ${index + 1} · ${teamLooks[index]?.name ?? ''}`
}

function applyPreset(preset: TeamPreset) {
  const target = presetTarget.value[preset.id] ?? 0
  const goingIds = new Set(going.value.map((p) => p.user_id))
  // Ausentes (não confirmados) são simplesmente ignorados.
  for (const id of preset.member_ids) {
    if (goingIds.has(id)) assignment.value[id] = target
  }
}

function shuffleRest() {
  const pool = [...unassigned.value]
  for (let i = pool.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[pool[i], pool[j]] = [pool[j], pool[i]]
  }
  const sizes = Array.from({ length: teamCount.value }, (_, t) =>
    going.value.filter((p) => assignment.value[p.user_id] === t).length,
  )
  for (const p of pool) {
    let target = 0
    for (let t = 1; t < teamCount.value; t++) {
      if (sizes[t] < sizes[target]) target = t
    }
    assignment.value[p.user_id] = target
    sizes[target]++
  }
}

async function savePreset() {
  error.value = ''
  const memberIds = going.value
    .filter((p) => assignment.value[p.user_id] === presetSourceTeam.value)
    .map((p) => p.user_id)
  if (!presetName.value.trim() || memberIds.length === 0) {
    error.value = 'Dê um nome ao preset e escalone ao menos um jogador no time escolhido.'
    return
  }
  saving.value = true
  try {
    await api.post('/admin/team-presets', { name: presetName.value.trim(), member_ids: memberIds })
    const { data } = await api.get<TeamPreset[]>('/admin/team-presets')
    presets.value = data ?? []
    presetName.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function deletePreset(preset: TeamPreset) {
  error.value = ''
  try {
    await api.delete(`/admin/team-presets/${preset.id}`)
    presets.value = presets.value.filter((p) => p.id !== preset.id)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function submit() {
  error.value = ''
  saving.value = true
  try {
    let body: object
    if (mode.value === 'random') {
      body = { team_count: teamCount.value }
    } else {
      if (unassigned.value.length > 0) {
        error.value = `Ainda há ${unassigned.value.length} confirmado(s) sem time.`
        saving.value = false
        return
      }
      const teams = Array.from({ length: teamCount.value }, (_, t) =>
        going.value.filter((p) => assignment.value[p.user_id] === t).map((p) => p.user_id),
      )
      if (teams.some((t) => t.length === 0)) {
        error.value = 'Cada time precisa de ao menos um jogador.'
        saving.value = false
        return
      }
      body = { teams }
    }
    const { data } = await api.post<Team[]>(`/matches/${props.match.id}/draw-teams`, body)
    result.value = data ?? []
    emit('drawn')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal :open="true" title="Sortear times" @close="emit('close')">
    <div class="flex max-h-[75vh] flex-col gap-4 overflow-y-auto pr-1">
      <p class="text-sm text-ink2">
        {{ match.going_count }} jogadores confirmados. O resultado pode ser refeito quantas vezes quiser —
        cada sorteio descarta a distribuição anterior.
      </p>

      <!-- Modo -->
      <div class="grid grid-cols-2 gap-1 rounded-[11px] bg-bg p-1">
        <button
          type="button"
          class="h-9 rounded-[9px] text-[13px] font-semibold"
          :class="mode === 'random' ? 'bg-surface text-ink shadow-sm' : 'text-ink3'"
          @click="mode = 'random'"
        >
          Sorteio aleatório
        </button>
        <button
          type="button"
          class="h-9 rounded-[9px] text-[13px] font-semibold"
          :class="mode === 'manual' ? 'bg-surface text-ink shadow-sm' : 'text-ink3'"
          @click="mode = 'manual'"
        >
          Escalação manual
        </button>
      </div>

      <label class="flex flex-col gap-1.5">
        <span class="text-[12.5px] font-semibold text-ink2">Quantidade de times</span>
        <select v-model.number="teamCount" class="field">
          <option :value="2">2 times</option>
          <option :value="3">3 times</option>
          <option :value="4">4 times</option>
        </select>
      </label>

      <template v-if="mode === 'manual'">
        <!-- Presets -->
        <div class="flex flex-col gap-2 rounded-[11px] border border-border p-3">
          <SectionLabel>Presets</SectionLabel>
          <div v-if="presets.length === 0" class="text-[12.5px] text-ink3">
            Nenhum preset salvo. Escale um time abaixo e salve como preset.
          </div>
          <div v-for="preset in presets" :key="preset.id" class="flex items-center gap-2">
            <span class="min-w-0 flex-1 truncate text-[13px] font-medium">{{ preset.name }}</span>
            <select v-model.number="presetTarget[preset.id]" class="field !h-9 !px-2 text-[12px]">
              <option v-for="t in teamCount" :key="t" :value="t - 1">Time {{ t }}</option>
            </select>
            <button type="button" class="text-[12.5px] font-semibold text-brand" @click="applyPreset(preset)">
              Aplicar
            </button>
            <button type="button" class="text-[12.5px] font-semibold text-danger" @click="deletePreset(preset)">
              Excluir
            </button>
          </div>
          <div class="mt-1 flex items-center gap-2 border-t border-border pt-2.5">
            <input v-model="presetName" placeholder="Nome do preset" class="field !h-9 flex-1 !px-2 text-[12px]" />
            <select v-model.number="presetSourceTeam" class="field !h-9 !px-2 text-[12px]">
              <option v-for="t in teamCount" :key="t" :value="t - 1">Time {{ t }}</option>
            </select>
            <button type="button" class="text-[12.5px] font-semibold text-brand" @click="savePreset">Salvar</button>
          </div>
        </div>

        <!-- Escalação -->
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <SectionLabel>Confirmados</SectionLabel>
            <button
              v-if="unassigned.length"
              type="button"
              class="text-[12.5px] font-semibold text-brand"
              @click="shuffleRest"
            >
              Sortear o restante ({{ unassigned.length }})
            </button>
          </div>
          <div v-for="player in going" :key="player.user_id" class="flex items-center gap-2">
            <Avatar :name="player.name" :color="player.avatar_color" size="xs" />
            <span class="min-w-0 flex-1 truncate text-[13px] font-medium">{{ player.name }}</span>
            <select v-model.number="assignment[player.user_id]" class="field !h-9 !px-2 text-[12px]">
              <option :value="-1">Sem time</option>
              <option v-for="t in teamCount" :key="t" :value="t - 1">{{ teamLabel(t - 1) }}</option>
            </select>
          </div>
        </div>
      </template>

      <p v-if="error" class="rounded-xl bg-dangerBg px-3.5 py-2.5 text-[12.5px] font-medium text-danger">{{ error }}</p>

      <BaseButton :loading="saving" @click="submit">
        {{ match.status === 'teams_drawn' ? 'Refazer sorteio' : 'Sortear' }}
      </BaseButton>

      <!-- Resultado do último sorteio (modal continua aberto) -->
      <div v-if="result" class="flex flex-col gap-2 rounded-[11px] bg-bg p-3">
        <SectionLabel>Resultado</SectionLabel>
        <div v-for="team in result" :key="team.id" class="flex items-start gap-2">
          <span class="mt-1 h-3 w-3 shrink-0 rounded-full" :style="{ backgroundColor: safeColor(team.team_color) }" />
          <div class="min-w-0">
            <div class="font-condensed text-[13px] font-bold tracking-[.06em]">{{ team.team_name.toUpperCase() }}</div>
            <div class="text-[12px] text-ink2">{{ team.members.map((m) => m.name).join(', ') }}</div>
          </div>
        </div>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
</style>
