<script setup lang="ts">
// Inscrições avulsas (admin): filtros por partida, estado e nome/WhatsApp;
// reenvio das mensagens de WhatsApp e baixa de reembolso. Cada linha é uma
// inscrição: a mesma pessoa pode aparecer em várias partidas.
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import type { AdminGuestBooking, GuestBookingStatus, GuestMessageKind } from '../../api/adminGuests'
import type { Match } from '../../api/types'
import { formatCents, formatDateShort, formatHour, formatPhoneBR, formatTimestamp } from '../../lib/format'
import { GUEST_STATUSES, GUEST_STATUS_INFO } from '../../lib/guests'
import type { SearchOption } from '../../lib/search'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import Modal from '../../components/ui/Modal.vue'
import SearchSelect from '../../components/ui/SearchSelect.vue'

const route = useRoute()

const bookings = ref<AdminGuestBooking[]>([])
const matches = ref<Match[]>([])
const loaded = ref(false)
const loadError = ref('')

const matchId = ref<string | null>(typeof route.query.match_id === 'string' ? route.query.match_id : null)
const status = ref<GuestBookingStatus | ''>('')
const search = ref(typeof route.query.q === 'string' ? route.query.q : '')

// Retorno das ações, mostrado na própria linha.
const busy = ref<string | null>(null)
const feedback = ref<Record<string, { ok?: string; error?: string }>>({})

// Confirmações em modal: link de voto (gira o token) e reembolso.
const voteLinkFor = ref<AdminGuestBooking | null>(null)
const refundFor = ref<AdminGuestBooking | null>(null)
const modalError = ref('')

onMounted(async () => {
  await Promise.all([loadMatches(), load()])
})

let debounce: ReturnType<typeof setTimeout> | undefined
watch(search, () => {
  clearTimeout(debounce)
  debounce = setTimeout(load, 300)
})
watch([matchId, status], load)

async function loadMatches() {
  try {
    const { data } = await api.get<Match[]>('/matches')
    matches.value = [...(data ?? [])].sort((a, b) => b.match_date.localeCompare(a.match_date))
  } catch {
    matches.value = [] // o filtro de partida fica vazio; a lista ainda funciona
  }
}

async function load() {
  loadError.value = ''
  try {
    const { data } = await api.get<AdminGuestBooking[]>('/admin/guest-bookings', {
      params: {
        match_id: matchId.value || undefined,
        status: status.value || undefined,
        q: search.value.trim() || undefined,
      },
    })
    bookings.value = data ?? []
  } catch (e) {
    loadError.value = errorMessage(e)
  } finally {
    loaded.value = true
  }
}

const matchOptions = computed<SearchOption[]>(() =>
  matches.value.map((m) => ({
    value: m.id,
    label: `${formatDateShort(m.match_date)} · ${m.venue}`,
    hint: m.guest_confirmed_count ? `${m.guest_confirmed_count} avulsos` : undefined,
  })),
)

const summary = computed(() => {
  const n = bookings.value.length
  const refunds = bookings.value.filter((b) => b.status === 'refund_pending').length
  const base = `${n} ${n === 1 ? 'inscrição' : 'inscrições'}`
  return refunds ? `${base} · ${refunds} para reembolsar` : base
})

function matchLabel(b: AdminGuestBooking): string {
  return `${formatDateShort(b.match_date)} · ${formatHour(b.start_time)} · ${b.venue}`
}

function sentLabel(ts: string | null, kind: GuestMessageKind): string {
  const sent = kind === 'confirmation' ? 'enviada' : 'enviado'
  return ts ? `${sent} ${formatTimestamp(ts)}` : `não ${sent}`
}

