package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/store"
)

// memberProfile contém somente dados que outro membro pode ver.
type memberProfile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	AvatarColor string          `json:"avatar_color"`
	AvatarURL   string          `json:"avatar_url"`
	Role        string          `json:"role"`
	CreatedAt   time.Time       `json:"created_at"`
	Stats       store.UserStats `json:"stats"`
}

func (s *Server) handleMemberProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "jogador inválido")
		return
	}
	user, err := s.store.UserByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && user.Kind != store.KindMember) {
		writeError(w, http.StatusNotFound, "jogador não encontrado")
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	stats, err := s.store.UserStats(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memberProfile{
		ID: user.ID, Name: user.Name, AvatarColor: user.AvatarColor,
		AvatarURL: user.AvatarURL, Role: user.Role, CreatedAt: user.CreatedAt,
		Stats: stats,
	})
}
