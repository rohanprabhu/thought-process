package dashboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"thought-process/process"
)

func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	filter := process.ListFilter{
		ExitedSinceSecs: 10,
	}

	// Parse exited_since_secs query param
	if secs := r.URL.Query().Get("exited_since_secs"); secs != "" {
		if n, err := strconv.Atoi(secs); err == nil {
			filter.ExitedSinceSecs = n
		}
	}

	// Parse tag.* query params
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "tag.") && len(values) > 0 {
			tagName := strings.TrimPrefix(key, "tag.")
			if filter.Tags == nil {
				filter.Tags = make(map[string]string)
			}
			filter.Tags[tagName] = values[0]
		}
	}

	processes, err := s.mgr.List(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(processes)
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "process ID required", http.StatusBadRequest)
		return
	}

	logs, err := s.mgr.GetLogs(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(logs))
}

func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "process ID required", http.StatusBadRequest)
		return
	}

	logPath, err := s.mgr.GetLogPath(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Open the log file
	f, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer f.Close()

	// Get initial file size and send existing content
	stat, err := f.Stat()
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Read and send initial content (last 100KB)
	const maxInitialRead = 100 * 1024
	offset := int64(0)
	if stat.Size() > maxInitialRead {
		offset = stat.Size() - maxInitialRead
	}
	if offset > 0 {
		f.Seek(offset, io.SeekStart)
	}

	reader := bufio.NewReader(f)
	initialData, _ := io.ReadAll(reader)
	if len(initialData) > 0 {
		sendSSEData(w, flusher, string(initialData))
	}

	// Track position for tailing
	currentPos, _ := f.Seek(0, io.SeekCurrent)

	// Tail the file for new content
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for new content
			stat, err := f.Stat()
			if err != nil {
				continue
			}

			if stat.Size() > currentPos {
				// Seek to where we left off
				f.Seek(currentPos, io.SeekStart)
				newData := make([]byte, stat.Size()-currentPos)
				n, err := f.Read(newData)
				if err != nil && err != io.EOF {
					continue
				}
				if n > 0 {
					sendSSEData(w, flusher, string(newData[:n]))
					currentPos += int64(n)
				}
			}
		}
	}
}

func sendSSEData(w http.ResponseWriter, flusher http.Flusher, data string) {
	// SSE format: multi-line data uses "data:" prefix for each line
	// We send all lines as a single event to avoid overwhelming the client
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		if i < len(lines)-1 || line != "" {
			fmt.Fprintf(w, "data: %s\n", line)
		}
	}
	fmt.Fprintf(w, "\n") // Empty line marks end of event
	flusher.Flush()
}

func (s *Server) handleKillProcess(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "process ID required", http.StatusBadRequest)
		return
	}

	view, err := s.mgr.Kill(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}
