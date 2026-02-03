package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EchoArgs struct {
	Message string `json:"message" jsonschema:"the message to echo back"`
}

func RegisterEcho(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echoes back the provided message",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EchoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Hello, %s from thought-process!!", args.Message)},
			},
		}, nil, nil
	})
}
