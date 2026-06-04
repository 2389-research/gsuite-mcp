// ABOUTME: Tests for the real Google OAuth token revocation flow
// ABOUTME: Covers URL/param construction, refresh-token preference, and offline fallback

package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRevokeToken_URLAndParams verifies that RevokeToken sends a POST to the
// revoke endpoint with the token in the form-encoded body (not the URL) and the
// correct Content-Type header.
func TestRevokeToken_URLAndParams(t *testing.T) {
	var (
		capturedMethod      string
		capturedTokenParam  string
		capturedContentType string
		capturedRawQuery    string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedContentType = r.Header.Get("Content-Type")
		capturedRawQuery = r.URL.RawQuery
		_ = r.ParseForm()
		capturedTokenParam = r.PostForm.Get("token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"access_token":"my-access","refresh_token":"my-refresh"}`), 0600))

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	err := a.RevokeToken(context.Background())
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, capturedMethod, "should use POST")
	assert.Equal(t, "my-refresh", capturedTokenParam, "should send refresh_token as token param")
	assert.Equal(t, "application/x-www-form-urlencoded", capturedContentType, "should set Content-Type header")
	assert.Empty(t, capturedRawQuery, "token must not be in the URL query string")

	// Local file must be deleted
	_, statErr := os.Stat(tokenPath)
	assert.True(t, os.IsNotExist(statErr), "local token file must be deleted after revoke")
}

// TestRevokeToken_PrefersRefreshToken verifies that when both refresh_token and
// access_token are present, the refresh_token is sent (it invalidates the whole grant).
func TestRevokeToken_PrefersRefreshToken(t *testing.T) {
	var capturedToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		capturedToken = r.PostForm.Get("token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"access_token":"acc-tok","refresh_token":"ref-tok"}`), 0600))

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	require.NoError(t, a.RevokeToken(context.Background()))
	assert.Equal(t, "ref-tok", capturedToken, "should prefer refresh_token over access_token")
}

// TestRevokeToken_RemoteFailureStillDeletesLocal verifies that when the revoke
// endpoint returns an error status, the local token file is still removed and
// ErrRemoteRevokeFailed is returned.
func TestRevokeToken_RemoteFailureStillDeletesLocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"access_token":"tok","refresh_token":"ref"}`), 0600))

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	err := a.RevokeToken(context.Background())
	require.Error(t, err, "should return an error when remote revoke fails")
	assert.True(t, errors.Is(err, ErrRemoteRevokeFailed), "error must wrap ErrRemoteRevokeFailed")

	// Local file must still be gone
	_, statErr := os.Stat(tokenPath)
	assert.True(t, os.IsNotExist(statErr), "local token file must be deleted even when remote revoke fails")
}

// TestRevokeToken_TransportErrorStillDeletesLocal verifies that a transport-level
// error (e.g., closed server) also results in local deletion and ErrRemoteRevokeFailed.
func TestRevokeToken_TransportErrorStillDeletesLocal(t *testing.T) {
	// Start a server then immediately close it to cause a transport error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"access_token":"tok","refresh_token":"ref"}`), 0600))

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	err := a.RevokeToken(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRemoteRevokeFailed), "transport error must wrap ErrRemoteRevokeFailed")

	_, statErr := os.Stat(tokenPath)
	assert.True(t, os.IsNotExist(statErr), "local token file must be deleted even after transport error")
}

// TestRevokeToken_NoCallWhenNoToken verifies that when no token file exists,
// no HTTP request is made to the revoke endpoint.
func TestRevokeToken_NoCallWhenNoToken(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "nonexistent.json")

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	err := a.RevokeToken(context.Background())
	assert.NoError(t, err)
	assert.False(t, called, "must not call the revoke endpoint when no token file exists")
}

// TestRevokeToken_FallsBackToAccessToken verifies that when only an access_token
// is present (no refresh_token), the access_token is sent as the token param.
func TestRevokeToken_FallsBackToAccessToken(t *testing.T) {
	var capturedToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		capturedToken = r.PostForm.Get("token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte(`{"access_token":"acc-only"}`), 0600))

	a := &Authenticator{tokenPath: tokenPath, revokeURL: srv.URL}

	require.NoError(t, a.RevokeToken(context.Background()))
	assert.Equal(t, "acc-only", capturedToken, "should fall back to access_token when refresh_token is absent")
}
