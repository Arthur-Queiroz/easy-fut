<script setup lang="ts">
import BottomNav from './BottomNav.vue'
import PlayerSidebar from './PlayerSidebar.vue'

// Shell responsivo do painel do usuário: cabeçalho e navegação inferior no
// celular; barra lateral e conteúdo em container no desktop.
defineProps<{ noNav?: boolean; wide?: boolean }>()
</script>

<template>
  <div class="flex min-h-dvh bg-bg">
    <PlayerSidebar v-if="!noNav" />
    <div class="flex min-h-dvh min-w-0 flex-1 flex-col">
      <header
        class="bg-gradient-to-br from-pitch-3 to-pitch-4 px-5 pb-5 pt-[max(env(safe-area-inset-top),1.5rem)] text-white lg:hidden"
        style="background-image: linear-gradient(150deg, #0f3325, #1a6644)"
      >
        <slot name="header" />
      </header>
      <main class="min-w-0 flex-1 lg:px-7 lg:py-7">
        <div class="mx-auto" :class="wide ? 'lg:max-w-7xl 2xl:max-w-[1440px]' : 'lg:max-w-6xl'">
          <slot />
        </div>
      </main>
      <div class="sticky bottom-0 z-10 lg:hidden">
        <BottomNav v-if="!noNav" />
      </div>
    </div>
  </div>
</template>
