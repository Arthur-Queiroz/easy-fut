<script setup lang="ts">
import BottomNav from './BottomNav.vue'
import TopNav from './TopNav.vue'

// Shell responsivo do painel do usuário: header verde em degradê + bottom nav no
// mobile; top navigation escura + conteúdo em container no desktop.
defineProps<{ noNav?: boolean }>()
</script>

<template>
  <TopNav />
  <div class="mx-auto flex min-h-dvh w-full flex-col bg-bg lg:min-h-[calc(100dvh-64px)] lg:max-w-none">
    <header
      class="bg-gradient-to-br from-pitch-3 to-pitch-4 px-5 pb-5 pt-[max(env(safe-area-inset-top),1.5rem)] text-white lg:hidden"
      style="background-image: linear-gradient(150deg, #0f3325, #1a6644)"
    >
      <slot name="header" />
    </header>
    <main class="flex-1 lg:px-8 lg:py-7">
      <div class="mx-auto lg:max-w-6xl">
        <slot />
      </div>
    </main>
    <div class="sticky bottom-0 z-10 lg:hidden">
      <BottomNav v-if="!noNav" />
    </div>
  </div>
</template>
