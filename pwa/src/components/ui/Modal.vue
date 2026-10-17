<script setup lang="ts">
defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-end justify-center sm:items-center">
      <div class="absolute inset-0 bg-black/50" @click="emit('close')" />
      <div class="relative m-0 w-full max-w-md rounded-t-2xl border border-border bg-surface p-5 shadow-card sm:m-4 sm:rounded-2xl">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-lg font-bold text-ink">{{ title }}</h2>
          <button type="button" class="p-1 text-ink3" aria-label="Fechar" @click="emit('close')">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
