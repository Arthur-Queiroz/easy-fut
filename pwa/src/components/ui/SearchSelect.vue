<script setup lang="ts" generic="T extends SearchOption">
// Dropdown com busca (combobox): o botão mostra a opção escolhida; ao abrir,
// o campo de busca filtra a lista. Teclado: ↑ ↓ navegam, Enter escolhe, Esc fecha.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { normalizeSearch, type SearchOption } from '../../lib/search'
import NavIcon from '../layout/NavIcon.vue'

const props = withDefaults(
  defineProps<{
    options: T[]
    label: string // nome acessível do campo
    placeholder?: string
    searchPlaceholder?: string
    emptyText?: string
    clearable?: boolean
    filter?: (option: T, query: string) => boolean
  }>(),
  { placeholder: 'Selecione', searchPlaceholder: 'Buscar…', emptyText: 'Nada encontrado.', clearable: false, filter: undefined },
)
const model = defineModel<string | null>({ default: null })

const root = ref<HTMLElement | null>(null)
const input = ref<HTMLInputElement | null>(null)
const button = ref<HTMLButtonElement | null>(null)
const open = ref(false)
const query = ref('')
const active = ref(0)
const listId = useId()

const selected = computed(() => props.options.find((o) => o.value === model.value) ?? null)
const filtered = computed(() => {
  const q = normalizeSearch(query.value)
  if (!q) return props.options
  return props.options.filter((o) =>
    props.filter ? props.filter(o, query.value) : normalizeSearch(`${o.label} ${o.hint ?? ''}`).includes(q),
  )
})
watch(filtered, () => {
  active.value = 0
})

async function toggle() {
  if (open.value) return close()
  open.value = true
  query.value = ''
  active.value = Math.max(0, filtered.value.findIndex((o) => o.value === model.value))
  await nextTick()
  input.value?.focus()
  scrollActive()
}

function close(focusButton = false) {
  open.value = false
  if (focusButton) button.value?.focus()
}

function choose(option: T) {
  model.value = option.value
  close(true)
}

function clear() {
  model.value = null
}

function scrollActive() {
  document.getElementById(`${listId}-${active.value}`)?.scrollIntoView?.({ block: 'nearest' })
}

function onKey(event: KeyboardEvent) {
  const n = filtered.value.length
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!n) return
    active.value = (active.value + (event.key === 'ArrowDown' ? 1 : n - 1)) % n
    scrollActive()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const option = filtered.value[active.value]
    if (option) choose(option)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    close(true)
  }
}

function onOutside(event: PointerEvent) {
  if (open.value && root.value && !root.value.contains(event.target as Node)) close()
}
onMounted(() => document.addEventListener('pointerdown', onOutside))
onBeforeUnmount(() => document.removeEventListener('pointerdown', onOutside))
</script>

<template>
  <div ref="root" class="relative min-w-0">
    <button
      ref="button"
      type="button"
      class="flex h-12 w-full items-center gap-2.5 rounded-xl border bg-surface px-3.5 text-left text-[13.5px] shadow-card transition"
      :class="open ? 'border-brand' : 'border-border hover:border-ink3'"
      :aria-label="label"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="toggle"
    >
      <span class="flex min-w-0 flex-1 items-center gap-2.5">
        <slot v-if="selected" name="selected" :option="selected">
          <span class="truncate font-semibold">{{ selected.label }}</span>
        </slot>
        <span v-else class="truncate text-ink3">{{ placeholder }}</span>
      </span>
      <span
        v-if="clearable && selected"
        role="button"
        tabindex="0"
        :aria-label="`Limpar ${label.toLowerCase()}`"
        class="-mr-1 rounded-md p-1 text-ink3 hover:bg-surface2 hover:text-ink"
        @click.stop="clear"
        @keydown.enter.stop.prevent="clear"
      >
        <NavIcon name="x" :size="14" :stroke-width="2.2" />
      </span>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" class="shrink-0 text-ink3 transition" :class="open ? 'rotate-180' : ''">
        <path d="M6 9l6 6 6-6" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>

    <div v-if="open" class="absolute left-0 right-0 z-30 mt-1.5 overflow-hidden rounded-xl border border-border bg-surface shadow-card">
      <div class="flex items-center gap-2 border-b border-border px-3.5">
        <NavIcon name="search" :size="15" class="shrink-0 text-ink3" />
        <input
          ref="input"
          v-model="query"
          role="combobox"
          :aria-label="`Buscar ${label.toLowerCase()}`"
          :aria-controls="listId"
          aria-expanded="true"
          :aria-activedescendant="filtered.length ? `${listId}-${active}` : undefined"
          :placeholder="searchPlaceholder"
          class="h-11 min-w-0 flex-1 bg-transparent text-[13.5px] text-ink outline-none placeholder:text-ink3"
          @keydown="onKey"
        />
      </div>
      <ul :id="listId" role="listbox" :aria-label="label" class="max-h-[320px] overflow-y-auto py-1">
        <li
          v-for="(option, i) in filtered"
          :id="`${listId}-${i}`"
          :key="option.value"
          role="option"
          :aria-selected="option.value === model"
          class="flex cursor-pointer items-center gap-2.5 px-3.5 py-2.5 text-[13px]"
          :class="i === active ? 'bg-surface2' : ''"
          @pointerenter="active = i"
          @click="choose(option)"
        >
          <span class="flex min-w-0 flex-1 items-center gap-2.5">
            <slot name="option" :option="option">
              <span class="truncate font-medium">{{ option.label }}</span>
              <span v-if="option.hint" class="ml-auto shrink-0 text-[12px] text-ink3">{{ option.hint }}</span>
            </slot>
          </span>
          <NavIcon v-if="option.value === model" name="check" :size="16" :stroke-width="2.6" class="shrink-0 text-brand" />
          <span v-else class="w-4 shrink-0" />
        </li>
        <li v-if="!filtered.length" class="px-3.5 py-5 text-center text-[12.5px] text-ink3">{{ emptyText }}</li>
      </ul>
    </div>
  </div>
</template>
