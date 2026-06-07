// ABOUTME: OAuth 2.0 authentication for Google APIs
// ABOUTME: Handles credential loading, token caching, and refresh

package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/tasks/v1"
)

// googleRevokeURL is the Google OAuth revoke endpoint.
const googleRevokeURL = "https://oauth2.googleapis.com/revoke"

// ErrRemoteRevokeFailed is returned when the local token was deleted but the
// remote revocation call to Google could not be completed. The caller should
// advise the user to revoke access manually at https://myaccount.google.com/permissions.
var ErrRemoteRevokeFailed = errors.New("token deleted locally but Google revocation failed")

// DefaultScopes are the OAuth scopes for full GSuite access
var DefaultScopes = []string{
	gmail.GmailModifyScope,
	gmail.GmailLabelsScope,
	calendar.CalendarScope,
	people.ContactsScope,
	tasks.TasksScope,
}

// Authenticator handles OAuth 2.0 authentication
type Authenticator struct {
	credentialsPath string
	tokenPath       string
	config          *oauth2.Config
	revokeURL       string // injectable in tests; defaults to googleRevokeURL
}

// NewAuthenticator creates a new OAuth authenticator
func NewAuthenticator(credentialsPath, tokenPath string) (*Authenticator, error) {
	// Check if credentials file exists
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials.json not found at %s. Download from Google Cloud Console", credentialsPath)
	}

	// Read credentials file
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	// Parse credentials
	config, err := google.ConfigFromJSON(data, DefaultScopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return &Authenticator{
		credentialsPath: credentialsPath,
		tokenPath:       tokenPath,
		config:          config,
		revokeURL:       googleRevokeURL,
	}, nil
}

// GetClient returns an HTTP client with valid OAuth credentials
func (a *Authenticator) GetClient(ctx context.Context) (*http.Client, error) {
	token, err := a.loadToken()
	if err != nil {
		// No token found, need to authenticate
		token, err = a.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		if err := a.saveToken(token); err != nil {
			return nil, err
		}
	}

	// Wrap token source to persist refreshed tokens
	tokenSource := a.config.TokenSource(ctx, token)
	persistentSource := NewPersistentTokenSource(tokenSource, a.saveToken)

	return oauth2.NewClient(ctx, persistentSource), nil
}

// loadToken loads a cached token from disk
func (a *Authenticator) loadToken() (token *oauth2.Token, err error) {
	f, err := os.Open(a.tokenPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	token = &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to disk using atomic write (write to temp, then rename).
// This prevents partial writes and race conditions.
func (a *Authenticator) saveToken(token *oauth2.Token) error {
	if err := EnsureDir(a.tokenPath); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Write to temp file first for atomic operation
	dir := filepath.Dir(a.tokenPath)
	tmpFile, err := os.CreateTemp(dir, ".token-*.tmp")
	if err != nil {
		// Retry once if directory was removed between EnsureDir and CreateTemp (TOCTOU)
		if err := EnsureDir(a.tokenPath); err != nil {
			return fmt.Errorf("failed to create token directory: %w", err)
		}
		tmpFile, err = os.CreateTemp(dir, ".token-*.tmp")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// Set restrictive permissions before writing sensitive data
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if err := json.NewEncoder(tmpFile).Encode(token); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to encode token: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, a.tokenPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// authenticate performs the OAuth flow to get a new token
func (a *Authenticator) authenticate(ctx context.Context) (*oauth2.Token, error) {
	authURL := a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)
	fmt.Println("Enter authorization code: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("unable to read authorization code: %w", err)
		}
		return nil, fmt.Errorf("no authorization code provided")
	}
	authCode := strings.TrimSpace(scanner.Text())
	if authCode == "" {
		return nil, fmt.Errorf("authorization code cannot be empty")
	}

	token, err := a.config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token: %w", err)
	}

	return token, nil
}

// RevokeToken revokes the cached token at Google's OAuth revoke endpoint
// (best-effort), then deletes the local token file. If the remote revoke
// cannot be completed the local file is still removed and ErrRemoteRevokeFailed
// is returned so the caller can advise the user to revoke access manually.
func (a *Authenticator) RevokeToken(ctx context.Context) error {
	token, err := a.loadToken()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing cached, nothing to revoke
		}
		return err
	}

	// Prefer the refresh token (revoking it invalidates the whole grant);
	// fall back to the access token.
	revokeValue := token.RefreshToken
	if revokeValue == "" {
		revokeValue = token.AccessToken
	}

	remoteErr := a.revokeAtGoogle(ctx, revokeValue)

	// Always remove the local token, regardless of remote outcome.
	if rmErr := os.Remove(a.tokenPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return rmErr // real local failure takes precedence
	}

	if remoteErr != nil {
		return fmt.Errorf("%w: %v", ErrRemoteRevokeFailed, remoteErr)
	}
	return nil
}

