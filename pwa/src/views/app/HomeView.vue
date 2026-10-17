<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api/client'
import type { Activity, Charge, NextMatch } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import {
  countdown, dayOfMonth, formatCents, formatDateLong, formatHour,
  formatMonth, monthShort, timeAgo, weekdayShort, todayISO,
} from '../../lib/format'
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
  <MobileShell>
    <template #header>
      <div class="flex items-center justify-between">
        <div>
          <div class="text-[23px] font-bold">Fala, {{ auth.firstName }}!</div>
          <div class="text-[13.5px] text-white/65">Bora pra bola.</div>
        </div>
        <router-link to="/perfil" aria-label="Meu perfil">
          <Avatar :name="auth.user?.name ?? ''" color="#2EE07C" size="md" ring />
        </router-link>
      </div>
    </template>

    <div class="flex flex-col gap-3.5 px-4 pt-4 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden items-baseline justify-between lg:flex">
        <div class="text-2xl font-extrabold">Fala, {{ auth.firstName }}!</div>
        <div class="text-[13.5px] text-ink2">{{ formatDateLong(todayISO()) }}</div>
      </div>

      <div class="grid gap-3.5 lg:grid-cols-[1.55fr_1fr] lg:items-start">
        <!-- Próxima partida -->
        <Card class="p-4 lg:p-6">
          <div class="flex items-center justify-between">
            <SectionLabel>Próxima partida</SectionLabel>
            <Badge v-if="match" :tone="match.status === 'open' ? 'success' : 'info'" dot>
              {{ match.status === 'open' ? 'Confirmação aberta' : match.status === 'closed' ? 'Confirmação encerrada' : 'Times sorteados' }}
            </Badge>
          </div>

          <template v-if="match">
            <div class="mt-3 flex items-center gap-3.5 lg:mt-4 lg:gap-5">
              <div class="w-[58px] rounded-xl border border-border bg-surface2 py-[7px] text-center lg:w-[74px]">
                <div class="text-[11px] font-semibold tracking-[.06em] text-danger">{{ weekdayShort(match.match_date) }}</div>
                <div class="font-condensed text-[26px] font-bold leading-none text-ink lg:text-[34px]">{{ dayOfMonth(match.match_date) }}</div>
                <div class="text-[10px] font-medium text-ink3">{{ monthShort(match.match_date) }}</div>
              </div>
              <div class="flex flex-col gap-[3px] lg:gap-1">
                <div class="text-[17px] font-bold lg:text-xl">{{ formatHour(match.start_time) }} – {{ formatHour(match.end_time) }}</div>
                <div class="text-[13.5px] text-ink2">{{ match.venue }}</div>
                <div v-if="match.status === 'open' && countdown(match.confirmation_deadline)" class="flex items-center gap-1.5 text-xs font-semibold text-warn lg:text-[13px]">
                  <NavIcon name="clock" :size="13" :stroke-width="2" />
                  Faltam {{ countdown(match.confirmation_deadline) }} para confirmar
                </div>
              </div>
              <div v-if="match.status === 'open'" class="ml-auto hidden shrink-0 flex-col gap-2 lg:flex lg:w-[190px]">
                <button
                  type="button"
                  class="flex h-[46px] items-center justify-center gap-2 rounded-xl text-[15px] font-bold transition-all"
                  :class="myResponse === 'going' ? 'bg-brand text-brandInk' : 'border-[1.5px] border-border text-ink2'"
                  @click="confirm('going')"
                >
                  <NavIcon v-if="myResponse === 'going'" name="check" :size="16" :stroke-width="2.6" />
                  Vou participar
                </button>
                <button
                  type="button"
                  class="flex h-10 items-center justify-center rounded-xl text-[14px] font-semibold transition-all"
                  :class="myResponse === 'not_going' ? 'bg-danger text-white' : 'border-[1.5px] border-border text-ink2'"
                  @click="confirm('not_going')"
                >
                  Não vou
                </button>
              </div>
            </div>

            <div class="mt-3.5 flex items-center gap-2 lg:mt-4">
              <div class="flex">
                <Avatar
                  v-for="(entry, i) in goingAvatars"
                  :key="entry.user_id"
                  :name="entry.name"
                  :color="entry.avatar_color"
                  size="xs"
                  class="border-2 border-surface"
                  :class="i > 0 ? '-ml-2' : ''"
                />
              </div>
              <span class="text-[12.5px] font-medium text-ink2">
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
        <Card v-if="currentCharge" class="p-4 lg:p-6">
          <div class="flex items-center justify-between">
            <SectionLabel>Mensalidade · {{ formatMonth(currentCharge.reference_month).split(' / ')[0] }}</SectionLabel>
            <Badge :tone="currentCharge.status === 'overdue' ? 'danger' : 'warn'" dot>
              {{ currentCharge.status === 'overdue' ? 'Vencida' : 'Aguardando pagamento' }}
            </Badge>
          </div>
          <div class="mt-3 flex items-center justify-between lg:mt-4 lg:flex-col lg:items-start lg:gap-4">
            <div>
              <div class="font-condensed text-[30px] font-bold leading-none lg:text-[38px]">{{ formatCents(currentCharge.amount_cents) }}</div>
              <div class="mt-1 text-[12.5px] text-ink2">Vence {{ formatDateLong(currentCharge.due_date) }}</div>
            </div>
            <router-link
              to="/pagamentos"
              class="flex h-11 items-center gap-2 rounded-xl border-[1.5px] border-brand px-[18px] text-sm font-bold text-brand lg:w-full lg:justify-center"
            >
              <NavIcon name="pix" :size="15" :stroke-width="1.9" />
              Pagar com PIX
            </router-link>
          </div>
        </Card>
      </div>

      <div class="grid gap-3.5 lg:grid-cols-[1.55fr_1fr] lg:items-start">
        <!-- Atividades recentes -->
        <Card class="flex flex-col gap-3 p-4 lg:p-6">
          <SectionLabel>Atividades recentes</SectionLabel>
          <div v-for="item in activity.slice(0, 6)" :key="item.id" class="flex items-center gap-[11px]">
            <div
              class="flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[10px]"
              :class="activityIcon(item.kind).cls"
            >
              <NavIcon :name="activityIcon(item.kind).icon" :size="17" />
            </div>
            <div class="flex-1 text-[13.5px] leading-[1.35] text-ink">{{ item.message }}</div>
            <span class="text-[11px] text-ink3">{{ timeAgo(item.created_at) }}</span>
          </div>
          <p v-if="activity.length === 0" class="text-sm text-ink3">Nada por aqui ainda.</p>
        </Card>

        <!-- Estatísticas da temporada -->
        <Card class="p-4 lg:p-6">
          <SectionLabel>Sua temporada</SectionLabel>
          <div class="mt-3 grid grid-cols-3 gap-3 text-center lg:mt-4">
            <div>
              <div class="font-condensed text-[32px] font-bold leading-none lg:text-[30px]">{{ auth.stats?.matches_played ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">PARTIDAS</div>
            </div>
            <div>
              <div class="font-condensed text-[32px] font-bold leading-none text-brand lg:text-[30px]">{{ auth.stats?.top_scorer_count ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">ARTILHEIRO</div>
            </div>
            <div>
              <div class="font-condensed text-[32px] font-bold leading-none text-warn lg:text-[30px]">{{ auth.stats?.worst_count ?? 0 }}</div>
              <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">PERNA DE PAU</div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  </MobileShell>
</template>
