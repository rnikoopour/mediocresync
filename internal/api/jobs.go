package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rnikoopour/go-ftpes/internal/db"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
	"github.com/rnikoopour/go-ftpes/internal/sse"
)

type jobRequest struct {
	Name               string   `json:"name"`
	ConnectionID       string   `json:"connection_id"`
	RemotePath         string   `json:"remote_path"`
	LocalDest          string   `json:"local_dest"`
	IntervalValue      int      `json:"interval_value"`
	IntervalUnit       string   `json:"interval_unit"`
	Concurrency        int      `json:"concurrency"`
	RetryAttempts      int      `json:"retry_attempts"`
	RetryDelaySeconds  int      `json:"retry_delay_seconds"`
	Enabled            bool     `json:"enabled"`
	IncludePathFilters []string `json:"include_path_filters"`
	IncludeNameFilters []string `json:"include_name_filters"`
	ExcludePathFilters []string `json:"exclude_path_filters"`
	ExcludeNameFilters []string `json:"exclude_name_filters"`
}

type jobResponse struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	ConnectionID       string   `json:"connection_id"`
	RemotePath         string   `json:"remote_path"`
	LocalDest          string   `json:"local_dest"`
	IntervalValue      int      `json:"interval_value"`
	IntervalUnit       string   `json:"interval_unit"`
	Concurrency        int      `json:"concurrency"`
	RetryAttempts      int      `json:"retry_attempts"`
	RetryDelaySeconds  int      `json:"retry_delay_seconds"`
	Enabled            bool     `json:"enabled"`
	IncludePathFilters []string `json:"include_path_filters"`
	IncludeNameFilters []string `json:"include_name_filters"`
	ExcludePathFilters []string `json:"exclude_path_filters"`
	ExcludeNameFilters []string `json:"exclude_name_filters"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

func toJobResponse(j *db.SyncJob) jobResponse {
	return jobResponse{
		ID:                 j.ID,
		Name:               j.Name,
		ConnectionID:       j.ConnectionID,
		RemotePath:         j.RemotePath,
		LocalDest:          j.LocalDest,
		IntervalValue:      j.IntervalValue,
		IntervalUnit:       j.IntervalUnit,
		Concurrency:        j.Concurrency,
		RetryAttempts:      j.RetryAttempts,
		RetryDelaySeconds:  j.RetryDelaySeconds,
		Enabled:            j.Enabled,
		IncludePathFilters: j.IncludePathFilters,
		IncludeNameFilters: j.IncludeNameFilters,
		ExcludePathFilters: j.ExcludePathFilters,
		ExcludeNameFilters: j.ExcludeNameFilters,
		CreatedAt:          j.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          j.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

type jobsHandler struct {
	repo      *db.JobRepository
	fileState *db.FileStateRepository
	engine    *internalsync.Engine
	broker    *sse.Broker
	appCtx    context.Context
}

func (h *jobsHandler) list(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.repo.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	resp := make([]jobResponse, len(jobs))
	for i, j := range jobs {
		resp[i] = toJobResponse(j)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *jobsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req jobRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.ConnectionID == "" || req.LocalDest == "" {
		writeError(w, http.StatusBadRequest, "name, connection_id, and local_dest are required")
		return
	}
	if req.RemotePath == "" {
		req.RemotePath = "/"
	}
	if req.Concurrency < 1 {
		req.Concurrency = 1
	}
	if req.RetryAttempts < 1 {
		req.RetryAttempts = 3
	}
	if req.RetryDelaySeconds < 0 {
		req.RetryDelaySeconds = 2
	}

	job := &db.SyncJob{
		Name:               req.Name,
		ConnectionID:       req.ConnectionID,
		RemotePath:         req.RemotePath,
		LocalDest:          req.LocalDest,
		IntervalValue:      req.IntervalValue,
		IntervalUnit:       req.IntervalUnit,
		Concurrency:        req.Concurrency,
		RetryAttempts:      req.RetryAttempts,
		RetryDelaySeconds:  req.RetryDelaySeconds,
		Enabled:            req.Enabled,
		IncludePathFilters: req.IncludePathFilters,
		IncludeNameFilters: req.IncludeNameFilters,
		ExcludePathFilters: req.ExcludePathFilters,
		ExcludeNameFilters: req.ExcludeNameFilters,
	}
	if err := h.repo.Create(job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}
	writeJSON(w, http.StatusCreated, toJobResponse(job))
}

func (h *jobsHandler) get(w http.ResponseWriter, r *http.Request) {
	job, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, toJobResponse(job))
}

func (h *jobsHandler) update(w http.ResponseWriter, r *http.Request) {
	job, err := h.repo.Get(chi.URLParam(r, "id"))
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var req jobRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	job.Name = req.Name
	job.ConnectionID = req.ConnectionID
	job.RemotePath = req.RemotePath
	job.LocalDest = req.LocalDest
	job.IntervalValue = req.IntervalValue
	job.IntervalUnit = req.IntervalUnit
	job.Concurrency = max(req.Concurrency, 1)
	job.RetryAttempts = max(req.RetryAttempts, 1)
	job.RetryDelaySeconds = max(req.RetryDelaySeconds, 0)
	job.Enabled = req.Enabled
	job.IncludePathFilters = req.IncludePathFilters
	job.IncludeNameFilters = req.IncludeNameFilters
	job.ExcludePathFilters = req.ExcludePathFilters
	job.ExcludeNameFilters = req.ExcludeNameFilters

	if err := h.repo.Update(job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job")
		return
	}
	writeJSON(w, http.StatusOK, toJobResponse(job))
}

func (h *jobsHandler) putFileState(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	var body struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		Mtime     string `json:"mtime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	mtime, err := time.Parse(time.RFC3339Nano, body.Mtime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mtime format")
		return
	}
	fs := &db.FileState{
		JobID:      jobID,
		RemotePath: body.Path,
		SizeBytes:  body.SizeBytes,
		MTime:      mtime,
		CopiedAt:   time.Now(),
	}
	if err := h.fileState.Upsert(fs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set file state")
		return
	}
	h.engine.UpdateStoredPlanAction(jobID, body.Path, "skip")
	h.broker.Publish(jobID, sse.Event{Status: "plan_file_updated", PlanPath: body.Path, PlanAction: "skip"})
	w.WriteHeader(http.StatusNoContent)
}

