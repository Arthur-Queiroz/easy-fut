<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Media } from '../api/types'
import { safeUrl } from '../lib/security'

const props = defineProps<{ media: Media[]; initialIndex: number }>()
const emit = defineEmits<{ close: [] }>()

const index = ref(props.initialIndex)
const dialog = ref<HTMLElement | null>(null)
const photoViewport = ref<HTMLElement | null>(null)
const zoom = ref(1)
const item = computed(() => props.media[index.value])
const fileURL = computed(() => safeUrl(item.value?.url))
const downloadName = computed(() => {
  if (!item.value) return ''
  const extension = fileURL.value.match(/\.(jpg|jpeg|png|webp|gif|mp4|mov|webm)$/i)?.[1]?.toLowerCase() ?? (item.value.type === 'video' ? 'mp4' : 'jpg')
  return `partida-${item.value.match_id}-${index.value + 1}.${extension}`
})

function move(delta: number) {
  if (!props.media.length) return
  index.value = (index.value + delta + props.media.length) % props.media.length
  zoom.value = 1
}

async function setZoom(value: number) {
  zoom.value = Math.min(4, Math.max(1, value))
  await nextTick()
  if (photoViewport.value) {
    photoViewport.value.scrollLeft = (photoViewport.value.scrollWidth - photoViewport.value.clientWidth) / 2
    photoViewport.value.scrollTop = (photoViewport.value.scrollHeight - photoViewport.value.clientHeight) / 2
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
    return
  }
  // Setas no player devem continuar controlando o vídeo.
  if (event.target instanceof HTMLVideoElement) return
  if (item.value?.type === 'photo' && (event.key === '+' || event.key === '=' || event.key === '-')) {
    event.preventDefault()
    void setZoom(zoom.value + (event.key === '-' ? -0.5 : 0.5))
  }
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault()
    move(event.key === 'ArrowLeft' ? -1 : 1)
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  await nextTick()
  dialog.value?.focus()
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <div
      ref="dialog"
      role="dialog"
      aria-modal="true"
      aria-label="Visualizador de fotos e vídeos"
      tabindex="-1"
      class="fixed inset-0 z-50 flex flex-col bg-black/95 text-white outline-none"
    >
      <div class="flex items-center justify-between gap-3 px-4 py-3">
        <span class="text-sm font-semibold">{{ index + 1 }} de {{ media.length }}</span>
        <div class="flex items-center gap-1">
          <template v-if="item?.type === 'photo'">
            <button type="button" class="rounded-lg px-2 py-2 text-lg hover:bg-white/15 disabled:opacity-40" aria-label="Diminuir foto" :disabled="zoom === 1" @click="setZoom(zoom - 0.5)">−</button>
            <button type="button" class="rounded-lg px-1 py-2 text-xs font-semibold hover:bg-white/15" aria-label="Ajustar foto à tela" @click="setZoom(1)">{{ Math.round(zoom * 100) }}%</button>
            <button type="button" class="rounded-lg px-2 py-2 text-lg hover:bg-white/15 disabled:opacity-40" aria-label="Aumentar foto" :disabled="zoom === 4" @click="setZoom(zoom + 0.5)">+</button>
          </template>
          <button type="button" class="rounded-lg px-3 py-2 text-sm font-semibold hover:bg-white/15" aria-label="Fechar visualizador" @click="emit('close')">Fechar ✕</button>
        </div>
      </div>

      <div v-if="item" class="flex min-h-0 flex-1 items-center justify-center gap-2 px-2 sm:gap-4 sm:px-4">
        <button v-if="media.length > 1" type="button" class="shrink-0 rounded-full bg-white/10 px-3 py-2 text-xl hover:bg-white/25" aria-label="Mídia anterior" @click="move(-1)">‹</button>
        <div class="flex h-full min-h-0 min-w-0 flex-1 items-center justify-center">
          <div v-if="item.type === 'photo'" ref="photoViewport" class="h-full w-full overflow-auto overscroll-contain">
            <div class="flex items-center justify-center" :style="{ width: `${zoom * 100}%`, height: `${zoom * 100}%` }">
              <img :key="item.id" :src="fileURL" :alt="item.caption || 'Foto da partida'" :draggable="false" class="h-full w-full cursor-zoom-in object-contain" :class="zoom > 1 ? 'cursor-zoom-out' : ''" @click="setZoom(zoom > 1 ? 1 : 2)" />
            </div>
          </div>
          <video v-else :key="item.id" :src="fileURL" class="max-h-[calc(100dvh-10rem)] max-w-full" controls playsinline preload="metadata" />
        </div>
        <button v-if="media.length > 1" type="button" class="shrink-0 rounded-full bg-white/10 px-3 py-2 text-xl hover:bg-white/25" aria-label="Próxima mídia" @click="move(1)">›</button>
      </div>

      <div v-if="item" class="flex flex-wrap items-center justify-between gap-3 px-4 pb-[max(env(safe-area-inset-bottom),1rem)] pt-3">
        <p class="text-sm text-white/75">{{ item.caption || (item.type === 'video' ? 'Vídeo da partida' : 'Foto da partida') }}</p>
        <a v-if="fileURL" :href="fileURL" :download="downloadName" class="rounded-xl bg-brand px-4 py-2.5 text-sm font-bold text-brandInk">Baixar {{ item.type === 'video' ? 'vídeo' : 'foto' }}</a>
      </div>
    </div>
  </Teleport>
</template>
