// ABOUTME: OAuth 2.0 authentication for Google APIs
// ABOUTME: Handles credential loading, token caching, and refresh

package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/people/v1"
)

// DefaultScopes are the OAuth scopes for full GSuite access
var DefaultScopes = []string{
	gmail.GmailModifyScope,
	gmail.GmailLabelsScope,
	calendar.CalendarScope,
	people.ContactsScope,
}

// Authenticator handles OAuth 2.0 authentication
type Authenticator struct {
	credentialsPath string
	tokenPath       string
	config          *oauth2.Config
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

	// Check if token needs refresh
	tokenSource := a.config.TokenSource(ctx, token)

	return oauth2.NewClient(ctx, tokenSource), nil
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

// RevokeToken deletes the cached token
func (a *Authenticator) RevokeToken() error {
	if _, err := os.Stat(a.tokenPath); err == nil {
		return os.Remove(a.tokenPath)
	}
	return nil
}
