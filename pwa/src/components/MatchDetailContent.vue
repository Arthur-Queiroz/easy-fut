<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, errorMessage } from '../api/client'
import type { ConfirmationEntry, MatchDetail } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { formatDateShort, formatTimestamp } from '../lib/format'
import { safeColor, safeUrl, validateMediaFile } from '../lib/security'
import NavIcon from './layout/NavIcon.vue'
import Avatar from './ui/Avatar.vue'
import Card from './ui/Card.vue'
import SectionLabel from './ui/SectionLabel.vue'

const props = defineProps<{ matchId: string }>()

const router = useRouter()
const auth = useAuthStore()

const detail = ref<MatchDetail | null>(null)
const error = ref('')
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

onMounted(load)
watch(() => props.matchId, load)

async function load() {
  if (!props.matchId) return
  const { data } = await api.get<MatchDetail>(`/matches/${props.matchId}`)
  detail.value = data
}

const match = computed(() => detail.value?.match ?? null)
const participants = computed<ConfirmationEntry[]>(
  () => detail.value?.confirmations.filter((c) => c.response === 'going') ?? [],
)
const votingOpen = computed(() => match.value?.status === 'voting')
const canVote = computed(() => votingOpen.value && detail.value?.is_participant)

async function vote(category: 'top_scorer' | 'worst_player', candidateId: string) {
  if (!canVote.value) return
  error.value = ''
  try {
    await api.post(`/matches/${props.matchId}/votes`, { category, candidate_id: candidateId })
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function uploadMedia(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const validation = validateMediaFile(file)
  if (!validation.ok) {
    error.value = validation.error ?? 'Arquivo inválido.'
    input.value = ''
    return
  }

  error.value = ''
  uploading.value = true
  try {
    const form = new FormData()
    form.append('file', file)
    await api.post(`/matches/${props.matchId}/media`, form)
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function removeMedia(id: string) {
  await api.delete(`/media/${id}`)
  await load()
}

function resultFor(category: string) {
  return detail.value?.results?.find((r) => r.category === category) ?? null
}
</script>

<template>
  <div v-if="detail && match" class="flex flex-col gap-3 lg:gap-4">
    <!-- Cabeçalho mobile do detalhe -->
    <div class="flex items-center gap-2.5 lg:hidden">
      <button type="button" aria-label="Voltar" @click="router.back()">
        <NavIcon name="back" :size="20" :stroke-width="2" />
      </button>
      <div>
        <div class="text-[21px] font-bold">Partida de {{ formatDateShort(match.match_date) }}</div>
        <div class="text-[13px] text-ink2">{{ match.venue }} · {{ participants.length }} jogadores</div>
      </div>
    </div>

    <!-- Cabeçalho desktop -->
    <div class="hidden items-start justify-between lg:flex">
      <div>
        <div class="text-2xl font-extrabold">Partida de {{ formatDateShort(match.match_date) }}</div>
        <div class="mt-1 text-[14px] text-ink2">{{ match.venue }} · {{ participants.length }} jogadores</div>
      </div>
      <span
        v-if="match.status === 'voting'"
        class="inline-flex items-center gap-1.5 rounded-full bg-infoBg px-3 py-[5px] text-[13px] font-semibold text-info"
      >● Votação aberta</span>
      <span
        v-else-if="match.status === 'finished'"
        class="inline-flex items-center gap-1.5 rounded-full bg-surface2 px-3 py-[5px] text-[13px] font-semibold text-ink2"
      >Finalizada</span>
    </div>

    <!-- Aviso de votação -->
    <div
      v-if="votingOpen"
      class="flex items-center gap-2.5 rounded-[14px] border border-info bg-infoBg px-3.5 py-[11px]"
    >
      <NavIcon name="clock" :size="17" class="shrink-0 text-info" :stroke-width="1.9" />
      <span class="flex-1 text-[13px] font-medium text-info">
        Votação aberta<template v-if="match.voting_closes_at"> até {{ formatTimestamp(match.voting_closes_at) }}</template>
        · resultado sai depois do encerramento
      </span>
    </div>

    <p v-if="error" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ error }}</p>

    <div class="grid gap-3 lg:grid-cols-2 lg:items-start lg:gap-4">
      <!-- Resultados (partida finalizada) -->
      <template v-if="match.status === 'finished'">
        <Card v-for="category in ['top_scorer', 'worst_player']" :key="category" class="p-4">
          <div class="mb-[11px] flex items-center gap-[7px] text-[13px] font-bold">
            <span
              class="inline-flex h-7 w-7 items-center justify-center rounded-[9px]"
              :class="category === 'top_scorer' ? 'bg-brandSoft text-brand' : 'bg-warnBg text-warn'"
            >
              <NavIcon v-if="category === 'top_scorer'" name="trophy" :size="15" />
              <template v-else>🥴</template>
            </span>
            {{ category === 'top_scorer' ? 'Artilheiro da partida' : 'Perna de pau da partida' }}
          </div>
          <template v-if="resultFor(category)?.winners.length">
            <div
              v-for="winner in resultFor(category)!.winners"
              :key="winner.user_id"
              class="mb-2 flex items-center gap-2.5 rounded-xl border-2 px-3 py-[9px]"
              :class="category === 'top_scorer' ? 'border-brand bg-brandSoft' : 'border-warn bg-warnBg'"
            >
              <Avatar :name="winner.name" :color="winner.avatar_color" size="sm" />
              <span class="flex-1 text-sm font-semibold">{{ winner.name }}</span>
              <span class="text-[13px] font-bold" :class="category === 'top_scorer' ? 'text-brand' : 'text-warn'">
                {{ winner.votes }} {{ winner.votes === 1 ? 'voto' : 'votos' }}
              </span>
            </div>
          </template>
          <p v-else class="text-sm text-ink3">Sem votos nesta categoria.</p>
        </Card>
      </template>

      <!-- Votação aberta -->
      <template v-else-if="votingOpen">
        <Card class="p-4">
          <div class="mb-[11px] flex items-center justify-between">
            <span class="inline-flex items-center gap-[7px] text-[13px] font-bold">
              <span class="inline-flex h-7 w-7 items-center justify-center rounded-[9px] bg-brandSoft text-brand">
                <NavIcon name="trophy" :size="15" />
              </span>
              Vote no artilheiro
            </span>
            <span class="text-[11.5px] font-medium text-ink3">1 voto por categoria</span>
          </div>
          <div class="flex flex-col gap-2">
            <button
              v-for="player in participants"
              :key="player.user_id"
              type="button"
              :disabled="!canVote"
              class="flex items-center gap-2.5 rounded-xl px-3 py-[9px] text-left disabled:opacity-60"
              :class="detail.my_votes.top_scorer === player.user_id ? 'border-2 border-brand bg-brandSoft' : 'border border-border'"
              @click="vote('top_scorer', player.user_id)"
            >
              <Avatar :name="player.name" :color="player.avatar_color" size="sm" />
              <span class="flex-1 text-sm" :class="detail.my_votes.top_scorer === player.user_id ? 'font-semibold' : 'font-medium'">
                {{ player.user_id === auth.user?.id ? 'Você' : player.name }}
              </span>
              <span
                v-if="detail.my_votes.top_scorer === player.user_id"
                class="flex h-5 w-5 items-center justify-center rounded-full bg-brand"
              >
                <NavIcon name="check" :size="11" :stroke-width="3" class="text-white" />
              </span>
              <span v-else class="h-5 w-5 rounded-full border-2 border-border" />
            </button>
          </div>
        </Card>

        <Card class="p-4">
          <div class="mb-[11px] flex items-center justify-between">
            <span class="inline-flex items-center gap-[7px] text-[13px] font-bold">
              <span class="inline-flex h-7 w-7 items-center justify-center rounded-[9px] bg-warnBg text-[15px]">🥴</span>
              Vote no perna de pau
            </span>
            <span class="text-[11.5px] font-medium text-ink3">Sem voto em si mesmo</span>
          </div>
          <div class="flex flex-col gap-2">
            <button
              v-for="player in participants.filter((p) => p.user_id !== auth.user?.id)"
              :key="player.user_id"
              type="button"
              :disabled="!canVote"
              class="flex items-center gap-2.5 rounded-xl px-3 py-[9px] text-left disabled:opacity-60"
              :class="detail.my_votes.worst_player === player.user_id ? 'border-2 border-warn bg-warnBg' : 'border border-border'"
              @click="vote('worst_player', player.user_id)"
            >
              <Avatar :name="player.name" :color="player.avatar_color" size="sm" />
              <span class="flex-1 text-sm" :class="detail.my_votes.worst_player === player.user_id ? 'font-semibold' : 'font-medium'">
                {{ player.name }}
              </span>
              <span
                v-if="detail.my_votes.worst_player === player.user_id"
                class="flex h-5 w-5 items-center justify-center rounded-full bg-warn"
              >
                <NavIcon name="check" :size="11" :stroke-width="3" class="text-white" />
              </span>
              <span v-else class="h-5 w-5 rounded-full border-2 border-border" />
            </button>
          </div>
          <p v-if="!detail.is_participant" class="mt-3 text-xs text-ink3">
            Somente quem participou da partida pode votar.
          </p>
        </Card>
      </template>

      <!-- Participantes -->
      <Card class="p-4">
        <div class="mb-[11px] flex items-center justify-between">
          <SectionLabel>Participantes</SectionLabel>
          <span class="font-condensed text-[15px] font-bold text-brand">{{ participants.length }}</span>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div v-for="player in participants" :key="player.user_id" class="flex items-center gap-2">
            <Avatar :name="player.name" :color="player.avatar_color" size="xs" />
            <span class="truncate text-[13px] font-medium">{{ player.name }}</span>
          </div>
        </div>
      </Card>

      <!-- Times -->
      <Card v-if="detail.teams.length" class="p-4">
        <SectionLabel>Times</SectionLabel>
        <div class="mt-2.5 flex flex-col gap-3">
          <div v-for="team in detail.teams" :key="team.id">
            <div class="mb-1.5 flex items-center gap-2">
              <span class="h-3.5 w-3.5 rounded-full" :style="{ backgroundColor: safeColor(team.team_color) }" />
              <span class="font-condensed text-sm font-bold tracking-[.06em]">{{ team.team_name.toUpperCase() }}</span>
            </div>
            <div class="text-[13px] leading-relaxed text-ink2">
              {{ team.members.map((m) => m.name.split(' ')[0]).join(', ') }}
            </div>
          </div>
        </div>
      </Card>

      <!-- Fotos e vídeos -->
      <Card class="p-4" :class="match.status === 'finished' || match.status === 'voting' ? 'lg:col-span-2' : ''">
        <div class="mb-[11px] flex items-center justify-between">
          <SectionLabel>Fotos e vídeos · {{ detail.media.length }}</SectionLabel>
          <button
            v-if="detail.is_participant"
            type="button"
            class="text-[12.5px] font-semibold text-brand disabled:opacity-50"
            :disabled="uploading"
            @click="fileInput?.click()"
          >
            {{ uploading ? 'Enviando…' : '+ Adicionar' }}
          </button>
          <input ref="fileInput" type="file" accept="image/*,video/*" class="hidden" @change="uploadMedia" />
        </div>
        <div class="grid grid-cols-4 gap-2">
          <div v-for="item in detail.media" :key="item.id" class="group relative aspect-square overflow-hidden rounded-[10px] bg-surface2">
            <img v-if="item.type === 'photo'" :src="safeUrl(item.url)" :alt="item.caption || 'Foto da partida'" class="h-full w-full object-cover" loading="lazy" />
            <video v-else :src="safeUrl(item.url)" class="h-full w-full object-cover" preload="metadata" controls />
            <button
              v-if="item.uploaded_by === auth.user?.id || auth.isAdmin"
              type="button"
              class="absolute right-1 top-1 hidden h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white group-hover:flex"
              aria-label="Remover mídia"
              @click="removeMedia(item.id)"
            >
              <NavIcon name="x" :size="10" :stroke-width="2.6" />
            </button>
          </div>
          <button
            v-if="detail.is_participant"
            type="button"
            class="flex aspect-square items-center justify-center rounded-[10px] border-[1.5px] border-dashed border-border text-xl font-semibold text-ink3"
            :disabled="uploading"
            @click="fileInput?.click()"
          >
            +
          </button>
        </div>
        <p v-if="detail.media.length === 0 && !detail.is_participant" class="text-sm text-ink3">Sem mídias nesta partida.</p>
      </Card>
    </div>
  </div>
  <div v-else class="flex h-full items-center justify-center text-sm text-ink3">
    Carregando…
  </div>
</template>
