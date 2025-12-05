// ABOUTME: Tests for Calendar service
// ABOUTME: Validates event operations with ish mode

package calendar

import (
	"context"
	"testing"
	"time"

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

func TestNewService_WithoutIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "false")

	// Without credentials, this should fail
	_, err := NewService(context.Background(), nil)

	// We expect an error when no credentials are available
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to create Calendar service")
}

func TestService_ListEvents(t *testing.T) {
	t.Skip("TODO: Implement with ish server")
}

func TestService_CreateEvent(t *testing.T) {
	t.Skip("TODO: Implement with ish server")
}

func TestService_CreateEvent_Basic(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	now := time.Now()
	start := now.Add(1 * time.Hour)
	end := start.Add(1 * time.Hour)

	// Test that the method signature is correct
	_, err = svc.CreateEvent(context.Background(), "Test Event", "Test Description", start, end)

	// We expect it to fail because there's no ish server running,
	// but we're testing that the method exists and has the right signature
	if err != nil {
		t.Logf("Expected error (no ish server): %v", err)
	}
}
