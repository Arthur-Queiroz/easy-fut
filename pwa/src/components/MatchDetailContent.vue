<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, errorMessage } from '../api/client'
import type { ConfirmationEntry, MatchDetail } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { formatDateShort, formatTimestamp } from '../lib/format'
import { safeColor, safeUrl, validateMediaFile } from '../lib/security'
import type { VoteCategory } from '../api/types'
import NavIcon from './layout/NavIcon.vue'
import Avatar from './ui/Avatar.vue'
import Badge from './ui/Badge.vue'
import Card from './ui/Card.vue'
import SectionLabel from './ui/SectionLabel.vue'

const props = defineProps<{ matchId: string }>()

const router = useRouter()
const auth = useAuthStore()

const detail = ref<MatchDetail | null>(null)
const error = ref('')
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// FUT-005: 3 categorias (best_goal herda regras do top_scorer — pode votar em si)
const VOTE_CATEGORIES: { key: VoteCategory; label: string; icon: string; selfVote: boolean; warnColor: boolean }[] = [
  { key: 'top_scorer', label: 'Vote no artilheiro', icon: 'trophy', selfVote: true, warnColor: false },
  { key: 'worst_player', label: 'Vote no perna de pau', icon: '', selfVote: false, warnColor: true },
  { key: 'best_goal', label: 'Vote no melhor gol', icon: 'ball', selfVote: true, warnColor: false },
]
const CATEGORY_KEYS = VOTE_CATEGORIES.map((c) => c.key)

// FUT-005: categorias mostradas como resultado da partida finalizada
const RESULT_CATEGORIES: { key: VoteCategory; resultLabel: string; icon: string; warnColor: boolean }[] = [
  { key: 'top_scorer', resultLabel: 'Artilheiro da partida', icon: 'trophy', warnColor: false },
  { key: 'worst_player', resultLabel: 'Perna de pau da partida', icon: '', warnColor: true },
  { key: 'best_goal', resultLabel: 'Melhor gol da partida', icon: 'ball', warnColor: false },
]

// FUT-003: rascunho de votos { top_scorer: [], worst_player: [], best_goal: [] }.
// Toques alteram o rascunho localmente; "Enviar votos" manda à API o voto
// completo das categorias alteradas, numa requisição só.
const draft = ref<Record<string, string[]>>({})
const submitting = ref(false)
const savedToast = ref(false)
let savedToastTimer: ReturnType<typeof setTimeout> | undefined

function draftKey(): string {
  return `fut-vote-draft-${props.matchId}-${auth.user?.id ?? 'anon'}`
}

function loadDraft() {
  try {
    const raw = localStorage.getItem(draftKey())
    draft.value = raw ? JSON.parse(raw) : {}
  } catch {
    draft.value = {}
  }
}

function saveDraft() {
  try {
    if (draftIsDirty.value) localStorage.setItem(draftKey(), JSON.stringify(draft.value))
    else localStorage.removeItem(draftKey())
  } catch { /* storage cheio/indisponível: rascunho fica só na tela */ }
}

// votos efetivos (persistidos) por categoria
const persisted = computed<Record<string, string[]>>(() => ({
  top_scorer: detail.value?.my_votes.top_scorer ?? [],
  worst_player: detail.value?.my_votes.worst_player ?? [],
  best_goal: (detail.value?.my_votes as Record<string, string[]>).best_goal ?? [],
}))

// o que o usuário vê marcado: rascunho se existe, senão o persistido
const marks = computed<Record<string, string[]>>(() => ({ top_scorer: draft.value.top_scorer ?? persisted.value.top_scorer ?? [], worst_player: draft.value.worst_player ?? persisted.value.worst_player ?? [], best_goal: draft.value.best_goal ?? persisted.value.best_goal ?? [] }))

// categorias em que o marcado difere do persistido → lista que deve valer
const changedVotes = computed<Record<string, string[]>>(() => {
  const votes: Record<string, string[]> = {}
  for (const c of CATEGORY_KEYS) {
    if ([...marks.value[c]].sort().join(',') !== [...persisted.value[c]].sort().join(',')) votes[c] = marks.value[c]
  }
  return votes
})

