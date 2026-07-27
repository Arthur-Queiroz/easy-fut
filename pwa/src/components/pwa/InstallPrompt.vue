<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

type InstallPromptEvent = Event & {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const deferredPrompt = ref<InstallPromptEvent | null>(null)
const dismissed = ref(false)

const shouldShow = computed(() => deferredPrompt.value !== null && !dismissed.value)

function isStandalone() {
  return window.matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true
}

function handleBeforeInstallPrompt(event: Event) {
  event.preventDefault()
  if (!isStandalone()) deferredPrompt.value = event as InstallPromptEvent
}

function handleAppInstalled() {
  deferredPrompt.value = null
}

function dismiss() {
  dismissed.value = true
}

async function install() {
  const prompt = deferredPrompt.value
  if (!prompt) return

  await prompt.prompt()
  await prompt.userChoice
  deferredPrompt.value = null
}

onMounted(() => {
  window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
  window.addEventListener('appinstalled', handleAppInstalled)
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
  window.removeEventListener('appinstalled', handleAppInstalled)
})
</script>

<template>
  <aside v-if="shouldShow" class="fixed inset-x-4 bottom-4 z-50 mx-auto max-w-sm rounded-2xl border border-white/15 bg-pitch-1 p-4 text-white shadow-2xl md:border-border md:bg-surface md:text-ink">
    <button type="button" class="absolute right-2 top-2 flex size-8 items-center justify-center rounded-lg text-white/60 hover:bg-white/10 hover:text-white md:text-ink3 md:hover:bg-surface2 md:hover:text-ink" aria-label="Fechar aviso de instalação" @click="dismiss">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <path d="M6 6l12 12M18 6L6 18" />
      </svg>
    </button>

    <div class="pr-8">
      <p class="font-condensed text-xl font-bold">Instale o app</p>
      <p class="mt-1 text-sm text-white/70 md:text-ink2">Acesse o Fut da Rapaziada pela tela inicial do celular.</p>
    </div>
    <button type="button" class="mt-4 h-11 w-full rounded-xl bg-lime font-bold text-pitch-2" @click="install">Instalar app</button>
  </aside>
</template>
