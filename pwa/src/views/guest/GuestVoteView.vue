<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  getGuestVote,
  httpStatus,
  publicErrorMessage,
  submitGuestVotes,
  type GuestVote,
} from '../../api/publicGuests'
import type { VoteCategory } from '../../api/types'
import { formatDateShort, formatTimestamp } from '../../lib/format'
import {
  changedVotes as diffVotes,
  GUEST_VOTE_CATEGORIES,
  markedVotes,
  MAX_VOTES_PER_CATEGORY,
  toggleCandidate,
  type VoteMap,
} from '../../lib/voteDraft'
import PublicShell from '../../components/layout/PublicShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Card from '../../components/ui/Card.vue'

const route = useRoute()
const token = String(route.params.token ?? '')

const vote = ref<GuestVote | null>(null)
const state = ref<'loading' | 'ready' | 'invalid' | 'closed' | 'error'>('loading')
const loadError = ref('')
const error = ref('')
const submitting = ref(false)
const savedToast = ref(false)
let savedToastTimer: ReturnType<typeof setTimeout> | undefined

// Rascunho local: toques alteram só a tela; "Enviar votos" manda as
// categorias alteradas. Fica no localStorage para sobreviver a um recarregar.
const draft = ref<VoteMap>({})
const draftKey = `fut-guest-vote-draft-${token}`

function loadDraft() {
  try {
    const raw = localStorage.getItem(draftKey)
    draft.value = raw ? JSON.parse(raw) : {}
  } catch {
    draft.value = {}
  }
}

function saveDraft() {
  try {
    if (dirty.value) localStorage.setItem(draftKey, JSON.stringify(draft.value))
    else localStorage.removeItem(draftKey)
  } catch {
    /* storage indisponível: rascunho fica só na tela */
  }
}

const persisted = computed<VoteMap>(() => vote.value?.my_votes ?? {})
const marks = computed(() => markedVotes(draft.value, persisted.value))
const changed = computed(() => diffVotes(draft.value, persisted.value))
const dirty = computed(() => Object.keys(changed.value).length > 0)
const hasVotes = computed(() => Object.values(persisted.value).some((ids) => ids && ids.length > 0))

async function load() {
  try {
    vote.value = await getGuestVote(token)
    state.value = 'ready'
  } catch (e) {
    const status = httpStatus(e)
    if (status === 404) state.value = 'invalid'
    else if (status === 410) state.value = 'closed'
    else {
      state.value = 'error'
      loadError.value = publicErrorMessage(e)
    }
  }
}

onMounted(() => {
  loadDraft()
  void load()
})
onBeforeUnmount(() => {
  if (savedToastTimer) clearTimeout(savedToastTimer)
})

function toggle(category: VoteCategory, candidateId: string) {
  if (submitting.value) return
  const next = toggleCandidate(marks.value[category], candidateId)
  if (!next) {
    error.value = `Escolha no máximo ${MAX_VOTES_PER_CATEGORY} jogadores. Desmarque um para trocar.`
    return
  }
  draft.value = { ...draft.value, [category]: next }
  error.value = ''
  saveDraft()
}

async function submit() {
  if (submitting.value) return
  if (!dirty.value) {
    if (hasVotes.value) showToast()
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await submitGuestVotes(token, changed.value)
    draft.value = {}
    saveDraft()
    await load()
    showToast()
  } catch (e) {
    const status = httpStatus(e)
    if (status === 410) state.value = 'closed'
    else if (status === 404) state.value = 'invalid'
    // 400 inclui votar em si mesmo como perna de pau: a API valida e explica.
    else error.value = publicErrorMessage(e)
  } finally {
    submitting.value = false
  }
}

function showToast() {
  savedToast.value = true
  if (savedToastTimer) clearTimeout(savedToastTimer)
  savedToastTimer = setTimeout(() => (savedToast.value = false), 3000)
}
</script>

