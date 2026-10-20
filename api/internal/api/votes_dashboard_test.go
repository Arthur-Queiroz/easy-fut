package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"futdarapaziada/api/internal/config"
)

// FUT-005: rotas de dashboard de votação exigem sessão admin.
// (validação de uuid no handler é testada indiretamente via smoke E2E)

func TestVotesDashboardRoutesRequireAuth(t *testing.T) {
	s := NewServer(config.Config{JWTSecret: []byte("test")}, nil, nil)
	for _, path := range []string{
		"/api/admin/matches/6f7c16fc-0000-0000-0000-000000000000/votes",
		"/api/admin/users/6f7c16fc-0000-0000-0000-000000000000/votes",
		"/api/admin/stats/votes",
	} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d, queria 401", path, w.Code)
		}
	}
}
