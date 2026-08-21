<script setup lang="ts">
import { computed, ref } from 'vue'

// Os demais atributos (required, autocomplete, minlength, placeholder...) caem direto no input.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    /**
     * `on-dark` é o campo sobre o gradiente escuro das telas de auth, que vira
     * claro no desktop; `surface` é o campo já sobre a superfície clara.
     */
    variant?: 'on-dark' | 'surface'
  }>(),
  { variant: 'surface' },
)

const model = defineModel<string>({ required: true })
const visible = ref(false)

const inputClass = computed(() =>
  props.variant === 'on-dark'
    ? 'border-white/15 bg-white/5 text-white placeholder:text-white/40 focus:border-lime md:border-border md:bg-bg md:text-ink md:placeholder:text-ink3 md:focus:border-brand'
    : 'border-border bg-surface text-ink placeholder:text-ink3 focus:border-brand',
)

const toggleClass = computed(() =>
  props.variant === 'on-dark'
    ? 'text-white/70 hover:bg-white/10 hover:text-lime focus-visible:outline-lime md:text-ink2 md:hover:bg-brandSoft md:hover:text-brand'
    : 'text-ink2 hover:bg-brandSoft hover:text-brand focus-visible:outline-brand',
)
</script>

<template>
  <div class="relative">
    <input
      v-model="model"
      v-bind="$attrs"
      :type="visible ? 'text' : 'password'"
      class="h-[50px] w-full rounded-xl border px-4 pr-14 text-[15px] outline-none"
      :class="inputClass"
    />
    <button
      type="button"
      class="absolute right-1.5 top-1/2 flex size-9 -translate-y-1/2 items-center justify-center rounded-lg transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
      :class="toggleClass"
      :aria-label="visible ? 'Ocultar senha' : 'Mostrar senha'"
      :aria-pressed="visible"
      @pointerdown.prevent
      @click.stop="visible = !visible"
    >
      <svg v-if="visible" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M3 3l18 18M10.6 10.6a2 2 0 002.8 2.8M9.9 4.2A10.7 10.7 0 0112 4c6 0 9.5 8 9.5 8a17.3 17.3 0 01-3 3.8M6.2 6.2C3.8 8.1 2.5 12 2.5 12s3.5 8 9.5 8a10.8 10.8 0 004-.8" />
      </svg>
      <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M2.5 12s3.5-8 9.5-8 9.5 8 9.5 8-3.5 8-9.5 8-9.5-8-9.5-8z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
    </button>
  </div>
</template>
