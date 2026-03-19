package sse

type Event struct {
	RunID        string  `json:"run_id"`
	TransferID   string  `json:"transfer_id"`
	RemotePath   string  `json:"remote_path"`
	SizeBytes    int64   `json:"size_bytes"`
	BytesXferred int64   `json:"bytes_xferred"`
	Percent      float64 `json:"percent"`
	SpeedBPS     float64 `json:"speed_bps"`
	Status       string  `json:"status"` // pending | in_progress | done | skipped | failed
	Error        string  `json:"error,omitempty"`
}
