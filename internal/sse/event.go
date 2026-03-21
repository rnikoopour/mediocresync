package sse

type Event struct {
	RunID        string  `json:"run_id"`
	TransferID   string  `json:"transfer_id"`
	RemotePath   string  `json:"remote_path"`
	SizeBytes    int64   `json:"size_bytes"`
	BytesXferred int64   `json:"bytes_xferred"`
	Percent      float64 `json:"percent"`
	SpeedBPS     float64 `json:"speed_bps"`
	Status       string  `json:"status"`               // pending | in_progress | done | skipped | failed | plan_file_updated | planning | started | run_finished | runs_pruned
	Error        string  `json:"error,omitempty"`
	RunStatus    string  `json:"run_status,omitempty"` // final run status, set on last event only
	PlanPath     string  `json:"plan_path,omitempty"`  // set when Status == "plan_file_updated"
	PlanAction   string  `json:"plan_action,omitempty"` // "copy" | "skip"
}
