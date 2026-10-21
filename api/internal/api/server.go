// Package api monta o servidor HTTP: rotas, middlewares e handlers.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"futdarapaziada/api/internal/auth"
	"futdarapaziada/api/internal/config"
	"futdarapaziada/api/internal/mailer"
	"futdarapaziada/api/internal/mercadopago"
	"futdarapaziada/api/internal/notify"
	"futdarapaziada/api/internal/store"
)

type Server struct {
	cfg                  config.Config
	store                *store.Store
	mercadoPago          *mercadopago.Client
	mailer               *mailer.Resend
	sender               notify.Sender
	loginLimiter         *auth.LoginLimiter
	passwordResetLimiter *auth.LoginLimiter
	guestLimiters        guestPublicLimiters
}

func NewServer(cfg config.Config, st *store.Store, sender notify.Sender) *Server {
	return &Server{
		cfg:                  cfg,
		store:                st,
		mercadoPago:          mercadopago.New(cfg.MercadoPagoAccessToken),
		mailer:               mailer.NewResend(cfg.ResendAPIKey, cfg.EmailFrom),
		sender:               sender,
		loginLimiter:         auth.NewLoginLimiter(10, 15*time.Minute),
		passwordResetLimiter: auth.NewLoginLimiter(3, 15*time.Minute),
		guestLimiters:        newGuestPublicLimiters(),
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(120 * time.Second))

	r.Route("/api", func(r chi.Router) {
		// Rotas públicas
		r.Get("/healthz", s.handleHealth)
		r.Get("/invites/{token}", s.handleGetInvite)
		r.Post("/invites/{token}/signup", s.handleSignup)
		r.Post("/login", s.handleLogin)
		r.Post("/password-reset/request", s.handlePasswordResetRequest)
		r.Post("/password-reset/confirm", s.handlePasswordResetConfirm)
		r.Post("/logout", s.handleLogout)
		r.Post("/webhooks/mercadopago", s.handleMercadoPagoWebhook)

		// Participação avulsa — público (fase A). Sem sessão; cada rota com
		// limite próprio por IP (guest_public_handlers.go).
		r.With(limitByIP(s.guestLimiters.list)).Get("/public/guest-matches", s.handlePublicGuestMatches)
		r.With(limitByIP(s.guestLimiters.create)).Post("/public/guest-bookings", s.handleCreateGuestBooking)
		r.With(limitByIP(s.guestLimiters.status)).Get("/public/guest-bookings/{id}", s.handleGetGuestBooking)

		// Autenticado — acessível também a usuários inativos (consultar bloqueio e pagar)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/me", s.handleMe)
			r.Get("/push/config", s.handlePushConfig)
			r.Post("/push/subscriptions/status", s.handlePushStatus)
			r.Post("/push/subscriptions", s.handleSubscribePush)
			r.Delete("/push/subscriptions", s.handleUnsubscribePush)
			r.Patch("/me", s.handleUpdateMe)
			r.Get("/charges/me", s.handleMyCharges)
			r.Post("/charges/{id}/pix", s.handleCreatePixCharge)
			r.Get("/media/files/{matchId}/{filename}", s.handleServeMedia)
		})

		// Autenticado e ativo — operações do dia a dia
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth, s.requireActive)
			r.Get("/matches/next", s.handleNextMatch)
			r.Get("/matches", s.handleListMatches)
			r.Get("/matches/{id}", s.handleGetMatch)
			r.Post("/matches/{id}/confirm", s.handleConfirm)
			r.Get("/matches/{id}/confirmations", s.handleConfirmations)
			r.Get("/matches/{id}/teams", s.handleTeams)
			r.Post("/matches/{id}/votes", s.handleVote)
			r.Put("/matches/{id}/votes", s.handleReplaceVotes)
			r.Delete("/matches/{id}/votes/{category}/{candidateId}", s.handleRemoveVote)
			r.Get("/matches/{id}/media", s.handleListMedia)
			r.Post("/matches/{id}/media", s.handleUploadMedia)
			r.Delete("/media/{id}", s.handleDeleteMedia)
			r.Get("/activity", s.handleActivity)
		})

		// Administração
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth, s.requireActive, s.requireAdmin)
			r.Get("/admin/dashboard", s.handleDashboard)

			// Dashboard de votação (FUT-005)
			r.Get("/admin/matches/{id}/votes", s.handleMatchVotes)
			r.Get("/admin/users/{id}/votes", s.handleUserVotes)
			r.Get("/admin/stats/votes", s.handleVotesSummary)

			// Recorrências (FUT-004)
			r.Get("/admin/recurring", s.handleListRecurring)
			r.Post("/admin/recurring", s.handleCreateRecurring)
			r.Patch("/admin/recurring/{id}", s.handleUpdateRecurring)
			r.Patch("/admin/recurring/{id}/activate", s.handleActivateRecurring)
			r.Delete("/admin/recurring/{id}", s.handleDeleteRecurring)

			r.Get("/admin/users", s.handleListUsers)
			r.Patch("/admin/users/{id}", s.handleUpdateUser)

			r.Post("/admin/invites", s.handleCreateInvite)
			r.Get("/admin/invites", s.handleListInvites)
			r.Post("/admin/invites/{id}/revoke", s.handleRevokeInvite)
			r.Post("/admin/invites/{id}/whatsapp-send", s.handleInviteWhatsAppSend)

			r.Post("/admin/matches", s.handleCreateMatch)
			r.Patch("/admin/matches/{id}", s.handleUpdateMatch)
			r.Post("/matches/{id}/cancel", s.handleCancelMatch)
			r.Post("/matches/{id}/close-confirmations", s.handleCloseConfirmations)
			r.Post("/matches/{id}/reopen-confirmations", s.handleReopenConfirmations)
			r.Post("/matches/{id}/draw-teams", s.handleDrawTeams)
			r.Post("/matches/{id}/finish", s.handleFinishMatch)
			r.Post("/matches/{id}/close-voting", s.handleCloseVoting)

			r.Get("/admin/team-presets", s.handleListTeamPresets)
			r.Post("/admin/team-presets", s.handleCreateTeamPreset)
			r.Patch("/admin/team-presets/{id}", s.handleUpdateTeamPreset)
			r.Delete("/admin/team-presets/{id}", s.handleDeleteTeamPreset)

			r.Get("/admin/charges", s.handleAdminCharges)
			r.Get("/admin/charges/pix-fee", s.handlePixFeePreview)
			r.Post("/admin/charges/generate", s.handleGenerateCharges)
			r.Post("/admin/charges/generate-match", s.handleGenerateMatchCharges)
			r.Delete("/admin/charges/batches/{id}", s.handleDeleteBatch)
			r.Post("/admin/charges/{id}/whatsapp-send", s.handleWhatsAppSend)
			r.Post("/admin/charges/{id}/mark-paid", s.handleMarkPaid)
			r.Post("/admin/charges/{id}/cancel", s.handleCancelCharge)
			r.Post("/admin/charges/{id}/exempt", s.handleExemptCharge)

			r.Get("/admin/settings", s.handleGetSettings)
			r.Put("/admin/settings", s.handleUpdateSettings)

			r.Get("/admin/push/recipients", s.handleListPushRecipients)
			r.Get("/admin/push/broadcasts", s.handleListPushBroadcasts)
			r.Post("/admin/push/broadcasts", s.handleCreatePushBroadcast)
		})
	})

	return r
}
