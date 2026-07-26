import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Fluxo público (convite, cadastro, login)
    { path: '/convite/:token', name: 'invite', component: () => import('../views/auth/InviteView.vue'), meta: { public: true } },
    { path: '/cadastro/:token', name: 'signup', component: () => import('../views/auth/SignupView.vue'), meta: { public: true } },
    { path: '/login', name: 'login', component: () => import('../views/auth/LoginView.vue'), meta: { public: true } },
    { path: '/esqueci-minha-senha', name: 'forgot-password', component: () => import('../views/auth/ForgotPasswordView.vue'), meta: { public: true } },
    { path: '/redefinir-senha', name: 'reset-password', component: () => import('../views/auth/ResetPasswordView.vue'), meta: { public: true } },
    { path: '/bloqueado', name: 'blocked', component: () => import('../views/app/BlockedView.vue') },

    // Painel do usuário
    { path: '/', name: 'home', component: () => import('../views/app/HomeView.vue') },
    { path: '/partida', name: 'match', component: () => import('../views/app/MatchView.vue') },
    { path: '/partida/times', name: 'teams', component: () => import('../views/app/TeamsView.vue') },
    { path: '/pagamentos', name: 'payments', component: () => import('../views/app/PaymentsView.vue') },
    { path: '/historico', name: 'history', component: () => import('../views/app/HistoryView.vue') },
    { path: '/historico/:matchId', name: 'match-detail', component: () => import('../views/app/MatchDetailView.vue') },
    { path: '/perfil', name: 'profile', component: () => import('../views/app/ProfileView.vue') },

    // Painel administrativo
    { path: '/admin', name: 'admin-dashboard', component: () => import('../views/admin/DashboardView.vue'), meta: { admin: true } },
    { path: '/admin/partidas', name: 'admin-matches', component: () => import('../views/admin/MatchesView.vue'), meta: { admin: true } },
    { path: '/admin/usuarios', name: 'admin-users', component: () => import('../views/admin/UsersView.vue'), meta: { admin: true } },
    { path: '/admin/mensalidades', name: 'admin-charges', component: () => import('../views/admin/ChargesView.vue'), meta: { admin: true } },
    { path: '/admin/convites', name: 'admin-invites', component: () => import('../views/admin/InvitesView.vue'), meta: { admin: true } },
    { path: '/admin/configuracoes', name: 'admin-settings', component: () => import('../views/admin/SettingsView.vue'), meta: { admin: true } },

    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true

  const auth = useAuthStore()
  if (!auth.loaded) await auth.fetchMe()

  if (!auth.isLoggedIn) return { name: 'login' }
  if (to.name !== 'blocked' && to.name !== 'profile' && to.name !== 'payments' && !auth.isActive) {
    return { name: 'blocked' }
  }
  if (to.meta.admin && !auth.isAdmin) return { name: 'home' }
  return true
})
