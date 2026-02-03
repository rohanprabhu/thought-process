package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "echo-server",
		Version: "0.1.0",
	}, nil)

	type EchoArgs struct {
		Message string `json:"message" jsonschema:"the message to echo back"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echoes back the provided message",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EchoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: args.Message},
			},
		}, nil, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
