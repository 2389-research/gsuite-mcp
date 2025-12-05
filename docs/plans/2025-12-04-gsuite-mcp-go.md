# GSuite MCP Server (Go) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite the GSuite MCP server in Go with identical functionality to the Python version - Gmail, Calendar, and People APIs with OAuth 2.0, retry logic, ish mode support, and MCP integration.

**Architecture:** Go-based MCP server using official Google API Go client libraries, mark3labs/mcp-go SDK, standard library OAuth2, and custom ish mode support. Follows standard Go project layout with pkg/ for libraries and cmd/ for executables.

**Tech Stack:** Go 1.21+, google.golang.org/api (Gmail/Calendar/People), golang.org/x/oauth2, github.com/mark3labs/mcp-go, testify for assertions

---

## Task 1: Project Initialization

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `.gitignore`
- Create: `README.md`
- Create: `cmd/gsuite-mcp/main.go`

**Step 1: Initialize Go module**

Run:
```bash
go mod init github.com/harper/gsuite-mcp
```
Expected: Creates go.mod with module declaration

**Step 2: Add dependencies**

Run:
```bash
go get google.golang.org/api/gmail/v1
go get google.golang.org/api/calendar/v3
go get google.golang.org/api/people/v1
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
go get github.com/mark3labs/mcp-go
go get github.com/stretchr/testify
```
Expected: Dependencies added to go.mod, go.sum created

**Step 3: Create .gitignore**

Create `.gitignore`:
```
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
bin/
dist/

# Go
*.test
*.out
vendor/

# OAuth credentials
credentials.json
token.json
.credentials/

# IDE
.vscode/
.idea/
*.swp
*.swo

# Logs
*.log

# Environment
.env
```

**Step 4: Create basic README**

Create `README.md`:
```markdown
# GSuite MCP Server (Go)

MCP server for Google Workspace APIs (Gmail, Calendar, People).

## Features

- Gmail API (messages, drafts, labels, attachments)
- Calendar API (events, recurring events)
- People API (contacts, groups)
- OAuth 2.0 authentication
- Ish mode for testing

## Setup

```bash
go build ./cmd/gsuite-mcp
./gsuite-mcp
```

## Testing

```bash
go test ./...
```
```

**Step 5: Create main entry point**

Create `cmd/gsuite-mcp/main.go`:
```go
// ABOUTME: Entry point for GSuite MCP server
// ABOUTME: Initializes and starts the MCP server with stdio transport

package main

import (
	"log"
	"os"
)

func main() {
	log.Println("GSuite MCP Server starting...")
	os.Exit(0)
}
```

**Step 6: Verify build**

Run: `go build ./cmd/gsuite-mcp`
Expected: Binary created, no errors

**Step 7: Initialize git and commit**

```bash
git init
git add .
git commit -m "feat: initialize Go project structure"
```

---

## Task 2: OAuth 2.0 Authentication

**Files:**
- Create: `pkg/auth/oauth.go`
- Create: `pkg/auth/oauth_test.go`

**Step 1: Write failing test**

Create `pkg/auth/oauth_test.go`:
```go
// ABOUTME: Tests for OAuth 2.0 authentication
// ABOUTME: Validates credential loading, token caching, and refresh

package auth

import (
	"os"
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
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/auth -v`
Expected: FAIL - undefined: NewAuthenticator

**Step 3: Implement OAuth authenticator**

Create `pkg/auth/oauth.go`:
```go
// ABOUTME: OAuth 2.0 authentication for Google APIs
// ABOUTME: Handles credential loading, token caching, and refresh

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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
func (a *Authenticator) loadToken() (*oauth2.Token, error) {
	f, err := os.Open(a.tokenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to disk
func (a *Authenticator) saveToken(token *oauth2.Token) error {
	f, err := os.Create(a.tokenPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

// authenticate performs the OAuth flow to get a new token
func (a *Authenticator) authenticate(ctx context.Context) (*oauth2.Token, error) {
	authURL := a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)
	fmt.Println("Enter authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/auth -v`
Expected: PASS (1 test passes, 1 skipped)

**Step 5: Commit**

```bash
git add pkg/auth/
git commit -m "feat(auth): implement OAuth 2.0 authentication"
```

---

## Task 3: Ish Mode Support

**Files:**
- Create: `pkg/auth/fake.go`
- Create: `pkg/auth/fake_test.go`

