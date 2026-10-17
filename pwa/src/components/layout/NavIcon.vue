<script setup lang="ts">
// Ícones de linha do mockup, indexados por nome.
defineProps<{ name: string; size?: number; strokeWidth?: number }>()

const paths: Record<string, string[]> = {
  home: ['M3 11.5L12 4l9 7.5', 'M5.5 10.5V20h13v-9.5'],
  ball: ['M12 7l4.7 3.4-1.8 5.6H9.1L7.3 10.4z'],
  pix: [],
  clock: ['M12 7.5V12l3 2'],
  user: ['M5 19.5c1.5-3.2 4-4.8 7-4.8s5.5 1.6 7 4.8'],
  grid: [],
  users: ['M3.5 19c1.2-2.8 3.2-4.2 5.5-4.2s4.3 1.4 5.5 4.2M15.5 5.6a3.2 3.2 0 010 5.8M17.5 14.9c1.6.6 2.7 1.9 3.4 4.1'],
  chat: ['M4 6a3 3 0 013-3h10a3 3 0 013 3v7a3 3 0 01-3 3H9l-5 4z'],
  gear: ['M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M18.4 5.6l-2.1 2.1M7.7 16.3l-2.1 2.1'],
  trophy: ['M7 4h10v5a5 5 0 01-10 0z', 'M7 5H4.5a3 3 0 003 4M17 5h2.5a3 3 0 01-3 4M12 14v3M8.5 20h7'],
  camera: ['M8.5 7l1.5-2.5h4L15.5 7'],
  copy: ['M5 15V6a2 2 0 012-2h9'],
  check: ['M4.5 12.5l5 5 10-11'],
  plus: ['M12 5v14M5 12h14'],
  x: ['M6 6l12 12M18 6L6 18'],
  search: ['M20 20l-4.3-4.3'],
  redo: ['M20 12a8 8 0 11-2.3-5.6M20 4v4h-4'],
  link: ['M9 14.5L14.5 9M8 12l-2.5 2.5a3.5 3.5 0 005 5L13 17M11 6.5l2.5-2.5a3.5 3.5 0 015 5L16 11.5'],
  back: ['M15 5l-7 7 7 7'],
  mail: ['M3.5 6.5L12 13l8.5-6.5'],
  download: ['M12 4v10m0 0l-4-4m4 4l4-4M4 19h16'],
}
</script>

<template>
  <svg
    :width="size ?? 21"
    :height="size ?? 21"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth ?? 1.8"
  >
    <circle v-if="name === 'ball'" cx="12" cy="12" r="9" />
    <circle v-if="name === 'clock'" cx="12" cy="12" r="8.5" />
    <circle v-if="name === 'user'" cx="12" cy="8.5" r="3.5" />
    <circle v-if="name === 'users'" cx="9" cy="8.5" r="3.2" />
    <circle v-if="name === 'gear'" cx="12" cy="12" r="3" />
    <circle v-if="name === 'search'" cx="11" cy="11" r="6.5" />
    <rect v-if="name === 'pix'" x="7.05" y="7.05" width="9.9" height="9.9" rx="2.4" transform="rotate(45 12 12)" />
    <rect v-if="name === 'copy'" x="9" y="9" width="11" height="11" rx="2" />
    <rect v-if="name === 'mail'" x="3" y="5" width="18" height="14" rx="2.5" />
    <template v-if="name === 'grid'">
      <rect x="3.5" y="3.5" width="7.5" height="7.5" rx="2" />
      <rect x="13" y="3.5" width="7.5" height="7.5" rx="2" />
      <rect x="3.5" y="13" width="7.5" height="7.5" rx="2" />
      <rect x="13" y="13" width="7.5" height="7.5" rx="2" />
    </template>
    <template v-if="name === 'camera'">
      <rect x="3.5" y="7" width="17" height="13" rx="2.5" />
      <circle cx="12" cy="13" r="3.5" />
    </template>
    <path v-for="d in paths[name] ?? []" :key="d" :d="d" />
  </svg>
</template>
