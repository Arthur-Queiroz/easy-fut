<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import type { Invite } from '../../api/types'
import { formatDMY } from '../../lib/format'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import NavIcon from '../../components/layout/NavIcon.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const invites = ref<Invite[]>([])
const invitedName = ref('')
const role = ref<'player' | 'admin'>('player')
const validDays = ref(7)
const error = ref('')
const creating = ref(false)
const copiedId = ref('')

onMounted(load)

async function load() {
  const { data } = await api.get<Invite[]>('/admin/invites')
  invites.value = data ?? []
}

function inviteURL(invite: Invite): string {
  return `${window.location.origin}/convite/${invite.token}`
}

/** Encurta a URL para caber na coluna, preservando início e fim do token. */
function shortURL(invite: Invite): string {
  const url = inviteURL(invite).replace(/^https?:\/\//, '')
  const token = invite.token
  return url.replace(token, `${token.slice(0, 4)}…${token.slice(-3)}`)
}

async function create() {
  error.value = ''
  creating.value = true
  try {
    await api.post('/admin/invites', {
      invited_name: invitedName.value,
      role: role.value,
      valid_days: validDays.value,
    })
    invitedName.value = ''
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    creating.value = false
  }
}

async function copyLink(invite: Invite) {
  await navigator.clipboard.writeText(inviteURL(invite))
  copiedId.value = invite.id
  setTimeout(() => (copiedId.value = ''), 2000)
}

function sendWhatsApp(invite: Invite) {
  const name = invite.invited_name || 'jogador'
  const text = `Fala, ${name}! Você foi convidado pro Fut da Rapaziada. Crie sua conta aqui: ${inviteURL(invite)}`
  window.open(`https://wa.me/?text=${encodeURIComponent(text)}`, '_blank', 'noopener')
}

async function revoke(invite: Invite) {
  await api.post(`/admin/invites/${invite.id}/revoke`)
  await load()
}

type Status = 'used' | 'revoked' | 'expired' | 'pending'

function statusOf(invite: Invite): Status {
  if (invite.used_at) return 'used'
  if (invite.revoked_at) return 'revoked'
  if (new Date(invite.expires_at) < new Date()) return 'expired'
  return 'pending'
}

function statusBadge(invite: Invite) {
  switch (statusOf(invite)) {
    case 'used':
      return { cls: 'bg-brandSoft text-brand', label: `✓ Usado em ${formatDMY(invite.used_at!.slice(0, 10)).slice(0, 5)}` }
    case 'revoked':
      return { cls: 'bg-dangerBg text-danger', label: '✕ Revogado' }
    case 'expired':
      return { cls: 'bg-surface2 text-ink3', label: '○ Expirado' }
    default:
      return { cls: 'bg-warnBg text-warn', label: `● Pendente · ${invite.access_count} acessos` }
  }
}
</script>

<template>
  <AdminLayout>
    <template #title>Convites</template>
    <template #subtitle>Cadastro é só por convite — link único, uso único, com validade</template>

    <div class="flex flex-col gap-4">
      <!-- Gerar novo convite -->
      <Card class="px-5 py-[18px]">
        <SectionLabel>Gerar novo convite</SectionLabel>
        <form class="mt-3 flex flex-wrap items-end gap-3" @submit.prevent="create">
          <label class="flex min-w-[220px] flex-[1.4] flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Nome do convidado (opcional)</span>
            <input v-model="invitedName" placeholder="Ex.: Fernando Prates" class="field" />
          </label>
          <label class="flex min-w-[140px] flex-[.8] flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Perfil</span>
            <select v-model="role" class="field">
              <option value="player">Jogador</option>
              <option value="admin">Administrador</option>
            </select>
          </label>
          <label class="flex min-w-[140px] flex-[.8] flex-col gap-1.5">
            <span class="text-[12.5px] font-semibold text-ink2">Validade</span>
            <select v-model.number="validDays" class="field">
              <option :value="3">3 dias</option>
              <option :value="7">7 dias</option>
              <option :value="15">15 dias</option>
              <option :value="30">30 dias</option>
            </select>
          </label>
          <BaseButton :loading="creating" @click="create">
            <NavIcon name="link" :size="15" :stroke-width="2" />
            Gerar link
          </BaseButton>
        </form>
        <p v-if="error" class="mt-3 text-[13px] font-medium text-danger">{{ error }}</p>
      </Card>

      <!-- Lista de convites -->
      <Card class="overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full min-w-[900px] text-left">
            <thead>
              <tr class="bg-surface2 text-[10.5px] font-bold uppercase tracking-[.08em] text-ink3">
                <th class="whitespace-nowrap px-5 py-3 font-bold">Convidado</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Link</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Criado</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Validade</th>
                <th class="whitespace-nowrap px-3 py-3 font-bold">Status</th>
                <th class="whitespace-nowrap px-5 py-3 text-right font-bold">Ações</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="invite in invites"
                :key="invite.id"
                class="border-t border-border"
                :class="statusOf(invite) === 'expired' || statusOf(invite) === 'revoked' ? 'opacity-60' : ''"
              >
                <td class="px-5 py-3">
                  <div class="text-[13.5px] font-semibold">{{ invite.invited_name || 'Sem nome' }}</div>
                  <div class="text-[11.5px] text-ink3">por {{ invite.creator_name.split(' ')[0] }}</div>
                </td>
                <td class="max-w-[240px] px-3 py-3">
                  <span class="block truncate font-mono text-xs text-ink2">{{ shortURL(invite) }}</span>
                </td>
                <td class="px-3 py-3 text-[13px] text-ink2">{{ formatDMY(invite.created_at.slice(0, 10)) }}</td>
                <td class="px-3 py-3 text-[13px] text-ink2">{{ formatDMY(invite.expires_at.slice(0, 10)).slice(0, 5) }}</td>
                <td class="px-3 py-3">
                  <span class="inline-flex whitespace-nowrap rounded-full px-2.5 py-[3px] text-[11px] font-semibold" :class="statusBadge(invite).cls">
                    {{ statusBadge(invite).label }}
                  </span>
                </td>
                <td class="px-5 py-3">
                  <div v-if="statusOf(invite) === 'pending'" class="flex justify-end gap-2">
                    <button
                      type="button"
                      class="inline-flex h-[30px] items-center gap-1.5 rounded-lg border border-border px-2.5 text-xs font-semibold text-ink2"
                      @click="copyLink(invite)"
                    >
                      <NavIcon name="copy" :size="13" :stroke-width="1.9" />
                      {{ copiedId === invite.id ? 'Copiado!' : 'Copiar' }}
                    </button>
                    <button
                      type="button"
                      class="inline-flex h-[30px] items-center gap-1.5 rounded-lg bg-brandSoft px-2.5 text-xs font-semibold text-brand"
                      @click="sendWhatsApp(invite)"
                    >
                      <NavIcon name="chat" :size="13" :stroke-width="1.9" />
                      WhatsApp
                    </button>
                    <button type="button" class="inline-flex h-[30px] items-center px-2.5 text-xs font-semibold text-danger" @click="revoke(invite)">
                      Revogar
                    </button>
                  </div>
                  <div v-else class="flex justify-end">
                    <button
                      v-if="statusOf(invite) !== 'used'"
                      type="button"
                      class="inline-flex h-[30px] items-center rounded-lg border border-border px-2.5 text-xs font-semibold text-ink2"
                      @click="((invitedName = invite.invited_name), (role = invite.role), create())"
                    >
                      Gerar novo
                    </button>
                    <router-link
                      v-else
                      to="/admin/usuarios"
                      class="inline-flex h-[30px] items-center rounded-lg border border-border px-2.5 text-xs font-semibold text-ink3"
                    >
                      Ver usuário
                    </router-link>
                  </div>
                </td>
              </tr>
              <tr v-if="invites.length === 0">
                <td colspan="6" class="px-5 py-10 text-center text-sm text-ink3">Nenhum convite gerado ainda.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  </AdminLayout>
</template>

<style scoped>
.field {
  @apply h-[42px] rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
</style>
