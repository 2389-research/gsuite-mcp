// ABOUTME: Entry point for GSuite MCP server
// ABOUTME: CLI with help command and mcp subcommand to start the stdio server

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/harper/gsuite-mcp/pkg/accounts"
	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/server"
)

const version = "1.4.2"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]
	accountAlias := parseAccountFlag(os.Args[2:])

	switch command {
	case "mcp":
		startMCPServer()
	case "setup":
		runSetup(accountAlias)
	case "test":
		runTest(accountAlias)
	case "whoami":
		runWhoami(accountAlias)
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

// parseAccountFlag extracts the --account value from CLI arguments.
// Returns empty string if no --account flag is found.
func parseAccountFlag(args []string) string {
	for i, arg := range args {
		if arg == "--account" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "--account=") {
			return strings.TrimPrefix(arg, "--account=")
		}
	}
	return ""
}

// resolveTokenPath determines the token path for the given account alias.
// If alias is empty, returns the default token path.
// If the alias doesn't exist in the registry, creates it with a generated token path.
func resolveTokenPath(alias string, autoCreate bool) string {
	if alias == "" {
		return auth.GetTokenPath()
	}

	// Validate alias before using it in file paths
	if err := accounts.ValidateAlias(alias); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	configPath := auth.GetAccountsConfigPath()
	registry, err := accounts.LoadRegistry(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load accounts config: %v\n", err)
		return auth.GetTokenPath()
	}

	acct, err := registry.Resolve(alias)
	if err != nil {
		if !autoCreate {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// Auto-create account with a generated token path
		tokenPath := auth.GenerateAccountTokenPath(alias)
		if addErr := registry.AddAccount(alias, tokenPath); addErr != nil {
			fmt.Fprintf(os.Stderr, "Error creating account '%s': %v\n", alias, addErr)
			os.Exit(1)
		}
		fmt.Printf("[OK] Created account '%s' (token: %s)\n", alias, tokenPath)
		return tokenPath
	}

	tokenPath := acct.TokenPath
	if tokenPath == "" {
		return auth.GetTokenPath()
	}
	return tokenPath
}

func printHelp() {
	help := `GSuite MCP Server - Model Context Protocol server for Google Workspace

USAGE:
    gsuite-mcp <command> [flags]

COMMANDS:
    setup       Interactive setup wizard (start here!)
    test        Test connection to Google APIs
    whoami      Show authenticated user info
    mcp         Start the MCP server with stdio transport
    help        Show this help message
    version     Show version information

FLAGS:
    --account <alias>  Specify which account to use (e.g., 'work', 'personal')
                       Supported by: setup, test, whoami

EXAMPLES:
    # Start the MCP server
    gsuite-mcp mcp

    # Set up the default account
    gsuite-mcp setup

    # Set up a named account
    gsuite-mcp setup --account work

    # Test a specific account
    gsuite-mcp test --account personal

    # Show who you're logged in as
    gsuite-mcp whoami --account work

    # Show help
    gsuite-mcp help

    # Show version
    gsuite-mcp version

CONFIGURATION:
    The server requires OAuth 2.0 credentials from Google Cloud Console.

    Credentials Path (checked in order):
        1. GSUITE_MCP_CREDENTIALS_PATH env var
        2. $XDG_CONFIG_HOME/gsuite-mcp/credentials.json
        3. ~/.config/gsuite-mcp/credentials.json

    Token Path (checked in order):
        1. GSUITE_MCP_TOKEN_PATH env var
        2. $XDG_DATA_HOME/gsuite-mcp/token.json
        3. ~/.local/share/gsuite-mcp/token.json

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
    • 34 MCP tools for Gmail, Calendar, Contacts, and Tasks
    • 10 MCP prompts for common workflows
    • 11 MCP resources for dynamic data access
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

func runSetup(accountAlias string) {
	fmt.Println("=== GSuite MCP Server Setup ===")
	if accountAlias != "" {
		fmt.Printf("    Account: %s\n", accountAlias)
	}
	fmt.Println()

	credPath := auth.GetCredentialsPath()
	// resolveTokenPath auto-creates the account in the registry if it doesn't exist
	tokenPath := resolveTokenPath(accountAlias, true)

	// Step 1: Show where files will be stored
	fmt.Println("STEP 1: Configuration Paths")
	fmt.Println("----------------------------")
	fmt.Printf("Credentials file: %s\n", credPath)
	fmt.Printf("Token file:       %s\n", tokenPath)
	fmt.Println()

	// Check if credentials exist
	credExists := fileExists(credPath)
	tokenExists := fileExists(tokenPath)

	if credExists {
		fmt.Println("[OK] Credentials file found")
	} else {
		fmt.Println("[!!] Credentials file NOT found")
	}

	if tokenExists {
		fmt.Println("[OK] Token file found (already authenticated)")
	} else {
		fmt.Println("[..] Token file not found (will be created on first auth)")
	}
	fmt.Println()

	// Step 2: Instructions for getting credentials
	if !credExists {
		fmt.Println("STEP 2: Get Google OAuth Credentials")
		fmt.Println("-------------------------------------")
		fmt.Println("You need to create OAuth 2.0 credentials in Google Cloud Console:")
		fmt.Println()
		fmt.Println("  1. Go to: https://console.cloud.google.com/apis/credentials")
		fmt.Println()
		fmt.Println("  2. Create a new project (or select existing)")
		fmt.Println()
		fmt.Println("  3. Click '+ CREATE CREDENTIALS' -> 'OAuth client ID'")
		fmt.Println()
		fmt.Println("  4. If prompted, configure the OAuth consent screen:")
		fmt.Println("     - User Type: External (or Internal for Workspace)")
		fmt.Println("     - App name: GSuite MCP (or any name)")
		fmt.Println("     - Add your email as a test user")
		fmt.Println()
		fmt.Println("  5. For Application type, select 'Desktop app'")
		fmt.Println()
		fmt.Println("  6. Click 'Download JSON' and save as:")
		fmt.Printf("     %s\n", credPath)
		fmt.Println()
		fmt.Println("  7. Enable the APIs you need:")
		fmt.Println("     - Gmail API:    https://console.cloud.google.com/apis/library/gmail.googleapis.com")
		fmt.Println("     - Calendar API: https://console.cloud.google.com/apis/library/calendar-json.googleapis.com")
		fmt.Println("     - People API:   https://console.cloud.google.com/apis/library/people.googleapis.com")
		fmt.Println()

		// Offer to create the directory
		fmt.Print("Would you like to create the credentials directory now? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			if err := auth.EnsureDir(credPath); err != nil {
				fmt.Printf("Error creating directory: %v\n", err)
			} else {
				fmt.Println("[OK] Directory created!")
			}
		}
		fmt.Println()
	}

	// Step 3: Authentication
	if credExists && !tokenExists {
		fmt.Println("STEP 3: Authenticate")
		fmt.Println("--------------------")
		fmt.Println("Credentials found! Ready to authenticate with Google.")
		fmt.Println()
		fmt.Print("Would you like to authenticate now? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			fmt.Println()
			fmt.Println("Starting OAuth flow...")
			fmt.Println()

			ctx := context.Background()
			authenticator, err := auth.NewAuthenticator(credPath, tokenPath)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			_, err = authenticator.GetClient(ctx)
			if err != nil {
				fmt.Printf("Authentication failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Println()
			fmt.Println("[OK] Authentication successful! Token saved.")
		}
		fmt.Println()
	}

	// Final status
	fmt.Println("=== Next Steps ===")
	fmt.Println()

	// Re-check after potential changes
	credExists = fileExists(credPath)
	tokenExists = fileExists(tokenPath)

	if credExists && tokenExists {
		fmt.Println("[OK] All set! You can now use the MCP server.")
		fmt.Println()
		fmt.Println("To start the server, run:")
		fmt.Println("  gsuite-mcp mcp")
		fmt.Println()
		fmt.Println("Or add to your MCP client config:")
		fmt.Println(`  {`)
		fmt.Println(`    "mcpServers": {`)
		fmt.Println(`      "gsuite": {`)
		fmt.Printf(`        "command": "%s",`+"\n", os.Args[0])
		fmt.Println(`        "args": ["mcp"]`)
		fmt.Println(`      }`)
		fmt.Println(`    }`)
		fmt.Println(`  }`)
	} else if credExists && !tokenExists {
		accountFlag := ""
		if accountAlias != "" {
			accountFlag = " --account " + accountAlias
		}
		fmt.Printf("1. Run 'gsuite-mcp setup%s' again to authenticate with Google\n", accountFlag)
	} else {
		accountFlag := ""
		if accountAlias != "" {
			accountFlag = " --account " + accountAlias
		}
		fmt.Println("1. Download credentials.json from Google Cloud Console")
		fmt.Printf("2. Save it to: %s\n", credPath)
		fmt.Printf("3. Run 'gsuite-mcp setup%s' again to authenticate\n", accountFlag)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runTest(accountAlias string) {
	fmt.Println("Testing Google API connection...")
	if accountAlias != "" {
		fmt.Printf("Account: %s\n", accountAlias)
	}
	fmt.Println()

	credPath := auth.GetCredentialsPath()
	tokenPath := resolveTokenPath(accountAlias, false)

	// Check if credentials exist
	if !fileExists(credPath) {
		fmt.Printf("[FAIL] Credentials file not found: %s\n", credPath)
		fmt.Println("       Run 'gsuite-mcp setup' first.")
		os.Exit(1)
	}

	// Try to authenticate
	ctx := context.Background()
	authenticator, err := auth.NewAuthenticator(credPath, tokenPath)
	if err != nil {
		fmt.Printf("[FAIL] Could not load credentials: %v\n", err)
		os.Exit(1)
	}

	client, err := authenticator.GetClient(ctx)
	if err != nil {
		fmt.Printf("[FAIL] Authentication failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK] Authentication successful")

	// Test Gmail API
	svc, err := gmail.NewService(ctx, client)
	if err != nil {
		fmt.Printf("[FAIL] Could not create Gmail service: %v\n", err)
		os.Exit(1)
	}

	profile, err := svc.GetProfile(ctx)
	if err != nil {
		fmt.Printf("[FAIL] Could not connect to Gmail API: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Gmail API connected (email: %s)\n", profile.EmailAddress)

	fmt.Println()
	fmt.Println("All tests passed! You're ready to use gsuite-mcp.")
}

func runWhoami(accountAlias string) {
	credPath := auth.GetCredentialsPath()
	tokenPath := resolveTokenPath(accountAlias, false)

	// Check if credentials exist
	if !fileExists(credPath) {
		fmt.Println("Not configured. Run 'gsuite-mcp setup' first.")
		os.Exit(1)
	}

	if !fileExists(tokenPath) {
		fmt.Println("Not authenticated. Run 'gsuite-mcp setup' to authenticate.")
		os.Exit(1)
	}

	ctx := context.Background()
	authenticator, err := auth.NewAuthenticator(credPath, tokenPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	client, err := authenticator.GetClient(ctx)
	if err != nil {
		fmt.Printf("Authentication error: %v\n", err)
		os.Exit(1)
	}

	svc, err := gmail.NewService(ctx, client)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	profile, err := svc.GetProfile(ctx)
	if err != nil {
		fmt.Printf("Could not get profile: %v\n", err)
		os.Exit(1)
	}

	if accountAlias != "" {
		fmt.Printf("Account:  %s\n", accountAlias)
	}
	fmt.Printf("Email:    %s\n", profile.EmailAddress)
	fmt.Printf("Messages: %d total\n", profile.MessagesTotal)
	fmt.Printf("Threads:  %d total\n", profile.ThreadsTotal)
}
