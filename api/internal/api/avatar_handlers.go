package api

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const maxAvatarBytes = 10 << 20

// Foto de perfil: somente JPEG e PNG decodificados e regravados para descartar metadados.
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+(1<<20))
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		writeError(w, http.StatusBadRequest, "foto inválida ou acima de 10 MB")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "campo 'file' ausente")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil || len(data) > maxAvatarBytes {
		writeError(w, http.StatusBadRequest, "foto acima de 10 MB")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		writeError(w, http.StatusBadRequest, "envie uma foto JPG ou PNG de até 4096 pixels por lado")
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		writeError(w, http.StatusBadRequest, "foto inválida")
		return
	}
	dir := filepath.Join(s.cfg.MediaDir, "avatars", user.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeStoreError(w, err)
		return
	}
	ext := ".jpg"
	if format == "png" {
		ext = ".png"
	}
	name := uuid.NewString() + ext
	path := filepath.Join(dir, name)
	dst, err := os.Create(path)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if format == "png" {
		err = png.Encode(dst, img)
	} else {
		err = jpeg.Encode(dst, img, &jpeg.Options{Quality: 85})
	}
	closeErr := dst.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		writeStoreError(w, err)
		return
	}
	url := fmt.Sprintf("/api/avatars/%s/%s", user.ID, name)
	previous, err := s.store.SetAvatar(r.Context(), user.ID, url)
	if err != nil {
		os.Remove(path)
		writeStoreError(w, err)
		return
	}
	s.removeOldAvatar(user.ID, previous)
	updated, err := s.store.UserByID(r.Context(), user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	previous, err := s.store.SetAvatar(r.Context(), user.ID, "")
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.removeOldAvatar(user.ID, previous)
	updated, err := s.store.UserByID(r.Context(), user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) removeOldAvatar(userID, url string) {
	prefix := "/api/avatars/" + userID + "/"
	if strings.HasPrefix(url, prefix) {
		name := strings.TrimPrefix(url, prefix)
		if name == filepath.Base(name) {
			_ = os.Remove(filepath.Join(s.cfg.MediaDir, "avatars", userID, name))
		}
	}
}

func (s *Server) handleServeAvatar(w http.ResponseWriter, r *http.Request) {
	id, filename := r.PathValue("id"), r.PathValue("filename")
	user, err := s.store.UserByID(r.Context(), id)
	if err != nil || user.AvatarURL != fmt.Sprintf("/api/avatars/%s/%s", id, filename) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.cfg.MediaDir, "avatars", id, filename))
}