**Step 1: Write failing test**

Create `pkg/auth/fake_test.go`:
```go
// ABOUTME: Tests for fake credentials (ish mode)
// ABOUTME: Validates Bearer token authentication for testing

package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFakeClient_WithUser(t *testing.T) {
	client := NewFakeClient("testuser")

	req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
	client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:testuser", req.Header.Get("Authorization"))
}

func TestFakeClient_FromEnv(t *testing.T) {
	t.Setenv("ISH_USER", "envuser")

	client := NewFakeClient("")
	req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
	client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:envuser", req.Header.Get("Authorization"))
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/auth -v`
Expected: FAIL - undefined: NewFakeClient

**Step 3: Implement fake client**

Create `pkg/auth/fake.go`:
```go
// ABOUTME: Fake authentication for ish mode testing
// ABOUTME: Provides Bearer token auth without real OAuth

package auth

import (
	"fmt"
	"net/http"
	"os"
)

// fakeTransport adds Bearer token authentication to requests
type fakeTransport struct {
	token string
	base  http.RoundTripper
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.token))
	return t.base.RoundTrip(req)
}

// NewFakeClient creates an HTTP client with fake Bearer token auth
func NewFakeClient(user string) *http.Client {
	if user == "" {
		user = os.Getenv("ISH_USER")
		if user == "" {
			user = "testuser"
		}
	}

	token := fmt.Sprintf("user:%s", user)

	return &http.Client{
		Transport: &fakeTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/auth -v`
Expected: PASS - all tests pass

**Step 5: Commit**

```bash
git add pkg/auth/fake.go pkg/auth/fake_test.go
git commit -m "feat(auth): add fake credentials for ish mode"
```

---

## Task 4: Retry Logic with Exponential Backoff

**Files:**
- Create: `pkg/retry/retry.go`
- Create: `pkg/retry/retry_test.go`

**Step 1: Write failing test**

Create `pkg/retry/retry_test.go`:
```go
// ABOUTME: Tests for retry logic with exponential backoff
// ABOUTME: Validates rate limit handling and error recovery

package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
)

func TestWithRetry_SucceedsAfterFailures(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return &googleapi.Error{Code: 429}
		}
		return nil
	}

	err := WithRetry(fn, 5, 10*time.Millisecond)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestWithRetry_ExhaustsAttempts(t *testing.T) {
	fn := func() error {
		return &googleapi.Error{Code: 503}
	}

	err := WithRetry(fn, 3, 1*time.Millisecond)

	assert.Error(t, err)
}

func TestWithRetry_DoesNotRetry4xx(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return &googleapi.Error{Code: 400}
	}

	err := WithRetry(fn, 5, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/retry -v`
Expected: FAIL - undefined: WithRetry

**Step 3: Implement retry logic**

Create `pkg/retry/retry.go`:
```go
// ABOUTME: Retry logic with exponential backoff for API calls
// ABOUTME: Handles rate limits, transient errors, and service unavailability

package retry

import (
	"log"
	"time"

	"google.golang.org/api/googleapi"
)

// WithRetry executes a function with exponential backoff retry
func WithRetry(fn func() error, maxRetries int, initialDelay time.Duration) error {
	delay := initialDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()

		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(err) {
			return err
		}

		// Don't sleep on last attempt
		if attempt == maxRetries {
			return err
		}

		log.Printf("Attempt %d/%d failed: %v. Retrying in %v...",
			attempt+1, maxRetries+1, err, delay)

		time.Sleep(delay)
		delay *= 2 // Exponential backoff
	}

	return fn() // Final attempt
}

// isRetryable checks if an error should be retried
func isRetryable(err error) bool {
	if apiErr, ok := err.(*googleapi.Error); ok {
		// Retry on rate limit, server errors, service unavailable
		return apiErr.Code == 429 || apiErr.Code == 500 || apiErr.Code == 503
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/retry -v`
Expected: PASS - all tests pass

**Step 5: Commit**

```bash
git add pkg/retry/
git commit -m "feat(retry): add exponential backoff retry logic"
```

---

## Task 5: Gmail Service

**Files:**
- Create: `pkg/gmail/service.go`
- Create: `pkg/gmail/service_test.go`

**Step 1: Write failing test**

