<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Charge, User } from '../../api/types'
import { formatCents, formatDMY, formatMonth } from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import Avatar from '../../components/ui/Avatar.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import Modal from '../../components/ui/Modal.vue'

const users = ref<User[]>([])
const total = ref(0)
const page = ref(1)
const search = ref('')
const roleFilter = ref('')
const statusFilter = ref('')
const financialFilter = ref('')

const selected = ref<User | null>(null)
const selectedCharges = ref<Charge[]>([])
const error = ref('')
const saving = ref(false)

// Formulário do modal de detalhe.
const form = ref({ name: '', phone: '', email: '', role: 'player' })

onMounted(load)

let debounce: number
watch(search, () => {
  clearTimeout(debounce)
  debounce = setTimeout(() => {
    page.value = 1
    load()
  }, 300) as unknown as number
})
watch([roleFilter, statusFilter, financialFilter, page], load)

async function load() {
  const { data } = await api.get<{ users: User[]; total: number }>('/admin/users', {
    params: {
      search: search.value || undefined,
      role: roleFilter.value || undefined,
      status: statusFilter.value || undefined,
      financial: financialFilter.value || undefined,
      page: page.value,
    },
  })
  users.value = data.users
  total.value = data.total
}

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / 20)))
const summary = computed(() => {
  const active = users.value.filter((u) => u.status === 'active').length
  const inactive = users.value.filter((u) => u.status === 'inactive').length
  return `${total.value} usuários · ${active} ativos e ${inactive} inativos nesta página`
})

async function openUser(user: User) {
  selected.value = user
  form.value = { name: user.name, phone: user.phone, email: user.email, role: user.role }
  error.value = ''
  selectedCharges.value = []
  // Reaproveita a listagem de cobranças do mês; o detalhe mostra as do usuário.
  const { data } = await api.get<{ charges: Charge[] }>('/admin/charges')
  selectedCharges.value = (data.charges ?? []).filter((c) => c.user_id === user.id)
}

