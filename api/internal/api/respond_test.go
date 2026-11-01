package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/matches"
)

// Os status e mensagens da votação são o contrato que a PWA já trata; a
// extração das regras para matches não pode mudá-los.
func TestWriteStoreErrorVotingContract(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{matches.ErrInvalidCategory, http.StatusBadRequest},
		{matches.ErrSelfWorstVote, http.StatusBadRequest},
		{matches.ErrNotParticipant, http.StatusForbidden},
		{matches.ErrVotingClosed, http.StatusConflict},
		{matches.ErrDuplicateVote, http.StatusConflict},
		{matches.ErrVoteLimit, http.StatusConflict},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeStoreError(rec, tc.err)

		var body apiError
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tc.status || body.Error != tc.err.Error() {
			t.Errorf("%v: %d %q, want %d com a mensagem do domínio", tc.err, rec.Code, body.Error, tc.status)
		}
	}
	// O sentinel ErrVoteLimit carrega o texto padrão (2 votos) e é casado por
	// errors.Is; VoteLimitError ajusta o texto para as categorias de 1 voto.
	if got := matches.VoteLimitError(matches.TopScorer).Error(); got != "você já usou seu(s) 2 voto(s) nesta categoria" {
		t.Errorf("mensagem do limite (2 votos) mudou: %q", got)
	}
	if got := matches.VoteLimitError(matches.BestGoal).Error(); got != "você já usou seu(s) 2 voto(s) nesta categoria Sendo melhor gol, vale 1 voto." {
		t.Errorf("mensagem do limite (1 voto) mudou: %q", got)
	}
}

// Contrato da participação avulsa: conflitos de venda em 409 e dados do
// formulário em 400, sempre com a mensagem do domínio.
func TestWriteStoreErrorGuestsContract(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{guests.ErrSoldOut, http.StatusConflict},
		{guests.ErrSaleClosed, http.StatusConflict},
		{guests.ErrPhoneAlreadyBooked, http.StatusConflict},
		{guests.ErrInvalidTransition, http.StatusConflict},
		{guests.ErrInvalidPhone, http.StatusBadRequest},
		{guests.ErrInvalidEmail, http.StatusBadRequest},
		{guests.ErrInvalidName, http.StatusBadRequest},
		{guests.ErrInvalidExpense, http.StatusBadRequest},
		// Fase B: ações do admin.
		{guests.ErrInvalidSlots, http.StatusBadRequest},
		{guests.ErrInvalidPrice, http.StatusBadRequest},
		{guests.ErrMatchNotOpen, http.StatusConflict},
		{guests.ErrSlotsBelowBooked, http.StatusConflict},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeStoreError(rec, tc.err)

		var body apiError
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tc.status || body.Error != tc.err.Error() {
			t.Errorf("%v: %d %q, want %d com a mensagem do domínio", tc.err, rec.Code, body.Error, tc.status)
		}
	}
}

func TestWriteStoreErrorBillingContract(t *testing.T) {
	rec := httptest.NewRecorder()
	writeStoreError(rec, billing.ErrInvalidFeeRate)

	var body apiError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest || body.Error != billing.ErrInvalidFeeRate.Error() {
		t.Errorf("ErrInvalidFeeRate: %d %q, want 400 com a mensagem do domínio", rec.Code, body.Error)
	}
}
