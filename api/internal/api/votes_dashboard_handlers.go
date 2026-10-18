package api

import (
	"net/http"

	"github.com/google/uuid"
)

// FUT-005: endpoints de dashboard de votação (admin).

// handleMatchVotes: tally completo + votos individuais (?user= filtra por votante).
func (s *Server) handleMatchVotes(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("id")
	if _, err := uuid.Parse(matchID); err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	tally, err := s.store.VoteTallyByMatch(r.Context(), matchID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var userFilter string
	if u := r.URL.Query().Get("user"); u != "" {
		if _, err := uuid.Parse(u); err != nil {
			writeError(w, http.StatusBadRequest, "user deve ser uuid")
			return
		}
		userFilter = u
	}
	flat, err := s.store.MatchesVotesFlat(r.Context(), matchID, userFilter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tally": orEmpty(tally),
		"flat":  orEmpty(flat),
	})
}

// handleUserVotes: histórico de votos de um usuário em todas as partidas.
func (s *Server) handleUserVotes(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if _, err := uuid.Parse(userID); err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	hist, err := s.store.UserVoteHistory(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"votes": orEmpty(hist)})
}

// handleVotesSummary: comparativo de participação entre partidas (?from=&to= YYYY-MM-DD).
func (s *Server) handleVotesSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	out, err := s.store.MatchesVoteSummary(r.Context(), from, to)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": orEmpty(out)})
}
