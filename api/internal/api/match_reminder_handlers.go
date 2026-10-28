package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (s *Server) handleMatchReminderRecipients(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("id")
	if _, err := uuid.Parse(matchID); err != nil {
		writeError(w, http.StatusNotFound, "partida não encontrada")
		return
	}
	recipients, err := s.store.ReminderRecipients(r.Context(), matchID, r.URL.Query().Get("kind"), time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(recipients))
}

func (s *Server) handleQueueMatchReminders(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("id")
	if _, err := uuid.Parse(matchID); err != nil {
		writeError(w, http.StatusNotFound, "partida não encontrada")
		return
	}
	var body struct {
		Kind    string   `json:"kind"`
		UserIDs []string `json:"user_ids"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.UserIDs) == 0 || len(body.UserIDs) > 500 {
		writeError(w, http.StatusBadRequest, "selecione de 1 a 500 membros")
		return
	}
	for _, id := range body.UserIDs {
		if _, err := uuid.Parse(id); err != nil {
			writeError(w, http.StatusBadRequest, "user_ids contém ID inválido")
			return
		}
	}
	queued, err := s.store.QueueManualMatchReminders(r.Context(), matchID, body.Kind, body.UserIDs, s.cfg.AppURL, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	admin := currentUser(r)
	label := "confirmação de presença"
	if body.Kind == "vote" {
		label = "votação"
	}
	s.store.LogActivity(r.Context(), &admin.ID, "match_whatsapp_reminders",
		fmt.Sprintf("%s agendou %d lembrete(s) de %s pelo WhatsApp", admin.Name, queued, label))
	writeJSON(w, http.StatusAccepted, map[string]int{"queued": queued})
}
