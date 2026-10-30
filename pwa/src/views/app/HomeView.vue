<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api/client'
import type { Activity, Charge, NextMatch } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import {
  countdown, dayOfMonth, formatCents, formatDateLong, formatHour,
  formatMonth, monthShort, timeAgo, weekdayShort, todayISO,
} from '../../lib/format'
import { payableCents } from '../../lib/charges'
import MobileShell from '../../components/layout/MobileShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Badge from '../../components/ui/Badge.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const auth = useAuthStore()

const next = ref<NextMatch | null>(null)
const charges = ref<Charge[]>([])
const activity = ref<Activity[]>([])

onMounted(load)

async function load() {
  const [nextRes, chargesRes, activityRes] = await Promise.all([
    api.get<NextMatch>('/matches/next'),
    api.get<Charge[]>('/charges/me'),
    api.get<Activity[]>('/activity'),
  ])
  next.value = nextRes.data
  charges.value = chargesRes.data ?? []
  activity.value = activityRes.data
}

const match = computed(() => next.value?.match ?? null)
const myResponse = computed(() => next.value?.my_response ?? 'no_response')
const currentCharge = computed(() => charges.value.find((c) => c.status === 'pending' || c.status === 'overdue') ?? null)
const goingAvatars = computed(() =>
  (next.value?.confirmations ?? []).filter((c) => c.response === 'going').slice(0, 4),
)

async function confirm(response: 'going' | 'not_going') {
  if (!match.value) return
  await api.post(`/matches/${match.value.id}/confirm`, { response })
  const { data } = await api.get<NextMatch>('/matches/next')
  next.value = data
}

const activityIcon = (kind: string) => {
  if (kind === 'vote_result') return { icon: 'trophy', cls: 'bg-brandSoft text-brand' }
  if (kind === 'media_added') return { icon: 'camera', cls: 'bg-infoBg text-info' }
  if (kind.startsWith('user_')) return { icon: 'user', cls: 'bg-dangerBg text-danger' }
  if (kind === 'payment' || kind === 'charges_generated') return { icon: 'pix', cls: 'bg-warnBg text-warn' }
  return { icon: 'ball', cls: 'bg-surface2 text-ink2' }
}
</script>

