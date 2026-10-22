<script setup lang="ts">
defineProps<{ message: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out motion-reduce:transition-none"
      enter-from-class="translate-y-[-8px] opacity-0"
      leave-active-class="transition duration-150 ease-in motion-reduce:transition-none"
      leave-to-class="translate-y-[-8px] opacity-0"
    >
      <div
        v-if="message"
        role="status"
        aria-live="polite"
        aria-atomic="true"
        class="fixed left-[calc(env(safe-area-inset-left)+1rem)] right-[calc(env(safe-area-inset-right)+1rem)] z-[60] flex max-w-sm items-start gap-3 rounded-xl border border-border bg-surface px-4 py-3 text-ink shadow-card sm:left-auto"
        style="top: calc(env(safe-area-inset-top) + 1rem)"
      >
        <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-brandSoft text-brand" aria-hidden="true">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="m5 12 4 4L19 6" />
          </svg>
        </span>
        <p class="min-w-0 flex-1 pt-1 text-[13px] font-semibold leading-5">{{ message }}</p>
        <button
          type="button"
          class="-mr-2 -mt-1 rounded-lg p-2 text-ink3 hover:bg-surface2 hover:text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-brand"
          aria-label="Fechar aviso"
          @click="emit('close')"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
            <path d="M6 6l12 12M18 6 6 18" />
          </svg>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>
