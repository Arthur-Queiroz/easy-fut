<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../api/client'
import type { Dashboard } from '../../api/types'
import {
  countdown, dayOfMonth, formatCents, formatDateLong, formatDMY,
  formatHour, formatTimestamp, monthShort, weekdayShort,
} from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const router = useRouter()
const data = ref<Dashboard | null>(null)

onMounted(load)

async function load() {
  const res = await api.get<Dashboard>('/admin/dashboard')
  data.value = res.data
}

const stats = computed(() => data.value?.stats ?? null)
const match = computed(() => data.value?.next_match ?? null)
const progress = computed(() => {
  if (!match.value) return 0
  const total = match.value.going_count + match.value.not_going_count + match.value.no_response_count
  return total === 0 ? 0 : Math.round((match.value.going_count / total) * 100)
})

async function closeConfirmations() {
  if (!match.value) return
  await api.post(`/matches/${match.value.id}/close-confirmations`)
  await load()
}

async function drawTeams() {
  if (!match.value) return
  await api.post(`/matches/${match.value.id}/draw-teams`, { team_count: 2 })
  router.push('/partida/times')
}

const dotColor = (kind: string) => {
  if (kind === 'payment' || kind === 'vote_result') return 'bg-brand'
  if (kind.startsWith('user_')) return 'bg-danger'
  return 'bg-info'
}
</script>