<template>
  <MobileShell wide>
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[23px] font-bold">Fala, {{ auth.firstName }}!</div>
          <div class="text-[13.5px] text-white/65">Bora pra bola.</div>
        </div>
        <router-link to="/perfil" aria-label="Meu perfil">
          <Avatar :name="auth.user?.name ?? ''" :url="auth.user?.avatar_url" color="#2EE07C" size="md" ring />
        </router-link>
      </div>
    </template>

    <div class="flex flex-col gap-3.5 px-4 pt-4 lg:min-h-[calc(100dvh-3.5rem)] lg:justify-center lg:gap-5 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden items-baseline justify-between lg:flex">
        <div class="text-2xl font-extrabold lg:text-3xl">Fala, {{ auth.firstName }}!</div>
        <div class="text-[13.5px] text-ink2 lg:text-[15px]">{{ formatDateLong(todayISO()) }}</div>
      </div>

      <div class="grid gap-3.5 lg:gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(0,1fr)]">
        <!-- Próxima partida -->
        <Card class="p-4 lg:p-6 xl:col-start-1 xl:row-start-1">
          <div class="flex items-center justify-between lg:border-b lg:border-border lg:pb-4">
            <SectionLabel class="lg:text-xs">Próxima partida</SectionLabel>
            <Badge v-if="match" :tone="match.status === 'open' ? 'success' : 'info'" dot class="lg:px-3 lg:py-1 lg:text-[13px]">
              {{ match.status === 'open' ? 'Confirmação aberta' : match.status === 'closed' ? 'Confirmação encerrada' : 'Times sorteados' }}
            </Badge>
          </div>

          <template v-if="match">
            <div class="mt-3 flex items-center gap-3.5 lg:mt-5 lg:gap-6">
              <div class="w-[58px] shrink-0 rounded-xl border border-border bg-surface2 py-[7px] text-center lg:w-[90px] lg:py-3">
                <div class="text-[11px] font-semibold tracking-[.06em] text-danger lg:text-xs">{{ weekdayShort(match.match_date) }}</div>
                <div class="font-condensed text-[26px] font-bold leading-none text-ink lg:text-[42px]">{{ dayOfMonth(match.match_date) }}</div>
                <div class="text-[10px] font-medium text-ink3 lg:text-[11px]">{{ monthShort(match.match_date) }}</div>
              </div>
              <div class="flex min-w-0 flex-col gap-[3px] lg:gap-1.5">
                <div class="text-[17px] font-bold lg:text-2xl">{{ formatHour(match.start_time) }} – {{ formatHour(match.end_time) }}</div>
                <div class="text-[13.5px] text-ink2 lg:text-[15px]">{{ match.venue }}</div>
                <div v-if="match.status === 'open' && countdown(match.confirmation_deadline)" class="flex items-center gap-1.5 text-xs font-semibold text-warn lg:text-[14px]">
                  <NavIcon name="clock" :size="13" :stroke-width="2" />
                  Faltam {{ countdown(match.confirmation_deadline) }} para confirmar
                </div>
              </div>
              <div v-if="match.status === 'open'" class="ml-auto hidden shrink-0 flex-col gap-2 lg:flex lg:w-[210px]">
                <button
                  type="button"
                  class="flex h-[50px] items-center justify-center gap-2 rounded-xl text-[16px] font-bold transition-all"
                  :class="myResponse === 'going' ? 'bg-brand text-brandInk' : 'border-[1.5px] border-border text-ink2'"
                  @click="confirm('going')"
                >
                  <NavIcon v-if="myResponse === 'going'" name="check" :size="16" :stroke-width="2.6" />
                  Vou participar
                </button>
                <button
                  type="button"
                  class="flex h-[46px] items-center justify-center rounded-xl text-[15px] font-semibold transition-all"
                  :class="myResponse === 'not_going' ? 'bg-danger text-white' : 'border-[1.5px] border-border text-ink2'"
                  @click="confirm('not_going')"
                >
                  Não vou
                </button>
              </div>
            </div>

            <div class="mt-3.5 flex items-center gap-2 lg:mt-5 lg:border-t lg:border-border lg:pt-4">
              <div class="flex">
                <Avatar
                  v-for="(entry, i) in goingAvatars"
                  :key="entry.user_id"
                  :name="entry.name"
                  :color="entry.avatar_color"
                  :url="entry.avatar_url"
                  size="xs"
                  class="border-2 border-surface lg:h-8 lg:w-8 lg:text-xs"
                  :class="i > 0 ? '-ml-2' : ''"
                />
              </div>
              <span class="text-[12.5px] font-medium text-ink2 lg:text-[14px]">
                <strong class="text-ink">{{ match.going_count }} confirmados</strong>
                · {{ match.not_going_count }} não vão · {{ match.no_response_count }} sem resposta
              </span>
            </div>

            <div v-if="match.status === 'open'" class="mt-3.5 flex gap-2.5 lg:hidden">
              <button
                type="button"
                class="flex h-[50px] items-center justify-center gap-2 rounded-[13px] text-[15px] transition-all"
                :class="
                  myResponse === 'going'
                    ? 'flex-[1.4] bg-brand font-bold text-brandInk'
                    : myResponse === 'no_response'
                      ? 'flex-1 border-2 border-brand font-bold text-brand'
                      : 'flex-1 border-[1.5px] border-border font-semibold text-ink2'
                "
                @click="confirm('going')"
              >
                <NavIcon v-if="myResponse === 'going'" name="check" :size="16" :stroke-width="2.6" />
                Vou participar
              </button>
              <button
                type="button"
                class="flex h-[50px] items-center justify-center gap-2 rounded-[13px] text-[15px] transition-all"
                :class="myResponse === 'not_going' ? 'flex-[1.4] bg-danger font-bold text-white' : 'flex-1 border-[1.5px] border-border font-semibold text-ink2'"
                @click="confirm('not_going')"
              >
                <NavIcon v-if="myResponse === 'not_going'" name="x" :size="15" :stroke-width="2.6" />
                Não vou
              </button>
            </div>
            <router-link
              v-else-if="match.status === 'teams_drawn'"
              to="/partida/times"
              class="mt-3.5 flex h-[50px] items-center justify-center rounded-[13px] bg-brand text-[15px] font-bold text-brandInk lg:mt-4"
            >
              Ver times sorteados
            </router-link>
          </template>
          <p v-else class="mt-3 text-sm text-ink2">Nenhuma partida agendada no momento.</p>
        </Card>

        <!-- Mensalidade -->
        <Card v-if="currentCharge" class="p-4 lg:flex lg:flex-col lg:p-6 xl:col-start-2 xl:row-start-1">
          <div class="flex items-center justify-between lg:border-b lg:border-border lg:pb-4">
            <SectionLabel class="lg:text-xs">Mensalidade · {{ formatMonth(currentCharge.reference_month).split(' / ')[0] }}</SectionLabel>
            <Badge :tone="currentCharge.status === 'overdue' ? 'danger' : 'warn'" dot class="lg:px-3 lg:py-1 lg:text-[13px]">
              {{ currentCharge.status === 'overdue' ? 'Vencida' : 'Aguardando pagamento' }}
            </Badge>
          </div>
          <div class="mt-3 flex items-center justify-between lg:mt-5 lg:flex-1 lg:flex-col lg:items-start lg:justify-center lg:gap-5">
            <div>
              <div class="font-condensed text-[30px] font-bold leading-none lg:text-[42px]">{{ formatCents(payableCents(currentCharge)) }}</div>
              <div class="mt-1 text-[12.5px] text-ink2 lg:text-[14px]">Vence {{ formatDateLong(currentCharge.due_date) }}</div>
            </div>
            <router-link
              to="/pagamentos"
              class="flex h-11 items-center gap-2 rounded-xl border-[1.5px] border-brand px-[18px] text-sm font-bold text-brand lg:h-[50px] lg:w-full lg:justify-center lg:text-[15px]"
            >
              <NavIcon name="pix" :size="15" :stroke-width="1.9" />
              Pagar com PIX
            </router-link>
          </div>
        </Card>
        <!-- Atividades recentes -->
        <Card
          class="flex min-w-0 flex-col gap-3 p-4 lg:gap-4 lg:p-6 xl:row-start-2"
          :class="currentCharge ? 'xl:col-start-1' : 'xl:col-span-2'"
        >
          <SectionLabel class="lg:border-b lg:border-border lg:pb-4 lg:text-xs">Atividades recentes</SectionLabel>
          <div class="grid gap-3 lg:gap-4" :class="currentCharge ? '' : 'xl:grid-cols-2 xl:gap-x-8'">
            <div v-for="item in activity.slice(0, 6)" :key="item.id" class="flex min-w-0 items-center gap-[11px] lg:gap-3 lg:py-1">
              <div
                class="flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[10px] lg:h-10 lg:w-10"
                :class="activityIcon(item.kind).cls"
              >
                <NavIcon :name="activityIcon(item.kind).icon" :size="17" />
              </div>
              <div class="min-w-0 flex-1 text-[13.5px] leading-[1.35] text-ink lg:text-[14px]">{{ item.message }}</div>
              <span class="shrink-0 text-[11px] text-ink3 lg:text-xs">{{ timeAgo(item.created_at) }}</span>
            </div>
          </div>
          <p v-if="activity.length === 0" class="text-sm text-ink3">Nada por aqui ainda.</p>
        </Card>

        <!-- Estatísticas da temporada -->
        <Card
          class="p-4 lg:flex lg:flex-col lg:p-6 xl:col-start-2"
          :class="currentCharge ? 'xl:row-start-2' : 'xl:row-start-1'"
        >
          <SectionLabel class="lg:border-b lg:border-border lg:pb-4 lg:text-xs">Sua temporada</SectionLabel>
          <div class="mt-3 grid grid-cols-3 gap-3 text-center lg:mt-4 xl:flex xl:flex-1 xl:flex-col xl:gap-0 xl:text-left">
            <div class="xl:flex xl:flex-1 xl:flex-row-reverse xl:items-center xl:justify-between xl:border-b xl:border-border xl:px-1">
              <div class="font-condensed text-[32px] font-bold leading-none lg:text-[42px]">{{ auth.stats?.matches_played ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3 xl:mt-0 xl:text-xs">PARTIDAS</div>
            </div>
            <div class="xl:flex xl:flex-1 xl:flex-row-reverse xl:items-center xl:justify-between xl:border-b xl:border-border xl:px-1">
              <div class="font-condensed text-[32px] font-bold leading-none text-brand lg:text-[42px]">{{ auth.stats?.top_scorer_count ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3 xl:mt-0 xl:text-xs">ARTILHEIRO</div>
            </div>
            <div class="xl:flex xl:flex-1 xl:flex-row-reverse xl:items-center xl:justify-between xl:px-1">
              <div class="font-condensed text-[32px] font-bold leading-none text-warn lg:text-[42px]">{{ auth.stats?.worst_count ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3 xl:mt-0 xl:text-xs">PERNA DE PAU</div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  </MobileShell>
</template>
