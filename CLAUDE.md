# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

An MCP (Model Context Protocol) server written in Go using the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk). Communicates over stdio transport (stdin/stdout JSON-RPC), not HTTP. MCP clients launch it as a subprocess.

## Build & Development Commands

```bash
make build          # Compile binary (./thought-process)
make run            # Build and run
make dev            # Hot-reload development via air (auto-installs air if missing)
make clean          # Remove binary and tmp/
```

## Architecture

`main.go` wires together the store, process manager, and MCP tools, then runs the server on `mcp.StdioTransport`. Tools are added with `mcp.AddTool` using typed argument structs — the SDK infers JSON schemas from struct tags automatically.

The `jsonschema` struct tag provides property descriptions but must not start with `WORD=` (e.g., use `jsonschema:"the message"` not `jsonschema:"description=the message"`).

```
main.go
  ├── store.NewDirStore(~/.thought-process/data/)
  ├── process.NewManager(store, ~/.thought-process/logs/)
  ├── tools.RegisterEcho(server)
  └── tools.RegisterProcessTools(server, manager)
```

**Data directory:** `~/.thought-process/` contains `data/` (one file per key, no long-running locks) and `logs/` (process stdout/stderr).

### MCP Tools

| Tool | Args | Description |
|------|------|-------------|
| `echo` | `message` (string) | Echoes back a greeting with the provided message |
| `start_process` | `command` (string, required), `args` ([]string), `cwd` (string), `tags` (map), `ports` ([]int) | Start a subprocess and track it |
| `list_processes` | `exited_since_duration` (int, default 10) | List tracked processes; exited ones filtered to last N seconds |
| `get_process_logs` | `process_id` (string, required) | Get the last ~100KB of a process's logs |
| `kill_process` | `process_id` (string, required) | Kill a process (SIGTERM, then SIGKILL after 5s) |

## Maintaining Documentation

Keep project documentation up to date as the codebase evolves:

- **Update on every change**: When adding, modifying, or removing functionality, update the relevant documentation in the same pass. Never leave docs stale.
- **Split by directory**: Each package or major directory should have its own `CLAUDE.md` covering the specifics of that area (e.g., `tools/CLAUDE.md`, `internal/auth/CLAUDE.md`). Keep these focused on what's needed to work in that directory.
- **Link from root**: This root `CLAUDE.md` is the entry point. Reference subdirectory docs here so they're discoverable. Use relative links:

### Subdirectory Documentation

<!-- Add links here as new directories get their own CLAUDE.md -->
<!-- Example: - [tools/CLAUDE.md](tools/CLAUDE.md) — MCP tool implementations -->
