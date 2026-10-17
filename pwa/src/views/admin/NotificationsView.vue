<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { PushBroadcast, PushRecipient } from '../../api/types'
import { formatTimestamp } from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import Modal from '../../components/ui/Modal.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const DEFAULT_TITLE = 'Aviso do Easy Fut'
const MAX_TITLE = 60
const MAX_BODY = 240

const pushEnabled = ref(true)
const recipients = ref<PushRecipient[]>([])
const history = ref<PushBroadcast[]>([])
const title = ref('')
const body = ref('')
const audience = ref<'all' | 'selected'>('all')
const selected = ref<string[]>([])
const search = ref('')
const confirming = ref(false)
const sending = ref(false)
const error = ref('')
const success = ref('')

onMounted(async () => {
  const [config] = await Promise.all([api.get<{ enabled: boolean }>('/push/config'), loadRecipients(), loadHistory()])
  pushEnabled.value = config.data.enabled
})

async function loadRecipients() {
  const { data } = await api.get<PushRecipient[]>('/admin/push/recipients')
  recipients.value = data ?? []
}

async function loadHistory() {
  const { data } = await api.get<PushBroadcast[]>('/admin/push/broadcasts')
  history.value = data ?? []
}

/** Conta caracteres como a API (code points), para emojis não estourarem o limite. */
const length = (text: string) => [...text.trim()].length

const normalize = (text: string) =>
  text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()

const filtered = computed(() => {
  const term = normalize(search.value.trim())
  return term ? recipients.value.filter((r) => normalize(r.name).includes(term)) : recipients.value
})

/** Quem efetivamente recebe: precisa ter ao menos um aparelho com notificações ativas. */
const reached = computed(() =>
  audience.value === 'all'
    ? recipients.value.filter((r) => r.status === 'active' && r.devices > 0)
    : recipients.value.filter((r) => selected.value.includes(r.id) && r.devices > 0),
)
const reachedDevices = computed(() => reached.value.reduce((sum, r) => sum + r.devices, 0))
const selectedWithoutDevice = computed(
  () => recipients.value.filter((r) => selected.value.includes(r.id) && r.devices === 0).length,
)

const previewTitle = computed(() => title.value.trim() || DEFAULT_TITLE)
const titleTooLong = computed(() => length(title.value) > MAX_TITLE)
const bodyTooLong = computed(() => length(body.value) > MAX_BODY)
const canSend = computed(
  () => pushEnabled.value && length(body.value) > 0 && !bodyTooLong.value && !titleTooLong.value && reached.value.length > 0,
)

function toggle(id: string) {
  selected.value = selected.value.includes(id) ? selected.value.filter((s) => s !== id) : [...selected.value, id]
}

function selectAllWithDevices() {
  const ids = filtered.value.filter((r) => r.devices > 0).map((r) => r.id)
  selected.value = [...new Set([...selected.value, ...ids])]
}

function openConfirm() {
  error.value = ''
  success.value = ''
  if (canSend.value) confirming.value = true
}

