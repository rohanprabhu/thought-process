package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"thought-process/dashboard"
	"thought-process/process"
	"thought-process/store"
	"thought-process/tools"
)

func main() {
	dashboardAddr := flag.String("dashboard", "", "address to serve dashboard on (e.g. :8080)")
	flag.Parse()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("getting home directory: %v", err)
	}

	baseDir := filepath.Join(homeDir, ".thought-process")
	dataDir := filepath.Join(baseDir, "data")
	logDir := filepath.Join(baseDir, "logs")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("creating data directory: %v", err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Fatalf("creating logs directory: %v", err)
	}

	dirStore := store.NewDirStore(dataDir)

	mgr := process.NewManager(dirStore, logDir)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "thought-process",
		Version: "0.3.0",
	}, nil)

	tools.RegisterEcho(server)
	tools.RegisterProcessTools(server, mgr)

	// Graceful shutdown on signal or when server.Run returns (stdin closed).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start dashboard HTTP server if requested.
	var dashServer *dashboard.Server
	if *dashboardAddr != "" {
		dashServer = dashboard.NewServer(*dashboardAddr, mgr)
		go func() {
			log.Printf("Dashboard available at http://%s", *dashboardAddr)
			if err := dashServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("dashboard server error: %v", err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if dashServer != nil {
			dashServer.Shutdown(context.Background())
		}
		mgr.Shutdown()
		cancel()
	}()

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		// Context cancellation from signal is expected.
		if ctx.Err() == nil {
			log.Fatalf("server error: %v", err)
		}
	}

	if dashServer != nil {
		dashServer.Shutdown(context.Background())
	}
	mgr.Shutdown()
}
