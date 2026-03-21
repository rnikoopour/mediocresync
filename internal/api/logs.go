package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rnikoopour/mediocresync/internal/logbuffer"
)

type logsHandler struct {
	buf *logbuffer.Buffer
}

func (h *logsHandler) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send buffered history first.
	if h.buf != nil {
		for _, entry := range h.buf.Entries() {
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
	}
	flusher.Flush()

	var ch <-chan logbuffer.Entry
	var unsub func()
	if h.buf != nil {
		ch, unsub = h.buf.Subscribe()
		defer unsub()
	} else {
		ch = make(chan logbuffer.Entry) // never receives
	}

	for {
		select {
		case entry := <-ch:
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
