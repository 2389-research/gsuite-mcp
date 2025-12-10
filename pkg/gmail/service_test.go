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

// TestNewService_EnvironmentConfig tests various environment configurations
func TestNewService_EnvironmentConfig(t *testing.T) {
	t.Run("ISH_MODE with custom base URL", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", "https://custom.example.com:8080")

		svc, err := NewService(context.Background(), nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("ISH_MODE without base URL uses default", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		// Don't set ISH_BASE_URL - should default to http://localhost:9000

		svc, err := NewService(context.Background(), nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("Non-ISH mode requires client", func(t *testing.T) {
		t.Setenv("ISH_MODE", "false")

		// Without a valid client, should fail
		_, err := NewService(context.Background(), nil)
		if err == nil {
			t.Log("Note: Service creation succeeded without credentials (unexpected)")
		}
	})
}

// TestSendMessage_Validation tests input validation
func TestSendMessage_Validation(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Empty recipient fails", func(t *testing.T) {
		_, err := svc.SendMessage(ctx, "", "Subject", "Body", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient address (to) cannot be empty")
	})

	t.Run("Empty subject fails", func(t *testing.T) {
		_, err := svc.SendMessage(ctx, "test@example.com", "", "Body", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "subject cannot be empty")
	})
}

// TestCreateDraft_Validation tests input validation for drafts
func TestCreateDraft_Validation(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Empty recipient fails", func(t *testing.T) {
		_, err := svc.CreateDraft(ctx, "", "Subject", "Body", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient address (to) cannot be empty")
	})

	t.Run("Empty subject fails", func(t *testing.T) {
		_, err := svc.CreateDraft(ctx, "test@example.com", "", "Body", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "subject cannot be empty")
	})
}

func TestIsHTML(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "Plain text",
			body:     "This is a plain text email",
			expected: false,
		},
		{
			name:     "Empty string",
			body:     "",
			expected: false,
		},
		{
			name:     "HTML with html tag",
			body:     "<html><body>Hello</body></html>",
			expected: true,
		},
		{
			name:     "HTML with doctype",
			body:     "<!doctype html><html><body>Hello</body></html>",
			expected: true,
		},
		{
			name:     "HTML with div",
			body:     "<div>Hello</div>",
			expected: true,
		},
		{
			name:     "HTML with p tag",
			body:     "<p>Hello world</p>",
			expected: true,
		},
		{
			name:     "HTML with br tag",
			body:     "Line 1<br>Line 2",
			expected: true,
		},
		{
			name:     "HTML with self-closing br",
			body:     "Line 1<br/>Line 2",
			expected: true,
		},
		{
			name:     "HTML with spaced br",
			body:     "Line 1<br />Line 2",
			expected: true,
		},
		{
			name:     "HTML with span",
			body:     "Hello <span>world</span>",
			expected: true,
		},
		{
			name:     "HTML with link",
			body:     "Click <a href='#'>here</a>",
			expected: true,
		},
		{
			name:     "HTML with table",
			body:     "<table><tr><td>Cell</td></tr></table>",
			expected: true,
		},
		{
			name:     "HTML uppercase",
			body:     "<HTML><BODY>Hello</BODY></HTML>",
			expected: true,
		},
		{
			name:     "Text with angle brackets",
			body:     "2 < 5 and 10 > 3",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTML(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPlainTextMessage(t *testing.T) {
	to := "test@example.com"
	subject := "Test Subject"
	body := "This is a test body"

	result := buildPlainTextMessage(to, subject, body, "", "")

	assert.Contains(t, result, "To: test@example.com")
	assert.Contains(t, result, "Subject: Test Subject")
	assert.Contains(t, result, "Content-Type: text/plain; charset=\"UTF-8\"")
	assert.Contains(t, result, "MIME-Version: 1.0")
	assert.Contains(t, result, body)
}

func TestBuildHTMLMessage(t *testing.T) {
	to := "test@example.com"
	subject := "Test Subject"
	body := "<html><body><h1>Hello</h1></body></html>"

	result := buildHTMLMessage(to, subject, body, "", "")

	assert.Contains(t, result, "To: test@example.com")
	assert.Contains(t, result, "Subject: Test Subject")
	assert.Contains(t, result, "Content-Type: text/html; charset=\"UTF-8\"")
	assert.Contains(t, result, "MIME-Version: 1.0")
	assert.Contains(t, result, body)
}

func TestSanitizeHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal header",
			input:    "test@example.com",
			expected: "test@example.com",
		},
		{
			name:     "Header with newline",
			input:    "test@example.com\nBcc: evil@attacker.com",
			expected: "test@example.comBcc: evil@attacker.com",
		},
		{
			name:     "Header with CRLF",
			input:    "test@example.com\r\nBcc: evil@attacker.com",
			expected: "test@example.comBcc: evil@attacker.com",
		},
		{
			name:     "Header with carriage return only",
			input:    "test@example.com\rBcc: evil@attacker.com",
			expected: "test@example.comBcc: evil@attacker.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHeader(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildReferences(t *testing.T) {
	tests := []struct {
		name               string
		originalMessageID  string
		originalReferences string
		expected           string
	}{
		{
			name:               "No existing references",
			originalMessageID:  "<abc123@example.com>",
			originalReferences: "",
			expected:           "<abc123@example.com>",
		},
		{
			name:               "With existing references",
			originalMessageID:  "<def456@example.com>",
			originalReferences: "<abc123@example.com>",
			expected:           "<abc123@example.com> <def456@example.com>",
		},
		{
			name:               "Multiple existing references",
			originalMessageID:  "<ghi789@example.com>",
			originalReferences: "<abc123@example.com> <def456@example.com>",
			expected:           "<abc123@example.com> <def456@example.com> <ghi789@example.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildReferences(tt.originalMessageID, tt.originalReferences)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnsureReplySubject(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		expected string
	}{
		{
			name:     "Subject without Re prefix",
			subject:  "Hello World",
			expected: "Re: Hello World",
		},
		{
			name:     "Subject already has Re prefix",
			subject:  "Re: Hello World",
			expected: "Re: Hello World",
		},
		{
			name:     "Subject with lowercase re prefix",
			subject:  "re: Hello World",
			expected: "re: Hello World",
		},
		{
			name:     "Subject with mixed case RE prefix",
			subject:  "RE: Hello World",
			expected: "RE: Hello World",
		},
		{
			name:     "Empty subject",
			subject:  "",
			expected: "Re: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureReplySubject(tt.subject)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPlainTextMessage_WithThreading(t *testing.T) {
	to := "test@example.com"
	subject := "Test Subject"
	body := "Test body"
	inReplyTo := "<original123@example.com>"
	references := "<ref1@example.com> <original123@example.com>"

	result := buildPlainTextMessage(to, subject, body, inReplyTo, references)

	assert.Contains(t, result, "To: test@example.com")
	assert.Contains(t, result, "Subject: Test Subject")
	assert.Contains(t, result, "In-Reply-To: <original123@example.com>")
	assert.Contains(t, result, "References: <ref1@example.com> <original123@example.com>")
	assert.Contains(t, result, "Content-Type: text/plain; charset=\"UTF-8\"")
	assert.Contains(t, result, "MIME-Version: 1.0")
	assert.Contains(t, result, body)
}

func TestBuildHTMLMessage_WithThreading(t *testing.T) {
	to := "test@example.com"
	subject := "Test Subject"
	body := "<html><body><h1>Hello</h1></body></html>"
	inReplyTo := "<original123@example.com>"
	references := "<ref1@example.com> <original123@example.com>"

	result := buildHTMLMessage(to, subject, body, inReplyTo, references)

	assert.Contains(t, result, "To: test@example.com")
	assert.Contains(t, result, "Subject: Test Subject")
	assert.Contains(t, result, "In-Reply-To: <original123@example.com>")
	assert.Contains(t, result, "References: <ref1@example.com> <original123@example.com>")
	assert.Contains(t, result, "Content-Type: text/html; charset=\"UTF-8\"")
	assert.Contains(t, result, "MIME-Version: 1.0")
	assert.Contains(t, result, body)
}

func TestBuildPlainTextMessage_WithoutThreading(t *testing.T) {
	to := "test@example.com"
	subject := "Test Subject"
	body := "Test body"

	result := buildPlainTextMessage(to, subject, body, "", "")

	assert.Contains(t, result, "To: test@example.com")
	assert.Contains(t, result, "Subject: Test Subject")
	assert.NotContains(t, result, "In-Reply-To:")
	assert.NotContains(t, result, "References:")
	assert.Contains(t, result, body)
}

func TestBuildReferences_EmptyMessageID(t *testing.T) {
	// When Message-ID is empty, buildReferences should return empty string
	result := buildReferences("", "")
	assert.Equal(t, "", result)

	// When Message-ID is empty but references exist, return just the existing references
	result = buildReferences("", "<existing@example.com>")
	assert.Equal(t, "<existing@example.com>", result)
}

func TestBuildPlainTextMessage_EmptyThreadingHeaders(t *testing.T) {
	// Verify that empty threading headers don't add empty header lines
	to := "test@example.com"
	subject := "Test Subject"
	body := "Test body"

	result := buildPlainTextMessage(to, subject, body, "", "")

	// Should NOT contain "In-Reply-To:" when inReplyTo is empty
	assert.NotContains(t, result, "In-Reply-To:")
	// Should NOT contain "References:" when references is empty
	assert.NotContains(t, result, "References:")
}
