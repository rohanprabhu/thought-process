package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"thought-process/process"
)

type StartProcessArgs struct {
	Command string            `json:"command" jsonschema:"the command to run"`
	Args    []string          `json:"args,omitempty" jsonschema:"arguments for the command"`
	Cwd     string            `json:"cwd,omitempty" jsonschema:"working directory for the command"`
	Tags    map[string]string `json:"tags,omitempty" jsonschema:"key-value tags for the process"`
	Ports   []int             `json:"ports,omitempty" jsonschema:"ports the process listens on"`
}

type ListProcessesArgs struct {
	ExitedSinceSecs *int `json:"exited_since_duration,omitempty" jsonschema:"only include exited processes that exited within this many seconds ago, defaults to 10"`
}

type GetProcessLogsArgs struct {
	ProcessID string `json:"process_id" jsonschema:"the ID of the process to get logs for"`
}

type KillProcessArgs struct {
	ProcessID string `json:"process_id" jsonschema:"the ID of the process to kill"`
}

// RegisterProcessTools registers start_process, list_processes, and
// get_process_logs on the given MCP server.
func RegisterProcessTools(server *mcp.Server, mgr *process.Manager) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_process",
		Description: "Start a subprocess and track it. Returns process metadata including an ID for later reference.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args StartProcessArgs) (*mcp.CallToolResult, any, error) {
		if args.Command == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "command is required"},
				},
			}, nil, nil
		}

		view, err := mgr.Start(args.Command, args.Args, args.Cwd, args.Tags, args.Ports)
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
		Name:        "list_processes",
		Description: "List all tracked processes with their current status.",
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
		Name:        "get_process_logs",
		Description: "Get the last ~100KB of stdout/stderr logs for a tracked process.",
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
		Name:        "kill_process",
		Description: "Kill a tracked process. Sends SIGTERM, then SIGKILL after 5 seconds if still alive.",
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
