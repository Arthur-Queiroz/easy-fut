<script setup lang="ts">
import { computed } from 'vue'
import { initials } from '../../lib/format'

const props = withDefaults(
  defineProps<{ name: string; color: string; size?: 'xs' | 'sm' | 'md' | 'lg'; ring?: boolean }>(),
  { size: 'sm', ring: false },
)

const sizeClass = computed(
  () =>
    ({
      xs: 'w-6 h-6 text-[10px]',
      sm: 'w-[30px] h-[30px] text-[11px]',
      md: 'w-11 h-11 text-[15px]',
      lg: 'w-[68px] h-[68px] text-2xl',
    })[props.size],
)

// Texto claro ou escuro conforme a luminância da cor de fundo.
const textColor = computed(() => {
  const hex = props.color.replace('#', '')
  if (hex.length !== 6) return '#fff'
  const r = parseInt(hex.slice(0, 2), 16)
  const g = parseInt(hex.slice(2, 4), 16)
  const b = parseInt(hex.slice(4, 6), 16)
  return 0.299 * r + 0.587 * g + 0.114 * b > 150 ? '#0A3B28' : '#ffffff'
})
</script>

<template>
  <div
    class="flex shrink-0 items-center justify-center rounded-full font-semibold"
    :class="[sizeClass, ring ? 'border-2 border-white/30' : '']"
    :style="{ backgroundColor: color, color: textColor }"
  >
    {{ initials(name) }}
  </div>
</template>
