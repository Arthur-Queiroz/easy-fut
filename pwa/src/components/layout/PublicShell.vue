<script setup lang="ts">
import NavIcon from './NavIcon.vue'

// Layout das telas públicas do avulso (sem login): topo escuro com a marca,
// como o login e o convite, e conteúdo em cards numa coluna estreita.
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-bg">
    <header
      class="px-5 pb-6 pt-[max(env(safe-area-inset-top),1.5rem)] text-white"
      style="background-image: linear-gradient(165deg, #0c100f 0%, #13251f 45%, #0f3325 100%)"
    >
      <div class="mx-auto w-full max-w-xl">
        <router-link to="/login" class="inline-flex items-center gap-2 text-lime" aria-label="Easy Fut">
          <span class="flex h-8 w-8 items-center justify-center rounded-full border-[1.5px] border-lime bg-lime/10">
            <NavIcon name="ball" :size="17" :stroke-width="1.7" />
          </span>
          <span class="font-condensed text-[17px] font-bold tracking-[.14em]">EASY FUT</span>
        </router-link>
        <h1 class="mt-5 text-[24px] font-bold leading-tight">{{ title }}</h1>
        <p v-if="subtitle" class="mt-1 text-[13.5px] text-white/65">{{ subtitle }}</p>
      </div>
    </header>
    <main class="flex-1 px-4 pb-[max(env(safe-area-inset-bottom),2rem)] pt-4">
      <div class="mx-auto flex w-full max-w-xl flex-col gap-3.5">
        <slot />
      </div>
    </main>
  </div>
</template>
