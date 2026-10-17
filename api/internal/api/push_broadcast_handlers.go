package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/push"
	"futdarapaziada/api/internal/store"
)

const (
	defaultBroadcastTitle = "Aviso do Easy Fut"
	maxBroadcastTitle     = 60
	maxBroadcastBody      = 240
	maxBroadcastUsers     = 500
)

func (s *Server) handleListPushRecipients(w http.ResponseWriter, r *http.Request) {
	recipients, err := s.store.ListPushRecipients(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(recipients))
}

func (s *Server) handleListPushBroadcasts(w http.ResponseWriter, r *http.Request) {
	broadcasts, err := s.store.ListPushBroadcasts(r.Context(), 20)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(broadcasts))
}

// decodeBroadcastBody valida o aviso antes de tocar no banco.
func decodeBroadcastBody(w http.ResponseWriter, r *http.Request) (store.BroadcastInput, bool) {
	var body struct {
		Title    string   `json:"title"`
		Body     string   `json:"body"`
		Audience string   `json:"audience"`
		UserIDs  []string `json:"user_ids"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if !decodeJSON(w, r, &body) {
		return store.BroadcastInput{}, false
	}
	in := store.BroadcastInput{Title: strings.TrimSpace(body.Title), Body: strings.TrimSpace(body.Body), Audience: body.Audience}
	if in.Title == "" {
		in.Title = defaultBroadcastTitle
	}
	switch {
	case in.Body == "":
		writeError(w, http.StatusBadRequest, "escreva a mensagem")
		return in, false
	case utf8.RuneCountInString(in.Body) > maxBroadcastBody:
		writeError(w, http.StatusBadRequest, "a mensagem pode ter até 240 caracteres")
		return in, false
	case utf8.RuneCountInString(in.Title) > maxBroadcastTitle:
		writeError(w, http.StatusBadRequest, "o título pode ter até 60 caracteres")
		return in, false
	case in.Audience != "all" && in.Audience != "selected":
		writeError(w, http.StatusBadRequest, "escolha os destinatários")
		return in, false
	}
	if in.Audience == "selected" {
		seen := map[string]bool{}
		for _, id := range body.UserIDs {
			if _, err := uuid.Parse(id); err != nil {
				writeError(w, http.StatusBadRequest, "user_ids contém ID inválido")
				return in, false
			}
			if !seen[id] {
				seen[id] = true
				in.UserIDs = append(in.UserIDs, id)
			}
		}
		if len(in.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "selecione ao menos um usuário")
			return in, false
		}
		if len(in.UserIDs) > maxBroadcastUsers {
			writeError(w, http.StatusBadRequest, "selecione no máximo 500 usuários")
			return in, false
		}
	}
	return in, true
}

func (s *Server) handleCreatePushBroadcast(w http.ResponseWriter, r *http.Request) {
	if !push.New(s.cfg.VAPIDPublicKey, s.cfg.VAPIDPrivateKey, s.cfg.VAPIDSubject).Enabled() {
		writeError(w, http.StatusServiceUnavailable, "notificações ainda não estão disponíveis")
		return
	}
	in, ok := decodeBroadcastBody(w, r)
	if !ok {
		return
	}
	broadcast, err := s.store.CreatePushBroadcast(r.Context(), in, currentUser(r).ID)
	if errors.Is(err, store.ErrNoPushRecipients) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, broadcast)
}
