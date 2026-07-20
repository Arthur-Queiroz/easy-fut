<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errorMessage } from '../../api/client'
import type { Charge } from '../../api/types'
import { useAuthStore } from '../../stores/auth'
import { useThemeStore } from '../../stores/theme'
import { formatDMY, formatMonth, parseISODate } from '../../lib/format'
import MobileShell from '../../components/layout/MobileShell.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import Modal from '../../components/ui/Modal.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

const charges = ref<Charge[]>([])
const editing = ref<'phone' | 'email' | null>(null)
const editValue = ref('')
const error = ref('')
const saving = ref(false)

onMounted(async () => {
  await auth.fetchMe()
  const { data } = await api.get<Charge[]>('/charges/me')
  charges.value = data ?? []
})

const memberSince = computed(() => {
  if (!auth.user) return ''
  const d = parseISODate(auth.user.created_at.slice(0, 10))
  const months = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']
  return `Desde ${months[d.getMonth()]}/${d.getFullYear()}`
})

function startEdit(field: 'phone' | 'email') {
  editing.value = field
  editValue.value = field === 'phone' ? (auth.user?.phone ?? '') : (auth.user?.email ?? '')
  error.value = ''
}

async function saveEdit() {
  if (!editing.value) return
  saving.value = true
  error.value = ''
  try {
    await api.patch('/me', { [editing.value]: editValue.value })
    await auth.fetchMe()
    editing.value = null
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

function chargeBadge(charge: Charge) {
  switch (charge.status) {
    case 'paid':
    case 'manual_paid':
      return { cls: 'bg-brandSoft text-brand', label: `Pago${charge.paid_at ? ' · ' + formatDMY(charge.paid_at.slice(0, 10)).slice(0, 5) : ''}` }
    case 'overdue':
      return { cls: 'bg-dangerBg text-danger', label: 'Vencida' }
    case 'exempt':
      return { cls: 'bg-surface2 text-ink2', label: 'Isento' }
    case 'cancelled':
      return { cls: 'bg-surface2 text-ink2', label: 'Cancelada' }
    default:
      return { cls: 'bg-warnBg text-warn', label: 'Aguardando' }
  }
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <MobileShell>
    <template #header>
      <div class="flex items-center gap-4">
        <Avatar :name="auth.user?.name ?? ''" color="#C8F14B" size="lg" ring />
        <div>
          <div class="text-[22px] font-bold">{{ auth.user?.name }}</div>
          <div class="mt-1.5 flex flex-wrap gap-[7px]">
            <span class="rounded-full bg-lime/15 px-2.5 py-[3px] text-[11.5px] font-semibold text-lime">● Ativo</span>
            <span class="rounded-full bg-white/10 px-2.5 py-[3px] text-[11.5px] font-semibold text-white/80">
              {{ auth.isAdmin ? 'Admin' : 'Jogador' }}
            </span>
            <span class="rounded-full bg-white/10 px-2.5 py-[3px] text-[11.5px] font-semibold text-white/80">
              {{ memberSince }}
            </span>
          </div>
        </div>
      </div>
    </template>

    <div class="flex flex-col gap-3.5 px-4 pt-4">
      <!-- Estatísticas -->
      <div class="grid grid-cols-3 gap-2.5">
        <Card class="px-2 py-3.5 text-center">
          <div class="font-condensed text-[32px] font-bold leading-none">{{ auth.stats?.matches_played ?? 0 }}</div>
          <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">PARTIDAS</div>
        </Card>
        <Card class="px-2 py-3.5 text-center">
          <div class="font-condensed text-[32px] font-bold leading-none text-brand">{{ auth.stats?.top_scorer_count ?? 0 }}</div>
          <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">ARTILHEIRO</div>
        </Card>
        <Card class="px-2 py-3.5 text-center">
          <div class="font-condensed text-[32px] font-bold leading-none text-warn">{{ auth.stats?.worst_count ?? 0 }}</div>
          <div class="mt-1 text-[11px] font-medium tracking-[.04em] text-ink3">PERNA DE PAU</div>
        </Card>
      </div>

      <!-- Contato -->
      <Card class="px-4 py-1.5">
        <div class="flex items-center gap-3 border-b border-border py-[11px]">
          <NavIcon name="chat" :size="17" class="text-ink3" />
          <div class="flex-1">
            <div class="text-[11.5px] text-ink3">WhatsApp</div>
            <div class="text-sm font-medium">{{ auth.user?.phone || '—' }}</div>
          </div>
          <button type="button" class="text-[12.5px] font-semibold text-brand" @click="startEdit('phone')">Editar</button>
        </div>
        <div class="flex items-center gap-3 py-[11px]">
          <NavIcon name="mail" :size="17" class="text-ink3" />
          <div class="flex-1">
            <div class="text-[11.5px] text-ink3">E-mail</div>
            <div class="text-sm font-medium">{{ auth.user?.email }}</div>
          </div>
          <button type="button" class="text-[12.5px] font-semibold text-brand" @click="startEdit('email')">Editar</button>
        </div>
      </Card>

      <!-- Tema -->
      <Card class="flex items-center justify-between px-4 py-3.5">
        <span class="text-sm font-medium">Modo escuro</span>
        <button
          type="button"
          class="relative h-7 w-12 rounded-full transition-colors"
          :class="theme.theme === 'dark' ? 'bg-brand' : 'bg-surface2 border border-border'"
          role="switch"
          :aria-checked="theme.theme === 'dark'"
          @click="theme.toggle()"
        >
          <span
            class="absolute top-1 h-5 w-5 rounded-full bg-white shadow transition-all"
            :class="theme.theme === 'dark' ? 'left-6' : 'left-1'"
          />
        </button>
      </Card>

      <!-- Pagamentos -->
      <Card class="p-4">
        <SectionLabel>Pagamentos</SectionLabel>
        <div class="mt-2 flex flex-col">
          <div
            v-for="(charge, i) in charges.slice(0, 6)"
            :key="charge.id"
            class="flex items-center justify-between py-2"
            :class="i < Math.min(charges.length, 6) - 1 ? 'border-b border-border' : ''"
          >
            <span class="text-[13.5px] font-medium">{{ formatMonth(charge.reference_month) }}</span>
            <span class="inline-flex rounded-full px-2.5 py-[3px] text-[11px] font-semibold" :class="chargeBadge(charge).cls">
              {{ chargeBadge(charge).label }}
            </span>
          </div>
          <p v-if="charges.length === 0" class="pt-2 text-sm text-ink3">Nenhuma cobrança até agora.</p>
        </div>
      </Card>

      <router-link
        v-if="auth.isAdmin"
        to="/admin"
        class="flex h-[50px] items-center justify-center gap-2 rounded-[13px] border-[1.5px] border-info text-[15px] font-bold text-info"
      >
        <NavIcon name="grid" :size="16" />
        Painel administrativo
      </router-link>

      <button type="button" class="py-2 text-center text-[13.5px] font-semibold text-danger" @click="logout">
        Sair da conta
      </button>
    </div>

    <Modal :open="editing !== null" :title="editing === 'phone' ? 'Editar WhatsApp' : 'Editar e-mail'" @close="editing = null">
      <form class="flex flex-col gap-4" @submit.prevent="saveEdit">
        <input
          v-model="editValue"
          :type="editing === 'email' ? 'email' : 'tel'"
          class="h-[50px] rounded-xl border border-border bg-bg px-4 text-[15px] text-ink outline-none focus:border-brand"
          required
        />
        <p v-if="error" class="text-[13px] font-medium text-danger">{{ error }}</p>
        <BaseButton size="lg" :loading="saving" @click="saveEdit">Salvar</BaseButton>
      </form>
    </Modal>
  </MobileShell>
</template>