const draftIsDirty = computed(() => Object.keys(changedVotes.value).length > 0)

function toggleMark(category: VoteCategory, candidateId: string) {
  if (!canVote.value) return
  const cur = [...(draft.value[category] ?? persisted.value[category] ?? [])]
  const idx = cur.indexOf(candidateId)
  if (idx >= 0) cur.splice(idx, 1)
  else {
    if (cur.length >= 2) {
      error.value = 'Escolha no máximo 2 jogadores. Desmarque um para trocar.'
      return
    }
    cur.push(candidateId)
  }
  draft.value = { ...draft.value, [category]: cur }
  error.value = ''
  saveDraft()
}

// PUT substitui o voto da categoria de uma vez: trocar um jogador com os 2
// votos usados não depende da ordem entre remover e adicionar.
async function submitVotes() {
  if (submitting.value || !votingOpen.value) return
  if (!draftIsDirty.value) {
    savedToast.value = true
    resetToastTimer()
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await api.put(`/matches/${props.matchId}/votes`, { votes: changedVotes.value })
    draft.value = {}
    saveDraft()
    await load()
    savedToast.value = true
    resetToastTimer()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    submitting.value = false
  }
}

// Auto-envio: sair da tela ou fechar o app com rascunho pendente envia o que
// falta. fetch com keepalive termina mesmo com a página fechando (o Axios
// não). Se falhar, o rascunho continua no localStorage e volta na reabertura.
function flushDraft() {
  if (!draftIsDirty.value || submitting.value || !votingOpen.value) return
  void fetch(`/api/matches/${props.matchId}/votes`, {
    method: 'PUT',
    keepalive: true,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ votes: changedVotes.value }),
  }).catch(() => {})
}

function resetToastTimer() {
  if (savedToastTimer) clearTimeout(savedToastTimer)
  savedToastTimer = setTimeout(() => { savedToast.value = false }, 3000)
}

