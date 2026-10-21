<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getGuestBooking,
  httpStatus,
  publicErrorMessage,
  readBookingToken,
  type GuestBooking,
  type GuestBookingStatus,
} from '../../api/publicGuests'
import { formatCents, formatDateLong, formatHour, formatRemaining } from '../../lib/format'
import { safeUrl } from '../../lib/security'
import PublicShell from '../../components/layout/PublicShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Badge from '../../components/ui/Badge.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const POLL_INTERVAL_MS = 5000 // contrato: polling do endpoint público a cada 5 s

const route = useRoute()
const router = useRouter()
const bookingId = String(route.params.id ?? '')
const token = readBookingToken(bookingId)

const booking = ref<GuestBooking | null>(null)
const notFound = ref(false)
const loadError = ref('')
const now = ref(Date.now())
const copied = ref(false)
const copyError = ref('')

let pollTimer: ReturnType<typeof setTimeout> | undefined
let clockTimer: ReturnType<typeof setInterval> | undefined
let copiedTimer: ReturnType<typeof setTimeout> | undefined
let stopped = false

// Enquanto o pagamento não sai, a tela consulta a API; o retorno do navegador
// não confirma nada, só o status que a API devolve.
const WAITING: GuestBookingStatus[] = ['reserved', 'pending_payment']
const waiting = computed(() => !!booking.value && WAITING.includes(booking.value.status))

async function fetchBooking() {
  if (!token) return
  try {
    booking.value = await getGuestBooking(bookingId, token)
    loadError.value = ''
  } catch (e) {
    const status = httpStatus(e)
    if (status === 404 || status === 401 || status === 403) {
      notFound.value = true
      stopPolling()
      return
    }
    // Falha passageira (rede): mantém o que já tinha e tenta no próximo ciclo.
    if (!booking.value) loadError.value = publicErrorMessage(e)
  }
}

async function poll() {
  await fetchBooking()
  if (stopped) return
  if (waiting.value || (!booking.value && !notFound.value)) {
    pollTimer = setTimeout(poll, POLL_INTERVAL_MS)
  } else {
    stopPolling()
  }
}

function stopPolling() {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = undefined
  if (clockTimer) clearInterval(clockTimer)
  clockTimer = undefined
}

onMounted(() => {
  if (!token) return
  clockTimer = setInterval(() => (now.value = Date.now()), 1000)
  void poll()
})

onBeforeUnmount(() => {
  stopped = true
  stopPolling()
  if (copiedTimer) clearTimeout(copiedTimer)
})

const remaining = computed(() => (booking.value ? formatRemaining(booking.value.expires_at, now.value) : ''))
const timeIsUp = computed(() => !!booking.value && new Date(booking.value.expires_at).getTime() <= now.value)

async function copyPix() {
  const payload = booking.value?.pix_payload
  if (!payload) return
  copyError.value = ''
  try {
    await navigator.clipboard.writeText(payload)
    copied.value = true
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => (copied.value = false), 2000)
  } catch {
    copyError.value = 'Não deu pra copiar. Segura o código acima e copia na mão.'
  }
}

const matchTime = computed(() => {
  const m = booking.value?.match
  if (!m?.start_time) return ''
  return m.end_time ? `${formatHour(m.start_time)} às ${formatHour(m.end_time)}` : formatHour(m.start_time)
})
</script>