async function resend(b: AdminGuestBooking, kind: GuestMessageKind): Promise<boolean> {
  busy.value = `${b.id}:${kind}`
  feedback.value[b.id] = {}
  try {
    await api.post(`/admin/guest-bookings/${b.id}/resend`, { kind })
    feedback.value[b.id] = {
      ok: kind === 'confirmation'
        ? 'Confirmação agendada. O envio pode levar até 5 minutos.'
        : 'Novo link de voto agendado. O anterior não vale mais; o envio pode levar até 5 minutos.',
    }
    await load()
    return true
  } catch (e) {
    feedback.value[b.id] = { error: errorMessage(e) }
    return false
  } finally {
    busy.value = null
  }
}

async function confirmVoteLink() {
  if (!voteLinkFor.value) return
  const b = voteLinkFor.value
  voteLinkFor.value = null
  await resend(b, 'vote_link')
}

function openRefund(b: AdminGuestBooking) {
  modalError.value = ''
  refundFor.value = b
}

async function confirmRefund() {
  if (!refundFor.value) return
  const b = refundFor.value
  busy.value = `${b.id}:refund`
  modalError.value = ''
  try {
    await api.post(`/admin/guest-bookings/${b.id}/refunded`)
    refundFor.value = null
    feedback.value[b.id] = { ok: 'Reembolso registrado e lançado no caixa.' }
    await load()
  } catch (e) {
    modalError.value = errorMessage(e)
  } finally {
    busy.value = null
  }
}
</script>

