package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"futdarapaziada/api/internal/auth"
)

func avatarRequest(t *testing.T, s *Server, method, path, userID string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, body)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
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

func TestAvatarUploadAndRemoval(t *testing.T) {
	s, raw := apiTestServer(t)
	s.cfg.MediaDir = t.TempDir()
	userID := insertUser(t, raw, "member", "player", "avatar@example.invalid", "senha-de-teste-1")
	otherID := insertUser(t, raw, "member", "player", "other@example.invalid", "senha-de-teste-1")

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var picture bytes.Buffer
	if err := png.Encode(&picture, img); err != nil {
		t.Fatal(err)
	}
	upload := func(data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "profile.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return avatarRequest(t, s, "PUT", "/api/me/avatar", userID, &body, writer.FormDataContentType())
	}

	if rec := upload([]byte("not an image")); rec.Code != http.StatusBadRequest {
		t.Fatalf("arquivo inválido: %d %s", rec.Code, rec.Body.String())
	}
	rec := upload(picture.Bytes())
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var user struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil || user.AvatarURL == "" {
		t.Fatalf("avatar_url ausente: %v %s", err, rec.Body.String())
	}
	if rec := avatarRequest(t, s, "GET", user.AvatarURL, "", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("arquivo sem sessão: %d", rec.Code)
	}
	if rec := avatarRequest(t, s, "GET", user.AvatarURL, otherID, nil, ""); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Errorf("arquivo para outro membro: %d", rec.Code)
	}
	if rec := avatarRequest(t, s, "DELETE", "/api/me/avatar", userID, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("remoção: %d %s", rec.Code, rec.Body.String())
	}
	if rec := avatarRequest(t, s, "GET", user.AvatarURL, userID, nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("arquivo removido: %d", rec.Code)
	}
	var stored string
	if err := raw.QueryRow(context.Background(), `select avatar_url from users where id = $1`, userID).Scan(&stored); err != nil || stored != "" {
		t.Errorf("avatar no banco após remoção: %q, %v", stored, err)
	}
}
