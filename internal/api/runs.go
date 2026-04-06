package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rnikoopour/mediocresync/internal/db"
	"github.com/rnikoopour/mediocresync/internal/sse"
)

type runResponse struct {
	ID             string             `json:"id"`
	JobID          string             `json:"job_id"`
	Status         string             `json:"status"`
	StartedAt      string             `json:"started_at"`
	FinishedAt     *string            `json:"finished_at,omitempty"`
	TotalFiles     int                `json:"total_files"`
	CopiedFiles    int                `json:"copied_files"`
	SkippedFiles   int                `json:"skipped_files"`
	FailedFiles    int                `json:"failed_files"`
	TotalSizeBytes      int64              `json:"total_size_bytes"`
	BytesCopied         int64              `json:"bytes_copied"`
	TransfersStartedAt  *string            `json:"transfers_started_at,omitempty"`
	ErrorMsg            *string            `json:"error_msg,omitempty"`
	Transfers      []transferResponse `json:"transfers"`
}

type transferResponse struct {
	ID                 string  `json:"id"`
	RemotePath         string  `json:"remote_path"`
	LocalPath          string  `json:"local_path"`
	SizeBytes          int64   `json:"size_bytes"`
	BytesXferred       int64   `json:"bytes_xferred"`
	DurationMs         *int64  `json:"duration_ms,omitempty"`
	Status             string  `json:"status"`
	ErrorMsg           *string `json:"error_msg,omitempty"`
	StartedAt          *string `json:"started_at,omitempty"`
	FinishedAt         *string `json:"finished_at,omitempty"`
	PreviousCommitHash *string `json:"previous_commit_hash,omitempty"`
	CurrentCommitHash  *string `json:"current_commit_hash,omitempty"`
}

func toRunResponse(run *db.Run, transfers []*db.Transfer) runResponse {
	r := runResponse{
		ID:             run.ID,
		JobID:          run.JobID,
		Status:         run.Status,
		StartedAt:      run.StartedAt.Format("2006-01-02T15:04:05Z"),
		TotalFiles:     run.TotalFiles,
		CopiedFiles:    run.CopiedFiles,
		SkippedFiles:   run.SkippedFiles,
		FailedFiles:    run.FailedFiles,
		TotalSizeBytes: run.TotalSizeBytes,
		BytesCopied:    run.BytesCopied,
		ErrorMsg:       run.ErrorMsg,
		Transfers:      []transferResponse{},
	}
	if run.FinishedAt != nil {
		s := run.FinishedAt.Format("2006-01-02T15:04:05Z")
		r.FinishedAt = &s
	}
	if run.TransfersStartedAt != nil {
		s := run.TransfersStartedAt.Format("2006-01-02T15:04:05Z")
		r.TransfersStartedAt = &s
	}
	for _, t := range transfers {
		tr := transferResponse{
			ID:                 t.ID,
			RemotePath:         t.RemotePath,
			LocalPath:          t.LocalPath,
			SizeBytes:          t.SizeBytes,
			BytesXferred:       t.BytesXferred,
			DurationMs:         t.DurationMs,
			Status:             t.Status,
			ErrorMsg:           t.ErrorMsg,
			PreviousCommitHash: t.PreviousCommitHash,
			CurrentCommitHash:  t.CurrentCommitHash,
		}
		if t.StartedAt != nil {
			s := t.StartedAt.Format("2006-01-02T15:04:05Z")
			tr.StartedAt = &s
		}
		if t.FinishedAt != nil {
			s := t.FinishedAt.Format("2006-01-02T15:04:05Z")
			tr.FinishedAt = &s
		}
		r.Transfers = append(r.Transfers, tr)
	}
	return r
}

type runsHandler struct {
	runs      *db.RunRepository
	transfers *db.TransferRepository
	broker    *sse.Broker
	appCtx    context.Context
}

func (h *runsHandler) listByJob(w http.ResponseWriter, r *http.Request) {
	runs, err := h.runs.ListByJob(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}
	resp := make([]runResponse, len(runs))
	for i, run := range runs {
		resp[i] = toRunResponse(run, nil)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *runsHandler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.runs.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get run")
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	transfers, err := h.transfers.ListByRun(run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get transfers")
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run, transfers))
}

func (h *runsHandler) progress(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := h.broker.Subscribe(runID)
	defer unsub()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				// broker closed the channel — run is finished
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(ev)
			if ev.RunStatus != "" {
				fmt.Fprintf(w, "event: run_status\ndata: %s\n\n", data)
			} else {
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-h.appCtx.Done():
			return
		}
	}
}
