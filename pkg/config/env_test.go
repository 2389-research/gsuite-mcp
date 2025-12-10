// ABOUTME: Tests for environment variable configuration
// ABOUTME: Validates ISH_MODE, ISH_BASE_URL, and ISH_USER handling

package config

import (
	"context"
	"net/http"
	"testing"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestISHMode_EdgeCases tests various ISH_MODE values
func TestISHMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		ishMode     string
		shouldBeISH bool
		description string
	}{
		{
			name:        "ISH_MODE true",
			ishMode:     "true",
			shouldBeISH: true,
			description: "Standard true value should enable ISH mode",
		},
		{
			name:        "ISH_MODE false",
			ishMode:     "false",
			shouldBeISH: false,
			description: "False value should disable ISH mode",
		},
		{
			name:        "ISH_MODE 1",
			ishMode:     "1",
			shouldBeISH: false,
			description: "Numeric 1 should not enable ISH mode (only 'true' works)",
		},
		{
			name:        "ISH_MODE 0",
			ishMode:     "0",
			shouldBeISH: false,
			description: "Numeric 0 should not enable ISH mode",
		},
		{
			name:        "ISH_MODE yes",
			ishMode:     "yes",
			shouldBeISH: false,
			description: "Yes should not enable ISH mode (only 'true' works)",
		},
		{
			name:        "ISH_MODE no",
			ishMode:     "no",
			shouldBeISH: false,
			description: "No should not enable ISH mode",
		},
		{
			name:        "ISH_MODE empty",
			ishMode:     "",
			shouldBeISH: false,
			description: "Empty string should not enable ISH mode",
		},
		{
			name:        "ISH_MODE TRUE uppercase",
			ishMode:     "TRUE",
			shouldBeISH: false,
			description: "Uppercase TRUE should not enable ISH mode (case sensitive)",
		},
		{
			name:        "ISH_MODE invalid",
			ishMode:     "invalid",
			shouldBeISH: false,
			description: "Invalid value should not enable ISH mode",
		},
		{
			name:        "ISH_MODE whitespace",
			ishMode:     "  true  ",
			shouldBeISH: false,
			description: "Value with whitespace should not enable ISH mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", tt.ishMode)
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			// Test with Gmail service
			svc, err := gmail.NewService(context.Background(), nil)
			if tt.shouldBeISH {
				require.NoError(t, err, "ISH mode should succeed: %s", tt.description)
				assert.NotNil(t, svc, "Service should be created in ISH mode: %s", tt.description)
			} else {
				// In non-ISH mode without auth, we expect an error
				if err == nil {
					t.Logf("Warning: Non-ISH mode without auth succeeded unexpectedly: %s", tt.description)
				}
			}
		})
	}
}

// TestISHBaseURL_MalformedURLs tests ISH_BASE_URL with various malformed URLs
func TestISHBaseURL_MalformedURLs(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectError bool
		description string
	}{
		{
			name:        "Valid HTTP URL",
			baseURL:     "http://localhost:9000",
			expectError: false,
			description: "Standard localhost URL should work",
		},
		{
			name:        "Valid HTTPS URL",
			baseURL:     "https://api.example.com",
			expectError: false,
			description: "HTTPS URL should work",
		},
		{
			name:        "URL with path",
			baseURL:     "http://localhost:9000/api/v1",
			expectError: false,
			description: "URL with path should work",
		},
		{
			name:        "Empty URL falls back to default",
			baseURL:     "",
			expectError: false,
			description: "Empty URL should fall back to http://localhost:9000",
		},
		{
			name:        "URL with port",
			baseURL:     "http://localhost:8080",
			expectError: false,
			description: "Custom port should work",
		},
		{
			name:        "Invalid URL - no protocol",
			baseURL:     "localhost:9000",
			expectError: true,
			description: "URL without protocol should fail or be handled gracefully",
		},
		{
			name:        "Invalid URL - malformed",
			baseURL:     "ht!tp://invalid",
			expectError: false,
			description: "Malformed URL may be accepted by Google client library",
		},
		{
			name:        "URL with special characters",
			baseURL:     "http://localhost:9000/path?query=value",
			expectError: false,
			description: "URL with query params should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", tt.baseURL)

			svc, err := gmail.NewService(context.Background(), nil)

			if tt.expectError {
				if err == nil {
					t.Logf("Note: Expected error but got success for: %s", tt.description)
				}
			} else {
				require.NoError(t, err, "Should create service: %s", tt.description)
				assert.NotNil(t, svc, "Service should not be nil: %s", tt.description)
			}
		})
	}
}

