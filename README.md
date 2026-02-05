# thought-process

An MCP (Model Context Protocol) server for managing long-running processes. Designed for AI coding agents that need to start, monitor, and control dev servers, file watchers, build tools, databases, and other background processes.

## The Problem

AI coding agents often need to run long-lived processes — starting a dev server, running a database, watching files for changes. But these agents typically work in ephemeral sessions:

- Processes started in one conversation may still be running in the next
- There's no easy way to check what's already running before starting duplicates
- Port conflicts happen when multiple branches or worktrees run simultaneously
- Debugging a misbehaving process requires access to its logs

## The Solution

thought-process provides MCP tools that let agents manage processes across sessions:

- **Persistent tracking** — processes survive conversation boundaries
- **Tagging and metadata** — organize by branch, worktree, role, or any custom dimension
- **Port awareness** — track which ports processes use to avoid conflicts
- **Log access** — retrieve stdout/stderr for debugging
- **Graceful shutdown** — SIGTERM with SIGKILL fallback

## Tools

| Tool | Description |
|------|-------------|
| `start_process` | Start a long-running process with optional tags, ports, env vars, and working directory. Returns a process ID for later reference. |
| `list_processes` | List all tracked processes with their status, tags, and ports. Use before starting new processes to avoid duplicates. |
| `get_process_logs` | Get the last ~100KB of stdout/stderr for a process. Primary debugging tool. |
| `kill_process` | Stop a process (SIGTERM, then SIGKILL after 5s). Use when switching branches or cleaning up. |
| `get_free_port` | Get an available TCP port for dynamic port assignment. |
| `echo` | Simple echo tool for testing connectivity. |

## Installation

Build from source (requires Go 1.21+):

```bash
git clone https://github.com/rohanprabhu/thought-process.git
cd thought-process
make build
```

## Configuration

Add to your MCP client configuration. For Claude Code, add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "thought-process": {
      "command": "/path/to/thought-process"
    }
  }
}
```

## Data Storage

thought-process stores data in `~/.thought-process/`:

- `data/` — process metadata (one file per tracked process)
- `logs/` — stdout/stderr logs for each process

## Usage Examples

### Starting a dev server

```
start_process(
  command: "npm",
  args: ["run", "dev"],
  cwd: "/path/to/project",
  env: {"PORT": "3001"},
  tags: {"branch": "feature-x", "role": "frontend"},
  ports: [3001]
)
```

### Checking what's running

```
list_processes()
```

Returns all tracked processes with status (running/exited/failed), tags, and ports.

### Debugging a failing process

```
get_process_logs(process_id: "abc123")
```

### Cleaning up before switching branches

```
kill_process(process_id: "abc123")
```

### Getting a dynamic port

```
port = get_free_port()
start_process(command: "node", args: ["server.js"], env: {"PORT": port}, ports: [port])
```

## Development

```bash
make build    # Compile binary
make run      # Build and run
make dev      # Hot-reload with air
make clean    # Remove artifacts
```

## License

MIT
