<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { initials } from '../../lib/format'
import { safeColor } from '../../lib/security'

const props = withDefaults(
  defineProps<{ name: string; color: string; url?: string; size?: 'xs' | 'sm' | 'md' | 'lg'; ring?: boolean }>(),
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

const safeBgColor = computed(() => safeColor(props.color))
const imageFailed = ref(false)
watch(() => props.url, () => { imageFailed.value = false })
const imageURL = computed(() => {
  const url = props.url ?? ''
  return /^\/api\/avatars\/[0-9a-f-]{36}\/[0-9a-f-]{36}\.(jpg|png)$/.test(url) ? url : ''
})

// Texto claro ou escuro conforme a luminância da cor de fundo.
const textColor = computed(() => {
  const hex = safeBgColor.value.replace('#', '')
  if (hex.length !== 6 || hex === '000000') return '#fff'
  const r = parseInt(hex.slice(0, 2), 16)
  const g = parseInt(hex.slice(2, 4), 16)
  const b = parseInt(hex.slice(4, 6), 16)
  return 0.299 * r + 0.587 * g + 0.114 * b > 150 ? '#06120c' : '#ffffff'
})
</script>

<template>
  <div
    class="flex shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold"
    :class="[sizeClass, ring ? 'border-2 border-white/30' : '']"
    :style="{ backgroundColor: safeBgColor, color: textColor }"
  >
    <img v-if="imageURL && !imageFailed" :src="imageURL" :alt="name" class="h-full w-full object-cover" @error="imageFailed = true" />
    <span v-else>{{ initials(name) }}</span>
  </div>
</template>
