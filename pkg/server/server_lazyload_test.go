// ABOUTME: Verifies lazy account resolution never triggers interactive OAuth.
// ABOUTME: A missing token yields an auth_init hint, not a stdin prompt.

package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/gsuite-mcp/pkg/accounts"
)

func TestResolveServicesUnauthedAccountReturnsError(t *testing.T) {
	// Provide a fake-but-parseable credentials file so NewAuthenticator succeeds
	// in every environment (CI has no real credentials.json). This ensures execution
	// always reaches the GetClient/GetClientIfAuthenticated call, making the
	// RED→GREEN distinction environment-independent.
	credsDir := t.TempDir()
	credsPath := filepath.Join(credsDir, "credentials.json")
	fakeCreds := `{"installed":{"client_id":"test-client-id.apps.googleusercontent.com","project_id":"test-project","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","client_secret":"test-client-secret","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(credsPath, []byte(fakeCreds), 0600); err != nil {
		t.Fatalf("write fake creds: %v", err)
	}
	t.Setenv("GSUITE_MCP_CREDENTIALS_PATH", credsPath)

	s := &Server{
		registry: &accounts.Registry{
			DefaultAlias: "default",
			Accounts: map[string]*accounts.Account{
				"default": {Alias: "default"},
				"work":    {Alias: "work", TokenPath: "/nonexistent/path/token.json"},
			},
		},
		services: make(map[string]*AccountServices),
	}

	// Capture stdout to detect any interactive OAuth prompt emitted on the old path.
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("create pipe: %v", pipeErr)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = s.resolveServices(context.Background(), "work")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		os.Stdout = oldStdout
		_ = w.Close()
		t.Fatal("resolveServices blocked — it must not prompt on stdin")
	}

	os.Stdout = oldStdout
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if err == nil {
		t.Fatal("expected error for unauthenticated account, got nil")
	}
	if !strings.Contains(err.Error(), "auth_init") {
		t.Fatalf("error should point user to auth_init, got: %v", err)
	}
	if strings.Contains(string(out), "Go to the following link") || strings.Contains(string(out), "Enter authorization code") {
		t.Fatalf("resolveServices emitted an interactive OAuth prompt to stdout:\n%s", out)
	}
}
