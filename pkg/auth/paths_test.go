// ABOUTME: Tests for XDG-compliant path resolution
// ABOUTME: Covers env var overrides, XDG vars, and default fallbacks

package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCredentialsPath(t *testing.T) {
	tests := []struct {
		name        string
		override    string
		xdgConfig   string
		wantSuffix  string
		wantExact   string
	}{
		{
			name:      "explicit override takes priority",
			override:  "/custom/path/creds.json",
			xdgConfig: "/should/be/ignored",
			wantExact: "/custom/path/creds.json",
		},
		{
			name:       "XDG_CONFIG_HOME when set",
			override:   "",
			xdgConfig:  "/tmp/xdg-config",
			wantSuffix: "/tmp/xdg-config/gsuite-mcp/credentials.json",
		},
		{
			name:       "falls back to ~/.config",
			override:   "",
			xdgConfig:  "",
			wantSuffix: ".config/gsuite-mcp/credentials.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GSUITE_MCP_CREDENTIALS_PATH", tt.override)
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfig)

			got := GetCredentialsPath()

			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("GetCredentialsPath() = %q, want %q", got, tt.wantExact)
				}
			} else if tt.wantSuffix != "" {
				if tt.xdgConfig != "" {
					// Exact match for XDG case
					if got != tt.wantSuffix {
						t.Errorf("GetCredentialsPath() = %q, want %q", got, tt.wantSuffix)
					}
				} else {
					// Suffix match for home dir case
					if !hasPathSuffix(got, tt.wantSuffix) {
						t.Errorf("GetCredentialsPath() = %q, want suffix %q", got, tt.wantSuffix)
					}
				}
			}
		})
	}
}

func TestGetTokenPath(t *testing.T) {
	tests := []struct {
		name       string
		override   string
		xdgData    string
		wantSuffix string
		wantExact  string
	}{
		{
			name:      "explicit override takes priority",
			override:  "/custom/path/tok.json",
			xdgData:   "/should/be/ignored",
			wantExact: "/custom/path/tok.json",
		},
		{
			name:       "XDG_DATA_HOME when set",
			override:   "",
			xdgData:    "/tmp/xdg-data",
			wantSuffix: "/tmp/xdg-data/gsuite-mcp/token.json",
		},
		{
			name:       "falls back to ~/.local/share",
			override:   "",
			xdgData:    "",
			wantSuffix: ".local/share/gsuite-mcp/token.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GSUITE_MCP_TOKEN_PATH", tt.override)
			t.Setenv("XDG_DATA_HOME", tt.xdgData)

			got := GetTokenPath()

			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("GetTokenPath() = %q, want %q", got, tt.wantExact)
				}
			} else if tt.wantSuffix != "" {
				if tt.xdgData != "" {
					// Exact match for XDG case
					if got != tt.wantSuffix {
						t.Errorf("GetTokenPath() = %q, want %q", got, tt.wantSuffix)
					}
				} else {
					// Suffix match for home dir case
					if !hasPathSuffix(got, tt.wantSuffix) {
						t.Errorf("GetTokenPath() = %q, want suffix %q", got, tt.wantSuffix)
					}
				}
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "creates nested directories",
			filePath: filepath.Join(tmpDir, "a", "b", "c", "file.json"),
			wantErr:  false,
		},
		{
			name:     "handles existing directory",
			filePath: filepath.Join(tmpDir, "existing.json"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureDir(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureDir() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				dir := filepath.Dir(tt.filePath)
				info, err := os.Stat(dir)
				if err != nil {
					t.Errorf("Directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Errorf("Expected directory, got file")
				}
			}
		})
	}
}

func TestEnsureDir_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "secure", "token.json")

	err := EnsureDir(filePath)
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	dir := filepath.Dir(filePath)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	// Check that directory has restricted permissions (0700)
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("Directory permissions = %o, want 0700", perm)
	}
}

// hasPathSuffix checks if path ends with the given suffix
func hasPathSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}