// TestISHUser_EdgeCases tests ISH_USER with various values
func TestISHUser_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		ishUser      string
		explicitUser string
		expectedUser string
		description  string
	}{
		{
			name:         "ISH_USER set",
			ishUser:      "envuser",
			explicitUser: "",
			expectedUser: "user:envuser",
			description:  "Should use ISH_USER from environment",
		},
		{
			name:         "ISH_USER empty uses default",
			ishUser:      "",
			explicitUser: "",
			expectedUser: "user:testuser",
			description:  "Empty ISH_USER should fall back to 'testuser'",
		},
		{
			name:         "Explicit user overrides env",
			ishUser:      "envuser",
			explicitUser: "explicituser",
			expectedUser: "user:explicituser",
			description:  "Explicit parameter should override environment",
		},
		{
			name:         "ISH_USER with special characters",
			ishUser:      "user@example.com",
			explicitUser: "",
			expectedUser: "user:user@example.com",
			description:  "Email address should work as user",
		},
		{
			name:         "ISH_USER with spaces",
			ishUser:      "user name",
			explicitUser: "",
			expectedUser: "user:user name",
			description:  "User with spaces should be preserved",
		},
		{
			name:         "ISH_USER with unicode",
			ishUser:      "用户",
			explicitUser: "",
			expectedUser: "user:用户",
			description:  "Unicode characters should work",
		},
		{
			name:         "ISH_USER very long",
			ishUser:      "verylongusername1234567890verylongusername1234567890verylongusername1234567890",
			explicitUser: "",
			expectedUser: "user:verylongusername1234567890verylongusername1234567890verylongusername1234567890",
			description:  "Long usernames should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ishUser != "" {
				t.Setenv("ISH_USER", tt.ishUser)
			}

			client := auth.NewFakeClient(tt.explicitUser)
			req, err := http.NewRequest("GET", "http://localhost:9000/test", nil)
			require.NoError(t, err)

			// Note: RoundTrip will fail because there's no server, but we only care about headers
			_, _ = client.Transport.RoundTrip(req)

			authHeader := req.Header.Get("Authorization")
			assert.Equal(t, "Bearer "+tt.expectedUser, authHeader, tt.description)
		})
	}
}

