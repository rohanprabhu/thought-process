package process

import "time"

// ProcessStatus represents the current state of a managed process.
type ProcessStatus string

const (
	StatusRunning ProcessStatus = "running"
	StatusExited  ProcessStatus = "exited"
	StatusFailed  ProcessStatus = "failed"
	StatusUnknown ProcessStatus = "unknown"
)

// ProcessInfo holds the persisted metadata for a managed process.
type ProcessInfo struct {
	ID        string            `json:"id"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	Ports     []int             `json:"ports,omitempty"`
	PID       int               `json:"pid"`
	StartedAt time.Time         `json:"started_at"`
	ExitCode  *int              `json:"exit_code,omitempty"`
	ExitedAt  *time.Time        `json:"exited_at,omitempty"`
	LogPath   string            `json:"log_path"`
}

// ProcessView extends ProcessInfo with a computed Status field.
type ProcessView struct {
	ProcessInfo
	Status ProcessStatus `json:"status"`
}

// ListFilter controls which processes are returned by List.
type ListFilter struct {
	// ExitedSinceSecs limits exited/failed processes to those that exited
	// within this many seconds ago. Running and unknown processes are always
	// included. A value of 0 means no filtering.
	ExitedSinceSecs int
}
