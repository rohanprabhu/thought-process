package dashboard

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"thought-process/process"
)

//go:embed static/*
var staticFS embed.FS

// Server serves the web dashboard for viewing and managing processes.
type Server struct {
	mgr    process.ProcessManager
	server *http.Server
}

// NewServer creates a new dashboard server bound to the given address.
func NewServer(addr string, mgr process.ProcessManager) *Server {
	s := &Server{mgr: mgr}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/processes", s.handleListProcesses)
	mux.HandleFunc("GET /api/processes/{id}/logs", s.handleGetLogs)
	mux.HandleFunc("GET /api/processes/{id}/logs/stream", s.handleStreamLogs)
	mux.HandleFunc("POST /api/processes/{id}/kill", s.handleKillProcess)

	// Static files
	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start begins serving HTTP requests. This blocks until the server is shut down.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
