<script setup lang="ts">
import BottomNav from './BottomNav.vue'

// Shell mobile do painel do usuário: header verde em degradê, conteúdo e
// navegação inferior. Em telas largas o app fica centralizado como no celular.
defineProps<{ noNav?: boolean }>()
</script>

<template>
  <div class="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-bg shadow-card">
    <header
      class="bg-gradient-to-br from-pitch-2 to-pitch-3 px-5 pb-5 pt-[max(env(safe-area-inset-top),1.5rem)] text-white"
      style="background-image: linear-gradient(150deg, #0a3b28, #0f5238)"
    >
      <slot name="header" />
    </header>
    <main class="flex-1 pb-6">
      <slot />
    </main>
    <div class="sticky bottom-0 z-10">
      <BottomNav v-if="!noNav" />
    </div>
  </div>
</template>