<template>
  <AdminLayout>
    <template #title>Dashboard</template>
    <template #subtitle>{{ formatDateLong(new Date().toISOString().slice(0, 10)) }}</template>
    <template #actions>
      <BaseButton variant="outline" size="sm" class="bg-surface" @click="router.push('/admin/convites')">
        <NavIcon name="chat" :size="15" :stroke-width="1.9" />
        Gerar convite
      </BaseButton>
      <BaseButton size="sm" @click="router.push('/admin/partidas')">
        <NavIcon name="plus" :size="15" :stroke-width="2.2" />
        Nova partida
      </BaseButton>
    </template>

    <div v-if="stats" class="flex flex-col gap-4">
      <!-- Indicadores -->
      <div class="grid grid-cols-2 gap-3.5 xl:grid-cols-4">
        <Card class="px-[18px] py-4">
          <div class="text-[11px] font-semibold tracking-[.08em] text-ink3">USUÁRIOS ATIVOS</div>
          <div class="font-condensed text-[34px] font-bold leading-[1.1]">{{ stats.active_users }}</div>
          <div class="text-xs font-medium text-ink3">{{ stats.inactive_users }} inativos</div>
        </Card>
        <Card class="px-[18px] py-4">
          <div class="text-[11px] font-semibold tracking-[.08em] text-ink3">INADIMPLENTES</div>
          <div class="font-condensed text-[34px] font-bold leading-[1.1] text-danger">{{ stats.delinquents }}</div>
          <div class="text-xs font-medium text-ink3">inativação automática ativa</div>
        </Card>
        <Card class="px-[18px] py-4">
          <div class="text-[11px] font-semibold tracking-[.08em] text-ink3">ARRECADADO NO MÊS</div>
          <div class="font-condensed text-[34px] font-bold leading-[1.1] text-brand">
            {{ formatCents(stats.month_paid_cents) }}
          </div>
          <div class="text-xs font-medium text-ink3">
            de {{ formatCents(stats.month_due_cents) }} · {{ stats.paid_count }} de {{ stats.charge_count }} pagas
          </div>
        </Card>
        <Card class="px-[18px] py-4">
          <div class="text-[11px] font-semibold tracking-[.08em] text-ink3">COBRANÇAS PENDENTES</div>
          <div class="font-condensed text-[34px] font-bold leading-[1.1] text-warn">{{ stats.pending_count }}</div>
          <div class="text-xs font-medium text-ink3">{{ stats.pending_invites }} convites pendentes</div>
        </Card>
      </div>

      <div class="grid gap-3.5 xl:grid-cols-[1.5fr_1fr]">
        <!-- Próxima partida -->
        <Card class="px-5 py-[18px]">
          <div class="flex items-center justify-between">
            <SectionLabel>Próxima partida</SectionLabel>
            <Badge v-if="match" :tone="match.status === 'open' ? 'success' : 'info'" dot>
              {{
                match.status === 'open'
                  ? `Confirmação aberta${countdown(match.confirmation_deadline) ? ' · faltam ' + countdown(match.confirmation_deadline) : ''}`
                  : match.status === 'closed'
                    ? 'Confirmação encerrada'
                    : 'Times sorteados'
              }}
            </Badge>
          </div>

          <template v-if="match">
            <div class="mt-3.5 flex items-center gap-4">
              <div class="w-[60px] rounded-xl border border-border bg-surface2 py-2 text-center">
                <div class="text-[10.5px] font-semibold text-danger">{{ weekdayShort(match.match_date) }}</div>
                <div class="font-condensed text-[26px] font-bold leading-none">{{ dayOfMonth(match.match_date) }}</div>
                <div class="text-[10px] font-medium text-ink3">{{ monthShort(match.match_date) }}</div>
              </div>
              <div class="flex-1">
                <div class="text-base font-bold">{{ formatHour(match.start_time) }} · {{ match.venue }}</div>
                <div class="mt-2 h-2 overflow-hidden rounded-full bg-surface2">
                  <div class="h-full rounded-full bg-brand" :style="{ width: progress + '%' }" />
                </div>
                <div class="mt-1.5 text-[12.5px] font-medium text-ink2">
                  <strong class="text-ink">{{ match.going_count }} confirmados</strong>
                  · {{ match.not_going_count }} não vão · {{ match.no_response_count }} sem resposta
                </div>
              </div>
            </div>
            <div class="mt-4 flex flex-wrap gap-2.5">
              <BaseButton v-if="match.status === 'open'" size="sm" @click="closeConfirmations">Encerrar confirmações</BaseButton>
              <BaseButton v-if="match.status !== 'open'" variant="outline" size="sm" @click="drawTeams">
                <NavIcon name="redo" :size="14" :stroke-width="1.9" />
                {{ match.status === 'teams_drawn' ? 'Refazer sorteio' : 'Sortear times' }}
              </BaseButton>
              <BaseButton variant="ghost" size="sm" @click="router.push('/admin/partidas')">Gerenciar</BaseButton>
            </div>
          </template>
          <p v-else class="mt-3 text-sm text-ink2">
            Nenhuma partida agendada.
            <router-link to="/admin/partidas" class="font-semibold text-brand">Criar partida</router-link>
          </p>
        </Card>

        <!-- Atenção -->
        <Card class="px-5 py-[18px]">
          <SectionLabel>Atenção</SectionLabel>
          <div class="mt-3 flex flex-col gap-2.5">
            <div
              v-for="charge in data!.delinquents.slice(0, 4)"
              :key="charge.id"
              class="flex items-center gap-2.5 rounded-[11px] bg-dangerBg px-3 py-2.5"
            >
              <Avatar :name="charge.user_name" :color="charge.avatar_color" size="xs" />
              <div class="min-w-0 flex-1">
                <div class="truncate text-[13px] font-semibold text-danger">
                  {{ charge.user_name }} · vencida em {{ formatDMY(charge.due_date).slice(0, 5) }}
                </div>
                <div class="text-[11.5px] text-ink2">{{ formatCents(charge.amount_cents) }} pendente</div>
              </div>
            </div>
            <p v-if="data!.delinquents.length === 0" class="text-sm text-ink3">Nenhuma cobrança vencida. 👏</p>
          </div>
        </Card>
      </div>

      <!-- Atividade recente -->
      <Card class="px-5 py-[18px]">
        <SectionLabel>Atividade recente</SectionLabel>
        <div class="mt-2 flex flex-col">
          <div
            v-for="(item, i) in data!.activity"
            :key="item.id"
            class="flex items-center gap-3 py-[9px]"
            :class="i < data!.activity.length - 1 ? 'border-b border-border' : ''"
          >
            <span class="h-2 w-2 shrink-0 rounded-full" :class="dotColor(item.kind)" />
            <span class="flex-1 text-[13.5px]">{{ item.message }}</span>
            <span class="whitespace-nowrap text-xs text-ink3">{{ formatTimestamp(item.created_at) }}</span>
          </div>
          <p v-if="data!.activity.length === 0" class="pt-2 text-sm text-ink3">Sem atividade registrada.</p>
        </div>
      </Card>
    </div>
  </AdminLayout>
</template>
