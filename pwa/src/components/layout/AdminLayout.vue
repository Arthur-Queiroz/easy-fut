<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { useThemeStore } from '../../stores/theme'
import Avatar from '../ui/Avatar.vue'
import NavIcon from './NavIcon.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

const items = [
  { label: 'Dashboard', to: '/admin', icon: 'grid' },
  { label: 'Partidas', to: '/admin/partidas', icon: 'ball' },
  { label: 'Usuários', to: '/admin/usuarios', icon: 'users' },
  { label: 'Mensalidades', to: '/admin/mensalidades', icon: 'pix' },
  { label: 'Convites', to: '/admin/convites', icon: 'chat' },
  { label: 'Configurações', to: '/admin/configuracoes', icon: 'gear' },
]

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-bg lg:flex-row">
    <!-- Sidebar desktop / barra superior mobile -->
    <aside
      class="flex shrink-0 flex-row items-center gap-1 overflow-x-auto px-3 py-2 lg:min-h-dvh lg:w-[216px] lg:flex-col lg:items-stretch lg:gap-0 lg:px-3 lg:py-[18px]"
      style="background-image: linear-gradient(180deg, #0b1210, #132a20)"
    >
      <router-link to="/" class="hidden items-center gap-2.5 px-2 pb-[18px] lg:flex" title="Voltar ao app">
        <span
          class="flex h-[34px] w-[34px] items-center justify-center rounded-full border-[1.5px] border-lime bg-lime/10 text-lime"
        >
          <NavIcon name="ball" :size="18" :stroke-width="1.7" />
        </span>
        <span class="font-condensed text-[15px] font-bold leading-[1.1] tracking-[.1em] text-lime">
          EASY FUT
        </span>
      </router-link>

      <nav class="flex flex-row gap-1 lg:flex-col lg:gap-0.5">
        <router-link
          v-for="item in items"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-2.5 whitespace-nowrap rounded-[10px] px-3 py-2 text-[13.5px]"
          :class="route.path === item.to ? 'bg-lime/15 font-semibold text-lime' : 'font-medium text-white/70 hover:text-white'"
        >
          <NavIcon :name="item.icon" :size="17" />
          {{ item.label }}
        </router-link>
      </nav>

      <div class="flex-1" />

      <div class="hidden items-center gap-2.5 border-t border-white/10 px-2 py-2.5 lg:flex">
        <Avatar :name="auth.user?.name ?? ''" color="#2EE07C" size="sm" />
        <div class="min-w-0 flex-1">
          <div class="truncate text-[13px] font-semibold text-white">{{ auth.firstName }}</div>
          <div class="text-[10.5px] font-medium text-white/50">Administrador</div>
        </div>
        <button type="button" class="text-white/50 hover:text-white" title="Sair" @click="logout">
          <NavIcon name="x" :size="14" :stroke-width="2" />
        </button>
      </div>
    </aside>

    <div class="min-w-0 flex-1 p-4 lg:overflow-y-auto lg:p-7">
      <div class="mb-4 flex items-center justify-between gap-3">
        <div>
          <h1 class="text-[22px] font-bold text-ink lg:text-2xl lg:font-extrabold"><slot name="title" /></h1>
          <p class="text-[13.5px] text-ink2"><slot name="subtitle" /></p>
        </div>
        <div class="flex items-center gap-2.5">
          <button
            type="button"
            class="flex h-10 w-10 items-center justify-center rounded-[11px] border border-border bg-surface text-ink2"
            :title="theme.theme === 'dark' ? 'Tema claro' : 'Tema escuro'"
            @click="theme.toggle()"
          >
            <svg v-if="theme.theme === 'dark'" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <circle cx="12" cy="12" r="4.5" />
              <path d="M12 2.5v2.5M12 19v2.5M2.5 12H5M19 12h2.5M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M19.1 4.9l-1.8 1.8M6.7 17.3l-1.8 1.8" />
            </svg>
            <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M20 14.5A8.5 8.5 0 019.5 4a8.5 8.5 0 1010.5 10.5z" />
            </svg>
          </button>
          <slot name="actions" />
        </div>
      </div>
      <slot />
    </div>
  </div>
</template>
