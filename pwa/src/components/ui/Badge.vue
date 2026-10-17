<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ tone?: 'success' | 'warn' | 'danger' | 'info' | 'neutral'; dot?: boolean; solid?: boolean }>(),
  { tone: 'neutral', dot: false, solid: false },
)

const toneClass = computed(() => {
  if (props.solid) {
    return {
      success: 'bg-brand text-brandInk',
      warn: 'bg-warn text-white',
      danger: 'bg-danger text-white',
      info: 'bg-info text-white',
      neutral: 'bg-surface2 text-ink2',
    }[props.tone]
  }
  return {
    success: 'bg-brandSoft text-brand',
    warn: 'bg-warnBg text-warn',
    danger: 'bg-dangerBg text-danger',
    info: 'bg-infoBg text-info',
    neutral: 'bg-surface2 text-ink2',
  }[props.tone]
})
</script>

<template>
  <span
    class="inline-flex w-fit items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-[3px] text-[11.5px] font-semibold"
    :class="toneClass"
  >
    <span v-if="dot" class="text-[8px] leading-none">●</span>
    <slot />
  </span>
</template>
