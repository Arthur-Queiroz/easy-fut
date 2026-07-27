<script setup lang="ts">
import InstallPrompt from './components/pwa/InstallPrompt.vue'
import { useThemeStore } from './stores/theme'

useThemeStore().apply()
</script>

<template>
  <router-view />
  <InstallPrompt />
</template>
