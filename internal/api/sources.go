package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rnikoopour/mediocresync/internal/crypto"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/ftpes"
)

type sourceRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"` // "ftpes" | "git"
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	SkipTLSVerify  bool   `json:"skip_tls_verify"`
	EnableEPSV     bool   `json:"enable_epsv"`
	AuthType       string `json:"auth_type"`
	AuthCredential string `json:"auth_credential"`
}

type sourceResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Host          string `json:"host,omitempty"`
	Port          int    `json:"port,omitempty"`
	Username      string `json:"username,omitempty"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	EnableEPSV    bool   `json:"enable_epsv"`
	AuthType      string `json:"auth_type,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toSourceResponse(s *db.Source) sourceResponse {
	return sourceResponse{
		ID:            s.ID,
		Name:          s.Name,
		Type:          s.Type,
		Host:          s.Host,
		Port:          s.Port,
		Username:      s.Username,
		SkipTLSVerify: s.SkipTLSVerify,
		EnableEPSV:    s.EnableEPSV,
		AuthType:      s.AuthType,
		CreatedAt:     s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

type sourcesHandler struct {
	repo   *db.SourceRepository
	encKey []byte
}

func (h *sourcesHandler) list(w http.ResponseWriter, r *http.Request) {
	sources, err := h.repo.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sources")
		return
	}
	resp := make([]sourceResponse, len(sources))
	for i, s := range sources {
		resp[i] = toSourceResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *sourcesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req sourceRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	src := &db.Source{
		Name:          req.Name,
		Type:          req.Type,
		SkipTLSVerify: req.SkipTLSVerify,
		EnableEPSV:    req.EnableEPSV,
		AuthType:      req.AuthType,
	}

	switch req.Type {
	case db.SourceTypeFTPES:
		if req.Host == "" || req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "host, username, and password are required for ftpes sources")
			return
		}
		if req.Port == 0 {
			req.Port = 21
		}
		encrypted, err := crypto.Encrypt(h.encKey, req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		src.Host = req.Host
		src.Port = req.Port
		src.Username = req.Username
		src.Password = encrypted
	case db.SourceTypeGit:
		if req.AuthCredential != "" {
			encrypted, err := crypto.Encrypt(h.encKey, req.AuthCredential)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to encrypt credential")
				return
			}
			src.AuthCredential = encrypted
		}
	default:
		writeError(w, http.StatusBadRequest, "type must be 'ftpes' or 'git'")
		return
	}

	if err := h.repo.Create(src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create source")
		return
	}
	writeJSON(w, http.StatusCreated, toSourceResponse(src))
}

func (h *sourcesHandler) get(w http.ResponseWriter, r *http.Request) {
	src, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get source")
		return
	}
	if src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	writeJSON(w, http.StatusOK, toSourceResponse(src))
}

func (h *sourcesHandler) update(w http.ResponseWriter, r *http.Request) {
	src, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	var req sourceRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	src.Name = req.Name
	src.SkipTLSVerify = req.SkipTLSVerify
	src.EnableEPSV = req.EnableEPSV
	src.AuthType = req.AuthType

	switch src.Type {
	case db.SourceTypeFTPES:
		src.Host = req.Host
		src.Port = req.Port
		src.Username = req.Username
		if req.Password != "" {
			encrypted, err := crypto.Encrypt(h.encKey, req.Password)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to encrypt password")
				return
			}
			src.Password = encrypted
		}
	case db.SourceTypeGit:
		if req.AuthCredential != "" {
			encrypted, err := crypto.Encrypt(h.encKey, req.AuthCredential)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to encrypt credential")
				return
			}
			src.AuthCredential = encrypted
		}
	}

	if err := h.repo.Update(src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update source")
		return
	}
	writeJSON(w, http.StatusOK, toSourceResponse(src))
}

func (h *sourcesHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *sourcesHandler) browse(w http.ResponseWriter, r *http.Request) {
	src, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	if src.Type != db.SourceTypeFTPES {
		writeError(w, http.StatusBadRequest, "browse is only supported for ftpes sources")
		return
	}

	remotePath := r.URL.Query().Get("path")
	if remotePath == "" {
		remotePath = "/"
	}

	password, err := crypto.Decrypt(h.encKey, src.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	client, err := ftpes.Dial(src.Host, src.Port, src.SkipTLSVerify, src.EnableEPSV)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer client.Close()

	if err := client.Login(src.Username, password); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	entries, err := client.List(remotePath)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	type entry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}
	resp := make([]entry, len(entries))
	for i, e := range entries {
		resp[i] = entry{Name: e.Name, Path: e.Path, IsDir: e.IsDir}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *sourcesHandler) test(w http.ResponseWriter, r *http.Request) {
	src, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	if src.Type != db.SourceTypeFTPES {
		writeError(w, http.StatusBadRequest, "test is only supported for ftpes sources")
		return
	}

	password, err := crypto.Decrypt(h.encKey, src.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	client, err := ftpes.Dial(src.Host, src.Port, src.SkipTLSVerify, src.EnableEPSV)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer client.Close()

	if err := client.Login(src.Username, password); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// testDirect tests credentials supplied in the request body without requiring a saved source.
// When editing an ftpes source, if password is omitted, the saved password for the given id is used as a fallback.
func (h *sourcesHandler) testDirect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		sourceRequest
		FallbackID string `json:"fallback_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type != db.SourceTypeFTPES {
		writeError(w, http.StatusBadRequest, "test is only supported for ftpes sources")
		return
	}

	password := req.Password
	if password == "" && req.FallbackID != "" {
		src, err := h.repo.Get(req.FallbackID)
		if err != nil || src == nil {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		decrypted, err := crypto.Decrypt(h.encKey, src.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to decrypt password")
			return
		}
		password = decrypted
	}

	if req.Host == "" || req.Username == "" || password == "" {
		writeError(w, http.StatusBadRequest, "host, username, and password are required")
		return
	}
	if req.Port == 0 {
		req.Port = 21
	}

	client, err := ftpes.Dial(req.Host, req.Port, req.SkipTLSVerify, req.EnableEPSV)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer client.Close()

	if err := client.Login(req.Username, password); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
