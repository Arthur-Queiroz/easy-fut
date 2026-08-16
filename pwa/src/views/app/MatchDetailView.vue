<script setup lang="ts">
import { useRoute } from 'vue-router'
import MobileShell from '../../components/layout/MobileShell.vue'
import MatchDetailContent from '../../components/MatchDetailContent.vue'

const route = useRoute()
const matchId = route.params.matchId as string
</script>

<template>
  <MobileShell>
    <div class="px-4 pt-3.5 lg:px-0 lg:pt-0">
      <MatchDetailContent :match-id="matchId" />
    </div>
  </MobileShell>
</template>
