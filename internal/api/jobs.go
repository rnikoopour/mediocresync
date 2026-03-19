package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rnikoopour/go-ftpes/internal/db"
	internalsync "github.com/rnikoopour/go-ftpes/internal/sync"
)

type jobRequest struct {
	Name              string   `json:"name"`
	ConnectionID      string   `json:"connection_id"`
	RemotePath        string   `json:"remote_path"`
	LocalDest         string   `json:"local_dest"`
	IntervalValue     int      `json:"interval_value"`
	IntervalUnit      string   `json:"interval_unit"`
	Concurrency       int      `json:"concurrency"`
	RetryAttempts     int      `json:"retry_attempts"`
	RetryDelaySeconds int      `json:"retry_delay_seconds"`
	Enabled           bool     `json:"enabled"`
	IncludeFilters    []string `json:"include_filters"`
	ExcludeFilters    []string `json:"exclude_filters"`
}

type jobResponse struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	ConnectionID      string   `json:"connection_id"`
	RemotePath        string   `json:"remote_path"`
	LocalDest         string   `json:"local_dest"`
	IntervalValue     int      `json:"interval_value"`
	IntervalUnit      string   `json:"interval_unit"`
	Concurrency       int      `json:"concurrency"`
	RetryAttempts     int      `json:"retry_attempts"`
	RetryDelaySeconds int      `json:"retry_delay_seconds"`
	Enabled           bool     `json:"enabled"`
	IncludeFilters    []string `json:"include_filters"`
	ExcludeFilters    []string `json:"exclude_filters"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func toJobResponse(j *db.SyncJob) jobResponse {
	return jobResponse{
		ID:                j.ID,
		Name:              j.Name,
		ConnectionID:      j.ConnectionID,
		RemotePath:        j.RemotePath,
		LocalDest:         j.LocalDest,
		IntervalValue:     j.IntervalValue,
		IntervalUnit:      j.IntervalUnit,
		Concurrency:       j.Concurrency,
		RetryAttempts:     j.RetryAttempts,
		RetryDelaySeconds: j.RetryDelaySeconds,
		Enabled:           j.Enabled,
		IncludeFilters:    j.IncludeFilters,
		ExcludeFilters:    j.ExcludeFilters,
		CreatedAt:         j.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         j.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

type jobsHandler struct {
	repo      *db.JobRepository
	fileState *db.FileStateRepository
	engine    *internalsync.Engine
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
		Name:              req.Name,
		ConnectionID:      req.ConnectionID,
		RemotePath:        req.RemotePath,
		LocalDest:         req.LocalDest,
		IntervalValue:     req.IntervalValue,
		IntervalUnit:      req.IntervalUnit,
		Concurrency:       req.Concurrency,
		RetryAttempts:     req.RetryAttempts,
		RetryDelaySeconds: req.RetryDelaySeconds,
		Enabled:           req.Enabled,
		IncludeFilters:    req.IncludeFilters,
		ExcludeFilters:    req.ExcludeFilters,
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
	job.IncludeFilters = req.IncludeFilters
	job.ExcludeFilters = req.ExcludeFilters

	if err := h.repo.Update(job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update job")
		return
	}
	writeJSON(w, http.StatusOK, toJobResponse(job))
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

func (h *jobsHandler) plan(w http.ResponseWriter, r *http.Request) {
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
	w.WriteHeader(http.StatusOK)

	progress := func(files, dirs int) {
		fmt.Fprintf(w, "data: {\"files\":%d,\"folders\":%d}\n\n", files, dirs)
		flusher.Flush()
	}

	result, err := h.engine.PlanJobStream(r.Context(), jobID, progress)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\":%s}\n\n", jsonString(err.Error()))
		flusher.Flush()
		return
	}

	data, _ := json.Marshal(map[string]any{"done": true, "result": result})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
