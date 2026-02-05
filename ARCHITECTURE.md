# Architecture

This document describes the internal architecture of thought-process, its components, and the libraries it uses.

## Overview

thought-process is an MCP (Model Context Protocol) server that manages long-running processes. It runs as a subprocess of an MCP client (like Claude Code), communicating via JSON-RPC over stdin/stdout.

```
┌─────────────────┐       stdio        ┌─────────────────────────────────────┐
│   MCP Client    │◄──────────────────►│         thought-process             │
│  (Claude Code)  │    JSON-RPC        │                                     │
└─────────────────┘                    │  ┌─────────┐  ┌─────────────────┐   │
                                       │  │  Tools  │  │ Process Manager │   │
                                       │  └────┬────┘  └────────┬────────┘   │
                                       │       │                │            │
                                       │       │         ┌──────┴──────┐     │
                                       │       │         │  Subprocess │     │
                                       │       ▼         │   (dev srv) │     │
                                       │  ┌─────────┐    └─────────────┘     │
                                       │  │  Store  │                        │
                                       │  └────┬────┘                        │
                                       │       │                             │
                                       └───────┼─────────────────────────────┘
                                               │
                                               ▼
                                       ~/.thought-process/
                                       ├── data/   (process metadata)
                                       └── logs/   (stdout/stderr)
```

## Directory Structure

```
.
├── main.go              # Entry point, wires components together
├── tools/
│   ├── echo.go          # Echo tool (connectivity test)
│   └── process.go       # Process management tools
├── process/
│   ├── types.go         # ProcessInfo, ProcessView, status types
│   └── manager.go       # Process lifecycle management
└── store/
    ├── store.go         # Store interface
    └── dir.go           # File-based store implementation
```

## Components

### MCP Server (`main.go`)

The entry point creates and wires together all components:

1. Creates the data and log directories under `~/.thought-process/`
2. Initializes the `DirStore` for persistent metadata
3. Initializes the `Manager` for process lifecycle
4. Registers all MCP tools with the server
5. Runs the server on stdio transport
6. Handles graceful shutdown on SIGINT/SIGTERM

### Tools (`tools/`)

MCP tools are the interface exposed to AI agents. Each tool is registered with `mcp.AddTool()` using typed argument structs — the SDK automatically generates JSON schemas from struct tags.

| File | Tools | Purpose |
|------|-------|---------|
| `echo.go` | `echo` | Simple connectivity test |
| `process.go` | `start_process`, `list_processes`, `get_process_logs`, `kill_process`, `get_free_port` | Process management |

Tool handlers validate arguments, delegate to the process manager, and return JSON-serialized responses.

### Process Manager (`process/`)

The `Manager` handles the full lifecycle of tracked processes:

- **Starting** — Spawns subprocesses via the user's shell, detaches them into their own process group (so they survive server restarts), and captures stdout/stderr to log files
- **Tracking** — Persists process metadata (command, args, tags, ports, PID, timestamps) to the store
- **Monitoring** — Background goroutines wait for process exit and record exit codes
- **Querying** — Lists processes with current status (running/exited/failed), filtering out old exited processes
- **Killing** — Sends SIGTERM, waits up to 5 seconds, then SIGKILL if still alive
- **Shutdown** — Gracefully terminates all tracked processes when the server exits

Key design decisions:

- **Shell execution** — Commands run through the user's shell (`$SHELL` or `/bin/sh`) for familiar environment and PATH handling
- **Process groups** — `Setpgid: true` detaches children so they aren't killed when the MCP server's stdin closes
- **Non-blocking** — Process wait happens in goroutines; the manager never blocks on subprocess exit

### Store (`store/`)

The `Store` interface defines a simple key-value API:

```go
type Store interface {
    Get(key string) (string, error)
    Set(key, value string) error
    Delete(key string) error
    List(prefix string, limit int) ([]string, error)
    Close() error
}
```

The `DirStore` implementation uses the filesystem:

- **One file per key** — Keys map to filenames with path separator escaping
- **Atomic writes** — Write to temp file, then rename (no partial reads)
- **No locks** — Relies on filesystem atomicity; safe for concurrent access
- **Human-readable** — Data files are plain JSON, easy to inspect/debug

This approach was chosen over embedded databases (like Pebble, Bolt, or SQLite) for simplicity and debuggability. Process metadata is small and infrequently updated, so filesystem overhead is negligible.

## Libraries

### github.com/modelcontextprotocol/go-sdk

The [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) provides:

- `mcp.Server` — MCP server implementation
- `mcp.StdioTransport` — JSON-RPC over stdin/stdout
- `mcp.AddTool()` — Tool registration with automatic JSON schema generation from Go structs
- `mcp.CallToolRequest`, `mcp.CallToolResult` — Request/response types

The SDK handles all MCP protocol details: capability negotiation, request routing, JSON-RPC framing, and error handling.

### Standard Library

The rest of thought-process uses only Go's standard library:

| Package | Usage |
|---------|-------|
| `os/exec` | Spawning and managing subprocesses |
| `syscall` | Process groups (`Setpgid`), signals (`SIGTERM`, `SIGKILL`) |
| `encoding/json` | Serializing process metadata |
| `crypto/rand` | Generating process IDs |
| `net` | Finding free ports |
| `path/filepath` | File path handling |
| `sync` | Mutex for concurrent access to running process map |

## Data Flow

### Starting a Process

```
Agent calls start_process(command, args, env, tags, ports)
    │
    ▼
tools/process.go validates args, calls manager.Start()
    │
    ▼
manager.go:
    1. Generate unique ID
    2. Create log file in ~/.thought-process/logs/
    3. Build shell command with args
    4. Set environment (inherit + custom env vars)
    5. Spawn subprocess (detached process group)
    6. Persist ProcessInfo to store
    7. Start background goroutine to wait for exit
    │
    ▼
Return ProcessView (ID, status, metadata) to agent
```

### Listing Processes

```
Agent calls list_processes()
    │
    ▼
tools/process.go calls manager.List()
    │
    ▼
manager.go:
    1. List all proc:* keys from store
    2. For each: unmarshal ProcessInfo
    3. Determine current status:
       - Check in-memory running map
       - Fallback: signal-0 check on PID
       - Check recorded exit code
    4. Filter out old exited processes
    │
    ▼
Return []ProcessView to agent
```

## Future Considerations

- **Store backends** — The `Store` interface allows swapping in other backends (Pebble, SQLite, Redis) if needed for performance or features
- **Remote processes** — The architecture could extend to managing processes on remote hosts via SSH
- **Resource limits** — Could add cgroup/ulimit support for memory/CPU constraints
- **Health checks** — HTTP/TCP health probes for services that expose them
