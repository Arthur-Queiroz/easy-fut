<script setup lang="ts">
import { useRouter } from 'vue-router'
import BaseButton from '../components/ui/BaseButton.vue'

const router = useRouter()
</script>

<template>
  <main class="flex min-h-dvh flex-col items-center justify-center px-6 text-center text-white" style="background-image: linear-gradient(165deg, #0c100f 0%, #13251f 100%)">
    <h1 class="font-condensed text-6xl font-bold">404</h1>
    <p class="mt-2 text-lg font-medium">Página não encontrada</p>
    <p class="mt-1 text-sm text-white/70">O endereço que você acessou não existe ou foi movido.</p>
    <BaseButton class="mt-6" @click="router.push({ name: 'home' })">Voltar para o início</BaseButton>
  </main>
</template>
