// ABOUTME: Entry point for GSuite MCP server
// ABOUTME: CLI with help command and mcp subcommand to start the stdio server

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/harper/gsuite-mcp/pkg/server"
)

const version = "1.0.2"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "mcp":
		startMCPServer()
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version", "-v":
		fmt.Printf("gsuite-mcp version %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	help := `GSuite MCP Server - Model Context Protocol server for Google Workspace

USAGE:
    gsuite-mcp <command>

COMMANDS:
    mcp         Start the MCP server with stdio transport
    help        Show this help message
    version     Show version information

EXAMPLES:
    # Start the MCP server
    gsuite-mcp mcp

    # Show help
    gsuite-mcp help

    # Show version
    gsuite-mcp version

CONFIGURATION:
    The server requires OAuth 2.0 credentials from Google Cloud Console.

    Production Mode:
        Place credentials.json in the current directory
        The server will authenticate and save token.json

    Testing Mode (ish):
        Set environment variables:
            ISH_MODE=true
            ISH_BASE_URL=http://localhost:9000
            ISH_USER=testuser@example.com

MCP CLIENT CONFIGURATION:
    Add to your MCP client config (e.g., Claude Desktop):

    {
      "mcpServers": {
        "gsuite": {
          "command": "/path/to/gsuite-mcp",
          "args": ["mcp"]
        }
      }
    }

FEATURES:
    • 19 MCP tools for Gmail, Calendar, and Contacts
    • 8 MCP prompts for common workflows
    • 8 MCP resources for dynamic data access
    • Automatic retry logic with exponential backoff
    • OAuth 2.0 authentication

DOCUMENTATION:
    For more information, visit:
    https://github.com/2389-research/gsuite-mcp
`
	fmt.Println(help)
}

func startMCPServer() {
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
