<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, errorMessage } from '../api/client'
import type { MemberProfile, ProfilePreviewPerson } from '../api/types'
import { formatMonthYearShort } from '../lib/format'
import Avatar from './ui/Avatar.vue'
import Modal from './ui/Modal.vue'

const props = defineProps<{ person: ProfilePreviewPerson | null }>()
const emit = defineEmits<{ close: [] }>()
const profile = ref<MemberProfile | null>(null)
const loading = ref(false)
const error = ref('')
let request = 0

function onKeydown(event: KeyboardEvent) {
  if (props.person && event.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

watch(() => props.person, async (person) => {
  const current = ++request
  profile.value = null
  error.value = ''
  loading.value = false
  if (!person || person.kind === 'guest') return

  loading.value = true
  try {
    const { data } = await api.get<MemberProfile>(`/users/${encodeURIComponent(person.user_id)}/profile`)
    if (current === request) profile.value = data
  } catch (cause) {
    if (current === request) error.value = errorMessage(cause)
  } finally {
    if (current === request) loading.value = false
  }
})

</script>

<template>
  <Modal :open="person !== null" title="Perfil do jogador" @close="emit('close')">
    <div v-if="person" class="flex flex-col items-center text-center">
      <Avatar
        :name="profile?.name ?? person.name"
        :color="profile?.avatar_color ?? person.avatar_color"
        :url="profile?.avatar_url ?? person.avatar_url"
        size="lg"
        class="!h-44 !w-44 !text-[54px]"
      />
      <h3 class="mt-4 break-words text-[22px] font-extrabold leading-tight text-ink">{{ profile?.name ?? person.name }}</h3>
      <div class="mt-2 flex flex-wrap items-center justify-center gap-2 text-xs font-semibold">
        <span v-if="person.kind === 'guest'" class="rounded-full bg-infoBg px-3 py-1 text-info">Avulso</span>
        <template v-else-if="profile">
          <span class="rounded-full bg-brandSoft px-3 py-1 text-brand">{{ profile.role === 'admin' ? 'Admin' : 'Jogador' }}</span>
          <span class="rounded-full bg-surface2 px-3 py-1 text-ink2">No grupo desde {{ formatMonthYearShort(profile.created_at.slice(0, 10)) }}</span>
        </template>
      </div>
      <p v-if="loading" role="status" class="mt-5 text-sm text-ink3">Carregando perfil…</p>
      <p v-else-if="error" role="alert" class="mt-5 text-sm text-danger">{{ error }}</p>
      <div v-else-if="profile" class="mt-6 grid w-full grid-cols-3 gap-2">
        <div class="rounded-xl border border-border bg-surface2 px-1 py-4">
          <div class="font-condensed text-[30px] font-bold leading-none">{{ profile.stats.matches_played }}</div>
          <div class="mt-1 text-[10px] font-semibold tracking-wide text-ink3">PARTIDAS</div>
        </div>
        <div class="rounded-xl border border-border bg-surface2 px-1 py-4">
          <div class="font-condensed text-[30px] font-bold leading-none text-brand">{{ profile.stats.top_scorer_count }}</div>
          <div class="mt-1 text-[10px] font-semibold tracking-wide text-ink3">ARTILHEIRO</div>
        </div>
        <div class="rounded-xl border border-border bg-surface2 px-1 py-4">
          <div class="font-condensed text-[30px] font-bold leading-none text-warn">{{ profile.stats.worst_count }}</div>
          <div class="mt-1 text-[10px] font-semibold tracking-wide text-ink3">PERNA DE PAU</div>
        </div>
      </div>
      <p v-else-if="person.kind === 'guest'" class="mt-5 text-sm text-ink3">Participou como avulso nesta partida.</p>
    </div>
  </Modal>
</template>
