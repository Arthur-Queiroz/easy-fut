package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"futdarapaziada/api/internal/store"
)

// FUT-004: CRUD de templates recorrentes (admin).

// decodeRecurring valida e decodifica o body de criação/edição.
func (s *Server) decodeRecurring(w http.ResponseWriter, r *http.Request) (store.CreateRecurringInput, bool) {
	var in store.CreateRecurringInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return in, false
	}
	if in.Weekday < 0 || in.Weekday > 6 {
		writeError(w, http.StatusBadRequest, "weekday inválido (0=domingo .. 6=sábado)")
		return in, false
	}
	in.Venue = strings.TrimSpace(in.Venue)
	in.Address = strings.TrimSpace(in.Address)
	in.SameDayDeadlineTime = strings.TrimSpace(in.SameDayDeadlineTime)
	if in.Venue == "" {
		writeError(w, http.StatusBadRequest, "local é obrigatório")
		return in, false
	}
	if !validHHMM(in.StartTime) || !validHHMM(in.EndTime) {
		writeError(w, http.StatusBadRequest, "start_time/end_time devem ser HH:MM")
		return in, false
	}
	if in.SameDayDeadlineTime != "" && !validHHMM(in.SameDayDeadlineTime) {
		writeError(w, http.StatusBadRequest, "same_day_deadline_time deve ser HH:MM")
		return in, false
	}
	return in, true
}

func validHHMM(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60
}

func (s *Server) handleListRecurring(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListRecurringMatches(r.Context(), false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recurring": orEmpty(out)})
}

func (s *Server) handleCreateRecurring(w http.ResponseWriter, r *http.Request) {
	in, ok := s.decodeRecurring(w, r)
	if !ok {
		return
	}
	user := currentUser(r)
	rec, err := s.store.CreateRecurringMatch(r.Context(), in, user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// materializa na hora pra painel já mostrar as próximas
	if _, err := s.store.MaterializeRecurringMatches(r.Context(), 4); err != nil {
		// não falha a requisição; o cron diário cobre
		_ = err
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleUpdateRecurring(w http.ResponseWriter, r *http.Request) {
	in, ok := s.decodeRecurring(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := s.store.UpdateRecurringMatch(r.Context(), id, in); err != nil {
		writeStoreError(w, err)
		return
	}
	if _, err := s.store.MaterializeRecurringMatches(r.Context(), 4); err != nil {
		_ = err
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleActivateRecurring(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Active *bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Active == nil {
		writeError(w, http.StatusBadRequest, "body {active: bool} obrigatório")
		return
	}
	if err := s.store.ActivateRecurringMatch(r.Context(), r.PathValue("id"), *body.Active); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteRecurring(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRecurringMatch(r.Context(), r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
