<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errorMessage } from '../../api/client'
import AdminLayout from '../../components/layout/AdminLayout.vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import Card from '../../components/ui/Card.vue'
import SectionLabel from '../../components/ui/SectionLabel.vue'

const settings = ref<Record<string, string>>({})
const error = ref('')
const saved = ref(false)
const saving = ref(false)

// Ficam aqui e não no template porque as chaves duplas colidem com a
// interpolação do Vue.
const templateVars = ['{{nome}}', '{{mes_referencia}}', '{{valor}}', '{{data_vencimento}}', '{{codigo_pix}}']

onMounted(async () => {
  const { data } = await api.get<Record<string, string>>('/admin/settings')
  settings.value = data
})

async function save() {
  error.value = ''
  saving.value = true
  try {
    const { data } = await api.put<Record<string, string>>('/admin/settings', settings.value)
    settings.value = data
    saved.value = true
    setTimeout(() => (saved.value = false), 2500)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AdminLayout>
    <template #title>Configurações</template>
    <template #subtitle>Regras de cobrança, votação e convites</template>

    <div class="grid max-w-3xl gap-4">
      <Card class="px-5 py-[18px]">
        <SectionLabel>Cobrança</SectionLabel>
        <div class="mt-3 flex flex-col gap-4">
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">Dias de tolerância antes de inativar</span>
            <input v-model="settings.overdue_inactivate_days" type="number" min="0" class="field max-w-[160px]" />
            <span class="text-[11.5px] text-ink3">
              Contados após o vencimento. Com 0, o usuário é inativado no dia seguinte ao 5º dia útil.
            </span>
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">Modelo da mensagem de lembrete</span>
            <textarea v-model="settings.reminder_template" rows="4" class="field h-auto py-2.5" />
            <span class="text-[11.5px] leading-relaxed text-ink3">
              Variáveis disponíveis:
              <template v-for="(variable, i) in templateVars" :key="variable">
                <code>{{ variable }}</code><span v-if="i < templateVars.length - 1">, </span>
              </template>.
              O envio é agendado para um dia útil antes do vencimento.
            </span>
          </label>
        </div>
      </Card>

      <Card class="px-5 py-[18px]">
        <SectionLabel>Votação e convites</SectionLabel>
        <div class="mt-3 grid gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">Dias de votação após a partida</span>
            <input v-model="settings.voting_close_days" type="number" min="1" class="field" />
            <span class="text-[11.5px] text-ink3">O resultado só aparece depois do encerramento.</span>
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-[13px] font-semibold text-ink2">Validade padrão do convite (dias)</span>
            <input v-model="settings.invite_valid_days" type="number" min="1" class="field" />
            <span class="text-[11.5px] text-ink3">Pode ser alterada na hora de gerar cada convite.</span>
          </label>
        </div>
      </Card>

      <Card class="border-info/40 bg-infoBg px-5 py-4">
        <div class="text-[13px] font-semibold text-info">Pendentes de integração</div>
        <ul class="mt-2 list-inside list-disc text-[12.5px] leading-relaxed text-ink2">
          <li>Geração automática do código PIX: aguardando a definição do provedor (BR Code próprio ou gateway).</li>
          <li>WhatsApp: os lembretes saem pela instância Evolution Go configurada no servidor — envio manual pelo app e lembrete automático de cobranças vencidas.</li>
          <li>Feriados não entram no cálculo dos dias úteis — apenas fins de semana.</li>
        </ul>
      </Card>

      <div class="flex items-center gap-3">
        <BaseButton :loading="saving" @click="save">Salvar configurações</BaseButton>
        <span v-if="saved" class="text-[13px] font-semibold text-brand">Salvo!</span>
        <span v-if="error" class="text-[13px] font-medium text-danger">{{ error }}</span>
      </div>
    </div>
  </AdminLayout>
</template>

<style scoped>
.field {
  @apply h-11 rounded-[11px] border border-border bg-bg px-3.5 text-sm text-ink outline-none focus:border-brand;
}
code {
  @apply rounded bg-surface2 px-1 py-px font-mono text-[11px] text-ink2;
}
</style>
