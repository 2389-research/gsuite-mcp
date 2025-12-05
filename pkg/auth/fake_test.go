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
