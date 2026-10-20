<script setup lang="ts">
// FUT-005: dashboard de votos do admin. Escolha a partida (com busca) e,
// opcionalmente, um jogador; indicadores, gráficos e a lista de votos
// mostram o recorte escolhido. Partida e jogador ficam na URL.
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import type { ConfirmationEntry, Match, MatchDetail, VoteCategory } from '../../api/types'
import { countdown, formatDMY, formatTimestamp, weekdayShort } from '../../lib/format'
import type { SearchOption } from '../../lib/search'
import {
  MATCH_STATUS_LABELS, MAX_VOTES_PER_CATEGORY, VOTE_CATEGORIES, VOTE_CATEGORY_LABELS,
  categoryBars, filterVotes, matchMatchesQuery, playerSummary,
  type FlatVote, type MatchVoteSummary, type ParticipationItem, type VoteTally,
} from '../../lib/votes'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import ParticipationChart from '../../components/admin/ParticipationChart.vue'
import VoteBarChart from '../../components/admin/VoteBarChart.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import SearchSelect from '../../components/ui/SearchSelect.vue'

const route = useRoute()
const router = useRouter()

const CATEGORY_ICONS: Record<VoteCategory, string> = { top_scorer: 'trophy', worst_player: '', best_goal: 'ball' }

const matches = ref<Match[]>([])
const summaries = ref<MatchVoteSummary[]>([])
const selectedMatchId = ref<string | null>(null)
const playerId = ref<string | null>(null)
const tally = ref<VoteTally[]>([])
const flat = ref<FlatVote[]>([])
const participants = ref<ConfirmationEntry[]>([])
const loading = ref(false)
const loaded = ref(false)
const error = ref('')

// lista de votos
const tableQuery = ref('')
const tableCategory = ref<VoteCategory | 'all'>('all')
const barFilter = ref<{ category: VoteCategory; id: string; name: string } | null>(null)

// histórico do jogador em todas as partidas
const historyOpen = ref(false)
const history = ref<FlatVote[] | null>(null)

onMounted(load)

async function load() {
  try {
    const [m, s] = await Promise.all([
      api.get<Match[]>('/matches'),
      api.get<{ matches: MatchVoteSummary[] }>('/admin/stats/votes'),
    ])
    // Só partidas que já chegaram à votação; a mais recente primeiro.
    matches.value = (m.data ?? [])
      .filter((x) => x.status === 'voting' || x.status === 'finished')
      .sort((a, b) => b.match_date.localeCompare(a.match_date))
    summaries.value = s.data?.matches ?? []
    const fromUrl = String(route.query.partida ?? '')
    selectedMatchId.value = matches.value.some((x) => x.id === fromUrl) ? fromUrl : (matches.value[0]?.id ?? null)
    playerId.value = typeof route.query.jogador === 'string' ? route.query.jogador : null
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loaded.value = true
  }
}

