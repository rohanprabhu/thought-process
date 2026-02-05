package process

// ProcessManager defines the interface for managing long-running processes.
// This abstraction allows the MCP tools and HTTP dashboard to share the same
// process management logic.
type ProcessManager interface {
	// Start launches a subprocess and returns its ProcessView.
	Start(command string, args []string, cwd string, env map[string]string, tags map[string]string, ports []int) (*ProcessView, error)

	// List returns tracked processes with their current status, filtered by f.
	List(f ListFilter) ([]ProcessView, error)

	// GetLogs returns the last ~100KB of a process's log file.
	GetLogs(processID string) (string, error)

	// GetLogPath returns the path to a process's log file for streaming.
	GetLogPath(processID string) (string, error)

	// Kill sends SIGTERM to a tracked process, waits up to 5 seconds, then
	// SIGKILLs it if still alive. Returns the final ProcessView.
	Kill(processID string) (*ProcessView, error)

	// Shutdown sends SIGTERM to all running processes, waits up to 5 seconds,
	// then SIGKILLs any remaining. Safe to call multiple times.
	Shutdown()
}
