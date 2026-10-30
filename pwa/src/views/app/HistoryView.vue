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
const selectedMatchId = ref(typeof route.query.detalhe === 'string' ? route.query.detalhe : '')

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

function onCardClick(event: MouseEvent, match: Match) {
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  if (window.matchMedia('(min-width: 1280px)').matches) {
    event.preventDefault()
    selectedMatchId.value = match.id
  }
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
  <MobileShell wide>
    <template #header>
      <div class="text-[22px] font-bold">Histórico</div>
      <div class="text-[13.5px] text-white/65">Todas as partidas do grupo</div>
    </template>

    <div class="flex flex-col gap-3 px-4 pt-3.5 lg:min-h-[calc(100dvh-3.5rem)] lg:justify-center lg:gap-5 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden items-center justify-between lg:flex">
        <div>
          <div class="text-[30px] font-extrabold leading-tight">Histórico</div>
          <div class="mt-1 text-[15px] text-ink2">{{ matches.length }} partidas registradas</div>
        </div>
        <div class="flex gap-2">
          <button
            type="button"
            class="whitespace-nowrap rounded-full px-4 py-2 text-[13px]"
            :class="filter === '' ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
            @click="setFilter('')"
          >
            Todas
          </button>
          <button
            v-for="chip in monthChips"
            :key="chip.value"
            type="button"
            class="whitespace-nowrap rounded-full px-4 py-2 text-[13px]"
            :class="filter === chip.value ? 'bg-ink font-semibold text-canvas' : 'border border-border bg-surface font-medium text-ink2'"
            @click="setFilter(chip.value)"
          >
            {{ chip.label }}
          </button>
        </div>
      </div>

      <div class="grid min-w-0 gap-3 lg:gap-5 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)] xl:items-start">
        <!-- Lista -->
        <div class="flex min-w-0 flex-col gap-3 lg:gap-4">
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

          <router-link
            v-for="match in matches"
            :key="match.id"
            :to="{ name: 'match-detail', params: { matchId: match.id } }"
            class="block rounded-2xl text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand"
            @click="onCardClick($event, match)"
          >
            <Card
              class="px-4 py-3.5 transition-colors hover:border-brand lg:p-5"
              :class="selectedMatchId === match.id ? 'border-2 border-brand' : ''"
            >
              <div class="flex items-center gap-3.5 lg:gap-4">
                <div class="w-[52px] shrink-0 rounded-[11px] border border-border bg-surface2 py-1.5 text-center lg:w-16 lg:py-2.5">
                  <div class="font-condensed text-[22px] font-bold leading-none lg:text-[27px]">{{ dayOfMonth(match.match_date) }}</div>
                  <div class="text-[9.5px] font-medium text-ink3 lg:text-[11px]">{{ monthShort(match.match_date) }}</div>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[15px] font-semibold lg:text-[16px]">{{ match.venue }}</div>
                  <div class="text-[12.5px] text-ink2 lg:text-[14px]">
                    {{ match.going_count }} jogadores<template v-if="match.media_count"> · {{ match.media_count }} mídias</template>
                  </div>
                </div>
                <span class="inline-flex whitespace-nowrap rounded-full px-2.5 py-[3px] text-[11px] font-semibold lg:px-3 lg:py-1 lg:text-[12px]" :class="statusBadge(match).cls">
                  {{ statusBadge(match).label }}
                </span>
              </div>
              <div v-if="match.status === 'finished' && (winners(match, 'top_scorer') || winners(match, 'worst_player'))" class="mt-[11px] flex flex-wrap gap-2 lg:mt-4">
                <span
                  v-if="winners(match, 'top_scorer')"
                  class="inline-flex items-center gap-1.5 rounded-[9px] bg-brandSoft px-[11px] py-[5px] text-xs font-semibold text-brand lg:text-[13px]"
                >
                  <NavIcon name="trophy" :size="13" />
                  Artilheiro · {{ winners(match, 'top_scorer') }}
                </span>
                <span
                  v-if="winners(match, 'worst_player')"
                  class="inline-flex items-center gap-1.5 rounded-[9px] bg-warnBg px-[11px] py-[5px] text-xs font-semibold text-warn lg:text-[13px]"
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
              <div class="mt-3 flex items-center justify-between border-t border-border pt-2.5 text-[12.5px] font-semibold text-brand lg:mt-4 lg:pt-3 lg:text-[13px]">
                <span>Ver detalhes e fotos</span>
                <span aria-hidden="true">›</span>
              </div>
            </Card>
          </router-link>

          <EmptyState v-if="matches.length === 0">Nenhuma partida encontrada nesse período.</EmptyState>
        </div>

        <div class="hidden min-w-0 xl:sticky xl:top-7 xl:block">
          <Card v-if="selectedMatchId" class="max-h-[calc(100dvh-3.5rem)] overflow-y-auto p-5 2xl:p-6">
            <MatchDetailContent :match-id="selectedMatchId" />
          </Card>
          <Card v-else class="flex min-h-[440px] flex-col items-center justify-center p-6 text-center">
            <div class="max-w-sm text-[15px] text-ink3">Selecione uma partida ao lado para ver detalhes, votar e fotos.</div>
          </Card>
        </div>
      </div>
    </div>
  </MobileShell>
</template>
