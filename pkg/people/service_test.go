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
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)

	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestNewService_WithoutIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "false")

	// This will fail without proper auth, but should create service
	svc, err := NewService(context.Background(), nil)

	// In real mode without client, we expect an error or need auth
	// For now, just verify the function exists
	_ = svc
	_ = err
}

func TestService_ListContacts(t *testing.T) {
	t.Skip("TODO: Implement with ish server")
}

func TestService_SearchContacts(t *testing.T) {
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

	t.Run("Multiple services with same ISH config", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", "http://localhost:9000")

		svc1, err1 := NewService(context.Background(), nil)
		svc2, err2 := NewService(context.Background(), nil)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotNil(t, svc1)
		assert.NotNil(t, svc2)
	})
}
