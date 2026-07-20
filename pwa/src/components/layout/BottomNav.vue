<script setup lang="ts">
import { useRoute } from 'vue-router'
import NavIcon from './NavIcon.vue'

const route = useRoute()

const items = [
  { name: 'home', label: 'Início', to: '/', icon: 'home' },
  { name: 'match', label: 'Partida', to: '/partida', icon: 'ball' },
  { name: 'payments', label: 'Pagamentos', to: '/pagamentos', icon: 'pix' },
  { name: 'history', label: 'Histórico', to: '/historico', icon: 'clock' },
  { name: 'profile', label: 'Perfil', to: '/perfil', icon: 'user' },
]

function isActive(name: string): boolean {
  if (name === 'match') return route.name === 'match' || route.name === 'teams'
  if (name === 'history') return route.name === 'history' || route.name === 'match-detail'
  return route.name === name
}
</script>

<template>
  <nav
    class="flex border-t border-border bg-surface px-2 pb-[max(env(safe-area-inset-bottom),1.25rem)] pt-2"
  >
    <router-link
      v-for="item in items"
      :key="item.name"
      :to="item.to"
      class="flex flex-1 flex-col items-center gap-[3px] text-[10.5px]"
      :class="isActive(item.name) ? 'font-semibold text-brand' : 'font-medium text-ink3'"
    >
      <NavIcon :name="item.icon" :stroke-width="isActive(item.name) ? 2 : 1.8" />
      {{ item.label }}
    </router-link>
  </nav>
</template>
