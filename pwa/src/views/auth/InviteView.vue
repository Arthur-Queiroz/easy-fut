<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../../api/client'
import type { InvitePublic } from '../../api/types'
import { formatDMY } from '../../lib/format'
import NavIcon from '../../components/layout/NavIcon.vue'

const route = useRoute()
const router = useRouter()
const token = route.params.token as string

const invite = ref<InvitePublic | null>(null)
const notFound = ref(false)

onMounted(async () => {
  try {
    const { data } = await api.get<InvitePublic>(`/invites/${token}`)
    invite.value = data
  } catch {
    notFound.value = true
  }
})

const statusMessage: Record<string, string> = {
  used: 'Este convite já foi utilizado.',
  revoked: 'Este convite foi revogado pelo administrador.',
  expired: 'Este convite expirou. Peça um novo link ao administrador.',
}
</script>

<template>
  <div
    class="flex min-h-dvh flex-col px-7 pb-12 pt-20 text-white md:items-center md:justify-center md:px-[12vw] md:py-16"
    style="background-image: linear-gradient(165deg, #0b1210 0%, #132a20 45%, #102a20 100%)"
  >
    <div class="flex flex-1 flex-col items-center justify-center text-center md:max-w-md md:flex-none">
      <div class="flex h-[92px] w-[92px] items-center justify-center rounded-full border-2 border-lime bg-lime/10 text-lime">
        <NavIcon name="ball" :size="46" :stroke-width="1.6" />
      </div>
      <div class="mt-[18px] font-condensed text-[30px] font-bold leading-none tracking-[.14em] text-lime">
        EASY FUT
      </div>
      <div class="mt-1 text-[13px] font-semibold tracking-[.22em] text-white/55">GESTÃO DO GRUPO</div>

      <template v-if="invite && invite.status === 'pending'">
        <div class="mt-10 text-[26px] font-bold leading-[1.15]">Você foi convidado!</div>
        <div class="mt-2.5 max-w-[280px] text-[15px] leading-normal text-white/75">
          {{ invite.creator_name }} convidou você para entrar no grupo e jogar com a rapaziada.
        </div>
        <div class="mt-7 flex w-full flex-col gap-3 rounded-2xl border border-white/15 bg-white/5 px-[18px] py-4 text-left">
          <div class="flex justify-between text-[13.5px] text-white/70">
            <span>Convidado por</span>
            <span class="font-semibold text-white">{{ invite.creator_name }}</span>
          </div>
          <div class="flex justify-between text-[13.5px] text-white/70">
            <span>Válido até</span>
            <span class="font-semibold text-white">{{ formatDMY(invite.expires_at.slice(0, 10)) }}</span>
          </div>
          <div class="flex justify-between text-[13.5px] text-white/70">
            <span>Uso</span>
            <span class="font-semibold text-white">Único · link pessoal</span>
          </div>
        </div>
      </template>

      <template v-else-if="invite || notFound">
        <div class="mt-10 text-[26px] font-bold leading-[1.15]">Convite indisponível</div>
        <div class="mt-2.5 max-w-[280px] text-[15px] leading-normal text-white/75">
          {{ notFound ? 'Este link de convite não existe.' : statusMessage[invite!.status] }}
        </div>
      </template>
    </div>

    <div class="flex w-full flex-col items-center gap-3.5 md:max-w-md">
      <button
        v-if="invite && invite.status === 'pending'"
        type="button"
        class="flex h-[54px] w-full items-center justify-center rounded-[14px] bg-lime text-base font-bold text-pitch-1"
        @click="router.push(`/cadastro/${token}`)"
      >
        Criar minha conta
      </button>
      <div class="text-[13px] text-white/55">
        Já tem conta?
        <router-link to="/login" class="font-semibold text-lime">Entrar</router-link>
      </div>
    </div>
  </div>
</template>