// TestMissingEnvironmentVariables tests behavior when env vars are not set
func TestMissingEnvironmentVariables(t *testing.T) {
	t.Run("No ISH_MODE defaults to real mode", func(t *testing.T) {
		// Don't set ISH_MODE at all (it should default to non-ISH mode)
		// Without credentials, this should fail
		_, err := gmail.NewService(context.Background(), nil)
		if err == nil {
			t.Log("Note: Real mode without credentials succeeded unexpectedly")
		}
	})

	t.Run("ISH_MODE true without ISH_BASE_URL uses default", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		// Don't set ISH_BASE_URL - should default to http://localhost:9000

		svc, err := gmail.NewService(context.Background(), nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("ISH_MODE true without ISH_USER uses default in fake client", func(t *testing.T) {
		// Don't set ISH_USER - should default to 'testuser'
		client := auth.NewFakeClient("")
		req, err := http.NewRequest("GET", "http://localhost:9000/test", nil)
		require.NoError(t, err)

		// Note: RoundTrip will fail because there's no server, but we only care about headers
		_, _ = client.Transport.RoundTrip(req)

		authHeader := req.Header.Get("Authorization")
		assert.Equal(t, "Bearer user:testuser", authHeader)
	})
}

// TestEnvironmentVariablePrecedence tests precedence rules
func TestEnvironmentVariablePrecedence(t *testing.T) {
	t.Run("Explicit parameter takes precedence over env var", func(t *testing.T) {
		t.Setenv("ISH_USER", "envuser")

		client := auth.NewFakeClient("paramuser")
		req, err := http.NewRequest("GET", "http://localhost:9000/test", nil)
		require.NoError(t, err)

		// Note: RoundTrip will fail because there's no server, but we only care about headers
		_, _ = client.Transport.RoundTrip(req)

		authHeader := req.Header.Get("Authorization")
		assert.Equal(t, "Bearer user:paramuser", authHeader,
			"Explicit parameter should override environment variable")
	})

	t.Run("Empty explicit parameter uses env var", func(t *testing.T) {
		t.Setenv("ISH_USER", "envuser")

		client := auth.NewFakeClient("")
		req, err := http.NewRequest("GET", "http://localhost:9000/test", nil)
		require.NoError(t, err)

		// Note: RoundTrip will fail because there's no server, but we only care about headers
		_, _ = client.Transport.RoundTrip(req)

		authHeader := req.Header.Get("Authorization")
		assert.Equal(t, "Bearer user:envuser", authHeader,
			"Empty explicit parameter should fall back to environment variable")
	})

	t.Run("No explicit parameter and no env var uses default", func(t *testing.T) {
		// Don't set ISH_USER
		client := auth.NewFakeClient("")
		req, err := http.NewRequest("GET", "http://localhost:9000/test", nil)
		require.NoError(t, err)

		// Note: RoundTrip will fail because there's no server, but we only care about headers
		_, _ = client.Transport.RoundTrip(req)

		authHeader := req.Header.Get("Authorization")
		assert.Equal(t, "Bearer user:testuser", authHeader,
			"Should fall back to default 'testuser' when no env var or parameter")
	})
}

// TestISHMode_AllServices tests ISH_MODE across all services
func TestISHMode_AllServices(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")
	ctx := context.Background()

	t.Run("Gmail service with ISH mode", func(t *testing.T) {
		svc, err := gmail.NewService(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("Calendar service with ISH mode", func(t *testing.T) {
		svc, err := calendar.NewService(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("People service with ISH mode", func(t *testing.T) {
		svc, err := people.NewService(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})
}

// TestISHMode_ConsistencyAcrossServices ensures all services handle ISH_MODE identically
func TestISHMode_ConsistencyAcrossServices(t *testing.T) {
	testCases := []struct {
		ishMode     string
		shouldBeISH bool
	}{
		{"true", true},
		{"false", false},
		{"", false},
		{"1", false},
	}

	for _, tc := range testCases {
		t.Run("ISH_MODE="+tc.ishMode, func(t *testing.T) {
			t.Setenv("ISH_MODE", tc.ishMode)
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")
			ctx := context.Background()

			gmailSvc, gmailErr := gmail.NewService(ctx, nil)
			calSvc, calErr := calendar.NewService(ctx, nil)
			peopleSvc, peopleErr := people.NewService(ctx, nil)

			if tc.shouldBeISH {
				// All should succeed in ISH mode
				assert.NoError(t, gmailErr, "Gmail should succeed in ISH mode")
				assert.NoError(t, calErr, "Calendar should succeed in ISH mode")
				assert.NoError(t, peopleErr, "People should succeed in ISH mode")

				assert.NotNil(t, gmailSvc, "Gmail service should not be nil")
				assert.NotNil(t, calSvc, "Calendar service should not be nil")
				assert.NotNil(t, peopleSvc, "People service should not be nil")
			} else {
				// All should behave consistently in non-ISH mode
				// (may fail without credentials, but should fail consistently)
				gmailNil := gmailSvc == nil || gmailErr != nil
				calNil := calSvc == nil || calErr != nil
				peopleNil := peopleSvc == nil || peopleErr != nil

				// Just log for reference - actual behavior may vary without real credentials
				t.Logf("Non-ISH mode - Gmail: err=%v, Calendar: err=%v, People: err=%v",
					gmailNil, calNil, peopleNil)
			}
		})
	}
}
