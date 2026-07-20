<script setup lang="ts">
import { useThemeStore } from './stores/theme'

useThemeStore().apply()
</script>

<template>
  <router-view />
</template>
