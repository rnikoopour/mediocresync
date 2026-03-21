package api

import (
	"log/slog"
	"net/http"

	"github.com/rnikoopour/mediocresync/internal/config"
)

type settingsHandler struct {
	logLevel *slog.LevelVar
}

type settingsResponse struct {
	LogLevel config.LogLevel `json:"log_level"`
}

func (h *settingsHandler) get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{LogLevel: slogLevelToConfig(h.logLevel.Level())})
}

func (h *settingsHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LogLevel config.LogLevel `json:"log_level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	level, ok := configLevelToSlog(req.LogLevel)
	if !ok {
		writeError(w, http.StatusBadRequest, "log_level must be one of: debug, info, warn, error")
		return
	}

	h.logLevel.Set(level)
	slog.Info("log level changed", "level", req.LogLevel)
	w.WriteHeader(http.StatusNoContent)
}

func configLevelToSlog(l config.LogLevel) (slog.Level, bool) {
	switch l {
	case config.LogLevelDebug:
		return slog.LevelDebug, true
	case config.LogLevelInfo:
		return slog.LevelInfo, true
	case config.LogLevelWarn:
		return slog.LevelWarn, true
	case config.LogLevelError:
		return slog.LevelError, true
	default:
		return 0, false
	}
}

func slogLevelToConfig(l slog.Level) config.LogLevel {
	switch {
	case l <= slog.LevelDebug:
		return config.LogLevelDebug
	case l <= slog.LevelInfo:
		return config.LogLevelInfo
	case l <= slog.LevelWarn:
		return config.LogLevelWarn
	default:
		return config.LogLevelError
	}
}