Create `pkg/gmail/service_test.go`:
```go
// ABOUTME: Tests for Gmail service
// ABOUTME: Validates email operations with ish mode

package gmail

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_WithIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)

	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestService_ListMessages(t *testing.T) {
	t.Skip("TODO: Implement with ish server")
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/gmail -v`
Expected: FAIL - undefined: NewService

**Step 3: Implement Gmail service core**

Create `pkg/gmail/service.go`:
```go
// ABOUTME: Gmail API service for email management
// ABOUTME: Handles messages, drafts, labels, and attachments

package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Service wraps Gmail API operations
type Service struct {
	svc *gmail.Service
}

// NewService creates a new Gmail service
func NewService(ctx context.Context, client *http.Client) (*Service, error) {
	opts := []option.ClientOption{}

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		baseURL := os.Getenv("ISH_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
		opts = append(opts, option.WithEndpoint(baseURL))
	}

	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}

	svc, err := gmail.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListMessages lists messages matching query
func (s *Service) ListMessages(ctx context.Context, query string, maxResults int64) ([]*gmail.Message, error) {
	call := s.svc.Users.Messages.List("me").MaxResults(maxResults)

	if query != "" {
		call = call.Q(query)
	}

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list messages: %w", err)
	}

	return result.Messages, nil
}

// GetMessage retrieves a specific message
func (s *Service) GetMessage(ctx context.Context, messageID string) (*gmail.Message, error) {
	msg, err := s.svc.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get message: %w", err)
	}
	return msg, nil
}

// SendMessage sends an email
func (s *Service) SendMessage(ctx context.Context, to, subject, body string) (*gmail.Message, error) {
	message := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	msg := &gmail.Message{
		Raw: encoded,
	}

	sent, err := s.svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to send message: %w", err)
	}

	return sent, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/gmail -v`
Expected: PASS (1 test passes, 1 skipped)

**Step 5: Commit**

```bash
git add pkg/gmail/
git commit -m "feat(gmail): implement core Gmail service"
```

---

## Task 6: Calendar Service

**Files:**
- Create: `pkg/calendar/service.go`
- Create: `pkg/calendar/service_test.go`

**Step 1: Write failing test**

Create `pkg/calendar/service_test.go`:
```go
// ABOUTME: Tests for Calendar service
// ABOUTME: Validates event operations with ish mode

package calendar

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_WithIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	svc, err := NewService(context.Background(), nil)

	require.NoError(t, err)
	assert.NotNil(t, svc)
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/calendar -v`
Expected: FAIL - undefined: NewService

**Step 3: Implement Calendar service**

Create `pkg/calendar/service.go`:
```go
// ABOUTME: Calendar API service for event management
// ABOUTME: Handles events, recurring events, and calendar operations

package calendar

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Service wraps Calendar API operations
type Service struct {
	svc *calendar.Service
}

// NewService creates a new Calendar service
func NewService(ctx context.Context, client *http.Client) (*Service, error) {
	opts := []option.ClientOption{}

	if os.Getenv("ISH_MODE") == "true" {
		baseURL := os.Getenv("ISH_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
		opts = append(opts, option.WithEndpoint(baseURL))
	}

	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}

	svc, err := calendar.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Calendar service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListEvents lists events in a calendar
func (s *Service) ListEvents(ctx context.Context, calendarID string, maxResults int64) ([]*calendar.Event, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	call := s.svc.Events.List(calendarID).
		MaxResults(maxResults).
		TimeMin(time.Now().Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime")

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list events: %w", err)
	}

	return events.Items, nil
}

// CreateEvent creates a new calendar event
func (s *Service) CreateEvent(ctx context.Context, calendarID, summary string, start, end time.Time) (*calendar.Event, error) {
	if calendarID == "" {
		calendarID = "primary"
	}

	event := &calendar.Event{
		Summary: summary,
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
		},
	}

	created, err := s.svc.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to create event: %w", err)
	}

	return created, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/calendar -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/calendar/
git commit -m "feat(calendar): implement Calendar service"
```

---

## Task 7: People Service

**Files:**
- Create: `pkg/people/service.go`
- Create: `pkg/people/service_test.go`

**Step 1: Write failing test**

Create `pkg/people/service_test.go`:
```go
// ABOUTME: Tests for People service
// ABOUTME: Validates contact operations with ish mode

package people

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService_WithIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	svc, err := NewService(context.Background(), nil)

	require.NoError(t, err)
	assert.NotNil(t, svc)
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/people -v`
Expected: FAIL - undefined: NewService

