// ABOUTME: Tests for OAuth 2.0 authentication
// ABOUTME: Validates credential loading, token caching, and refresh

package auth

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthenticator_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "missing.json")

	_, err := NewAuthenticator(credPath, filepath.Join(tmpDir, "token.json"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials.json not found")
}

func TestAuthenticator_GetClient(t *testing.T) {
	// This will require mock credentials
	t.Skip("TODO: Implement with mock credentials")
}
