<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Match, VoteCategory } from '../../api/types'
import { formatDMY, formatTimestamp } from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

// FUT-005: dashboard de votação — tally por partida, votos individuais com
// busca por usuário, histórico do usuário e comparativo entre partidas.

interface VoteTally {
  category: VoteCategory
  candidate_id: string
  name: string
  avatar_color: string
  votes: number
}
interface FlatVote {
  voter_id: string
  voter_name: string
  category: VoteCategory
  candidate_id: string
  candidate_name: string
  created_at: string
}
interface MatchVoteSummary {
  match_id: string
  match_date: string
  status: string
  voters: number
  votes: number
}
interface VoteUser {
  id: string
  name: string
  avatar_color: string
  role: string
  status: string
}

const CAT_LABELS: Record<VoteCategory, string> = {
  top_scorer: 'Artilheiro',
  worst_player: 'Perna de pau',
  best_goal: 'Melhor gol',
}

const matches = ref<Match[]>([])
const selectedMatchId = ref('')
const tally = ref<VoteTally[]>([])
const flat = ref<FlatVote[]>([])
const summaries = ref<MatchVoteSummary[]>([])
const users = ref<VoteUser[]>([])
const userSearch = ref('')
const selectedUserId = ref('')
const userHistory = ref<FlatVote[]>([])
const matchSearch = ref('')
const error = ref('')

onMounted(load)