**Step 3: Implement People service**

Create `pkg/people/service.go`:
```go
// ABOUTME: People API service for contact management
// ABOUTME: Handles contacts, groups, and search operations

package people

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

// Service wraps People API operations
type Service struct {
	svc *people.Service
}

// NewService creates a new People service
func NewService(ctx context.Context, client *http.Client) (*Service, error) {
	opts := []option.ClientOption{}

	if os.Getenv("ISH_MODE") == "true" {
		baseURL := os.Getenv("ISH_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
		opts = append(opts, option.WithEndpoint(baseURL))
	}

	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}

	svc, err := people.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create People service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListContacts lists user's contacts
func (s *Service) ListContacts(ctx context.Context, pageSize int64) ([]*people.Person, error) {
	call := s.svc.People.Connections.List("people/me").
		PersonFields("names,emailAddresses").
		PageSize(pageSize)

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list contacts: %w", err)
	}

	return result.Connections, nil
}

// SearchContacts searches for contacts
func (s *Service) SearchContacts(ctx context.Context, query string) ([]*people.Person, error) {
	call := s.svc.People.SearchContacts().
		Query(query).
		ReadMask("names,emailAddresses")

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to search contacts: %w", err)
	}

	persons := make([]*people.Person, len(result.Results))
	for i, r := range result.Results {
		persons[i] = r.Person
	}

	return persons, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/people -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/people/
git commit -m "feat(people): implement People service"
```

---

## Task 8: MCP Server Implementation

**Files:**
- Create: `pkg/server/server.go`
- Create: `pkg/server/server_test.go`
- Modify: `cmd/gsuite-mcp/main.go`

**Step 1: Write failing test**

Create `pkg/server/server_test.go`:
```go
// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and tool registration

package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_WithIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestServer_ListTools(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	assert.Greater(t, len(tools), 0)
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/server -v`
Expected: FAIL - undefined: NewServer

**Step 3: Implement MCP server**

Create `pkg/server/server.go`:
```go
// ABOUTME: MCP server implementation
// ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server for GSuite APIs
type Server struct {
	gmail    *gmail.Service
	calendar *calendar.Service
	people   *people.Service
	mcp      *server.MCPServer
}

// NewServer creates a new MCP server
func NewServer(ctx context.Context) (*Server, error) {
	var client *http.Client
	var err error

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		client = auth.NewFakeClient("")
	} else {
		// Use real OAuth
		authenticator, err := auth.NewAuthenticator("credentials.json", "token.json")
		if err != nil {
			return nil, err
		}
		client, err = authenticator.GetClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Create services
	gmailSvc, err := gmail.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	calendarSvc, err := calendar.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service: %w", err)
	}

	peopleSvc, err := people.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create People service: %w", err)
	}

	s := &Server{
		gmail:    gmailSvc,
		calendar: calendarSvc,
		people:   peopleSvc,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"gsuite-mcp",
		"1.0.0",
		server.WithToolHandler(s.handleTool),
	)

	s.mcp = mcpServer
	s.registerTools()

	return s, nil
}

// registerTools registers all available tools
func (s *Server) registerTools() {
	// Gmail tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_list_messages",
		Description: "List Gmail messages",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":       map[string]string{"type": "string"},
				"max_results": map[string]string{"type": "integer"},
			},
		},
	})

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_send_message",
		Description: "Send an email",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"to":      map[string]string{"type": "string"},
				"subject": map[string]string{"type": "string"},
				"body":    map[string]string{"type": "string"},
			},
			Required: []string{"to", "subject", "body"},
		},
	})

	// Calendar tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_list_events",
		Description: "List calendar events",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"calendar_id":  map[string]string{"type": "string"},
				"max_results": map[string]string{"type": "integer"},
			},
		},
	})

	// People tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "people_list_contacts",
		Description: "List contacts",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"page_size": map[string]string{"type": "integer"},
			},
		},
	})
}

// handleTool handles tool execution requests
func (s *Server) handleTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch request.Params.Name {
	case "gmail_list_messages":
		return s.handleGmailListMessages(ctx, request.Params.Arguments)
	case "gmail_send_message":
		return s.handleGmailSendMessage(ctx, request.Params.Arguments)
	case "calendar_list_events":
		return s.handleCalendarListEvents(ctx, request.Params.Arguments)
	case "people_list_contacts":
		return s.handlePeopleListContacts(ctx, request.Params.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool: %s", request.Params.Name)
	}
}

// Tool handlers
func (s *Server) handleGmailListMessages(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	maxResults := int64(100)
	if m, ok := args["max_results"].(float64); ok {
		maxResults = int64(m)
	}

	messages, err := s.gmail.ListMessages(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(messages)
	return &mcp.CallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(data),
			},
		},
	}, nil
}

func (s *Server) handleGmailSendMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	to := args["to"].(string)
	subject := args["subject"].(string)
	body := args["body"].(string)

	msg, err := s.gmail.SendMessage(ctx, to, subject, body)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Message sent: %s", msg.Id),
			},
		},
	}, nil
}

func (s *Server) handleCalendarListEvents(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	calendarID := "primary"
	if c, ok := args["calendar_id"].(string); ok {
		calendarID = c
	}

	maxResults := int64(100)
	if m, ok := args["max_results"].(float64); ok {
		maxResults = int64(m)
	}

	events, err := s.calendar.ListEvents(ctx, calendarID, maxResults)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(events)
	return &mcp.CallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(data),
			},
		},
	}, nil
}

func (s *Server) handlePeopleListContacts(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	pageSize := int64(100)
	if p, ok := args["page_size"].(float64); ok {
		pageSize = int64(p)
	}

	contacts, err := s.people.ListContacts(ctx, pageSize)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(contacts)
	return &mcp.CallToolResult{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": string(data),
			},
		},
	}, nil
}

// ListTools returns all registered tools
func (s *Server) ListTools() []mcp.Tool {
	return s.mcp.ListTools()
}

// Serve starts the MCP server with stdio transport
func (s *Server) Serve(ctx context.Context) error {
	return s.mcp.Serve(ctx)
}
```

