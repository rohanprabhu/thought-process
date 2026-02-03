package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"thought-process/tools"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "thought-process",
		Version: "0.2.0",
	}, nil)

	tools.RegisterEcho(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