async function save() {
  if (!selected.value) return
  saving.value = true
  error.value = ''
  try {
    await api.patch(`/admin/users/${selected.value.id}`, form.value)
    await load()
    selected.value = null
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function changeStatus(status: 'active' | 'inactive' | 'archived', reason = '') {
  if (!selected.value) return
  saving.value = true
  error.value = ''
  try {
    await api.patch(`/admin/users/${selected.value.id}`, { status, reason })
    await load()
    selected.value = null
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

async function markPaid(chargeId: string) {
  await api.post(`/admin/charges/${chargeId}/mark-paid`, { method: 'manual' })
  await load()
  if (selected.value) await openUser(users.value.find((u) => u.id === selected.value!.id) ?? selected.value)
}

function accessBadge(user: User) {
  if (user.status === 'active') return { cls: 'bg-brandSoft text-brand', label: '● Ativo' }
  if (user.status === 'inactive') return { cls: 'bg-danger text-white', label: '✕ Inativo' }
  return { cls: 'bg-surface2 text-ink2', label: '○ Arquivado' }
}

function financialBadge(user: User) {
  if (user.delinquent) return { cls: 'bg-danger text-white', label: '! Inadimplente' }
  return { cls: 'bg-brandSoft text-brand', label: '● Em dia' }
}
</script>

<template>
  <AdminLayout>
    <template #title>Usuários</template>
    <template #subtitle>{{ summary }}</template>
    <template #actions>
      <BaseButton size="sm" @click="$router.push('/admin/convites')">
        <NavIcon name="plus" :size="15" :stroke-width="2.2" />
        Convidar jogador
      </BaseButton>
    </template>

    <div class="flex flex-col gap-4">
      <!-- Filtros -->
      <div class="flex flex-wrap gap-2.5">
        <div class="flex h-10 min-w-[220px] flex-1 items-center gap-2.5 rounded-[11px] border border-border bg-surface px-3.5">
          <NavIcon name="search" :size="15" :stroke-width="1.9" class="text-ink3" />
          <input
            v-model="search"
            type="search"
            placeholder="Buscar por nome…"
            class="w-full bg-transparent text-[13.5px] text-ink outline-none placeholder:text-ink3"
          />
        </div>
        <select v-model="roleFilter" class="filter">
          <option value="">Perfil: Todos</option>
          <option value="admin">Admin</option>
          <option value="player">Jogador</option>
        </select>
        <select v-model="statusFilter" class="filter">
          <option value="">Acesso: Todos</option>
          <option value="active">Ativo</option>
          <option value="inactive">Inativo</option>
          <option value="archived">Arquivado</option>
        </select>
        <select v-model="financialFilter" class="filter">
          <option value="">Financeiro: Todos</option>
          <option value="paid">Em dia</option>
          <option value="pending">Aguardando</option>
          <option value="overdue">Inadimplente</option>
        </select>
      </div>

      <!-- Tabela -->
      <Card class="overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full min-w-[880px] text-left">
            <thead>
              <tr class="bg-surface2 text-[10.5px] font-bold uppercase tracking-[.08em] text-ink3">
                <th class="whitespace-nowrap px-5 py-3 font-bold">Jogador</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">WhatsApp</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Perfil</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Acesso</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Financeiro</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Últ. pagamento</th>
                <th class="whitespace-nowrap px-5 py-3 text-right font-bold">Ações</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="user in users"
                :key="user.id"
                class="border-t border-border"
                :class="user.delinquent ? 'bg-dangerBg' : ''"
              >
                <td class="px-5 py-2.5">
                  <div class="flex items-center gap-2.5">
                    <Avatar :name="user.name" :color="user.avatar_color" size="sm" />
                    <div class="min-w-0">
                      <div class="truncate text-[13.5px] font-semibold">{{ user.name }}</div>
                      <div class="truncate text-[11.5px] text-ink3">{{ user.email }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-3 py-2.5 text-[13px] text-ink2">{{ user.phone || '—' }}</td>
                <td class="px-3 py-2.5">
                  <span
                    class="inline-flex rounded-[7px] px-2.5 py-[3px] text-[11px] font-semibold"
                    :class="user.role === 'admin' ? 'bg-infoBg text-info' : 'bg-surface2 text-ink2'"
                  >
                    {{ user.role === 'admin' ? 'Admin' : 'Jogador' }}
                  </span>
                </td>
                <td class="px-3 py-2.5">
                  <span class="inline-flex rounded-full px-2.5 py-[3px] text-[11px] font-semibold" :class="accessBadge(user).cls">
                    {{ accessBadge(user).label }}
                  </span>
                </td>
                <td class="px-3 py-2.5">
                  <span class="inline-flex rounded-full px-2.5 py-[3px] text-[11px] font-semibold" :class="financialBadge(user).cls">
                    {{ financialBadge(user).label }}
                  </span>
                </td>
                <td class="px-3 py-2.5 text-[13px] text-ink2">
                  {{ user.last_payment_at ? formatDMY(user.last_payment_at.slice(0, 10)) : '—' }}
                </td>
                <td class="px-5 py-2.5 text-right">
                  <button type="button" class="px-2 text-base font-bold tracking-[2px] text-ink3 hover:text-ink" @click="openUser(user)">
                    ⋯
                  </button>
                </td>
              </tr>
              <tr v-if="users.length === 0">
                <td colspan="7" class="px-5 py-10 text-center text-sm text-ink3">Nenhum usuário encontrado com esses filtros.</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex items-center justify-between border-t border-border px-5 py-3">
          <span class="text-[12.5px] text-ink3">Mostrando {{ users.length }} de {{ total }} usuários</span>
          <div class="flex gap-1.5">
            <button
              v-for="p in pageCount"
              :key="p"
              type="button"
              class="flex h-[30px] w-[30px] items-center justify-center rounded-lg text-[12.5px]"
              :class="p === page ? 'bg-brand font-semibold text-brandInk' : 'border border-border font-medium text-ink2'"
              @click="page = p"
            >
              {{ p }}
            </button>
          </div>
        </div>
      </Card>
    </div>

    <!-- Detalhe / edição -->
    <Modal :open="selected !== null" :title="selected?.name ?? ''" @close="selected = null">
      <div v-if="selected" class="flex flex-col gap-4">
        <div class="grid grid-cols-2 gap-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Nome</span>
            <input v-model="form.name" class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">WhatsApp</span>
            <input v-model="form.phone" class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">E-mail</span>
            <input v-model="form.email" type="email" class="field" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Perfil</span>
            <select v-model="form.role" class="field">
              <option value="player">Jogador</option>
              <option value="admin">Administrador</option>
            </select>
          </label>
        </div>

        <div v-if="selectedCharges.length" class="rounded-xl border border-border bg-bg p-3">
          <div class="mb-2 text-[11px] font-bold uppercase tracking-[.1em] text-ink3">Cobranças do mês</div>
          <div v-for="charge in selectedCharges" :key="charge.id" class="flex items-center justify-between py-1.5">
            <span class="text-[13px]">{{ formatMonth(charge.reference_month) }} · {{ formatCents(charge.amount_cents) }}</span>
            <BaseButton
              v-if="charge.status === 'pending' || charge.status === 'overdue'"
              variant="ghost"
              size="sm"
              @click="markPaid(charge.id)"
            >
              Registrar pagamento
            </BaseButton>
            <span v-else class="text-[12px] font-semibold text-brand">Pago</span>
          </div>
        </div>

        <p v-if="error" class="text-[13px] font-medium text-danger">{{ error }}</p>

        <div class="flex flex-wrap gap-2">
          <BaseButton size="sm" :loading="saving" @click="save">Salvar dados</BaseButton>
          <BaseButton
            v-if="selected.status !== 'active'"
            variant="outline-brand"
            size="sm"
            :loading="saving"
            @click="changeStatus('active')"
          >
            Ativar
          </BaseButton>
          <BaseButton
            v-else
            variant="outline"
            size="sm"
            :loading="saving"
            @click="changeStatus('inactive', 'inativado pelo administrador')"
          >
            Inativar
          </BaseButton>
          <BaseButton
            v-if="selected.status !== 'archived'"
            variant="ghost"
            size="sm"
            class="text-danger"
            :loading="saving"
            @click="changeStatus('archived', 'arquivado pelo administrador')"
          >
            Arquivar
          </BaseButton>
        </div>
        <p class="text-[11.5px] leading-relaxed text-ink3">
          Arquivar preserva o histórico de partidas e pagamentos, mas remove o usuário do grupo e dos rateios futuros.
        </p>
      </div>
    </Modal>
  </AdminLayout>
</template>

<style scoped>
.filter {
  @apply h-10 rounded-[11px] border border-border bg-surface px-3.5 text-[13px] font-medium text-ink2 outline-none;
}
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
</style>
