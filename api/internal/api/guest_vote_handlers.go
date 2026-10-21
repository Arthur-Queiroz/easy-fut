package api

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/matches"
	"futdarapaziada/api/internal/store"
)

// Participação avulsa — voto pelo link privado e reenvio de mensagens
// (fase C). O avulso não tem sessão: o token do link identifica a inscrição
// e só permite ler a enquete e votar na partida dela.

const errGuestVoteLinkInvalid = "link de votação inválido ou substituído por um mais recente"

// guestVoteClientIP identifica quem chama os endpoints públicos. Em
// produção a API fica atrás do Cloudflare Tunnel e de dois Caddy, então
// RemoteAddr é sempre o proxy; o Cloudflare grava o IP real em
// CF-Connecting-IP (e sobrescreve o que o cliente mandar). Fora do túnel
// (local, testes) vale o RemoteAddr.
func guestVoteClientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// guestVoteAccess aplica o limite por IP, encontra o avulso pelo token e
// confere se a votação aceita o link. Responde e devolve ok = false quando
// não dá para seguir: 429, 404 (token inválido) ou 410 (votação fechada).
func (s *Server) guestVoteAccess(w http.ResponseWriter, r *http.Request) (store.GuestVoter, store.Match, bool) {
	if !s.guestVoteLimiter.Allow(guestVoteClientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "muitas tentativas; aguarde alguns minutos")
		return store.GuestVoter{}, store.Match{}, false
	}
	voter, err := s.store.GuestVoterByToken(r.Context(), guests.HashVoteToken(r.PathValue("token")))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, errGuestVoteLinkInvalid)
		return voter, store.Match{}, false
	}
	if err != nil {
		writeStoreError(w, err)
		return voter, store.Match{}, false
	}
	match, err := s.store.MatchByID(r.Context(), voter.MatchID)
	if err != nil {
		writeStoreError(w, err)
		return voter, match, false
	}
	if !guests.VoteLinkOpen(match.Status, voter.ExpiresAt, time.Now()) {
		writeError(w, http.StatusGone, matches.ErrVotingClosed.Error())
		return voter, match, false
	}
	return voter, match, true
}

// guestVoteParticipant é o que o avulso vê de cada participante: só o
// necessário para votar.
type guestVoteParticipant struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	AvatarColor string `json:"avatar_color"`
}

// handleGuestVote (GET /public/guest-vote/{token}) devolve a enquete da
// partida do link: participantes (quem foi, membros e avulsos), os votos do
// avulso e o prazo.
func (s *Server) handleGuestVote(w http.ResponseWriter, r *http.Request) {
	voter, match, ok := s.guestVoteAccess(w, r)
	if !ok {
		return
	}
	entries, err := s.store.Confirmations(r.Context(), match.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	participants := []guestVoteParticipant{}
	for _, e := range entries {
		if e.Response == "going" {
			participants = append(participants, guestVoteParticipant{UserID: e.UserID, Name: e.Name, AvatarColor: e.AvatarColor})
		}
	}
	myVotes, err := s.store.MyVotes(r.Context(), match.ID, voter.UserID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"match":            match,
		"participants":     participants,
		"my_votes":         myVotes,
		"voting_closes_at": match.VotingClosesAt,
	})
}

// handleGuestReplaceVotes (PUT /public/guest-vote/{token}/votes) é o
// "Enviar votos" do avulso: as mesmas regras e respostas de
// PUT /matches/{id}/votes, com o avulso como eleitor. Votação fechada é 410,
// como no GET.
func (s *Server) handleGuestReplaceVotes(w http.ResponseWriter, r *http.Request) {
	voter, match, ok := s.guestVoteAccess(w, r)
	if !ok {
		return
	}
	votes, ok := decodeVoteSubmission(w, r)
	if !ok {
		return
	}
	err := s.replaceVotes(r.Context(), match.ID, voter.UserID, votes)
	if errors.Is(err, matches.ErrVotingClosed) {
		writeError(w, http.StatusGone, err.Error())
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// replaceVotes valida o voto completo (matches.Submission) e o grava; é o
// mesmo caminho do membro e do avulso.
func (s *Server) replaceVotes(ctx context.Context, matchID, voterID string, votes map[matches.Category][]string) error {
	match, err := s.store.MatchByID(ctx, matchID)
	if err != nil {
		return err
	}
	participants, err := s.store.MatchParticipants(ctx, matchID)
	if err != nil {
		return err
	}
	submission := matches.Submission{
		MatchStatus:  match.Status,
		VoterID:      voterID,
		Votes:        votes,
		Participants: participants,
	}
	if err := submission.Validate(); err != nil {
		return err
	}
	return s.store.ReplaceVotes(ctx, matchID, voterID, votes)
}

// handleResendGuestMessage (POST /admin/guest-bookings/{id}/resend) reenvia
// a confirmação ou o link de voto. O link reenviado gira o token.
func (s *Server) handleResendGuestMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind string `json:"kind"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := s.store.ResendGuestMessage(r.Context(), r.PathValue("id"), body.Kind, s.cfg.AppURL, time.Now()); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// scheduleGuestVoteLinks agenda os links de voto dos avulsos quando o admin
// abre a votação. Falha aqui não desfaz a abertura: o job de votação
// (openUnopenedVoting) agenda o que faltar.
func (s *Server) scheduleGuestVoteLinks(ctx context.Context, matchID string) {
	if _, err := s.store.ScheduleGuestVoteLinks(ctx, matchID, s.cfg.AppURL, time.Now()); err != nil {
		log.Printf("agendando links de voto dos avulsos da partida %s: %v", matchID, err)
	}
}
