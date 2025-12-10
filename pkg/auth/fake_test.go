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
	_, _ = client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:testuser", req.Header.Get("Authorization"))
}

func TestFakeClient_FromEnv(t *testing.T) {
	t.Setenv("ISH_USER", "envuser")

	client := NewFakeClient("")
	req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
	_, _ = client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:envuser", req.Header.Get("Authorization"))
}

func TestFakeClient_DefaultUser(t *testing.T) {
	// Don't set ISH_USER - should default to "testuser"
	client := NewFakeClient("")
	req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
	_, _ = client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:testuser", req.Header.Get("Authorization"))
}

func TestFakeClient_ExplicitOverridesEnv(t *testing.T) {
	t.Setenv("ISH_USER", "envuser")

	// Explicit parameter should override environment
	client := NewFakeClient("explicituser")
	req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
	_, _ = client.Transport.(*fakeTransport).RoundTrip(req)

	assert.Equal(t, "Bearer user:explicituser", req.Header.Get("Authorization"))
}

func TestFakeClient_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name         string
		user         string
		expectedAuth string
	}{
		{
			name:         "Email as user",
			user:         "user@example.com",
			expectedAuth: "Bearer user:user@example.com",
		},
		{
			name:         "User with spaces",
			user:         "John Doe",
			expectedAuth: "Bearer user:John Doe",
		},
		{
			name:         "User with unicode",
			user:         "用户123",
			expectedAuth: "Bearer user:用户123",
		},
		{
			name:         "User with special chars",
			user:         "user+tag@example.com",
			expectedAuth: "Bearer user:user+tag@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFakeClient(tt.user)
			req, _ := http.NewRequest("GET", "http://localhost:9000/test", nil)
			_, _ = client.Transport.(*fakeTransport).RoundTrip(req)

			assert.Equal(t, tt.expectedAuth, req.Header.Get("Authorization"))
		})
	}
}