// Recarregar mantém o quadro anterior em opacidade reduzida, sem piscar.
async function loadMatch(id: string) {
  loading.value = true
  error.value = ''
  try {
    const [votes, detail] = await Promise.all([
      api.get<{ tally: VoteTally[]; flat: FlatVote[] }>(`/admin/matches/${id}/votes`),
      api.get<MatchDetail>(`/matches/${id}`),
    ])
    if (selectedMatchId.value !== id) return // trocou de partida no meio
    tally.value = votes.data.tally ?? []
    flat.value = votes.data.flat ?? []
    participants.value = (detail.data.confirmations ?? [])
      .filter((c) => c.response === 'going')
      .sort((a, b) => a.name.localeCompare(b.name, 'pt-BR'))
    if (playerId.value && !participants.value.some((p) => p.user_id === playerId.value)) playerId.value = null
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

watch(selectedMatchId, (id) => {
  barFilter.value = null
  if (id) void loadMatch(id)
})

watch(playerId, () => {
  barFilter.value = null
  historyOpen.value = false
  history.value = null
})

watch([selectedMatchId, playerId], ([partida, jogador]) => {
  if (!partida) return
  void router.replace({ query: jogador ? { partida, jogador } : { partida } })
})

const selectedMatch = computed(() => matches.value.find((m) => m.id === selectedMatchId.value) ?? null)
const summaryById = computed(() => new Map(summaries.value.map((s) => [s.match_id, s])))

// --- opções dos filtros ---

type MatchOption = SearchOption & { match: Match; voters: number }
const matchOptions = computed<MatchOption[]>(() =>
  matches.value.map((m) => ({
    value: m.id,
    label: `${formatDMY(m.match_date)} · ${m.venue}`,
    match: m,
    voters: summaryById.value.get(m.id)?.voters ?? 0,
  })),
)
const filterMatch = (o: MatchOption, q: string) => matchMatchesQuery(o.match, q)

type PlayerOption = SearchOption & { color: string }
const playerOptions = computed<PlayerOption[]>(() =>
  participants.value.map((p) => ({
    value: p.user_id,
    label: p.name,
    hint: voterIds.value.has(p.user_id) ? 'votou' : 'não votou',
    color: p.avatar_color,
  })),
)
const player = computed(() => participants.value.find((p) => p.user_id === playerId.value) ?? null)

// --- indicadores ---

const voterIds = computed(() => new Set(flat.value.map((v) => v.voter_id)))
const votersCount = computed(() => participants.value.filter((p) => voterIds.value.has(p.user_id)).length)
const participation = computed(() => (participants.value.length ? Math.round((votersCount.value / participants.value.length) * 100) : 0))
const possibleVotes = computed(() => participants.value.length * VOTE_CATEGORIES.length * MAX_VOTES_PER_CATEGORY)
const notVoted = computed(() => participants.value.filter((p) => !voterIds.value.has(p.user_id)))
const votingOpen = computed(() => selectedMatch.value?.status === 'voting')
// Encerrada pelo admin antes do prazo, voting_closes_at fica no futuro: aí
// não há data confiável para mostrar.
const closedAt = computed(() => {
  const at = selectedMatch.value?.voting_closes_at
  return at && new Date(at).getTime() <= Date.now() ? at : null
})
const closesIn = computed(() => (selectedMatch.value?.voting_closes_at ? countdown(selectedMatch.value.voting_closes_at) : null))

// --- gráficos ---

const charts = computed(() =>
  VOTE_CATEGORIES.map((category) => {
    const { total, bars } = categoryBars(tally.value, flat.value, category)
    return { category, total, bars, leaders: bars.filter((b) => b.winner) }
  }),
)
const summary = computed(() => (playerId.value ? playerSummary(flat.value, playerId.value) : null))
const highlightFor = (category: VoteCategory) => summary.value?.given[category].map((v) => v.candidate_id) ?? null

function selectBar(category: VoteCategory, id: string, name: string) {
  const same = barFilter.value?.category === category && barFilter.value.id === id
  barFilter.value = same ? null : { category, id, name }
}

// --- lista de votos ---

const colorById = computed(() => {
  const map = new Map<string, string>()
  for (const p of participants.value) map.set(p.user_id, p.avatar_color)
  for (const t of tally.value) map.set(t.candidate_id, t.avatar_color)
  return map
})
const shownVotes = computed(() =>
  filterVotes(flat.value, {
    query: tableQuery.value,
    category: barFilter.value?.category ?? tableCategory.value,
    playerId: playerId.value ?? undefined,
    candidateId: barFilter.value?.id,
  }).sort((a, b) => b.created_at.localeCompare(a.created_at)),
)

// --- participação entre partidas ---

const participationItems = computed<ParticipationItem[]>(() =>
  matches.value
    .slice(0, 10)
    .reverse()
    .map((m) => ({
      id: m.id,
      label: formatDMY(m.match_date).slice(0, 5),
      venue: m.venue,
      voters: summaryById.value.get(m.id)?.voters ?? 0,
      participants: m.going_count,
    })),
)

async function toggleHistory() {
  historyOpen.value = !historyOpen.value
  if (!historyOpen.value || history.value || !playerId.value) return
  try {
    const { data } = await api.get<{ votes: FlatVote[] }>(`/admin/users/${playerId.value}/votes`)
    history.value = data.votes ?? []
  } catch (e) {
    error.value = errorMessage(e)
  }
}

const votesLabel = (n: number) => `${n} ${n === 1 ? 'voto' : 'votos'}`
const names = (list: { name: string }[]) => list.map((x) => x.name).join(', ')
</script>

<template>
  <AdminLayout>
    <template #title>Votos</template>
    <template #subtitle>Apuração, participação e votos individuais por partida</template>

    <div class="flex flex-col gap-4">
      <!-- Filtros: valem para tudo abaixo -->
      <div class="grid gap-2.5 md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <SearchSelect
          v-model="selectedMatchId"
          :options="matchOptions"
          :filter="filterMatch"
          label="Partida"
          placeholder="Escolha a partida"
          search-placeholder="Buscar por data (18/10), local ou status"
          empty-text="Nenhuma partida encontrada."
        >
          <template #selected="{ option }">
            <NavIcon name="ball" :size="17" class="shrink-0 text-ink3" />
            <span class="min-w-0 truncate">
              <span class="font-bold">{{ weekdayShort(option.match.match_date) }}, {{ formatDMY(option.match.match_date) }}</span>
              <span class="text-ink2"> · {{ option.match.venue }}</span>
            </span>
            <Badge :tone="option.match.status === 'voting' ? 'info' : 'neutral'" class="ml-auto hidden sm:inline-flex">
              {{ MATCH_STATUS_LABELS[option.match.status] }}
            </Badge>
          </template>
          <template #option="{ option }">
            <span class="w-[86px] shrink-0 font-bold tabular-nums">{{ formatDMY(option.match.match_date) }}</span>
            <span class="min-w-0 truncate text-ink2">{{ option.match.venue }}</span>
            <span class="ml-auto shrink-0 text-[12px] text-ink3">{{ option.voters }} {{ option.voters === 1 ? 'votante' : 'votantes' }}</span>
            <Badge :tone="option.match.status === 'voting' ? 'info' : 'neutral'" class="shrink-0">
              {{ option.match.status === 'voting' ? 'Aberta' : 'Encerrada' }}
            </Badge>
          </template>
        </SearchSelect>

        <SearchSelect
          v-model="playerId"
          :options="playerOptions"
          clearable
          label="Jogador"
          placeholder="Todos os jogadores"
          search-placeholder="Buscar jogador da partida"
          empty-text="Ninguém com esse nome nesta partida."
        >
          <template #selected="{ option }">
            <Avatar :name="option.label" :color="option.color" size="xs" />
            <span class="truncate font-semibold">{{ option.label }}</span>
          </template>
          <template #option="{ option }">
            <Avatar :name="option.label" :color="option.color" size="xs" />
            <span class="truncate font-medium">{{ option.label }}</span>
            <span class="ml-auto shrink-0 text-[12px]" :class="option.hint === 'votou' ? 'text-ink3' : 'font-semibold text-warn'">{{ option.hint }}</span>
          </template>
        </SearchSelect>
      </div>

      <p v-if="error" class="text-[13px] font-semibold text-danger">{{ error }}</p>

      <Card v-if="loaded && !matches.length">
        <EmptyState>Nenhuma partida chegou à votação ainda.</EmptyState>
      </Card>

      <div v-else-if="selectedMatch" class="flex flex-col gap-4 transition-opacity" :class="loading ? 'opacity-60' : ''">
        <!-- Indicadores -->
        <div class="grid grid-cols-2 gap-3 xl:grid-cols-4">
          <Card class="px-[18px] py-4">
            <div class="text-[12px] font-semibold text-ink3">Participação</div>
            <div class="font-condensed text-[38px] font-bold leading-[1.05]">{{ participation }}%</div>
            <div class="mt-1.5 h-[6px] overflow-hidden rounded-full bg-brandSoft">
              <div class="h-full rounded-full bg-brand transition-all" :style="{ width: `${participation}%` }" />
            </div>
            <div class="mt-1.5 text-[12px] text-ink2">{{ votersCount }} de {{ participants.length }} participantes votaram</div>
          </Card>
          <Card class="px-[18px] py-4">
            <div class="text-[12px] font-semibold text-ink3">Votos registrados</div>
            <div class="font-condensed text-[38px] font-bold leading-[1.05]">{{ flat.length }}</div>
            <div class="mt-1 text-[12px] text-ink2">de {{ possibleVotes }} possíveis ({{ VOTE_CATEGORIES.length }} categorias × {{ MAX_VOTES_PER_CATEGORY }})</div>
          </Card>
          <Card class="px-[18px] py-4">
            <div class="text-[12px] font-semibold text-ink3">Votação</div>
            <div class="mt-1.5">
              <Badge :tone="votingOpen ? 'info' : 'neutral'" dot>{{ votingOpen ? 'Aberta' : 'Encerrada' }}</Badge>
            </div>
            <div v-if="votingOpen" class="mt-2 text-[12px] text-ink2">
              <template v-if="closesIn">Fecha em <strong class="text-ink">{{ closesIn }}</strong></template>
              <template v-else>Fechando agora</template>
              <span class="block text-ink3">Resultado parcial: os jogadores ainda não veem</span>
            </div>
            <div v-else-if="closedAt" class="mt-2 text-[12px] text-ink2">Encerrada em {{ formatTimestamp(closedAt) }}</div>
          </Card>
          <Card class="px-[18px] py-4" data-test="not-voted">
            <div class="text-[12px] font-semibold text-ink3">Faltam votar</div>
            <div class="font-condensed text-[38px] font-bold leading-[1.05]">{{ notVoted.length }}</div>
            <div v-if="notVoted.length" class="mt-1 flex items-center gap-2">
              <div class="flex shrink-0 -space-x-1.5">
                <Avatar v-for="p in notVoted.slice(0, 5)" :key="p.user_id" :name="p.name" :color="p.avatar_color" size="xs" ring />
              </div>
              <span class="truncate text-[12px] text-ink2" :title="names(notVoted)">{{ names(notVoted) }}</span>
            </div>
            <div v-else class="mt-1 text-[12px] text-ink2">Todo mundo votou</div>
          </Card>
        </div>

        <!-- Jogador escolhido -->
        <Card v-if="player && summary" class="p-4" data-test="player-panel">
          <div class="flex flex-wrap items-center gap-3">
            <Avatar :name="player.name" :color="player.avatar_color" size="md" />
            <div class="min-w-0">
              <div class="truncate text-[15px] font-bold">{{ player.name }}</div>
              <div class="text-[12px] text-ink2">
                {{ voterIds.has(player.user_id) ? 'Votou nesta partida' : 'Ainda não votou nesta partida' }}
                · recebeu {{ votesLabel(VOTE_CATEGORIES.reduce((n, c) => n + summary!.received[c].length, 0)) }}
              </div>
            </div>
            <button type="button" class="ml-auto rounded-lg border border-border px-3 py-1.5 text-[12.5px] font-semibold text-ink2 hover:bg-surface2" @click="toggleHistory">
              {{ historyOpen ? 'Fechar histórico' : 'Histórico em todas as partidas' }}
            </button>
          </div>
          <div class="mt-3 grid gap-2 sm:grid-cols-3">
            <div v-for="c in VOTE_CATEGORIES" :key="c" class="rounded-xl bg-surface2 px-3 py-2.5">
              <div class="text-[11px] font-bold uppercase tracking-[.08em] text-ink3">{{ VOTE_CATEGORY_LABELS[c] }}</div>
              <div class="mt-1 text-[12.5px]">
                <span class="text-ink3">Votou em </span>
                <strong v-if="summary.given[c].length">{{ summary.given[c].map((x) => x.candidate_name).join(', ') }}</strong>
                <span v-else class="text-ink3">ninguém</span>
              </div>
              <div class="text-[12.5px]">
                <span class="text-ink3">Recebeu </span>
                <strong>{{ votesLabel(summary.received[c].length) }}</strong>
                <span v-if="summary.received[c].length" class="text-ink2"> de {{ summary.received[c].map((x) => x.voter_name).join(', ') }}</span>
              </div>
            </div>
          </div>
          <div v-if="historyOpen" class="mt-3 border-t border-border pt-3">
            <p v-if="!history" class="text-[12.5px] text-ink3">Carregando…</p>
            <p v-else-if="!history.length" class="text-[12.5px] text-ink3">{{ player.name }} nunca votou.</p>
            <ul v-else class="flex max-h-[240px] flex-col gap-1 overflow-y-auto">
              <li v-for="(h, i) in history" :key="i" class="flex items-center gap-2 text-[12.5px]">
                <span class="w-[92px] shrink-0 tabular-nums text-ink3">{{ formatTimestamp(h.created_at) }}</span>
                <Badge class="shrink-0">{{ VOTE_CATEGORY_LABELS[h.category] }}</Badge>
                <span class="truncate">{{ h.candidate_name }}</span>
              </li>
            </ul>
          </div>
        </Card>

        <!-- Apuração por categoria -->
        <div class="grid gap-3 lg:grid-cols-3">
          <Card v-for="chart in charts" :key="chart.category" class="flex flex-col p-4" :data-test="`chart-${chart.category}`">
            <div class="flex items-center gap-2">
              <NavIcon v-if="CATEGORY_ICONS[chart.category]" :name="CATEGORY_ICONS[chart.category]" :size="16" class="text-ink2" />
              <span v-else class="text-[15px] leading-none">🥴</span>
              <span class="text-[14px] font-bold">{{ VOTE_CATEGORY_LABELS[chart.category] }}</span>
              <span class="ml-auto text-[12px] text-ink3">{{ votesLabel(chart.total) }}</span>
            </div>
            <div class="mb-2 mt-1 min-h-[18px] text-[12px] text-ink2">
              <template v-if="chart.leaders.length">
                {{ votingOpen ? 'Liderando' : chart.leaders.length > 1 ? 'Empate' : 'Vencedor' }}:
                <strong class="text-ink">{{ names(chart.leaders) }}</strong> · {{ votesLabel(chart.leaders[0].votes) }}
              </template>
              <template v-else>Sem votos ainda</template>
              <span v-if="player" class="block text-ink3">Em verde: em quem {{ player.name }} votou</span>
            </div>
            <VoteBarChart
              v-if="chart.bars.length"
              :bars="chart.bars"
              :highlight="playerId ? highlightFor(chart.category) : null"
              :selected-id="barFilter?.category === chart.category ? barFilter.id : null"
              @select="(id) => selectBar(chart.category, id, chart.bars.find((b) => b.id === id)!.name)"
            />
            <p v-else class="py-6 text-center text-[12.5px] text-ink3">Ninguém votou nesta categoria.</p>
          </Card>
        </div>

        <!-- Votos individuais -->
        <Card class="p-4">
          <div class="flex flex-wrap items-center gap-2">
            <div class="text-[14px] font-bold">Votos individuais</div>
            <span class="text-[12px] text-ink3">{{ shownVotes.length }} de {{ flat.length }}</span>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <label class="flex h-10 min-w-[200px] flex-1 items-center gap-2 rounded-xl border border-border bg-surface px-3 focus-within:border-brand">
              <NavIcon name="search" :size="15" class="shrink-0 text-ink3" />
              <input v-model="tableQuery" aria-label="Buscar nos votos" placeholder="Buscar eleitor ou votado" class="min-w-0 flex-1 bg-transparent text-[13px] outline-none placeholder:text-ink3" />
            </label>
            <div class="flex flex-wrap gap-1.5" role="group" aria-label="Categoria">
              <button
                v-for="c in (['all', ...VOTE_CATEGORIES] as const)"
                :key="c"
                type="button"
                class="rounded-full border px-3 py-1.5 text-[12px] font-semibold transition"
                :class="(barFilter?.category ?? tableCategory) === c ? 'border-brand bg-brandSoft text-brand' : 'border-border text-ink2 hover:bg-surface2'"
                :aria-pressed="(barFilter?.category ?? tableCategory) === c"
                @click="barFilter = null; tableCategory = c"
              >
                {{ c === 'all' ? 'Todas' : VOTE_CATEGORY_LABELS[c] }}
              </button>
            </div>
          </div>
          <div v-if="barFilter || player" class="mt-2 flex flex-wrap gap-1.5">
            <button v-if="barFilter" type="button" class="inline-flex items-center gap-1.5 rounded-full bg-surface2 px-2.5 py-1 text-[12px] font-semibold text-ink2 hover:text-ink" @click="barFilter = null">
              {{ VOTE_CATEGORY_LABELS[barFilter.category] }}: votos em {{ barFilter.name }}
              <NavIcon name="x" :size="12" :stroke-width="2.4" />
            </button>
            <button v-if="player" type="button" class="inline-flex items-center gap-1.5 rounded-full bg-surface2 px-2.5 py-1 text-[12px] font-semibold text-ink2 hover:text-ink" @click="playerId = null">
              Dados ou recebidos por {{ player.name }}
              <NavIcon name="x" :size="12" :stroke-width="2.4" />
            </button>
          </div>

          <div class="mt-3 max-h-[440px] overflow-y-auto">
            <table class="w-full table-fixed text-[12.5px]">
              <thead class="sticky top-0 bg-surface text-left text-[11px] uppercase tracking-[.08em] text-ink3">
                <tr>
                  <th class="w-1/2 py-2 pr-2 font-semibold sm:w-[34%]">Eleitor</th>
                  <th class="w-1/2 py-2 pr-2 font-semibold sm:w-[34%]">Votou em</th>
                  <th class="hidden py-2 pr-2 font-semibold sm:table-cell sm:w-[18%]">Categoria</th>
                  <th class="hidden py-2 text-right font-semibold sm:table-cell sm:w-[14%]">Quando</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, i) in shownVotes" :key="i" class="border-t border-border" data-test="vote-row">
                  <td class="py-2 pr-2">
                    <span class="flex min-w-0 items-center gap-2">
                      <Avatar :name="row.voter_name" :color="colorById.get(row.voter_id) ?? '#8ca094'" size="xs" />
                      <span class="truncate font-semibold">{{ row.voter_name }}</span>
                    </span>
                  </td>
                  <td class="py-2 pr-2">
                    <span class="flex min-w-0 items-center gap-2">
                      <Avatar :name="row.candidate_name" :color="colorById.get(row.candidate_id) ?? '#8ca094'" size="xs" />
                      <span class="min-w-0">
                        <span class="block truncate">{{ row.candidate_name }}</span>
                        <span class="block truncate text-[11px] text-ink3 sm:hidden">{{ VOTE_CATEGORY_LABELS[row.category] }}</span>
                      </span>
                    </span>
                  </td>
                  <td class="hidden py-2 pr-2 sm:table-cell"><Badge>{{ VOTE_CATEGORY_LABELS[row.category] }}</Badge></td>
                  <td class="hidden py-2 text-right tabular-nums text-ink3 sm:table-cell">{{ formatTimestamp(row.created_at) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-if="!shownVotes.length" class="py-6 text-center text-[12.5px] text-ink3">
              {{ flat.length ? 'Nenhum voto com esses filtros.' : 'Ninguém votou nesta partida ainda.' }}
            </p>
          </div>
        </Card>

        <!-- Comparativo entre partidas -->
        <Card v-if="participationItems.length > 1" class="p-4">
          <ParticipationChart :items="participationItems" :selected-id="selectedMatchId" @select="(id) => (selectedMatchId = id)" />
        </Card>
      </div>
    </div>
  </AdminLayout>
</template>
