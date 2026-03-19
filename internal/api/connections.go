package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rnikoopour/go-ftpes/internal/crypto"
	"github.com/rnikoopour/go-ftpes/internal/db"
	"github.com/rnikoopour/go-ftpes/internal/ftpes"
)

type connectionRequest struct {
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	EnableEPSV    bool   `json:"enable_epsv"`
}

type connectionResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	EnableEPSV    bool   `json:"enable_epsv"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toConnectionResponse(c *db.Connection) connectionResponse {
	return connectionResponse{
		ID:            c.ID,
		Name:          c.Name,
		Host:          c.Host,
		Port:          c.Port,
		Username:      c.Username,
		SkipTLSVerify: c.SkipTLSVerify,
		EnableEPSV:    c.EnableEPSV,
		CreatedAt:     c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

type connectionsHandler struct {
	repo   *db.ConnectionRepository
	encKey []byte
}

func (h *connectionsHandler) list(w http.ResponseWriter, r *http.Request) {
	conns, err := h.repo.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list connections")
		return
	}
	resp := make([]connectionResponse, len(conns))
	for i, c := range conns {
		resp[i] = toConnectionResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *connectionsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req connectionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Host == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, host, username, and password are required")
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

	conn := &db.Connection{
		Name:          req.Name,
		Host:          req.Host,
		Port:          req.Port,
		Username:      req.Username,
		Password:      encrypted,
		SkipTLSVerify: req.SkipTLSVerify,
		EnableEPSV:    req.EnableEPSV,
	}
	if err := h.repo.Create(conn); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create connection")
		return
	}
	writeJSON(w, http.StatusCreated, toConnectionResponse(conn))
}

func (h *connectionsHandler) get(w http.ResponseWriter, r *http.Request) {
	conn, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get connection")
		return
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}
	writeJSON(w, http.StatusOK, toConnectionResponse(conn))
}

func (h *connectionsHandler) update(w http.ResponseWriter, r *http.Request) {
	conn, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	var req connectionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conn.Name = req.Name
	conn.Host = req.Host
	conn.Port = req.Port
	conn.Username = req.Username
	conn.SkipTLSVerify = req.SkipTLSVerify
	conn.EnableEPSV = req.EnableEPSV

	// Only re-encrypt if a new password was provided.
	if req.Password != "" {
		encrypted, err := crypto.Encrypt(h.encKey, req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		conn.Password = encrypted
	}

	if err := h.repo.Update(conn); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update connection")
		return
	}
	writeJSON(w, http.StatusOK, toConnectionResponse(conn))
}

func (h *connectionsHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete connection")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *connectionsHandler) browse(w http.ResponseWriter, r *http.Request) {
	conn, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	remotePath := r.URL.Query().Get("path")
	if remotePath == "" {
		remotePath = "/"
	}

	password, err := crypto.Decrypt(h.encKey, conn.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	client, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify, conn.EnableEPSV)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer client.Close()

	if err := client.Login(conn.Username, password); err != nil {
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

func (h *connectionsHandler) test(w http.ResponseWriter, r *http.Request) {
	conn, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || conn == nil {
		writeError(w, http.StatusNotFound, "connection not found")
		return
	}

	password, err := crypto.Decrypt(h.encKey, conn.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt password")
		return
	}

	client, err := ftpes.Dial(conn.Host, conn.Port, conn.SkipTLSVerify, conn.EnableEPSV)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer client.Close()

	if err := client.Login(conn.Username, password); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
