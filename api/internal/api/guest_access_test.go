package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"futdarapaziada/api/internal/auth"
	"futdarapaziada/api/internal/config"
	"futdarapaziada/api/internal/store"
)

// apiTestServer sobe o servidor sobre um schema isolado com as migrations
// aplicadas, como newTestStore no pacote store. Devolve também um pool cru
// para montar os fatos que a API ainda não cria (avulsos).
func apiTestServer(t *testing.T) (*Server, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	// Mesmo cuidado de store.newTestStore: pgcrypto em public, tolerando a
	// corrida com o pacote store migrando ao mesmo tempo.
	if _, err := root.Exec(ctx, `create extension if not exists pgcrypto with schema public`); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			root.Close()
			t.Fatal(err)
		}
	}
	schema := "api_test_" + uuid.NewString()[:8]
	if _, err := root.Exec(ctx, `create schema `+schema); err != nil {
		root.Close()
		t.Fatal(err)
	}
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()

	st, err := store.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		st.Close()
		raw.Close()
		_, err := root.Exec(ctx, `drop schema `+schema+` cascade`)
		root.Close()
		if err != nil {
			t.Error(err)
		}
	})
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return NewServer(config.Config{JWTSecret: []byte("test-secret")}, st, nil), raw
}

func insertUser(t *testing.T, raw *pgxpool.Pool, kind, role, email, password string) string {
	t.Helper()
	var emailArg, hashArg any
	if email != "" {
		emailArg = email
	}
	if password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		hashArg = hash
	}
	var id string
	if err := raw.QueryRow(context.Background(), `insert into users (name, email, password_hash, kind, role)
		values ($1, $2, $3, $4, $5) returning id`, "Teste "+kind, emailArg, hashArg, kind, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func sessionRequest(t *testing.T, s *Server, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		token, _, err := auth.NewSessionToken(s.cfg.JWTSecret, userID, 1)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// Avulso não tem sessão: mesmo com um token válido (vazado ou forjado com o
// segredo), as rotas autenticadas o recusam.
func TestGuestHasNoSession(t *testing.T) {
	s, raw := apiTestServer(t)
	member := insertUser(t, raw, "member", "player", "membro@example.invalid", "senha-de-teste-1")
	guest := insertUser(t, raw, "guest", "player", "", "")

	if rec := sessionRequest(t, s, "GET", "/api/me", "", member); rec.Code != http.StatusOK {
		t.Fatalf("membro em /me: %d %s", rec.Code, rec.Body)
	}
	for _, path := range []string{"/api/me", "/api/charges/me", "/api/matches"} {
		if rec := sessionRequest(t, s, "GET", path, "", guest); rec.Code != http.StatusUnauthorized {
			t.Errorf("avulso em %s: %d, want 401", path, rec.Code)
		}
	}
}

// requireActive também barra quem não é membro, caso um dia fique sem o
// requireAuth na frente.
func TestRequireActiveRefusesGuest(t *testing.T) {
	s := NewServer(config.Config{}, nil, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	for kind, want := range map[string]int{store.KindMember: http.StatusTeapot, store.KindGuest: http.StatusForbidden} {
		req := httptest.NewRequest("GET", "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), userKey, store.User{Status: "active", Kind: kind}))
		rec := httptest.NewRecorder()
		s.requireActive(next).ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", kind, rec.Code, want)
		}
	}
}

// Login e recuperação de senha nunca encontram um avulso, mesmo que ele
// tenha e-mail e senha gravados.
func TestGuestCannotLogin(t *testing.T) {
	s, raw := apiTestServer(t)
	insertUser(t, raw, "guest", "player", "avulso@example.invalid", "senha-de-teste-1")
	insertUser(t, raw, "member", "player", "membro@example.invalid", "senha-de-teste-1")

	rec := sessionRequest(t, s, "POST", "/api/login", `{"email":"avulso@example.invalid","password":"senha-de-teste-1"}`, "")
	if rec.Code != http.StatusUnauthorized || len(rec.Result().Cookies()) != 0 {
		t.Errorf("login do avulso: %d, cookies %v", rec.Code, rec.Result().Cookies())
	}
	rec = sessionRequest(t, s, "POST", "/api/login", `{"email":"membro@example.invalid","password":"senha-de-teste-1"}`, "")
	if rec.Code != http.StatusOK {
		t.Errorf("login do membro: %d %s", rec.Code, rec.Body)
	}
}

func TestAdminUsersKindFilter(t *testing.T) {
	s, raw := apiTestServer(t)
	admin := insertUser(t, raw, "member", "admin", "admin@example.invalid", "senha-de-teste-1")
	guest := insertUser(t, raw, "guest", "player", "", "")

	list := func(query string) (int, []store.User, int) {
		t.Helper()
		rec := sessionRequest(t, s, "GET", "/api/admin/users"+query, "", admin)
		var body struct {
			Users []store.User `json:"users"`
			Total int          `json:"total"`
		}
		if rec.Code == http.StatusOK {
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		return rec.Code, body.Users, body.Total
	}

	code, users, total := list("")
	if code != http.StatusOK || total != 1 || len(users) != 1 || users[0].ID != admin {
		t.Errorf("padrão: %d, %d usuários (total %d)", code, len(users), total)
	}
	code, users, total = list("?kind=guest")
	if code != http.StatusOK || total != 1 || len(users) != 1 || users[0].ID != guest || users[0].Kind != "guest" {
		t.Errorf("avulsos: %d, %+v (total %d)", code, users, total)
	}
	if code, _, total = list("?kind=member"); code != http.StatusOK || total != 1 {
		t.Errorf("membros: %d (total %d)", code, total)
	}
	if code, _, _ = list("?kind=admin"); code != http.StatusBadRequest {
		t.Errorf("tipo inválido: %d, want 400", code)
	}
}

// O admin não promove nem ativa/inativa um avulso como se fosse membro.
func TestAdminCannotManageGuestAsMember(t *testing.T) {
	s, raw := apiTestServer(t)
	admin := insertUser(t, raw, "member", "admin", "admin@example.invalid", "senha-de-teste-1")
	guest := insertUser(t, raw, "guest", "player", "", "")

	for _, body := range []string{`{"role":"admin"}`, `{"status":"inactive","reason":"x"}`} {
		rec := sessionRequest(t, s, "PATCH", "/api/admin/users/"+guest, body, admin)
		if rec.Code != http.StatusConflict {
			t.Errorf("%s no avulso: %d, want 409", body, rec.Code)
		}
	}
	var role, status string
	if err := raw.QueryRow(context.Background(), `select role, status from users where id = $1`, guest).Scan(&role, &status); err != nil {
		t.Fatal(err)
	}
	if role != "player" || status != "active" {
		t.Errorf("avulso mudou: %s/%s", role, status)
	}
	// Corrigir o nome do avulso continua possível.
	if rec := sessionRequest(t, s, "PATCH", "/api/admin/users/"+guest, `{"name":"Avulso Corrigido"}`, admin); rec.Code != http.StatusOK {
		t.Errorf("corrigir nome: %d %s", rec.Code, rec.Body)
	}
}