func (h *jobsHandler) deleteFileState(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	remotePath := r.URL.Query().Get("path")
	if remotePath == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}
	if err := h.fileState.Delete(jobID, remotePath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear file state")
		return
	}
	h.engine.UpdateStoredPlanAction(jobID, remotePath, "copy")
	h.broker.Publish(jobID, sse.Event{Status: "plan_file_updated", PlanPath: remotePath, PlanAction: "copy"})
	w.WriteHeader(http.StatusNoContent)
}

func (h *jobsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.fileState.DeleteByJob(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete job state")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *jobsHandler) triggerRun(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	job, err := h.repo.Get(jobID)
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if h.engine.IsRunning(jobID) {
		writeError(w, http.StatusConflict, "job is already running")
		return
	}

	// RunJob is blocking and may run for minutes; dispatch to background so
	// the handler can return 202 immediately. Progress is streamed via SSE.
	// Errors are logged inside RunJob — ErrJobAlreadyRunning is excluded above,
	// and all other failures are surfaced through the run's status in the DB.
	go h.engine.RunJob(jobID) //nolint:errcheck

	w.WriteHeader(http.StatusAccepted)
}

func (h *jobsHandler) cancelRun(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if err := h.engine.CancelJob(jobID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *jobsHandler) planDismiss(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	h.engine.ClearStoredPlan(jobID)
	h.broker.Publish(jobID, sse.Event{Status: "plan_dismissed"})
	w.WriteHeader(http.StatusNoContent)
}

func (h *jobsHandler) planStart(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	job, err := h.repo.Get(jobID)
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	_ = job

	if err := h.engine.StartPlan(jobID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *jobsHandler) planEvents(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	job, err := h.repo.Get(jobID)
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	_ = job

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
	flusher.Flush()

	ch, unsub := h.engine.SubscribePlan(jobID)
	defer unsub()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if evt.Done || evt.Error != "" {
				return
			}
		case <-r.Context().Done():
			return
		case <-h.appCtx.Done():
			return
		}
	}
}

// jobEvents streams job-level events (e.g. run started) to connected clients.
// Unlike plan or run progress streams, this stays open indefinitely so any
// client on the job detail page is notified in real time.
func (h *jobsHandler) jobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

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
	flusher.Flush()

	ch, unsub := h.broker.Subscribe(jobID)
	defer unsub()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-h.appCtx.Done():
			return
		}
	}
}

