package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	if got := matches.ErrVoteLimit.Error(); got != "você já usou seus 2 votos nesta categoria" {
		t.Errorf("mensagem do limite mudou: %q", got)
	}
}
