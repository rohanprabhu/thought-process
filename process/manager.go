package process

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"thought-process/store"
)

const (
	keyPrefix  = "proc:"
	maxLogRead = 100 * 1024 // 100KB
)

// Manager manages subprocesses, persisting metadata in a Store and capturing
// output to log files.
type Manager struct {
	store  store.Store
	logDir string

	mu      sync.Mutex
	running map[string]*exec.Cmd // id -> cmd for live processes

	once sync.Once
}

// NewManager creates a Manager that persists process metadata in store and
// writes log files to logDir.
func NewManager(store store.Store, logDir string) *Manager {
	return &Manager{
		store:   store,
		logDir:  logDir,
		running: make(map[string]*exec.Cmd),
	}
}

// Start launches a subprocess and returns its ProcessView.
func (m *Manager) Start(command string, args []string, cwd string, tags map[string]string, ports []int) (*ProcessView, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating process ID: %w", err)
	}

	logPath := filepath.Join(m.logDir, id+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	shell := userShell()
	shellCmd := command
	if len(args) > 0 {
		for _, a := range args {
			shellCmd += " " + shellQuote(a)
		}
	}

	cmd := exec.Command(shell, "-c", shellCmd)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = cwd
	// Detach the child into its own process group so it isn't killed when the
	// MCP server's stdin is closed.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("starting process: %w", err)
	}

	info := ProcessInfo{
		ID:        id,
		Command:   command,
		Args:      args,
		Cwd:       cwd,
		Tags:      tags,
		Ports:     ports,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now().UTC(),
		LogPath:   logPath,
	}

	if err := m.persist(info); err != nil {
		cmd.Process.Kill()
		logFile.Close()
		return nil, fmt.Errorf("persisting process info: %w", err)
	}

	m.mu.Lock()
	m.running[id] = cmd
	m.mu.Unlock()

	// Wait for the process to exit in the background and record the result.
	go func() {
		defer logFile.Close()
		waitErr := cmd.Wait()

		m.mu.Lock()
		delete(m.running, id)
		m.mu.Unlock()

		now := time.Now().UTC()
		info.ExitedAt = &now
		code := cmd.ProcessState.ExitCode()
		info.ExitCode = &code

		// Best-effort update; ignore store errors.
		_ = m.persist(info)
		_ = waitErr
	}()

	return &ProcessView{
		ProcessInfo: info,
		Status:      StatusRunning,
	}, nil
}

// List returns tracked processes with their current status, filtered by f.
func (m *Manager) List(f ListFilter) ([]ProcessView, error) {
	keys, err := m.store.List(keyPrefix, 0)
	if err != nil {
		return nil, fmt.Errorf("listing process keys: %w", err)
	}

	var cutoff time.Time
	if f.ExitedSinceSecs > 0 {
		cutoff = time.Now().UTC().Add(-time.Duration(f.ExitedSinceSecs) * time.Second)
	}

	views := make([]ProcessView, 0, len(keys))
	for _, key := range keys {
		raw, err := m.store.Get(key)
		if err != nil {
			continue
		}
		var info ProcessInfo
		if err := json.Unmarshal([]byte(raw), &info); err != nil {
			continue
		}
		status := m.status(info)

		// Filter out exited/failed processes older than the cutoff.
		if !cutoff.IsZero() && (status == StatusExited || status == StatusFailed) {
			if info.ExitedAt != nil && info.ExitedAt.Before(cutoff) {
				continue
			}
		}

		views = append(views, ProcessView{
			ProcessInfo: info,
			Status:      status,
		})
	}
	return views, nil
}

// GetLogs returns the last ~100KB of a process's log file.
func (m *Manager) GetLogs(processID string) (string, error) {
	raw, err := m.store.Get(keyPrefix + processID)
	if err != nil {
		return "", fmt.Errorf("process %q not found", processID)
	}
	var info ProcessInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return "", fmt.Errorf("decoding process info: %w", err)
	}

	f, err := os.Open(info.LogPath)
	if err != nil {
		return "", fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat log file: %w", err)
	}

	offset := int64(0)
	if stat.Size() > maxLogRead {
		offset = stat.Size() - maxLogRead
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return "", fmt.Errorf("seeking log file: %w", err)
		}
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("reading log file: %w", err)
	}
	return string(data), nil
}

// Kill sends SIGTERM to a tracked process, waits up to 5 seconds, then
// SIGKILLs it if still alive. Returns the final ProcessView.
func (m *Manager) Kill(processID string) (*ProcessView, error) {
	raw, err := m.store.Get(keyPrefix + processID)
	if err != nil {
		return nil, fmt.Errorf("process %q not found", processID)
	}
	var info ProcessInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, fmt.Errorf("decoding process info: %w", err)
	}

	status := m.status(info)
	if status != StatusRunning {
		return &ProcessView{ProcessInfo: info, Status: status}, nil
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return nil, fmt.Errorf("finding process: %w", err)
	}

	_ = proc.Signal(syscall.SIGTERM)

	// Wait for the background goroutine to record the exit.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			_ = proc.Kill()
			time.Sleep(100 * time.Millisecond)
			// Re-read from store after kill.
			if raw, err = m.store.Get(keyPrefix + processID); err == nil {
				_ = json.Unmarshal([]byte(raw), &info)
			}
			return &ProcessView{ProcessInfo: info, Status: m.status(info)}, nil
		case <-time.After(100 * time.Millisecond):
			// Re-read to check if the wait goroutine recorded the exit.
			if raw, err = m.store.Get(keyPrefix + processID); err == nil {
				_ = json.Unmarshal([]byte(raw), &info)
			}
			if m.status(info) != StatusRunning {
				return &ProcessView{ProcessInfo: info, Status: m.status(info)}, nil
			}
		}
	}
}

// Shutdown sends SIGTERM to all running processes, waits up to 5 seconds, then
// SIGKILLs any remaining. Safe to call multiple times.
func (m *Manager) Shutdown() {
	m.once.Do(func() {
		m.mu.Lock()
		cmds := make(map[string]*exec.Cmd, len(m.running))
		for id, cmd := range m.running {
			cmds[id] = cmd
		}
		m.mu.Unlock()

		for _, cmd := range cmds {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}

		done := make(chan struct{})
		go func() {
			for _, cmd := range cmds {
				_ = cmd.Wait()
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			for _, cmd := range cmds {
				_ = cmd.Process.Kill()
			}
		}
	})
}

// status determines the ProcessStatus for a ProcessInfo.
func (m *Manager) status(info ProcessInfo) ProcessStatus {
	// Already recorded an exit.
	if info.ExitCode != nil {
		if *info.ExitCode == 0 {
			return StatusExited
		}
		return StatusFailed
	}

	// Check in-memory running map first.
	m.mu.Lock()
	_, live := m.running[info.ID]
	m.mu.Unlock()
	if live {
		return StatusRunning
	}

	// Fallback: signal-0 check for orphaned PIDs.
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return StatusUnknown
	}
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		return StatusRunning
	}

	return StatusUnknown
}

func (m *Manager) persist(info ProcessInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return m.store.Set(keyPrefix+info.ID, string(data))
}

func generateID() (string, error) {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// userShell returns the current user's default shell, falling back to /bin/sh.
func userShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// shellQuote wraps s in single quotes for safe shell interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
