// ABOUTME: XDG-compliant path resolution for credentials and tokens
// ABOUTME: Supports env var overrides, XDG dirs, and sensible defaults

package auth

import (
	"os"
	"path/filepath"
)

const (
	appName             = "gsuite-mcp"
	defaultCredentials  = "credentials.json"
	defaultToken        = "token.json"
	configSubdir        = ".config"
	dataSubdir          = ".local/share"
)

// GetCredentialsPath returns the path to credentials.json
// Priority: GSUITE_MCP_CREDENTIALS_PATH > XDG_CONFIG_HOME > ~/.config
// Note: Empty env vars are treated as unset (falls through to next priority).
// Env var overrides allow arbitrary paths for power users who need flexibility.
// XDG vars must be absolute paths per the XDG spec; relative paths are ignored.
// All paths are normalized with filepath.Clean for consistent path handling.
func GetCredentialsPath() string {
	if override := os.Getenv("GSUITE_MCP_CREDENTIALS_PATH"); override != "" {
		return filepath.Clean(override)
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" || !filepath.IsAbs(configHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return defaultCredentials // fallback to cwd
		}
		configHome = filepath.Join(home, configSubdir)
	}

	return filepath.Clean(filepath.Join(configHome, appName, defaultCredentials))
}

// GetTokenPath returns the path to token.json
// Priority: GSUITE_MCP_TOKEN_PATH > XDG_DATA_HOME > ~/.local/share
// Note: Empty env vars are treated as unset (falls through to next priority).
// Env var overrides allow arbitrary paths for power users who need flexibility.
// XDG vars must be absolute paths per the XDG spec; relative paths are ignored.
// All paths are normalized with filepath.Clean for consistent path handling.
func GetTokenPath() string {
	if override := os.Getenv("GSUITE_MCP_TOKEN_PATH"); override != "" {
		return filepath.Clean(override)
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" || !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return defaultToken // fallback to cwd
		}
		dataHome = filepath.Join(home, dataSubdir)
	}

	return filepath.Clean(filepath.Join(dataHome, appName, defaultToken))
}

// EnsureDir creates the parent directory for a file path if it doesn't exist.
// Directories are created with 0700 permissions (owner read/write/execute only).
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, 0700)
}
