<script setup lang="ts">
// Barras horizontais de uma categoria (uma série só: sem legenda). Destaque
// na cor da marca, resto em cinza: sem jogador filtrado, destaca os líderes;
// com jogador, destaca em quem ele votou. O valor fica escrito na ponta de
// cada barra, então o tooltip só complementa (quem votou).
import { computed, ref } from 'vue'
import type { CandidateBar } from '../../lib/votes'
import NavIcon from '../layout/NavIcon.vue'
import Avatar from '../ui/Avatar.vue'

const props = defineProps<{
  bars: CandidateBar[]
  highlight?: string[] | null // ids em destaque; null = líderes
  selectedId?: string | null // candidato filtrado na lista de votos
}>()
const emit = defineEmits<{ select: [id: string] }>()

const hovered = ref<string | null>(null)
const max = computed(() => Math.max(1, ...props.bars.map((b) => b.votes)))
const emphasized = (bar: CandidateBar) => (props.highlight ? props.highlight.includes(bar.id) : bar.winner)
const percent = (share: number) => `${Math.round(share * 100)}%`
const votesLabel = (n: number) => `${n} ${n === 1 ? 'voto' : 'votos'}`

function votersText(names: string[]): string {
  return names.length > 6 ? `${names.slice(0, 6).join(', ')} e mais ${names.length - 6}` : names.join(', ')
}
</script>

<template>
  <ul class="flex flex-col gap-1" role="list">
    <li v-for="bar in bars" :key="bar.id" class="relative">
      <button
        type="button"
        class="group grid w-full grid-cols-[minmax(0,9.5rem)_minmax(0,1fr)] items-center gap-3 rounded-lg px-1.5 py-[7px] text-left transition"
        :class="selectedId === bar.id ? 'bg-surface2 ring-1 ring-border' : 'hover:bg-surface2'"
        :aria-pressed="selectedId === bar.id"
        :aria-label="`${bar.name}: ${votesLabel(bar.votes)}, ${percent(bar.share)} da categoria${bar.winner ? ', na liderança' : ''}. Mostrar quem votou.`"
        @pointerenter="hovered = bar.id"
        @pointerleave="hovered = null"
        @focus="hovered = bar.id"
        @blur="hovered = null"
        @click="emit('select', bar.id)"
      >
        <span class="flex min-w-0 items-center gap-2">
          <Avatar :name="bar.name" :color="bar.avatarColor" :url="bar.avatarURL" size="xs" />
          <span class="truncate text-[12.5px]" :class="bar.winner ? 'font-bold text-ink' : 'font-medium text-ink2'">{{ bar.name }}</span>
          <NavIcon v-if="bar.winner" name="trophy" :size="13" :stroke-width="2" class="shrink-0 text-ink2" />
        </span>
        <span class="flex min-w-0 items-center gap-2 border-l border-border">
          <span
            class="h-[14px] min-w-[3px] rounded-r-[4px] transition group-hover:brightness-110"
            :class="emphasized(bar) ? 'bg-brand' : 'bg-chartMuted'"
            :style="{ width: `calc((100% - 2.25rem) * ${bar.votes / max})` }"
          />
          <span class="shrink-0 text-[12.5px] font-bold tabular-nums text-ink">{{ bar.votes }}</span>
        </span>
      </button>
      <div
        v-if="hovered === bar.id"
        role="tooltip"
        class="pointer-events-none absolute left-[10.5rem] top-full z-20 -mt-0.5 max-w-[16rem] rounded-lg border border-border bg-surface px-3 py-2 text-[12px] shadow-card"
      >
        <div><strong class="text-[13px] text-ink">{{ votesLabel(bar.votes) }}</strong> <span class="text-ink3">· {{ percent(bar.share) }} da categoria</span></div>
        <div class="mt-0.5 text-ink2">Votaram: {{ votersText(bar.voters) }}</div>
      </div>
    </li>
  </ul>
</template>
