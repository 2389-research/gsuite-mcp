// ABOUTME: Entry point for GSuite MCP server
// ABOUTME: Initializes and starts the MCP server with stdio transport

package main

import (
	"context"
	"log"

	"github.com/harper/gsuite-mcp/pkg/server"
)

func main() {
	ctx := context.Background()

	srv, err := server.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Println("GSuite MCP Server starting...")

	if err := srv.Serve(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