// revokeAtGoogle sends a best-effort POST to Google's token revocation endpoint.
// The token is sent in the form-encoded request body (RFC 7009), not the URL.
// If value is empty the call is skipped. Network or non-2xx responses return
// a non-nil error; the token value is never included in log output.
func (a *Authenticator) revokeAtGoogle(ctx context.Context, value string) error {
	if value == "" {
		log.Printf("auth: skipping remote revoke — no token value available")
		return nil
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	body := strings.NewReader(url.Values{"token": {value}}.Encode())
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, a.revokeURL, body)
	if err != nil {
		return fmt.Errorf("building revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		log.Printf("auth: remote revoke request failed (transport error)")
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("auth: remote revoke returned non-2xx status %d", resp.StatusCode)
		return fmt.Errorf("revoke endpoint returned status %d", resp.StatusCode)
	}

	log.Printf("auth: remote revoke succeeded (status %d)", resp.StatusCode)
	return nil
}

// HasToken checks if a token file exists (does not validate the token)
func (a *Authenticator) HasToken() bool {
	_, err := os.Stat(a.tokenPath)
	return err == nil
}

// GetClientIfAuthenticated returns an HTTP client only if a token already exists.
// Unlike GetClient, this never triggers interactive authentication.
// Returns (nil, nil) if no token exists - caller should handle this gracefully.
func (a *Authenticator) GetClientIfAuthenticated(ctx context.Context) (*http.Client, error) {
	token, err := a.loadToken()
	if err != nil {
		// No token file - return nil client (not an error, just not authenticated)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Wrap token source to persist refreshed tokens
	tokenSource := a.config.TokenSource(ctx, token)
	persistentSource := NewPersistentTokenSource(tokenSource, a.saveToken)

	return oauth2.NewClient(ctx, persistentSource), nil
}

// PersistentTokenSource wraps an oauth2.TokenSource and persists refreshed tokens to disk.
// This ensures that when the underlying TokenSource automatically refreshes an expired
// access token, the new token is saved so it survives server restarts.
type PersistentTokenSource struct {
	source    oauth2.TokenSource
	lastToken *oauth2.Token
	saveFn    func(*oauth2.Token) error
	mu        sync.Mutex
}

// NewPersistentTokenSource creates a TokenSource that persists tokens when they change.
func NewPersistentTokenSource(source oauth2.TokenSource, saveFn func(*oauth2.Token) error) *PersistentTokenSource {
	return &PersistentTokenSource{
		source: source,
		saveFn: saveFn,
	}
}

// Token returns a valid token, persisting it to disk if it changed.
func (p *PersistentTokenSource) Token() (*oauth2.Token, error) {
	token, err := p.source.Token()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Only save if token changed (new access token)
	if p.lastToken == nil || token.AccessToken != p.lastToken.AccessToken {
		// Best-effort save - don't fail the token fetch if persistence fails
		_ = p.saveFn(token)
		p.lastToken = token
	}

	return token, nil
}

// TokenInfo contains metadata about the cached OAuth token
type TokenInfo struct {
	Valid      bool          `json:"valid"`
	Expiry     time.Time     `json:"expiry"`
	ExpiresIn  time.Duration `json:"expires_in"`
	HasRefresh bool          `json:"has_refresh"`
}

// TokenInfo returns metadata about the cached token without making API calls.
func (a *Authenticator) TokenInfo() (*TokenInfo, error) {
	token, err := a.loadToken()
	if err != nil {
		// No token file or unreadable - return empty info
		return &TokenInfo{Valid: false}, nil
	}

	info := &TokenInfo{
		Valid:      token.AccessToken != "" && token.Valid(),
		Expiry:     token.Expiry,
		HasRefresh: token.RefreshToken != "",
	}

	if !token.Expiry.IsZero() {
		info.ExpiresIn = time.Until(token.Expiry)
	}

	return info, nil
}

// AuthURL returns the OAuth authorization URL for user authentication.
func (a *Authenticator) AuthURL() string {
	return a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges an authorization code for tokens and saves them.
func (a *Authenticator) ExchangeCode(ctx context.Context, code string) error {
	token, err := a.config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	return a.saveToken(token)
}
