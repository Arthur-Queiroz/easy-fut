<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../../api/client'
import type { Match, VoteResult } from '../../api/types'
import { dayOfMonth, monthShort } from '../../lib/format'
import MobileShell from '../../components/layout/MobileShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import MatchDetailContent from '../../components/MatchDetailContent.vue'

const route = useRoute()
const router = useRouter()

const matches = ref<Match[]>([])
const filter = ref('')
const selectedMatchId = ref<string>(route.query.detalhe as string ?? '')

onMounted(load)
watch(filter, load)
watch(selectedMatchId, (id) => {
  if (id) {
    router.replace({ query: { ...route.query, detalhe: id } })
  } else {
    const { detalhe, ...rest } = route.query
    router.replace({ query: rest })
  }
})

async function load() {
  const { data } = await api.get<Match[]>('/matches', { params: filter.value ? { month: filter.value } : {} })
  matches.value = data ?? []
  if (selectedMatchId.value && !matches.value.find((m) => m.id === selectedMatchId.value)) {
    selectedMatchId.value = ''
  }
}

function setFilter(month: string) {
  filter.value = month
}

function selectMatch(match: Match) {
  selectedMatchId.value = match.id
}

// Chips: "Todas" + últimos 3 meses.
const monthChips = computed(() => {
  const chips: { value: string; label: string }[] = []
  const now = new Date()
  for (let i = 0; i < 3; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    const value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    const label = d.toLocaleDateString('pt-BR', { month: 'long' })
    chips.push({ value, label: label[0].toUpperCase() + label.slice(1) })
  }
  return chips
})

function winners(match: Match, category: string): string {
  const result = match.results?.find((r: VoteResult) => r.category === category)
  if (!result || result.winners.length === 0) return ''
  return result.winners.map((w) => w.name.split(' ')[0]).join(', ')
}

function statusBadge(match: Match) {
  switch (match.status) {
    case 'voting':
      return { cls: 'bg-infoBg text-info', label: 'Votação aberta' }
    case 'cancelled':
      return { cls: 'bg-dangerBg text-danger', label: match.cancel_reason ? `Cancelada · ${match.cancel_reason}` : 'Cancelada' }
    case 'finished':
      return { cls: 'bg-surface2 text-ink2', label: 'Finalizada' }
    case 'open':
      return { cls: 'bg-brandSoft text-brand', label: 'Confirmação aberta' }
    default:
      return { cls: 'bg-surface2 text-ink2', label: 'Agendada' }
  }
}
</script>

<template>
  <MobileShell>
    <template #header>
      <div class="text-[22px] font-bold">Histórico</div>
      <div class="text-[13.5px] text-white/65">Todas as partidas do grupo</div>
    </template>

    <div class="flex flex-col gap-3 px-4 pt-3.5 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden items-center justify-between lg:flex">
        <div>
          <div class="text-2xl font-extrabold">Histórico</div>
          <div class="mt-1 text-[14px] text-ink2">{{ matches.length }} partidas registradas</div>
        </div>
        <div class="flex gap-2">
          <button
            type="button"
            class="whitespace-nowrap rounded-full px-4 py-[7px] text-[12.5px]"
            :class="filter === '' ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
            @click="setFilter('')"
          >
            Todas
          </button>
          <button
            v-for="chip in monthChips"
            :key="chip.value"
            type="button"
            class="whitespace-nowrap rounded-full px-4 py-[7px] text-[12.5px]"
            :class="filter === chip.value ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
            @click="setFilter(chip.value)"
          >
            {{ chip.label }}
          </button>
        </div>
      </div>

      <div class="grid gap-3 lg:grid-cols-[1fr_1.15fr] lg:items-start">
        <!-- Lista -->
        <div class="flex flex-col gap-3">
          <div class="flex gap-2 overflow-x-auto pb-1 lg:hidden">
            <button
              type="button"
              class="whitespace-nowrap rounded-full px-4 py-[7px] text-[12.5px]"
              :class="filter === '' ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
              @click="setFilter('')"
            >
              Todas
            </button>
            <button
              v-for="chip in monthChips"
              :key="chip.value"
              type="button"
              class="whitespace-nowrap rounded-full px-4 py-[7px] text-[12.5px]"
              :class="filter === chip.value ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
              @click="setFilter(chip.value)"
            >
              {{ chip.label }}
            </button>
          </div>

          <div
            v-for="match in matches"
            :key="match.id"
            class="cursor-pointer text-left"
            @click="selectMatch(match)"
          >
            <Card
              class="px-4 py-3.5 transition-colors"
              :class="selectedMatchId === match.id ? 'border-2 border-brand' : ''"
            >
              <div class="flex items-center gap-3.5">
                <div class="w-[52px] shrink-0 rounded-[11px] border border-border bg-surface2 py-1.5 text-center">
                  <div class="font-condensed text-[22px] font-bold leading-none">{{ dayOfMonth(match.match_date) }}</div>
                  <div class="text-[9.5px] font-medium text-ink3">{{ monthShort(match.match_date) }}</div>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[15px] font-semibold">{{ match.venue }}</div>
                  <div class="text-[12.5px] text-ink2">
                    {{ match.going_count }} jogadores<template v-if="match.media_count"> · {{ match.media_count }} mídias</template>
                  </div>
                </div>
                <span class="inline-flex whitespace-nowrap rounded-full px-2.5 py-[3px] text-[11px] font-semibold" :class="statusBadge(match).cls">
                  {{ statusBadge(match).label }}
                </span>
              </div>
              <div v-if="match.status === 'finished' && (winners(match, 'top_scorer') || winners(match, 'worst_player'))" class="mt-[11px] flex flex-wrap gap-2">
                <span
                  v-if="winners(match, 'top_scorer')"
                  class="inline-flex items-center gap-1.5 rounded-[9px] bg-brandSoft px-[11px] py-[5px] text-xs font-semibold text-brand"
                >
                  <NavIcon name="trophy" :size="13" />
                  Artilheiro · {{ winners(match, 'top_scorer') }}
                </span>
                <span
                  v-if="winners(match, 'worst_player')"
                  class="inline-flex items-center gap-1.5 rounded-[9px] bg-warnBg px-[11px] py-[5px] text-xs font-semibold text-warn"
                >
                  🥴 Perna de pau · {{ winners(match, 'worst_player') }}
                </span>
              </div>
              <div v-else-if="match.status === 'voting'" class="mt-[11px]">
                <span class="inline-flex items-center gap-1.5 rounded-[9px] bg-surface2 px-[11px] py-[5px] text-xs font-medium text-ink2">
                  <NavIcon name="trophy" :size="13" />
                  Destaques em votação
                </span>
              </div>
            </Card>
          </div>

          <EmptyState v-if="matches.length === 0">Nenhuma partida encontrada nesse período.</EmptyState>
        </div>

        <!-- Detalhe desktop -->
        <div class="hidden lg:block">
          <Card v-if="selectedMatchId" class="p-5">
            <MatchDetailContent :match-id="selectedMatchId" />
          </Card>
          <Card v-else class="flex h-full min-h-[400px] flex-col items-center justify-center p-5 text-center">
            <div class="text-ink3">Selecione uma partida ao lado para ver detalhes, votar e fotos.</div>
          </Card>
        </div>
      </div>
    </div>
  </MobileShell>
</template>