<template>
  <PublicShell
    title="Votação da partida"
    :subtitle="vote && state === 'ready' ? `${formatDateShort(vote.match.match_date)}${vote.match.venue ? ' · ' + vote.match.venue : ''}` : undefined"
  >
    <div v-if="state === 'loading'" class="py-10 text-center text-sm text-ink3">Carregando…</div>

    <Card v-else-if="state === 'invalid'" class="p-5 text-center" data-test="invalid">
      <div class="text-[17px] font-bold">Link inválido</div>
      <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
        Esse link de votação não existe ou foi trocado por um novo. Confere a última mensagem no seu WhatsApp.
      </p>
    </Card>

    <Card v-else-if="state === 'closed'" class="p-5 text-center" data-test="closed">
      <div class="text-[17px] font-bold">Votação encerrada</div>
      <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">O prazo pra votar nessa partida já acabou. Valeu por jogar!</p>
    </Card>

    <Card v-else-if="state === 'error'" class="p-4">
      <p class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ loadError }}</p>
    </Card>

    <template v-else-if="vote">
      <div class="flex items-center gap-2.5 rounded-[14px] border border-info bg-infoBg px-3.5 py-[11px]">
        <NavIcon name="clock" :size="17" class="shrink-0 text-info" :stroke-width="1.9" />
        <span class="flex-1 text-[13px] font-medium text-info">
          Votação aberta<template v-if="vote.voting_closes_at"> até {{ formatTimestamp(vote.voting_closes_at) }}</template>
          · até 2 votos por categoria
        </span>
      </div>

      <Card v-for="cat in GUEST_VOTE_CATEGORIES" :key="cat.key" class="p-4" :data-test="`cat-${cat.key}`">
        <div class="mb-[11px] flex items-center justify-between gap-2">
          <span class="inline-flex items-center gap-[7px] text-[13px] font-bold">
            <span
              class="inline-flex h-7 w-7 items-center justify-center rounded-[9px] text-[15px]"
              :class="cat.warnColor ? 'bg-warnBg' : 'bg-brandSoft text-brand'"
            ><template v-if="cat.warnColor">🥴</template><NavIcon v-else :name="cat.icon" :size="15" /></span>
            {{ cat.label }}
          </span>
          <span class="text-[11.5px] font-medium text-ink3">
            {{ marks[cat.key].length }}/2 votos<template v-if="cat.key === 'worst_player'"> · sem voto em si mesmo</template>
          </span>
        </div>
        <div class="flex flex-col gap-2">
          <button
            v-for="player in vote.participants"
            :key="player.user_id"
            type="button"
            :aria-pressed="marks[cat.key].includes(player.user_id)"
            class="flex items-center gap-2.5 rounded-xl px-3 py-[9px] text-left"
            :class="marks[cat.key].includes(player.user_id) ? 'border-2 ' + (cat.warnColor ? 'border-warn bg-warnBg' : 'border-brand bg-brandSoft') : 'border border-border'"
            @click="toggle(cat.key, player.user_id)"
          >
            <Avatar :name="player.name" :color="player.avatar_color" size="sm" />
            <span class="flex-1 text-sm" :class="marks[cat.key].includes(player.user_id) ? 'font-semibold' : 'font-medium'">
              {{ player.name }}
            </span>
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
      </Card>

      <div class="sticky bottom-[max(env(safe-area-inset-bottom),1rem)] z-10 flex flex-col gap-2">
        <p v-if="error" role="alert" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ error }}</p>
        <button
          type="button"
          :disabled="submitting"
          class="flex h-[50px] items-center justify-center gap-2 rounded-[13px] text-[15px] font-bold shadow-lg transition"
          :class="dirty ? 'bg-brand text-brandInk' : 'border border-border bg-surface text-ink3'"
          @click="submit"
        >
          <template v-if="submitting">Enviando…</template>
          <template v-else-if="dirty">Enviar votos</template>
          <template v-else-if="hasVotes">Votos registrados ✓</template>
          <template v-else>Marque seus votos acima</template>
        </button>
        <p v-if="savedToast" class="text-center text-[12.5px] font-semibold text-brand">Votos registrados ✓</p>
      </div>
    </template>
  </PublicShell>
</template>