<template>
  <PublicShell title="Sua vaga avulsa">
    <!-- Sem token neste aparelho -->
    <Card v-if="!token" class="p-5 text-center">
      <div class="text-[17px] font-bold">Não achamos sua inscrição aqui</div>
      <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
        O acompanhamento só abre no mesmo aparelho e navegador em que você fez a inscrição.
        Se já pagou, fica tranquilo: a confirmação chega no seu WhatsApp.
      </p>
      <BaseButton class="mt-4 w-full" variant="outline-brand" @click="router.push({ name: 'guest-matches' })">
        Ver partidas com vaga
      </BaseButton>
    </Card>

    <Card v-else-if="notFound" class="p-5 text-center">
      <div class="text-[17px] font-bold">Inscrição não encontrada</div>
      <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">Esse link não bate com nenhuma inscrição.</p>
      <BaseButton class="mt-4 w-full" variant="outline-brand" @click="router.push({ name: 'guest-matches' })">
        Ver partidas com vaga
      </BaseButton>
    </Card>

    <Card v-else-if="loadError" class="p-4">
      <p class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ loadError }}</p>
    </Card>

    <div v-else-if="!booking" class="py-10 text-center text-sm text-ink3">Carregando…</div>

    <template v-else>
      <!-- Aguardando pagamento -->
      <Card v-if="waiting" class="p-4" data-test="pending">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <SectionLabel>Pague com Pix</SectionLabel>
          <Badge tone="warn" dot>Aguardando pagamento</Badge>
        </div>

        <template v-if="booking.status === 'reserved' && !booking.pix_payload">
          <p class="mt-4 text-[14px] text-ink2">Gerando seu Pix… segura aí uns segundos.</p>
        </template>
        <template v-else>
          <div class="mt-3.5 flex items-center gap-4">
            <div class="flex h-[136px] w-[136px] shrink-0 items-center justify-center rounded-[14px] border border-border bg-white p-2">
              <img
                v-if="booking.pix_qr_code_base64"
                :src="safeUrl(`data:image/png;base64,${booking.pix_qr_code_base64}`)"
                alt="QR Code para pagamento via Pix"
                class="h-full w-full object-contain"
              />
            </div>
            <div class="flex flex-1 flex-col gap-1">
              <div class="font-condensed text-[34px] font-bold leading-none">{{ formatCents(booking.total_cents) }}</div>
              <div v-if="!timeIsUp" class="text-[13px] text-ink2">
                Vaga guardada por mais <strong class="font-condensed text-[16px] text-warn" data-test="countdown">{{ remaining }}</strong>
              </div>
              <div v-else class="text-[13px] font-semibold text-warn">O tempo acabou. Conferindo o pagamento…</div>
            </div>
          </div>

          <div v-if="booking.pix_payload" class="mt-3.5 flex items-center gap-2 rounded-xl border border-border bg-surface2 px-3 py-2.5">
            <span class="flex-1 select-all overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-ink2">
              {{ booking.pix_payload }}
            </span>
            <button type="button" class="inline-flex shrink-0 items-center gap-1.5 text-[12.5px] font-bold text-brand" @click="copyPix">
              <NavIcon name="copy" :size="14" :stroke-width="1.9" />
              {{ copied ? 'Copiado!' : 'Copiar' }}
            </button>
          </div>
          <p v-if="copyError" class="mt-2 text-[12.5px] text-danger">{{ copyError }}</p>
          <a
            v-if="safeUrl(booking.pix_ticket_url)"
            :href="safeUrl(booking.pix_ticket_url)"
            target="_blank"
            rel="noopener noreferrer"
            class="mt-3 inline-flex text-[12.5px] font-bold text-brand"
          >
            Abrir instruções de pagamento
          </a>
        </template>

        <p class="mt-3.5 text-[12.5px] leading-relaxed text-ink3">
          Pagou? Fica nesta tela: ela atualiza sozinha quando o pagamento cair.
        </p>
      </Card>

      <!-- Confirmada -->
      <Card v-else-if="booking.status === 'confirmed'" class="p-5" data-test="confirmed">
        <div class="flex items-center gap-3">
          <span class="flex h-11 w-11 items-center justify-center rounded-full bg-brand text-brandInk">
            <NavIcon name="check" :size="22" :stroke-width="2.6" />
          </span>
          <div>
            <div class="text-[19px] font-bold">Tá confirmado!</div>
            <div class="text-[13px] text-ink2">Pagamento aprovado, sua vaga tá garantida.</div>
          </div>
        </div>
        <div class="mt-4 flex flex-col gap-2 rounded-xl bg-surface2 px-3.5 py-3 text-[13.5px]">
          <div class="font-bold">{{ formatDateLong(booking.match.match_date) }}</div>
          <div v-if="matchTime" class="text-ink2">{{ matchTime }}</div>
          <div v-if="booking.match.venue || booking.match.address" class="text-ink2">
            <span class="font-semibold text-ink">{{ booking.match.venue }}</span>
            <template v-if="booking.match.address"><br />{{ booking.match.address }}</template>
          </div>
        </div>
        <p class="mt-4 text-[13.5px] leading-relaxed text-ink2">
          Você vai receber a confirmação e, depois do jogo, o link para votar pelo WhatsApp.
        </p>
      </Card>

      <!-- Expirada -->
      <Card v-else-if="booking.status === 'expired'" class="p-5" data-test="expired">
        <Badge tone="neutral">Reserva expirada</Badge>
        <div class="mt-3 text-[18px] font-bold">Sua reserva expirou</div>
        <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
          O Pix não foi pago em 30 minutos e a vaga foi liberada. Se ainda tiver vaga, dá pra tentar de novo.
        </p>
        <BaseButton class="mt-4 w-full" size="lg" @click="router.push({ name: 'guest-matches' })">Tentar de novo</BaseButton>
      </Card>

      <!-- Devolução a fazer -->
      <Card v-else-if="booking.status === 'refund_pending'" class="p-5" data-test="refund_pending">
        <Badge tone="warn">Devolução em andamento</Badge>
        <div class="mt-3 text-[18px] font-bold">Vamos devolver seu dinheiro</div>
        <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
          Essa vaga não vai valer: a partida foi cancelada ou o Pix caiu depois que a reserva já tinha acabado.
          O organizador vai devolver {{ formatCents(booking.total_cents) }} pra conta que fez o pagamento.
        </p>
      </Card>

      <!-- Devolvida -->
      <Card v-else-if="booking.status === 'refunded'" class="p-5" data-test="refunded">
        <Badge tone="success">Valor devolvido</Badge>
        <div class="mt-3 text-[18px] font-bold">Dinheiro devolvido</div>
        <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
          Os {{ formatCents(booking.total_cents) }} já voltaram pra conta que fez o pagamento.
        </p>
      </Card>

      <!-- Cancelada -->
      <Card v-else-if="booking.status === 'cancelled'" class="p-5" data-test="cancelled">
        <Badge tone="danger">Cancelada</Badge>
        <div class="mt-3 text-[18px] font-bold">Inscrição cancelada</div>
        <p class="mt-2 text-[13.5px] leading-relaxed text-ink2">
          Essa inscrição foi cancelada e a vaga não vale mais. Se você chegou a pagar e o dinheiro não voltou,
          fala com o organizador do grupo.
        </p>
        <BaseButton class="mt-4 w-full" variant="outline-brand" @click="router.push({ name: 'guest-matches' })">
          Ver partidas com vaga
        </BaseButton>
      </Card>
    </template>
  </PublicShell>
</template>
