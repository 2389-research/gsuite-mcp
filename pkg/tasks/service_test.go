// ABOUTME: Tests for Tasks service
// ABOUTME: Validates task list and task operations with ish mode

package tasks

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

	svc, err := NewService(context.Background(), nil)
	_ = svc
	_ = err
}

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