onMounted(() => {
  load()
  loadDraft()
  window.addEventListener('pagehide', flushDraft)
})
onBeforeUnmount(() => {
  window.removeEventListener('pagehide', flushDraft)
  flushDraft()
})
watch(() => props.matchId, () => { load(); loadDraft() })

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
        <Card v-for="cat in RESULT_CATEGORIES" :key="cat.key" class="p-4">
          <div class="mb-[11px] flex items-center gap-[7px] text-[13px] font-bold">
            <span
              class="inline-flex h-7 w-7 items-center justify-center rounded-[9px]"
              :class="cat.warnColor ? 'bg-warnBg text-warn' : 'bg-brandSoft text-brand'"
            >
              <template v-if="cat.warnColor">🥴</template><NavIcon v-else :name="cat.icon" :size="15" />
            </span>
            {{ cat.resultLabel }}
          </div>
          <template v-if="resultFor(cat.key)?.winners.length">
            <div
              v-for="winner in resultFor(cat.key)!.winners"
              :key="winner.user_id"
              class="mb-2 flex items-center gap-2.5 rounded-xl border-2 px-3 py-[9px]"
              :class="cat.warnColor ? 'border-warn bg-warnBg' : 'border-brand bg-brandSoft'"
            >
              <Avatar :name="winner.name" :color="winner.avatar_color" size="sm" />
              <span class="flex-1 text-sm font-semibold">{{ winner.name }}</span>
              <span class="text-[13px] font-bold" :class="cat.warnColor ? 'text-warn' : 'text-brand'">
                {{ winner.votes }} {{ winner.votes === 1 ? 'voto' : 'votos' }}
              </span>
            </div>
          </template>
          <p v-else class="text-sm text-ink3">Sem votos nesta categoria.</p>
        </Card>
      </template>

      <!-- Votação aberta (FUT-005: 3 categorias) -->
      <template v-else-if="votingOpen">
        <Card v-for="cat in VOTE_CATEGORIES" :key="cat.key" class="p-4">
          <div class="mb-[11px] flex items-center justify-between">
            <span class="inline-flex items-center gap-[7px] text-[13px] font-bold">
              <span
                class="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-[15px]"
                :class="cat.warnColor ? 'bg-warnBg' : 'bg-brandSoft text-brand'"
              ><template v-if="cat.warnColor">🥴</template><NavIcon v-else :name="cat.icon" :size="15" /></span>
              {{ cat.label }}
            </span>
            <span class="text-[11.5px] font-medium text-ink3">
              {{ marks[cat.key].length }}/2 votos<template v-if="!cat.selfVote"> · sem voto em si mesmo</template>
            </span>
          </div>
          <div class="flex flex-col gap-2">
            <button
              v-for="player in (cat.selfVote ? participants : participants.filter((p) => p.user_id !== auth.user?.id))"
              :key="player.user_id"
              type="button"
              :disabled="!canVote"
              class="flex items-center gap-2.5 rounded-xl px-3 py-[9px] text-left disabled:opacity-60"
              :class="marks[cat.key].includes(player.user_id) ? 'border-2 ' + (cat.warnColor ? 'border-warn bg-warnBg' : 'border-brand bg-brandSoft') : 'border border-border'"
              @click="toggleMark(cat.key, player.user_id)"
            >
              <Avatar :name="player.name" :color="player.avatar_color" size="sm" />
              <span class="flex-1 text-sm" :class="marks[cat.key].includes(player.user_id) ? 'font-semibold' : 'font-medium'">
                {{ player.user_id === auth.user?.id ? 'Você' : player.name }}
              </span>
              <Badge v-if="player.kind === 'guest'" tone="info">Avulso</Badge>
              <span
                v-if="marks[cat.key].includes(player.user_id)"
                class="flex h-5 w-5 items-center justify-center rounded-full"
                :class="cat.warnColor ? 'bg-warn' : 'bg-brand'"
              >
                <NavIcon name="check" :size="11" :stroke-width="3" class="text-white" />
              </span>
              <span v-else class="h-5 w-5 rounded-full border-2 border-border" />
            </button>
          </div>
          <p v-if="!detail.is_participant && cat.key === 'worst_player'" class="mt-3 text-xs text-ink3">
            Somente quem participou da partida pode votar.
          </p>
        </Card>

        <!-- Botão de envio (FUT-003) -->
        <div v-if="canVote" class="sticky bottom-[calc(3.5rem+max(env(safe-area-inset-bottom),1.25rem))] z-10 flex flex-col gap-2 lg:bottom-4">
          <button
            type="button"
            :disabled="submitting"
            class="flex h-[50px] items-center justify-center gap-2 rounded-[13px] text-[15px] font-bold shadow-lg transition"
            :class="
              draftIsDirty
                ? 'bg-brand text-brandInk'
                : 'bg-surface text-ink3 border border-border'
            "
            @click="submitVotes"
          >
            <template v-if="submitting">Enviando…</template>
            <template v-else-if="draftIsDirty">Enviar votos</template>
            <template v-else>Votos registrados ✓</template>
          </button>
          <p v-if="savedToast" class="text-center text-[12.5px] font-semibold text-brand">Votos registrados ✓</p>
        </div>
      </template>

      <!-- Participantes -->
      <Card class="p-4">
        <div class="mb-[11px] flex items-center justify-between">
          <SectionLabel>Participantes</SectionLabel>
          <span class="font-condensed text-[15px] font-bold text-brand">{{ participants.length }}</span>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div v-for="player in participants" :key="player.user_id" class="flex min-w-0 items-center gap-2">
            <Avatar :name="player.name" :color="player.avatar_color" size="xs" />
            <span class="truncate text-[13px] font-medium">{{ player.name }}</span>
            <Badge v-if="player.kind === 'guest'" tone="info">Avulso</Badge>
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
            <div class="flex flex-wrap items-center gap-x-1 gap-y-1 text-[13px] leading-relaxed text-ink2">
              <span v-for="(m, i) in team.members" :key="m.user_id" class="inline-flex items-center gap-1">
                {{ m.name.split(' ')[0] }}<Badge v-if="m.kind === 'guest'" tone="info">Avulso</Badge><template v-if="i < team.members.length - 1">,</template>
              </span>
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
