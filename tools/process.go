package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"thought-process/process"
)

type StartProcessArgs struct {
	Command string            `json:"command" jsonschema:"the command to run (e.g. npm, python, go, docker-compose). Do NOT use this for short-lived commands like grep, ls, cat, etc. — use your built-in shell tools for those instead"`
	Args    []string          `json:"args,omitempty" jsonschema:"arguments for the command (e.g. [\"run\", \"dev\", \"--port\", \"3001\"])"`
	Cwd     string            `json:"cwd,omitempty" jsonschema:"working directory for the command. Set this to the worktree or repo root so the process runs in the correct context"`
	Env     map[string]string `json:"env,omitempty" jsonschema:"environment variables to set for the process (e.g. {\"NODE_ENV\": \"development\", \"PORT\": \"3001\"}). These are added to the current environment, not replacing it"`
	Tags    map[string]string `json:"tags,omitempty" jsonschema:"key-value metadata tags for organizing and filtering processes. Always tag with context you have: 'branch' (git branch name), 'worktree' (worktree path), 'role' (e.g. 'frontend', 'backend', 'db'), 'stack' (e.g. 'next', 'rails'). Tags let you find and manage related processes later"`
	Ports   []int             `json:"ports,omitempty" jsonschema:"ports this process listens on. Always specify known ports so you can detect conflicts and avoid port collisions across branches/worktrees"`
}

type ListProcessesArgs struct {
	ExitedSinceSecs *int `json:"exited_since_duration,omitempty" jsonschema:"only include exited processes that exited within this many seconds ago (default 10). Increase this to see processes that crashed or exited further in the past"`
}

type GetProcessLogsArgs struct {
	ProcessID string `json:"process_id" jsonschema:"the ID of the process to get logs for (from start_process or list_processes)"`
}

type KillProcessArgs struct {
	ProcessID string `json:"process_id" jsonschema:"the ID of the process to kill (from start_process or list_processes)"`
}

// RegisterProcessTools registers start_process, list_processes, and
// get_process_logs on the given MCP server.
func RegisterProcessTools(server *mcp.Server, mgr *process.Manager) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "start_process",
		Description: `Start and track a long-running process (dev servers, watchers, builds, databases, etc.). Returns a process ID for checking logs and stopping it later.

USE THIS FOR: processes that run continuously or for a long time — dev servers (npm run dev, python manage.py runserver), file watchers, docker-compose, database servers, queue workers, build processes, test suites.
DO NOT USE FOR: short-lived commands like grep, ls, cat, git status, curl — use your built-in shell/bash tools for those.

IMPORTANT — always tag your processes for isolation:
- Set 'branch' tag to the current git branch name
- Set 'worktree' tag to the worktree path when working in git worktrees
- Set 'role' tag to describe the process role (e.g. 'frontend', 'backend', 'api', 'db', 'worker')
- Specify 'ports' so you can detect conflicts across branches/worktrees
- Use 'cwd' to pin the process to the correct directory

Before starting a process, call list_processes first to check if an equivalent process is already running — avoid spawning duplicates. When working across multiple branches or worktrees, use different ports per branch to prevent conflicts.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args StartProcessArgs) (*mcp.CallToolResult, any, error) {
		if args.Command == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "command is required"},
				},
			}, nil, nil
		}

		view, err := mgr.Start(args.Command, args.Args, args.Cwd, args.Env, args.Tags, args.Ports)
		if err != nil {
			return nil, nil, fmt.Errorf("starting process: %w", err)
		}

		data, err := json.Marshal(view)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_processes",
		Description: `List all tracked long-running processes with their current status, tags, and ports.

Call this BEFORE starting a new process to avoid duplicates and port conflicts. Use it to:
- Check if a dev server or service is already running for your current branch/worktree
- Find processes by their tags (branch, worktree, role) to manage isolation
- Detect port conflicts before starting a new service
- Find the process ID you need for get_process_logs or kill_process
- Check if a previously started process has crashed (look for exited processes)

Running processes persist across conversations — always check what's already running.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListProcessesArgs) (*mcp.CallToolResult, any, error) {
		secs := 10
		if args.ExitedSinceSecs != nil {
			secs = *args.ExitedSinceSecs
		}
		views, err := mgr.List(process.ListFilter{ExitedSinceSecs: secs})
		if err != nil {
			return nil, nil, fmt.Errorf("listing processes: %w", err)
		}

		data, err := json.Marshal(views)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_process_logs",
		Description: `Get the last ~100KB of combined stdout/stderr logs for a tracked process.

Use this to debug issues with long-running processes: check for startup errors, runtime exceptions, request failures, build errors, or test output. This is your primary debugging tool for any process started with start_process — always check logs when something isn't working as expected (e.g. a dev server won't respond, a build seems stuck, tests are failing).`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetProcessLogsArgs) (*mcp.CallToolResult, any, error) {
		if args.ProcessID == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "process_id is required"},
				},
			}, nil, nil
		}

		logs, err := mgr.GetLogs(args.ProcessID)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: err.Error()},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: logs},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "kill_process",
		Description: `Kill a tracked process (SIGTERM, then SIGKILL after 5s if still alive).

Use this to stop processes you no longer need — e.g. when switching branches, tearing down a dev environment, freeing a port for reuse, or cleaning up before starting a fresh instance. Always kill old processes for a branch/worktree before starting replacements to avoid port conflicts and resource waste.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, args KillProcessArgs) (*mcp.CallToolResult, any, error) {
		if args.ProcessID == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "process_id is required"},
				},
			}, nil, nil
		}

		view, err := mgr.Kill(args.ProcessID)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: err.Error()},
				},
			}, nil, nil
		}

		data, err := json.Marshal(view)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(data)},
			},
		}, nil, nil
	})
}