// TestGetCredentialsPath_PathNormalization verifies that filepath.Clean normalizes
// paths for consistent handling. Note: env var overrides allow arbitrary paths by
// design for power users; this tests normalization, not restriction.
func TestGetCredentialsPath_PathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		override string
		want     string
	}{
		{
			name:     "normalizes path traversal sequences",
			override: "/home/user/../../../etc/passwd",
			want:     "/etc/passwd",
		},
		{
			name:     "normalizes redundant slashes",
			override: "/home//user///config.json",
			want:     "/home/user/config.json",
		},
		{
			name:     "normalizes dot segments",
			override: "/home/user/./config/../creds.json",
			want:     "/home/user/creds.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GSUITE_MCP_CREDENTIALS_PATH", tt.override)
			t.Setenv("XDG_CONFIG_HOME", "")

			got := GetCredentialsPath()
			if got != tt.want {
				t.Errorf("GetCredentialsPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGetTokenPath_PathNormalization verifies path normalization for token paths.
func TestGetTokenPath_PathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		override string
		want     string
	}{
		{
			name:     "normalizes path traversal sequences",
			override: "/tmp/../../../etc/shadow",
			want:     "/etc/shadow",
		},
		{
			name:     "normalizes redundant slashes",
			override: "/tmp//tokens///token.json",
			want:     "/tmp/tokens/token.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GSUITE_MCP_TOKEN_PATH", tt.override)
			t.Setenv("XDG_DATA_HOME", "")

			got := GetTokenPath()
			if got != tt.want {
				t.Errorf("GetTokenPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetCredentialsPath_HomeDirFailure(t *testing.T) {
	// When HOME is unset and XDG vars are unset, should fall back to cwd
	t.Setenv("GSUITE_MCP_CREDENTIALS_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	got := GetCredentialsPath()
	// Should either use home dir or fall back to credentials.json
	// On most systems HOME will still resolve, but the fallback exists
	if got == "" {
		t.Error("GetCredentialsPath() returned empty string")
	}
}

func TestGetTokenPath_HomeDirFailure(t *testing.T) {
	// When HOME is unset and XDG vars are unset, should fall back to cwd
	t.Setenv("GSUITE_MCP_TOKEN_PATH", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	got := GetTokenPath()
	// Should either use home dir or fall back to token.json
	if got == "" {
		t.Error("GetTokenPath() returned empty string")
	}
}

// TestGetCredentialsPath_RelativeXDG verifies that relative XDG_CONFIG_HOME paths
// are ignored per the XDG spec (which requires absolute paths).
func TestGetCredentialsPath_RelativeXDG(t *testing.T) {
	t.Setenv("GSUITE_MCP_CREDENTIALS_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "./relative/config") // Relative path should be ignored

	got := GetCredentialsPath()
	// Should NOT contain the relative path; should fall back to ~/.config
	if hasPathSuffix(got, "relative/config/gsuite-mcp/credentials.json") {
		t.Errorf("GetCredentialsPath() used relative XDG path: %q", got)
	}
	// Should contain the home fallback
	if !hasPathSuffix(got, ".config/gsuite-mcp/credentials.json") {
		t.Errorf("GetCredentialsPath() = %q, expected suffix .config/gsuite-mcp/credentials.json", got)
	}
}

// TestGetTokenPath_RelativeXDG verifies that relative XDG_DATA_HOME paths
// are ignored per the XDG spec (which requires absolute paths).
func TestGetTokenPath_RelativeXDG(t *testing.T) {
	t.Setenv("GSUITE_MCP_TOKEN_PATH", "")
	t.Setenv("XDG_DATA_HOME", "relative/data") // Relative path should be ignored

	got := GetTokenPath()
	// Should NOT contain the relative path; should fall back to ~/.local/share
	if hasPathSuffix(got, "relative/data/gsuite-mcp/token.json") {
		t.Errorf("GetTokenPath() used relative XDG path: %q", got)
	}
	// Should contain the home fallback
	if !hasPathSuffix(got, ".local/share/gsuite-mcp/token.json") {
		t.Errorf("GetTokenPath() = %q, expected suffix .local/share/gsuite-mcp/token.json", got)
	}
}
