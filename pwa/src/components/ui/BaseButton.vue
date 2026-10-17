<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'outline' | 'outline-brand' | 'danger' | 'ghost' | 'lime'
    size?: 'sm' | 'md' | 'lg'
    disabled?: boolean
    loading?: boolean
  }>(),
  { variant: 'primary', size: 'md', disabled: false, loading: false },
)

const variantClass = computed(
  () =>
    ({
      primary: 'bg-brand text-brandInk font-bold',
      outline: 'border-[1.5px] border-border text-ink2 font-semibold',
      'outline-brand': 'border-[1.5px] border-brand text-brand font-bold',
      danger: 'bg-danger text-white font-bold',
      ghost: 'text-brand font-semibold',
      lime: 'bg-lime text-pitch-1 font-bold',
    })[props.variant],
)

const sizeClass = computed(
  () =>
    ({
      sm: 'h-[38px] px-4 text-[13px] rounded-[10px]',
      md: 'h-11 px-4 text-sm rounded-xl',
      lg: 'h-[52px] px-5 text-[15px] rounded-[13px]',
    })[props.size],
)
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center justify-center gap-2 transition active:scale-[.98] disabled:pointer-events-none disabled:opacity-50"
    :class="[variantClass, sizeClass]"
    :disabled="disabled || loading"
  >
    <svg v-if="loading" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity=".25" stroke-width="3" />
      <path d="M21 12a9 9 0 00-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
    </svg>
    <slot />
  </button>
</template>