<template>
  <AdminLayout>
    <template #title>Avulsos</template>
    <template #subtitle>{{ loaded ? summary : 'Inscrições de quem pagou uma vaga avulsa' }}</template>

    <div class="flex flex-col gap-4">
      <!-- Filtros -->
      <div class="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_200px]">
        <SearchSelect
          v-model="matchId"
          :options="matchOptions"
          label="Partida"
          placeholder="Todas as partidas"
          search-placeholder="Buscar por data ou local…"
          empty-text="Nenhuma partida encontrada."
          clearable
        />
        <div class="flex h-12 items-center gap-2.5 rounded-xl border border-border bg-surface px-3.5 shadow-card">
          <NavIcon name="search" :size="15" :stroke-width="1.9" class="shrink-0 text-ink3" />
          <input
            v-model="search"
            type="search"
            aria-label="Buscar por nome ou WhatsApp"
            placeholder="Nome ou WhatsApp…"
            class="w-full min-w-0 bg-transparent text-[13.5px] text-ink outline-none placeholder:text-ink3"
          />
        </div>
        <select v-model="status" aria-label="Estado da inscrição" class="h-12 rounded-xl border border-border bg-surface px-3.5 text-[13.5px] font-medium text-ink2 shadow-card outline-none">
          <option value="">Estado: Todos</option>
          <option v-for="s in GUEST_STATUSES" :key="s" :value="s">{{ GUEST_STATUS_INFO[s].label }}</option>
        </select>
      </div>

      <p v-if="loadError" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ loadError }}</p>

      <!-- Lista -->
      <div v-if="bookings.length" class="grid gap-3 lg:grid-cols-2">
        <Card v-for="b in bookings" :key="b.id" class="flex flex-col gap-2.5 px-4 py-3.5" data-test="booking">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="truncate text-[15px] font-bold">{{ b.name }}</div>
              <div class="text-[12.5px] text-ink2">{{ matchLabel(b) }}</div>
            </div>
            <Badge :tone="GUEST_STATUS_INFO[b.status].tone" dot>{{ GUEST_STATUS_INFO[b.status].label }}</Badge>
          </div>

          <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[12.5px]">
            <dt class="text-ink3">WhatsApp</dt>
            <dd class="font-medium">{{ formatPhoneBR(b.phone) }}</dd>
            <dt class="text-ink3">E-mail</dt>
            <dd class="truncate font-medium">{{ b.email || '—' }}</dd>
            <dt class="text-ink3">Total</dt>
            <dd class="font-medium">
              {{ formatCents(b.total_cents) }}
              <span class="text-ink3">· {{ formatCents(b.base_cents) }} para o caixa</span>
            </dd>
            <dt class="text-ink3">Confirmação</dt>
            <dd :class="b.confirmation_sent_at ? 'text-ink2' : 'text-ink3'">{{ sentLabel(b.confirmation_sent_at, 'confirmation') }}</dd>
            <dt class="text-ink3">Link de voto</dt>
            <dd :class="b.vote_link_sent_at ? 'text-ink2' : 'text-ink3'">{{ sentLabel(b.vote_link_sent_at, 'vote_link') }}</dd>
            <template v-if="b.refund_reason">
              <dt class="text-ink3">Motivo</dt>
              <dd class="text-ink2">{{ b.refund_reason }}</dd>
            </template>
          </dl>

          <div
            v-if="b.status === 'confirmed' || b.status === 'refund_pending'"
            class="flex flex-wrap gap-2 border-t border-border pt-2.5"
          >
            <template v-if="b.status === 'confirmed'">
              <BaseButton
                size="sm"
                variant="outline"
                :loading="busy === `${b.id}:confirmation`"
                @click="resend(b, 'confirmation')"
              >
                Reenviar confirmação
              </BaseButton>
              <BaseButton
                size="sm"
                variant="outline"
                :loading="busy === `${b.id}:vote_link`"
                @click="voteLinkFor = b"
              >
                Reenviar link de voto
              </BaseButton>
            </template>
            <BaseButton
              v-if="b.status === 'refund_pending'"
              size="sm"
              variant="outline-brand"
              :loading="busy === `${b.id}:refund`"
              @click="openRefund(b)"
            >
              Marcar reembolso feito
            </BaseButton>
          </div>
          <p v-if="feedback[b.id]?.ok" class="text-[12.5px] font-semibold text-brand">{{ feedback[b.id].ok }}</p>
          <p v-if="feedback[b.id]?.error" class="rounded-lg bg-dangerBg px-3 py-2 text-[12.5px] font-medium text-danger">
            {{ feedback[b.id].error }}
          </p>
        </Card>
      </div>
      <Card v-else-if="loaded && !loadError">
        <EmptyState>Nenhuma inscrição avulsa com esses filtros.</EmptyState>
      </Card>
    </div>

    <!-- Reenvio do link de voto: o token gira -->
    <Modal :open="voteLinkFor !== null" title="Reenviar link de voto" @close="voteLinkFor = null">
      <div v-if="voteLinkFor" class="flex flex-col gap-4">
        <p class="text-sm leading-relaxed text-ink2">
          Um link novo vai para o WhatsApp de <strong class="text-ink">{{ voteLinkFor.name }}</strong>
          ({{ formatPhoneBR(voteLinkFor.phone) }}). <strong class="text-ink">O link enviado antes deixa de valer.</strong>
          O link só aceita votos enquanto a votação da partida estiver aberta.
        </p>
        <BaseButton @click="confirmVoteLink">Enviar link novo</BaseButton>
      </div>
    </Modal>

    <!-- Reembolso -->
    <Modal :open="refundFor !== null" title="Marcar reembolso feito" @close="refundFor = null">
      <div v-if="refundFor" class="flex flex-col gap-4">
        <p class="text-sm leading-relaxed text-ink2">
          Confirme que você já estornou <strong class="text-ink">{{ formatCents(refundFor.total_cents) }}</strong>
          para <strong class="text-ink">{{ refundFor.name }}</strong> no Mercado Pago. A saída entra no caixa e a
          inscrição fica como reembolsada.
        </p>
        <p v-if="modalError" class="rounded-lg bg-dangerBg px-3 py-2 text-[12.5px] font-medium text-danger">{{ modalError }}</p>
        <BaseButton :loading="busy === `${refundFor.id}:refund`" @click="confirmRefund">Já estornei</BaseButton>
      </div>
    </Modal>
  </AdminLayout>
</template>
