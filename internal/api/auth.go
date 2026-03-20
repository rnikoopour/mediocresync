package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/rnikoopour/mediocresync/internal/db"
)

const (
	sessionCookie    = "session"
	sessionMaxAge    = 7 * 24 * 60 * 60 // 7 days in seconds
	sessionInactivity = 7 * 24 * 60 * 60 // seconds
)

type authHandler struct {
	repo *db.AuthRepository
}

// --- request / response types ---

type setupRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateCredentialsRequest struct {
	CurrentPassword string `json:"current_password"`
	Username        string `json:"username"`
	NewPassword     string `json:"new_password"`
}

type meResponse struct {
	Username string `json:"username"`
}

// --- handlers ---

// POST /api/auth/setup — initial credential configuration.
func (h *authHandler) setup(w http.ResponseWriter, r *http.Request) {
	_, _, configured, err := h.repo.GetCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if configured {
		writeError(w, http.StatusConflict, "already_configured")
		return
	}

	var req setupRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Password != req.PasswordConfirm {
		writeError(w, http.StatusBadRequest, "passwords do not match")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.repo.SetCredentials(req.Username, string(hash)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// POST /api/auth/login — authenticate and receive a session cookie.
func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	username, passwordHash, configured, err := h.repo.GetCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !configured {
		writeError(w, http.StatusServiceUnavailable, "setup_required")
		return
	}

	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Constant-time username compare to avoid timing-based enumeration.
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(username)) == 1
	passwordMatch := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) == nil
	if !usernameMatch || !passwordMatch {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	token, err := generateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.repo.CreateSession(token); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
}

// POST /api/auth/logout — invalidate the current session.
func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = h.repo.DeleteSession(cookie.Value) // best-effort
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
}

// PUT /api/auth/credentials — update username and/or password.
func (h *authHandler) updateCredentials(w http.ResponseWriter, r *http.Request) {
	username, passwordHash, _, err := h.repo.GetCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req updateCredentialsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password is required")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	newUsername := username
	if req.Username != "" {
		newUsername = req.Username
	}

	newHash := passwordHash
	if req.NewPassword != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		newHash = string(h)
	}

	if err := h.repo.SetCredentials(newUsername, newHash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.repo.DeleteAllSessions(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GET /api/auth/me — return the current username.
func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	username, _, _, err := h.repo.GetCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{Username: username})
}

// --- helpers ---

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
