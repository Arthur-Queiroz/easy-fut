<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api/client'
import type { NextMatch } from '../../api/types'
import { countdown, formatDateLong, formatHour } from '../../lib/format'
import MobileShell from '../../components/layout/MobileShell.vue'
import Avatar from '../../components/ui/Avatar.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const next = ref<NextMatch | null>(null)

onMounted(load)

async function load() {
  const { data } = await api.get<NextMatch>('/matches/next')
  next.value = data
}

const match = computed(() => next.value?.match ?? null)
const myResponse = computed(() => next.value?.my_response ?? 'no_response')
const confirmations = computed(() => next.value?.confirmations ?? [])
const going = computed(() => confirmations.value.filter((c) => c.response === 'going'))
const notGoing = computed(() => confirmations.value.filter((c) => c.response === 'not_going'))
const noResponse = computed(() => confirmations.value.filter((c) => c.response === 'no_response'))

async function confirm(response: 'going' | 'not_going') {
  if (!match.value) return
  await api.post(`/matches/${match.value.id}/confirm`, { response })
  await load()
}

function toggleResponse() {
  confirm(myResponse.value === 'going' ? 'not_going' : 'going')
}
</script>

<template>
  <MobileShell>
    <template #header>
      <div v-if="match" class="flex items-center justify-between">
        <div>
          <div class="text-[22px] font-bold">{{ formatDateLong(match.match_date) }}</div>
          <div class="text-[13.5px] text-white/65">
            {{ formatHour(match.start_time) }} · {{ match.venue }}
          </div>
          <div v-if="match.address" class="text-xs text-white/45">{{ match.address }}</div>
        </div>
        <div
          v-if="match.status === 'open' && countdown(match.confirmation_deadline)"
          class="rounded-[14px] border border-lime/40 bg-lime/10 px-3 py-2 text-center"
        >
          <div class="font-condensed text-[22px] font-bold leading-none text-lime">
            {{ countdown(match.confirmation_deadline) }}
          </div>
          <div class="text-[9.5px] font-medium tracking-[.06em] text-white/60">PARA CONFIRMAR</div>
        </div>
      </div>
      <div v-else class="text-[22px] font-bold">Próxima partida</div>
    </template>

    <div v-if="match" class="flex flex-col gap-3 px-4 pt-4 lg:px-0 lg:pt-0">
      <!-- Cabeçalho desktop -->
      <div class="hidden items-start justify-between gap-5 lg:flex">
        <div>
          <div class="text-2xl font-extrabold">{{ formatDateLong(match.match_date) }} · {{ formatHour(match.start_time) }}</div>
          <div class="mt-1 text-[14px] text-ink2">{{ match.venue }}<template v-if="match.address"> · {{ match.address }}</template></div>
        </div>
        <div class="flex shrink-0 items-center gap-3">
          <span
            v-if="myResponse === 'going'"
            class="inline-flex items-center gap-1.5 rounded-full bg-brandSoft px-3 py-[5px] text-[13px] font-semibold text-brand"
          >✓ Você vai participar</span>
          <span
            v-else-if="myResponse === 'not_going'"
            class="inline-flex items-center gap-1.5 rounded-full bg-dangerBg px-3 py-[5px] text-[13px] font-semibold text-danger"
          >✕ Você não vai</span>
          <span
            v-else
            class="inline-flex items-center gap-1.5 rounded-full bg-warnBg px-3 py-[5px] text-[13px] font-semibold text-warn"
          >! Você ainda não respondeu</span>
          <button
            v-if="match.status === 'open'"
            type="button"
            class="text-[13px] font-semibold text-brand"
            @click="toggleResponse"
          >
            Alterar
          </button>
        </div>
      </div>

      <!-- Minha resposta mobile -->
      <Card class="flex items-center justify-between px-4 py-3.5 lg:hidden">
        <span
          v-if="myResponse === 'going'"
          class="inline-flex items-center gap-1.5 rounded-full bg-brandSoft px-3 py-[5px] text-[13px] font-semibold text-brand"
        >✓ Sua resposta: vou participar</span>
        <span
          v-else-if="myResponse === 'not_going'"
          class="inline-flex items-center gap-1.5 rounded-full bg-dangerBg px-3 py-[5px] text-[13px] font-semibold text-danger"
        >✕ Sua resposta: não vou</span>
        <span
          v-else
          class="inline-flex items-center gap-1.5 rounded-full bg-warnBg px-3 py-[5px] text-[13px] font-semibold text-warn"
        >! Você ainda não respondeu</span>
        <button
          v-if="match.status === 'open'"
          type="button"
          class="text-[13px] font-semibold text-brand"
          @click="toggleResponse"
        >
          Alterar
        </button>
        <span v-else class="text-[12px] text-ink3">Confirmação encerrada</span>
      </Card>

      <div class="grid gap-3 lg:grid-cols-2 lg:items-start">
        <!-- Confirmados -->
        <Card class="p-4">
          <div class="mb-[11px] flex items-center justify-between">
            <SectionLabel>Confirmados</SectionLabel>
            <span class="font-condensed text-[15px] font-bold text-brand">{{ going.length }}</span>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div v-for="entry in going" :key="entry.user_id" class="flex items-center gap-2">
              <Avatar :name="entry.name" :color="entry.avatar_color" size="xs" />
              <span class="truncate text-[13px] font-medium">
                {{ entry.name }}
                <span
                  v-if="entry.role === 'admin'"
                  class="rounded-md bg-infoBg px-1.5 py-px text-[10px] font-semibold text-info"
                >ADMIN</span>
              </span>
            </div>
          </div>
          <p v-if="going.length === 0" class="text-sm text-ink3">Ninguém confirmou ainda.</p>
        </Card>

        <!-- Não vão / sem resposta -->
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-1">
          <Card class="px-4 py-3.5">
            <div class="mb-[9px] flex items-center justify-between">
              <SectionLabel>Não vão</SectionLabel>
              <span class="font-condensed text-[15px] font-bold text-danger">{{ notGoing.length }}</span>
            </div>
            <div class="flex flex-col gap-[7px]">
              <div v-for="entry in notGoing" :key="entry.user_id" class="flex items-center gap-2 opacity-75">
                <Avatar :name="entry.name" color="#8CA094" size="xs" />
                <span class="truncate text-[12.5px] font-medium">{{ entry.name }}</span>
              </div>
            </div>
          </Card>
          <div class="rounded-2xl border border-dashed border-border bg-surface px-4 py-3.5">
            <div class="mb-[9px] flex items-center justify-between">
              <SectionLabel>Sem resposta</SectionLabel>
              <span class="font-condensed text-[15px] font-bold text-warn">{{ noResponse.length }}</span>
            </div>
            <div class="flex flex-col gap-[7px]">
              <div v-for="entry in noResponse" :key="entry.user_id" class="flex items-center gap-2 opacity-60">
                <Avatar :name="entry.name" color="#8CA094" size="xs" />
                <span class="truncate text-[12.5px] font-medium">{{ entry.name }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <router-link
        v-if="match.status === 'teams_drawn'"
        to="/partida/times"
        class="flex h-[50px] items-center justify-center rounded-[13px] bg-brand text-[15px] font-bold text-brandInk"
      >
        Ver times sorteados
      </router-link>
      <p v-else class="text-center text-xs text-ink3">Os times são sorteados quando a confirmação encerrar</p>
    </div>

    <EmptyState v-else>Nenhuma partida agendada. Aguarde o administrador abrir a próxima.</EmptyState>
  </MobileShell>
</template>