async function send() {
  sending.value = true
  error.value = ''
  try {
    const { data } = await api.post<PushBroadcast>('/admin/push/broadcasts', {
      title: title.value,
      body: body.value,
      audience: audience.value,
      user_ids: audience.value === 'selected' ? selected.value : [],
    })
    success.value = `Aviso enviado para ${plural(data.users, 'membro', 'membros')} (${plural(data.devices, 'aparelho', 'aparelhos')}). A entrega leva até um minuto.`
    title.value = ''
    body.value = ''
    selected.value = []
    await loadHistory()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    sending.value = false
    confirming.value = false
  }
}

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`
}

function deliverySummary(b: PushBroadcast): string {
  const parts = [`${b.sent} de ${b.devices} entregues`]
  if (b.pending) parts.push(`${b.pending} na fila`)
  if (b.failed) parts.push(`${b.failed} com falha`)
  return parts.join(' · ')
}
</script>

<template>
  <AdminLayout>
    <template #title>Avisos</template>
    <template #subtitle>Notificação no celular para todos os membros ou para quem você escolher</template>

    <div class="flex flex-col gap-4">
      <p
        v-if="!pushEnabled"
        class="rounded-xl bg-warnBg px-4 py-3 text-[13px] font-medium text-warn"
      >
        As notificações ainda não estão configuradas no servidor; não é possível enviar avisos.
      </p>

      <Card class="px-5 py-[18px]">
        <SectionLabel>Novo aviso</SectionLabel>
        <form class="mt-3 flex flex-col gap-4" @submit.prevent="openConfirm">
          <label class="flex flex-col gap-1.5">
            <span class="flex justify-between text-[12.5px] font-semibold text-ink2">
              Título (opcional)
              <span :class="titleTooLong ? 'text-danger' : 'text-ink3'">{{ length(title) }}/{{ MAX_TITLE }}</span>
            </span>
            <input v-model="title" :placeholder="DEFAULT_TITLE" class="field" />
          </label>

          <label class="flex flex-col gap-1.5">
            <span class="flex justify-between text-[12.5px] font-semibold text-ink2">
              Mensagem
              <span :class="bodyTooLong ? 'text-danger' : 'text-ink3'">{{ length(body) }}/{{ MAX_BODY }}</span>
            </span>
            <textarea
              v-model="body"
              rows="3"
              placeholder="Ex.: Jogo de hoje cancelado por causa da chuva. Remarcamos para quinta."
              class="field h-auto resize-y py-2.5"
            />
          </label>

          <fieldset class="flex flex-col gap-2">
            <legend class="mb-1.5 text-[12.5px] font-semibold text-ink2">Destinatários</legend>
            <div class="flex flex-wrap gap-2">
              <label class="choice" :class="audience === 'all' ? 'choice-on' : ''">
                <input v-model="audience" type="radio" value="all" class="sr-only" />
                <NavIcon name="users" :size="15" />
                Todos os membros ativos
              </label>
              <label class="choice" :class="audience === 'selected' ? 'choice-on' : ''">
                <input v-model="audience" type="radio" value="selected" class="sr-only" />
                <NavIcon name="user" :size="15" />
                Escolher membros
              </label>
            </div>

            <div v-if="audience === 'selected'" class="mt-1 rounded-xl border border-border">
              <div class="flex flex-wrap items-center gap-2 border-b border-border p-2.5">
                <input v-model="search" placeholder="Buscar por nome" class="field h-[36px] min-w-[180px] flex-1" />
                <button type="button" class="text-xs font-semibold text-brand" @click="selectAllWithDevices">
                  Marcar todos com notificações
                </button>
                <button v-if="selected.length" type="button" class="text-xs font-semibold text-ink3" @click="selected = []">
                  Limpar
                </button>
              </div>
              <ul class="max-h-[300px] overflow-y-auto">
                <li v-for="r in filtered" :key="r.id" class="border-b border-border last:border-0">
                  <label class="flex cursor-pointer items-center gap-3 px-3 py-2">
                    <input
                      type="checkbox"
                      class="h-4 w-4 shrink-0 accent-brand"
                      :checked="selected.includes(r.id)"
                      @change="toggle(r.id)"
                    />
                    <Avatar :name="r.name" :color="r.avatar_color" size="xs" />
                    <span class="min-w-0 flex-1 truncate text-[13.5px] font-medium">{{ r.name }}</span>
                    <span v-if="r.status === 'inactive'" class="rounded-full bg-surface2 px-2 py-[2px] text-[10.5px] font-semibold text-ink3">
                      Inativo
                    </span>
                    <span
                      class="whitespace-nowrap rounded-full px-2 py-[2px] text-[10.5px] font-semibold"
                      :class="r.devices ? 'bg-brandSoft text-brand' : 'bg-surface2 text-ink3'"
                    >
                      {{ r.devices ? plural(r.devices, 'aparelho', 'aparelhos') : 'sem notificações' }}
                    </span>
                  </label>
                </li>
                <li v-if="filtered.length === 0" class="px-3 py-6 text-center text-sm text-ink3">Ninguém encontrado.</li>
              </ul>
            </div>
          </fieldset>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-[13px] text-ink2">
              <template v-if="reached.length">
                Vai para <strong>{{ plural(reached.length, 'membro', 'membros') }}</strong>
                ({{ plural(reachedDevices, 'aparelho', 'aparelhos') }}).
              </template>
              <template v-else>Nenhum destinatário com notificações ativas.</template>
              <span v-if="audience === 'selected' && selectedWithoutDevice" class="text-ink3">
                {{ plural(selectedWithoutDevice, 'selecionado não receberá', 'selecionados não receberão') }}: não ativaram as notificações.
              </span>
            </p>
            <BaseButton :disabled="!canSend" @click="openConfirm">
              <NavIcon name="bell" :size="15" :stroke-width="2" />
              Revisar e enviar
            </BaseButton>
          </div>
          <p v-if="error" class="text-[13px] font-medium text-danger">{{ error }}</p>
          <p v-if="success" class="text-[13px] font-medium text-brand">{{ success }}</p>
        </form>
      </Card>

      <Card class="overflow-hidden">
        <div class="flex items-center justify-between px-5 pt-[18px]">
          <SectionLabel>Enviados recentemente</SectionLabel>
          <button type="button" class="inline-flex items-center gap-1 text-xs font-semibold text-ink3" @click="loadHistory">
            <NavIcon name="redo" :size="13" /> Atualizar
          </button>
        </div>
        <ul class="mt-2">
          <li v-for="b in history" :key="b.id" class="border-t border-border px-5 py-3">
            <div class="flex flex-wrap items-baseline justify-between gap-x-3">
              <span class="text-[13.5px] font-semibold">{{ b.title }}</span>
              <span class="text-[11.5px] text-ink3">
                {{ formatTimestamp(b.created_at) }} · {{ b.creator_name.split(' ')[0] || 'Admin' }} ·
                {{ b.audience === 'all' ? 'todos os ativos' : plural(b.users, 'membro escolhido', 'membros escolhidos') }}
              </span>
            </div>
            <p class="mt-0.5 whitespace-pre-line text-[13px] text-ink2">{{ b.body }}</p>
            <p class="mt-1 text-[11.5px] font-medium" :class="b.failed ? 'text-warn' : 'text-ink3'">{{ deliverySummary(b) }}</p>
          </li>
          <li v-if="history.length === 0" class="border-t border-border px-5 py-8 text-center text-sm text-ink3">
            Nenhum aviso enviado ainda.
          </li>
        </ul>
      </Card>
    </div>

    <Modal :open="confirming" title="Enviar aviso?" @close="confirming = false">
      <div class="rounded-xl border border-border bg-bg p-3">
        <div class="flex items-start gap-2.5">
          <img src="/icons/pwa-64x64.png" alt="" class="h-8 w-8 rounded-lg" />
          <div class="min-w-0">
            <div class="text-[13px] font-semibold text-ink">{{ previewTitle }}</div>
            <p class="whitespace-pre-line text-[13px] text-ink2">{{ body.trim() }}</p>
          </div>
        </div>
      </div>
      <p class="mt-3 text-[13px] text-ink2">
        Será enviado para {{ plural(reached.length, 'membro', 'membros') }} ({{ plural(reachedDevices, 'aparelho', 'aparelhos') }}).
        Depois de enviado, o aviso não pode ser retirado dos celulares.
      </p>
      <div class="mt-4 flex justify-end gap-2">
        <BaseButton variant="outline" size="sm" @click="confirming = false">Cancelar</BaseButton>
        <BaseButton size="sm" :loading="sending" @click="send">Enviar agora</BaseButton>
      </div>
    </Modal>
  </AdminLayout>
</template>

<style scoped>
.field {
  @apply h-[42px] rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
.choice {
  @apply inline-flex cursor-pointer items-center gap-2 rounded-[11px] border-[1.5px] border-border px-3.5 py-2 text-[13px] font-semibold text-ink2;
}
.choice-on {
  @apply border-brand bg-brandSoft text-brand;
}
</style>
