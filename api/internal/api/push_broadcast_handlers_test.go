package api

import (
	"futdarapaziada/api/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushBroadcastRoutesRequireSession(t *testing.T) {
	s := NewServer(config.Config{JWTSecret: []byte("test")}, nil, nil)
	for _, test := range []struct{ method, path string }{{"GET", "/api/admin/push/recipients"}, {"GET", "/api/admin/push/broadcasts"}, {"POST", "/api/admin/push/broadcasts"}} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`)))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: %d", test.method, test.path, w.Code)
		}
	}
}
func TestPushBroadcastDisabledWithoutKeys(t *testing.T) {
	s := NewServer(config.Config{}, nil, nil)
	w := httptest.NewRecorder()
	s.handleCreatePushBroadcast(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"body":"Oi","audience":"all"}`)))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}

// The store is nil: every invalid body must be rejected before reaching the database.
func TestPushBroadcastValidatesBeforeDatabase(t *testing.T) {
	s := NewServer(config.Config{VAPIDPublicKey: "public", VAPIDPrivateKey: "private", VAPIDSubject: "mailto:admin@example.com"}, nil, nil)
	for name, body := range map[string]string{
		"sem mensagem":       `{"body":"   ","audience":"all"}`,
		"mensagem longa":     `{"body":"` + strings.Repeat("á", 241) + `","audience":"all"}`,
		"título longo":       `{"title":"` + strings.Repeat("a", 61) + `","body":"Oi","audience":"all"}`,
		"público inválido":   `{"body":"Oi","audience":"admins"}`,
		"sem selecionados":   `{"body":"Oi","audience":"selected","user_ids":[]}`,
		"ID inválido":        `{"body":"Oi","audience":"selected","user_ids":["1; drop table users"]}`,
		"JSON inválido":      `{"body":`,
		"selecionados nulos": `{"body":"Oi","audience":"selected"}`,
	} {
		w := httptest.NewRecorder()
		s.handleCreatePushBroadcast(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Errorf("%s: got %d %s", name, w.Code, w.Body.String())
		}
	}
}
func TestPushBroadcastBodyLimitCountsCharacters(t *testing.T) {
	w := httptest.NewRecorder()
	in, ok := decodeBroadcastBody(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"body":"`+strings.Repeat("ç", 240)+`","audience":"all"}`)))
	if !ok || in.Title != defaultBroadcastTitle {
		t.Fatalf("240 accented characters must be accepted with the default title: %d %+v", w.Code, in)
	}
	w = httptest.NewRecorder()
	in, ok = decodeBroadcastBody(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"body":"Oi","audience":"selected","user_ids":["7f9c1c1e-0000-4000-8000-000000000001","7f9c1c1e-0000-4000-8000-000000000001"]}`)))
	if !ok || len(in.UserIDs) != 1 {
		t.Fatalf("duplicate IDs must collapse: %+v", in)
	}
}