async function load() {
  try {
    const [m, s, u] = await Promise.all([
      api.get<{ matches: Match[] }>('/matches'),
      api.get<{ matches: MatchVoteSummary[] }>('/admin/stats/votes'),
      api.get<{ users: VoteUser[] }>('/admin/users'),
    ])
    matches.value = m.data.matches ?? []
    summaries.value = (s.data as { matches?: MatchVoteSummary[] }).matches ?? []
    users.value = (u.data as { users?: VoteUser[] }).users ?? []
    if (matches.value.length && !selectedMatchId.value) await selectMatch(matches.value[0].id)
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function selectMatch(id: string) {
  selectedMatchId.value = id
  tally.value = []
  flat.value = []
  try {
    const { data } = await api.get<{ tally: VoteTally[]; flat: FlatVote[] }>(`/admin/matches/${id}/votes`)
    tally.value = data.tally ?? []
    flat.value = data.flat ?? []
  } catch (e) {
    error.value = errorMessage(e)
  }
}

async function selectUser(id: string) {
  selectedUserId.value = id
  userHistory.value = []
  try {
    const { data } = await api.get<{ votes: FlatVote[] }>(`/admin/users/${id}/votes`)
    userHistory.value = data.votes ?? []
  } catch (e) {
    error.value = errorMessage(e)
  }
}

function tallyFor(cat: VoteCategory): VoteTally[] {
  return tally.value.filter((t) => t.category === cat)
}
function filteredMatches(): Match[] {
  const q = matchSearch.value.trim().toLowerCase()
  return matches.value.filter((m) => !q || m.match_date.includes(q) || (m.venue ?? '').toLowerCase().includes(q))
}
function filteredUsers(): VoteUser[] {
  const q = userSearch.value.trim().toLowerCase()
  return users.value.filter((u) => !q || u.name.toLowerCase().includes(q))
}

const maxVotes = (rows: VoteTally[]): number => Math.max(1, ...rows.map((r) => r.votes))
</script>

<template>
  <AdminLayout>
    <div class="flex flex-col gap-3 lg:gap-4">
      <header>
        <h1 class="text-[19px] font-bold lg:text-[22px]">Votos</h1>
        <span class="text-[12px] text-ink3">Tally por partida, votos individuais e comparativo</span>
      </header>

      <p v-if="error" class="text-[13px] font-semibold text-danger">{{ error }}</p>

      <!-- Comparativo -->
      <Card class="p-4">
        <SectionLabel>Participação por partida</SectionLabel>
        <div v-if="summaries.length" class="mt-2 flex flex-col gap-[6px]">
          <div v-for="s in summaries.slice(0, 8)" :key="s.match_id" class="flex items-center gap-2 text-[12.5px]">
            <button
              type="button"
              class="w-[70px] shrink-0 text-left font-semibold hover:underline"
              @click="selectMatch(s.match_id)"
            >{{ s.match_date }}</button>
            <div class="h-[16px] flex-1 overflow-hidden rounded-md bg-surface">
              <div class="h-full rounded-md bg-brand/70" :style="{ width: Math.min(100, s.voters * 8) + '%' }" />
            </div>
            <span class="shrink-0 text-ink3">{{ s.voters }} votantes · {{ s.votes }} votos</span>
          </div>
        </div>
        <p v-else class="mt-2 text-sm text-ink3">Sem partidas com votos.</p>
      </Card>

      <!-- Tally por partida -->
      <Card class="p-4">
        <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
          <SectionLabel>Votos por partida</SectionLabel>
          <select
            :value="selectedMatchId"
            class="rounded-xl border border-border bg-surface px-3 py-[7px] text-[13px]"
            @change="selectMatch(($event.target as HTMLSelectElement).value)"
          >
            <option v-for="m in filteredMatches()" :key="m.id" :value="m.id">{{ formatDMY(m.match_date) }} · {{ m.venue }}</option>
          </select>
        </div>

        <div v-if="tally.length" class="flex flex-col gap-3">
          <div v-for="cat in (Object.keys(CAT_LABELS) as VoteCategory[])" :key="cat">
            <div class="mb-1 flex items-center gap-1.5 text-[12.5px] font-bold">
              <NavIcon v-if="cat !== 'worst_player'" :name="cat === 'best_goal' ? 'ball' : 'trophy'" :size="14" />
              <template v-else>🥴</template>
              {{ CAT_LABELS[cat] }}
            </div>
            <div class="flex flex-col gap-[5px]">
              <div v-for="row in tallyFor(cat)" :key="row.candidate_id" class="flex items-center gap-2 text-[12.5px]">
                <Avatar :name="row.name" :color="row.avatar_color" size="xs" />
                <span class="w-[110px] shrink-0 truncate font-medium">{{ row.name }}</span>
                <div class="flex-1">
                  <div class="h-[12px] overflow-hidden rounded-sm bg-surface">
                    <div class="h-full bg-brand/60" :style="{ width: (row.votes / maxVotes(tallyFor(cat))) * 100 + '%' }" />
                  </div>
                </div>
                <span class="w-[26px] shrink-0 text-right font-bold">{{ row.votes }}</span>
              </div>
              <p v-if="tallyFor(cat).length === 0" class="text-[12px] text-ink3">Sem votos.</p>
            </div>
          </div>
        </div>
        <p v-else class="text-sm text-ink3">Selecione uma partida.</p>

        <!-- Votos individuais -->
        <details class="mt-3">
          <summary class="cursor-pointer text-[12.5px] font-semibold text-brand">Ver votos individuais ({{ flat.length }})</summary>
          <div class="mt-2 flex flex-col gap-[4px]">
            <div v-for="(v, i) in flat" :key="i" class="flex items-center gap-2 rounded-lg bg-surface px-2.5 py-[6px] text-[12.5px]">
              <span class="w-[110px] shrink-0 truncate font-semibold">{{ v.voter_name }}</span>
              <span class="text-ink3">→</span>
              <span class="w-[110px] shrink-0 truncate">{{ v.candidate_name }}</span>
              <span class="rounded-md bg-brandSoft px-1.5 py-px text-[10.5px] font-bold text-brand">{{ CAT_LABELS[v.category as VoteCategory] }}</span>
            </div>
            <p v-if="flat.length === 0" class="text-[12px] text-ink3">Nenhum voto nesta partida.</p>
          </div>
        </details>
      </Card>

      <!-- Por usuário -->
      <Card class="p-4">
        <SectionLabel>Histórico por usuário</SectionLabel>
        <div class="mt-2 flex flex-col gap-2 lg:flex-row lg:items-start">
          <div class="flex-1">
            <input
              v-model="userSearch"
              placeholder="Buscar usuário…"
              class="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm"
            />
            <div class="mt-1 flex flex-col gap-1">
              <button
                v-for="u in filteredUsers().slice(0, 6)"
                :key="u.id"
                type="button"
                class="flex items-center gap-2 rounded-lg px-2 py-[6px] text-left text-[12.5px]"
                :class="u.id === selectedUserId ? 'bg-brandSoft font-semibold' : 'hover:bg-surface'"
                @click="selectUser(u.id)"
              >
                <Avatar :name="u.name" :color="u.avatar_color" size="xs" />
                {{ u.name }}
              </button>
            </div>
          </div>
          <div class="flex-1">
            <p class="mb-1 text-[12px] font-semibold text-ink3">Votos ({{ userHistory.length }})</p>
            <div class="flex max-h-[280px] flex-col gap-[4px] overflow-y-auto">
              <div v-for="(v, i) in userHistory" :key="i" class="flex items-center gap-2 rounded-lg bg-surface px-2.5 py-[6px] text-[12.5px]">
                <span class="text-ink3">{{ formatTimestamp(v.created_at) }}</span>
                <span class="rounded-md bg-brandSoft px-1.5 py-px text-[10.5px] font-bold text-brand">{{ CAT_LABELS[v.category as VoteCategory] }}</span>
                <span class="truncate">→ {{ v.candidate_name }}</span>
              </div>
              <p v-if="selectedUserId && userHistory.length === 0" class="text-[12px] text-ink3">Este usuário nunca votou.</p>
              <p v-if="!selectedUserId" class="text-[12px] text-ink3">Selecione um usuário.</p>
            </div>
          </div>
        </div>
      </Card>
    </div>
  </AdminLayout>
</template>