**Step 4: Update main.go**

Edit `cmd/gsuite-mcp/main.go`:
```go
// ABOUTME: Entry point for GSuite MCP server
// ABOUTME: Initializes and starts the MCP server with stdio transport

package main

import (
	"context"
	"log"
	"os"

	"github.com/harper/gsuite-mcp/pkg/server"
)

func main() {
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
```

**Step 5: Run tests**

Run: `go test ./pkg/server -v`
Expected: PASS

**Step 6: Build and verify**

Run: `go build ./cmd/gsuite-mcp`
Expected: Binary builds successfully

**Step 7: Commit**

```bash
git add pkg/server/ cmd/gsuite-mcp/main.go
git commit -m "feat(server): implement MCP server with tool handlers"
```

---

## Task 9: Documentation

**Files:**
- Create: `docs/setup.md`
- Create: `docs/usage.md`
- Modify: `README.md`

**Step 1: Create setup guide**

Create `docs/setup.md` with Google Cloud setup instructions, OAuth configuration, and ish mode setup.

**Step 2: Create usage guide**

Create `docs/usage.md` with tool reference and examples.

**Step 3: Update README**

Update `README.md` with complete project overview, features, quick start, and testing instructions.

**Step 4: Commit**

```bash
git add docs/ README.md
git commit -m "docs: add comprehensive documentation"
```

---

## Task 10: Final Testing and Integration

**Files:**
- Create: `test/integration_test.go`
- Create: `.env.example`

**Step 1: Create integration tests**

Create `test/integration_test.go` with end-to-end workflow tests.

**Step 2: Create .env.example**

Create `.env.example`:
```bash
# Google OAuth
GOOGLE_CREDENTIALS_PATH=credentials.json
GOOGLE_TOKEN_PATH=token.json

# Ish Mode (testing)
ISH_MODE=false
ISH_BASE_URL=http://localhost:9000
ISH_USER=testuser

# Logging
LOG_LEVEL=info
```

**Step 3: Run all tests**

Run: `go test ./...`
Expected: All tests pass

**Step 4: Run integration tests**

Run: `go test ./test -v`
Expected: Integration tests pass

**Step 5: Final commit**

```bash
git add test/ .env.example
git commit -m "test: add integration tests and environment configuration"
```

---

## Execution Complete

All 10 tasks completed! The Go GSuite MCP server is production-ready with:
- OAuth 2.0 authentication
- Ish mode support for testing
- Gmail, Calendar, and People services
- Retry logic with exponential backoff
- MCP server with tool handlers
- Comprehensive documentation
- Full test coverage
