<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api/client'
import type { NextMatch, Team } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { formatDateShort, formatHour } from '../../lib/format'
import MobileShell from '../../components/layout/MobileShell.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'

const auth = useAuthStore()
const next = ref<NextMatch | null>(null)
const teams = ref<Team[]>([])

onMounted(async () => {
  const { data } = await api.get<NextMatch>('/matches/next')
  next.value = data
  if (data.match) {
    const res = await api.get<Team[]>(`/matches/${data.match.id}/teams`)
    teams.value = res.data ?? []
  }
})

const match = computed(() => next.value?.match ?? null)
const playerCount = computed(() => teams.value.reduce((n, t) => n + t.members.length, 0))

// Degradê do cabeçalho de cada time a partir da cor sorteada.
function headerStyle(team: Team) {
  const dark: Record<string, string> = {
    '#C8F14B': 'linear-gradient(135deg,#0A3B28,#116040)',
    '#F59E0B': 'linear-gradient(135deg,#7A3E12,#B45309)',
    '#3B82F6': 'linear-gradient(135deg,#1E3A8A,#2563EB)',
    '#1F2937': 'linear-gradient(135deg,#111827,#374151)',
  }
  return { backgroundImage: dark[team.team_color] ?? 'linear-gradient(135deg,#0A3B28,#116040)' }
}
</script>

<template>
  <MobileShell>
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[22px] font-bold">Times da partida</div>
          <div v-if="match" class="text-[13.5px] text-white/65">
            {{ formatDateShort(match.match_date) }} · {{ formatHour(match.start_time) }} · {{ playerCount }} jogadores
          </div>
        </div>
        <span
          v-if="teams.length"
          class="inline-flex items-center gap-1.5 rounded-full bg-lime/15 px-3 py-1 text-xs font-semibold text-lime"
        >● Times sorteados</span>
      </div>
    </template>

    <div v-if="teams.length" class="flex flex-col gap-3 px-4 pt-4">
      <template v-for="(team, index) in teams" :key="team.id">
        <div v-if="index > 0" class="-my-1 flex items-center gap-3">
          <div class="h-px flex-1 bg-border" />
          <div class="flex h-11 w-11 items-center justify-center rounded-full bg-ink font-condensed text-[15px] font-bold tracking-[.05em] text-canvas">
            VS
          </div>
          <div class="h-px flex-1 bg-border" />
        </div>

        <Card class="overflow-hidden">
          <div class="flex items-center justify-between px-4 py-3" :style="headerStyle(team)">
            <div class="flex items-center gap-2.5">
              <div class="h-[26px] w-[26px] rounded-full border-[3px] border-white/30" :style="{ backgroundColor: team.team_color }" />
              <span class="font-condensed text-[17px] font-bold tracking-[.08em] text-white">
                {{ team.team_name.toUpperCase() }}
              </span>
            </div>
            <span class="text-xs font-semibold text-white/60">{{ team.members.length }} jogadores</span>
          </div>
          <div class="grid grid-cols-2 gap-x-3 gap-y-[7px] px-4 py-2.5">
            <div v-for="member in team.members" :key="member.user_id" class="flex items-center gap-2">
              <Avatar :name="member.name" :color="member.avatar_color" size="xs" />
              <span class="truncate text-[13px]" :class="member.user_id === auth.user?.id ? 'font-semibold' : 'font-medium'">
                {{ member.user_id === auth.user?.id ? 'Você' : member.name }}
              </span>
            </div>
          </div>
        </Card>
      </template>
    </div>

    <template v-else>
      <div class="px-4 pt-4">
        <Badge v-if="match && match.status === 'open'" tone="warn" dot>Confirmação ainda aberta</Badge>
      </div>
      <EmptyState>Os times ainda não foram sorteados.</EmptyState>
    </template>
  </MobileShell>
</template>
