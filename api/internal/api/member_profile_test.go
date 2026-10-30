package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMemberProfileShowsOnlyPublicData(t *testing.T) {
	s, raw := apiTestServer(t)
	viewer := insertUser(t, raw, "member", "player", "viewer@example.invalid", "senha-de-teste-1")
	member := insertUser(t, raw, "member", "admin", "member@example.invalid", "senha-de-teste-1")
	guest := insertUser(t, raw, "guest", "player", "", "")

	path := "/api/users/" + member + "/profile"
	if rec := sessionRequest(t, s, "GET", path, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão: %d", rec.Code)
	}
	rec := sessionRequest(t, s, "GET", path, "", viewer)
	if rec.Code != http.StatusOK {
		t.Fatalf("perfil de membro: %d %s", rec.Code, rec.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "name", "avatar_color", "avatar_url", "role", "created_at", "stats"} {
		if _, ok := data[key]; !ok {
			t.Errorf("campo público %q ausente", key)
		}
	}
	for _, key := range []string{"email", "phone", "status", "inactive_reason", "delinquent", "last_payment_at", "session_version"} {
		if _, ok := data[key]; ok {
			t.Errorf("campo privado %q exposto", key)
		}
	}
	if data["id"] != member {
		t.Errorf("id: %v", data["id"])
	}
	stats, ok := data["stats"].(map[string]any)
	if !ok || stats["matches_played"] != float64(0) || stats["top_scorer_count"] != float64(0) || stats["worst_count"] != float64(0) {
		t.Errorf("estatísticas: %v", data["stats"])
	}
	if rec := sessionRequest(t, s, "GET", "/api/users/"+guest+"/profile", "", viewer); rec.Code != http.StatusNotFound {
		t.Errorf("perfil avulso: %d", rec.Code)
	}
	if rec := sessionRequest(t, s, "GET", "/api/users/not-a-uuid/profile", "", viewer); rec.Code != http.StatusBadRequest {
		t.Errorf("id inválido: %d", rec.Code)
	}
}
