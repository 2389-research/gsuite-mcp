// ABOUTME: Edge case tests for OAuth 2.0 authentication
// ABOUTME: Tests error handling, malformed data, and filesystem edge cases

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestLoadToken_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create corrupted JSON file
	err := os.WriteFile(tokenPath, []byte(`{"access_token": "foo", "malformed": `), 0600)
	require.NoError(t, err)

	// Create a minimal valid credentials file for the authenticator
	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Try to load the corrupted token
	token, err := auth.loadToken()
	assert.Error(t, err)
	// Token will be non-nil but the error indicates JSON parsing failure
	assert.NotNil(t, token)
}

func TestLoadToken_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create empty token file
	err := os.WriteFile(tokenPath, []byte(""), 0600)
	require.NoError(t, err)

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Try to load the empty token
	token, err := auth.loadToken()
	assert.Error(t, err)
	// Token will be non-nil but the error indicates EOF
	assert.NotNil(t, token)
}

func TestLoadToken_InvalidTokenStructure(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create valid JSON but not a token structure
	err := os.WriteFile(tokenPath, []byte(`{"not": "a", "token": "structure"}`), 0600)
	require.NoError(t, err)

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Should load without error but token will be empty/invalid
	token, err := auth.loadToken()
	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.Empty(t, token.AccessToken)
}

func TestLoadToken_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "nonexistent.json")

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Should return error for non-existent file
	token, err := auth.loadToken()
	assert.Error(t, err)
	assert.Nil(t, token)
	assert.True(t, os.IsNotExist(err))
}

func TestSaveToken_ValidToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Create a valid token
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
	}

	// Save the token
	err = auth.saveToken(token)
	assert.NoError(t, err)

	// Verify the file was created and is valid JSON
	data, err := os.ReadFile(tokenPath)
	require.NoError(t, err)

	var loadedToken oauth2.Token
	err = json.Unmarshal(data, &loadedToken)
	assert.NoError(t, err)
	assert.Equal(t, token.AccessToken, loadedToken.AccessToken)
	assert.Equal(t, token.RefreshToken, loadedToken.RefreshToken)
}

func TestSaveToken_ReadOnlyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0500) // Read and execute only
	require.NoError(t, err)

	// Make directory read-only
	defer func() { _ = os.Chmod(readOnlyDir, 0700) }() // Cleanup

	tokenPath := filepath.Join(readOnlyDir, "token.json")
	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	token := &oauth2.Token{
		AccessToken: "test-token",
	}

	// Should fail to save to read-only directory
	err = auth.saveToken(token)
	assert.Error(t, err)
}

func TestNewAuthenticator_MissingCredentialsEdgeCase(t *testing.T) {
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "nonexistent.json")
	tokenPath := filepath.Join(tmpDir, "token.json")

	auth, err := NewAuthenticator(credPath, tokenPath)
	assert.Error(t, err)
	assert.Nil(t, auth)
	assert.Contains(t, err.Error(), "credentials.json not found")
}

func TestNewAuthenticator_MalformedCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "credentials.json")

	// Create malformed credentials file
	err := os.WriteFile(credPath, []byte(`{"invalid": "json", `), 0600)
	require.NoError(t, err)

	tokenPath := filepath.Join(tmpDir, "token.json")

	auth, err := NewAuthenticator(credPath, tokenPath)
	assert.Error(t, err)
	assert.Nil(t, auth)
	assert.Contains(t, err.Error(), "unable to parse credentials")
}

func TestNewAuthenticator_InvalidCredentialsStructure(t *testing.T) {
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "credentials.json")

	// Create valid JSON but invalid OAuth credentials structure
	err := os.WriteFile(credPath, []byte(`{"not": "oauth", "credentials": "here"}`), 0600)
	require.NoError(t, err)

	tokenPath := filepath.Join(tmpDir, "token.json")

	auth, err := NewAuthenticator(credPath, tokenPath)
	assert.Error(t, err)
	assert.Nil(t, auth)
	assert.Contains(t, err.Error(), "unable to parse credentials")
}

func TestNewAuthenticator_EmptyCredentialsFile(t *testing.T) {
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "credentials.json")

	// Create empty credentials file
	err := os.WriteFile(credPath, []byte(""), 0600)
	require.NoError(t, err)

	tokenPath := filepath.Join(tmpDir, "token.json")

	auth, err := NewAuthenticator(credPath, tokenPath)
	assert.Error(t, err)
	assert.Nil(t, auth)
}

func TestNewAuthenticator_EmptyPaths(t *testing.T) {
	// Test with empty credentials path
	auth, err := NewAuthenticator("", "token.json")
	assert.Error(t, err)
	assert.Nil(t, auth)

	// Test with empty token path (this should succeed as token path is only used later)
	tmpDir := t.TempDir()
	credPath := createValidCredentialsFile(t, tmpDir)
	auth, err = NewAuthenticator(credPath, "")
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestRevokeToken_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create a token file
	err := os.WriteFile(tokenPath, []byte(`{"access_token": "test"}`), 0600)
	require.NoError(t, err)

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Revoke the token
	err = auth.RevokeToken()
	assert.NoError(t, err)

	// Verify token file is gone
	_, err = os.Stat(tokenPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRevokeToken_NonExistentToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "nonexistent.json")

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Revoking non-existent token should not error
	err = auth.RevokeToken()
	assert.NoError(t, err)
}

func TestLoadToken_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create a token file with no read permissions
	err := os.WriteFile(tokenPath, []byte(`{"access_token": "test"}`), 0000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(tokenPath, 0600) }() // Cleanup

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Should fail to load due to permissions
	token, err := auth.loadToken()
	assert.Error(t, err)
	assert.Nil(t, token)
}

func TestSaveToken_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	// Create an existing token file
	err := os.WriteFile(tokenPath, []byte(`{"access_token": "old"}`), 0600)
	require.NoError(t, err)

	credPath := createValidCredentialsFile(t, tmpDir)

	auth, err := NewAuthenticator(credPath, tokenPath)
	require.NoError(t, err)

	// Save a new token
	newToken := &oauth2.Token{
		AccessToken: "new-token",
	}

	err = auth.saveToken(newToken)
	assert.NoError(t, err)

	// Verify the token was overwritten
	data, err := os.ReadFile(tokenPath)
	require.NoError(t, err)

	var loadedToken oauth2.Token
	err = json.Unmarshal(data, &loadedToken)
	assert.NoError(t, err)
	assert.Equal(t, "new-token", loadedToken.AccessToken)
}

// Helper function to create a valid OAuth credentials file for testing
func createValidCredentialsFile(t *testing.T, dir string) string {
	t.Helper()

	credPath := filepath.Join(dir, "credentials.json")

	// Create a minimal valid OAuth client credentials structure
	credentials := map[string]interface{}{
		"installed": map[string]interface{}{
			"client_id":     "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"redirect_uris": []string{"http://localhost"},
			"auth_uri":      "https://accounts.google.com/o/oauth2/auth",
			"token_uri":     "https://oauth2.googleapis.com/token",
		},
	}

	data, err := json.Marshal(credentials)
	require.NoError(t, err)

	err = os.WriteFile(credPath, data, 0600)
	require.NoError(t, err)

	return credPath
}
