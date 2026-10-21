<script setup lang="ts">
// Venda de vagas avulsas de uma partida (admin). Só com a partida `open`:
// ao abrir, a API congela o valor da vaga e o total com a taxa do Pix.
// Depois do prazo de confirmação a venda fecha sozinha; aqui só o resumo.
import { computed, ref, watch } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { GuestSaleInput } from '../../api/adminGuests'
import type { Match } from '../../api/types'
import { formatCents } from '../../lib/format'
import Badge from '../ui/Badge.vue'
import BaseButton from '../ui/BaseButton.vue'

const props = defineProps<{ match: Match }>()
const emit = defineEmits<{ changed: [] }>()

const slotsInput = ref<number | ''>(props.match.guest_slots || '')
const saving = ref<'' | 'open' | 'slots' | 'close'>('')
const error = ref('')

watch(
  () => props.match.guest_slots,
  (slots) => {
    slotsInput.value = slots || ''
  },
)

const taken = computed(() => (props.match.guest_confirmed_count ?? 0) + (props.match.guest_pending_count ?? 0))
const free = computed(() => Math.max(0, (props.match.guest_slots ?? 0) - taken.value))
const slotsValid = computed(() => Number.isInteger(slotsInput.value) && Number(slotsInput.value) >= 1)
const hasGuests = computed(() => taken.value > 0 || props.match.guest_sale_open)

async function put(body: GuestSaleInput, action: 'open' | 'slots' | 'close') {
  error.value = ''
  saving.value = action
  try {
    await api.put(`/admin/matches/${props.match.id}/guest-sale`, body)
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = ''
  }
}

function openSale() {
  if (!slotsValid.value) return
  void put({ open: true, slots: Number(slotsInput.value) }, props.match.guest_sale_open ? 'slots' : 'open')
}

function closeSale() {
  void put({ open: false, slots: props.match.guest_slots }, 'close')
}
</script>

<template>
  <div v-if="match.status === 'open'" class="mt-3 rounded-xl border border-border bg-bg p-3" data-test="guest-sale">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <span class="text-[12.5px] font-bold text-ink">Vagas avulsas</span>
      <Badge v-if="match.guest_sale_open" tone="success" dot>Venda aberta</Badge>
      <Badge v-else tone="neutral">Venda fechada</Badge>
    </div>

    <template v-if="match.guest_sale_open">
      <p class="mt-1.5 text-[12.5px] text-ink2">
        <strong class="text-ink">{{ formatCents(match.guest_total_cents) }}</strong> por vaga
        ({{ formatCents(match.guest_price_cents) }} para o caixa, taxa do Pix embutida)
      </p>
      <p class="mt-0.5 text-[12.5px] text-ink2">
        <strong class="text-ink">{{ match.guest_confirmed_count }} confirmados</strong>
        · {{ match.guest_pending_count }} aguardando Pix · {{ free }} de {{ match.guest_slots }} vagas livres
      </p>
    </template>
    <p v-else class="mt-1.5 text-[12px] leading-relaxed text-ink3">
      Ao abrir, o valor da vaga das configurações fica congelado nesta partida, com a taxa do Pix embutida. A venda
      fecha junto com o prazo de confirmação.
      <template v-if="taken > 0">Hoje: {{ match.guest_confirmed_count }} confirmados · {{ match.guest_pending_count }} aguardando Pix.</template>
    </p>

    <div class="mt-2.5 flex flex-wrap items-end gap-2">
      <label class="flex flex-col gap-1">
        <span class="text-[11.5px] font-semibold text-ink2">Vagas</span>
        <input
          v-model.number="slotsInput"
          type="number"
          min="1"
          inputmode="numeric"
          aria-label="Número de vagas avulsas"
          class="h-[38px] w-[88px] rounded-[10px] border border-border bg-surface px-3 text-sm text-ink outline-none focus:border-brand"
        />
      </label>
      <BaseButton
        v-if="!match.guest_sale_open"
        size="sm"
        variant="outline-brand"
        :disabled="!slotsValid"
        :loading="saving === 'open'"
        @click="openSale"
      >
        Abrir venda
      </BaseButton>
      <template v-else>
        <BaseButton
          size="sm"
          variant="outline"
          :disabled="!slotsValid || slotsInput === match.guest_slots"
          :loading="saving === 'slots'"
          @click="openSale"
        >
          Salvar vagas
        </BaseButton>
        <BaseButton size="sm" variant="ghost" class="text-danger" :loading="saving === 'close'" @click="closeSale">
          Fechar venda
        </BaseButton>
      </template>
    </div>
    <p v-if="error" class="mt-2 rounded-lg bg-dangerBg px-3 py-2 text-[12.5px] font-medium text-danger">{{ error }}</p>
  </div>

  <p v-else-if="hasGuests" class="mt-2 text-[12.5px] text-ink2" data-test="guest-summary">
    Avulsos: <strong class="text-ink">{{ match.guest_confirmed_count }} confirmados</strong>
    · {{ match.guest_pending_count }} aguardando Pix
  </p>
</template>
