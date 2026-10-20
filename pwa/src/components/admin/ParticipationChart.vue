<script setup lang="ts">
// Colunas de participação (% dos participantes que votaram) nas últimas
// partidas, da mais antiga para a mais recente. A partida escolhida fica em
// destaque e tem o valor no topo; as outras mostram o valor no tooltip e na
// tabela. Clicar numa coluna escolhe a partida.
import { ref } from 'vue'
import type { ParticipationItem } from '../../lib/votes'

const props = defineProps<{ items: ParticipationItem[]; selectedId: string | null }>()
const emit = defineEmits<{ select: [id: string] }>()

const showTable = ref(false)
const hovered = ref<string | null>(null)
const pct = (item: ParticipationItem) => (item.participants ? Math.round((item.voters / item.participants) * 100) : 0)
const detail = (item: ParticipationItem) =>
  `${item.voters} de ${item.participants} ${item.participants === 1 ? 'participante votou' : 'participantes votaram'}`
</script>

<template>
  <div>
    <div class="mb-3 flex items-center justify-between gap-2">
      <div>
        <div class="text-[14px] font-bold">Participação nas últimas partidas</div>
        <div class="text-[12px] text-ink3">% dos participantes que votaram · toque numa coluna para abrir a partida</div>
      </div>
      <button type="button" class="shrink-0 rounded-lg border border-border px-2.5 py-1 text-[12px] font-semibold text-ink2 hover:bg-surface2" @click="showTable = !showTable">
        {{ showTable ? 'Ver gráfico' : 'Ver tabela' }}
      </button>
    </div>

    <table v-if="showTable" class="w-full text-[12.5px]">
      <thead class="text-left text-[11px] uppercase tracking-[.08em] text-ink3">
        <tr><th class="py-1.5 font-semibold">Partida</th><th class="font-semibold">Local</th><th class="text-right font-semibold">Votaram</th><th class="text-right font-semibold">%</th></tr>
      </thead>
      <tbody>
        <tr v-for="item in [...props.items].reverse()" :key="item.id" class="cursor-pointer border-t border-border hover:bg-surface2" :class="item.id === selectedId ? 'font-bold' : ''" @click="emit('select', item.id)">
          <td class="py-1.5">{{ item.label }}</td>
          <td class="truncate text-ink2">{{ item.venue }}</td>
          <td class="text-right tabular-nums">{{ item.voters }}/{{ item.participants }}</td>
          <td class="text-right tabular-nums">{{ pct(item) }}%</td>
        </tr>
      </tbody>
    </table>

    <div v-else class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-1.5">
      <!-- eixo y: 0 / 50 / 100% -->
      <div class="relative h-[132px] text-right text-[10.5px] tabular-nums text-ink3">
        <span class="absolute right-0 top-0 -translate-y-1/2">100%</span>
        <span class="absolute right-0 top-1/2 -translate-y-1/2">50%</span>
        <span class="absolute bottom-0 right-0 translate-y-1/2">0</span>
      </div>
      <div class="relative h-[132px]">
        <div class="absolute inset-x-0 top-0 border-t border-border" />
        <div class="absolute inset-x-0 top-1/2 border-t border-border" />
        <div class="absolute inset-x-0 bottom-0 border-t border-border" />
        <div class="relative flex h-full items-end">
          <button
            v-for="item in items"
            :key="item.id"
            type="button"
            class="group relative flex h-full flex-1 flex-col items-center justify-end"
            :aria-label="`${item.label}, ${item.venue}: ${detail(item)} (${pct(item)}%)`"
            :aria-pressed="item.id === selectedId"
            @pointerenter="hovered = item.id"
            @pointerleave="hovered = null"
            @focus="hovered = item.id"
            @blur="hovered = null"
            @click="emit('select', item.id)"
          >
            <span v-if="item.id === selectedId" class="mb-1 text-[11.5px] font-bold tabular-nums text-ink">{{ pct(item) }}%</span>
            <span
              class="w-[60%] max-w-[24px] rounded-t-[4px] transition group-hover:brightness-110"
              :class="item.id === selectedId ? 'bg-brand' : 'bg-chartMuted'"
              :style="{ height: `max(3px, ${pct(item)}%)` }"
            />
            <span
              v-if="hovered === item.id"
              role="tooltip"
              class="pointer-events-none absolute bottom-full z-20 mb-1 w-max max-w-[13rem] rounded-lg border border-border bg-surface px-2.5 py-1.5 text-left text-[12px] shadow-card"
            >
              <strong class="text-[13px] text-ink">{{ pct(item) }}%</strong>
              <span class="text-ink3"> · {{ item.label }}</span>
              <span class="block text-ink2">{{ detail(item) }}</span>
            </span>
          </button>
        </div>
      </div>
      <div />
      <div class="mt-1.5 flex">
        <span v-for="item in items" :key="item.id" class="flex-1 text-center text-[10.5px] tabular-nums" :class="item.id === selectedId ? 'font-bold text-ink' : 'text-ink3'">{{ item.label }}</span>
      </div>
    </div>
  </div>
</template>
