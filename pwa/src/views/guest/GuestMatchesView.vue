<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  createGuestBooking,
  httpStatus,
  listGuestMatches,
  publicErrorMessage,
  saveBookingToken,
  type GuestMatch,
} from '../../api/publicGuests'
import { formatCents, formatDateLong, formatHour } from '../../lib/format'
import PublicShell from '../../components/layout/PublicShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const router = useRouter()

const matches = ref<GuestMatch[]>([])
const loading = ref(true)
const loadError = ref('')
const selectedId = ref('')

const name = ref('')
const phone = ref('')
const email = ref('')
const formError = ref('')
const submitting = ref(false)

const selected = computed(() => matches.value.find((m) => m.match_id === selectedId.value) ?? null)

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    matches.value = await listGuestMatches()
    if (!matches.value.some((m) => m.match_id === selectedId.value)) {
      selectedId.value = matches.value.length === 1 ? matches.value[0].match_id : ''
    }
  } catch (e) {
    loadError.value = publicErrorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function slotsLabel(n: number): string {
  if (n <= 0) return 'Esgotado'
  return n === 1 ? 'Última vaga!' : `${n} vagas`
}

// Validação leve: a API normaliza o telefone e é a dona das regras.
function validate(): string {
  if (!selected.value) return 'Escolhe uma partida primeiro.'
  if (name.value.trim().length < 2) return 'Coloca seu nome.'
  const digits = phone.value.replace(/\D/g, '')
  if (digits.length < 10 || digits.length > 13) return 'Confere o WhatsApp: DDD + número, tipo (11) 91234-5678.'
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) return 'Confere o e-mail.'
  return ''
}

async function submit() {
  formError.value = validate()
  if (formError.value || !selected.value) return
  submitting.value = true
  try {
    const booking = await createGuestBooking({
      match_id: selected.value.match_id,
      name: name.value.trim(),
      phone: phone.value.trim(),
      email: email.value.trim(),
    })
    saveBookingToken(booking.booking_id, booking.access_token)
    router.push({ name: 'guest-booking', params: { id: booking.booking_id } })
  } catch (e) {
    formError.value = publicErrorMessage(e)
    // Vagas podem ter mudado (esgotou, venda fechou): atualiza a lista sem
    // apagar a mensagem de erro.
    if (httpStatus(e) === 409) void refreshQuietly()
  } finally {
    submitting.value = false
  }
}

async function refreshQuietly() {
  try {
    matches.value = await listGuestMatches()
  } catch {
    /* mantém a lista anterior */
  }
}
</script>

<template>
  <PublicShell title="Jogar uma partida avulsa" subtitle="Escolhe a partida, paga o Pix e tá dentro.">
    <div v-if="loading" class="py-10 text-center text-sm text-ink3">Carregando partidas…</div>

    <Card v-else-if="loadError" class="p-4">
      <p class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">{{ loadError }}</p>
      <BaseButton class="mt-3 w-full" variant="outline" @click="load">Tentar de novo</BaseButton>
    </Card>

    <Card v-else-if="matches.length === 0" class="p-4">
      <EmptyState>Nenhuma partida com vaga avulsa aberta agora. Volta mais tarde!</EmptyState>
    </Card>

    <template v-else>
      <SectionLabel>Partidas com vaga</SectionLabel>
      <div class="flex flex-col gap-2.5" role="radiogroup" aria-label="Partidas com vaga">
        <button
          v-for="m in matches"
          :key="m.match_id"
          type="button"
          role="radio"
          :aria-checked="selectedId === m.match_id"
          :disabled="m.slots_left <= 0"
          data-test="guest-match"
          class="flex flex-col gap-1.5 rounded-2xl bg-surface p-4 text-left shadow-card transition disabled:opacity-60"
          :class="selectedId === m.match_id ? 'border-2 border-brand' : 'border border-border'"
          @click="selectedId = m.match_id"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="text-[15px] font-bold">{{ formatDateLong(m.match_date) }}</div>
            <span class="font-condensed text-[22px] font-bold leading-none text-brand">{{ formatCents(m.total_cents) }}</span>
          </div>
          <div class="flex items-center gap-1.5 text-[13px] text-ink2">
            <NavIcon name="clock" :size="14" />
            {{ formatHour(m.start_time) }}<template v-if="m.end_time"> às {{ formatHour(m.end_time) }}</template>
          </div>
          <div class="text-[13px] text-ink2">
            <span class="font-semibold text-ink">{{ m.venue }}</span>
            <template v-if="m.address"><br />{{ m.address }}</template>
          </div>
          <div class="text-[12.5px] font-semibold" :class="m.slots_left <= 2 ? 'text-warn' : 'text-brand'">
            {{ slotsLabel(m.slots_left) }}
          </div>
        </button>
      </div>

      <Card v-if="selected" class="p-4">
        <SectionLabel>Seus dados</SectionLabel>
        <form class="mt-3 flex flex-col gap-3.5" novalidate @submit.prevent="submit">
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">Nome</span>
            <input
              v-model="name"
              name="name"
              autocomplete="name"
              maxlength="80"
              placeholder="Como te chamam no fut"
              class="h-[50px] rounded-xl border border-border bg-bg px-4 text-[15px] text-ink outline-none placeholder:text-ink3 focus:border-brand"
            />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">WhatsApp</span>
            <input
              v-model="phone"
              name="phone"
              type="tel"
              inputmode="tel"
              autocomplete="tel"
              placeholder="(11) 91234-5678"
              class="h-[50px] rounded-xl border border-border bg-bg px-4 text-[15px] text-ink outline-none placeholder:text-ink3 focus:border-brand"
            />
            <span class="text-[12px] text-ink3">A confirmação e o link pra votar chegam nele.</span>
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">E-mail</span>
            <input
              v-model="email"
              name="email"
              type="email"
              autocomplete="email"
              placeholder="voce@email.com"
              class="h-[50px] rounded-xl border border-border bg-bg px-4 text-[15px] text-ink outline-none placeholder:text-ink3 focus:border-brand"
            />
            <span class="text-[12px] text-ink3">O Mercado Pago pede o e-mail de quem paga.</span>
          </label>

          <div class="rounded-xl bg-surface2 px-3.5 py-3 text-[12.5px] leading-relaxed text-ink2">
            <div class="flex items-center justify-between text-[14px] text-ink">
              <span>Total no Pix</span>
              <strong class="font-condensed text-[20px]">{{ formatCents(selected.total_cents) }}</strong>
            </div>
            <p class="mt-1.5">
              A vaga fica guardada por 30 minutos enquanto você paga. Desistência não tem devolução;
              se a partida for cancelada, o valor volta pra você.
            </p>
          </div>

          <p v-if="formError" role="alert" class="rounded-xl bg-dangerBg px-4 py-3 text-[13px] font-medium text-danger">
            {{ formError }}
          </p>

          <BaseButton type="submit" size="lg" class="w-full" :loading="submitting">
            Reservar e gerar o Pix
          </BaseButton>
        </form>
      </Card>
      <p v-else class="text-center text-[13px] text-ink3">Toca numa partida pra preencher seus dados.</p>
    </template>

    <p class="mt-2 text-center text-[13px] text-ink3">
      Já é do grupo?
      <router-link to="/login" class="font-semibold text-brand">Entrar</router-link>
    </p>
  </PublicShell>
</template>
