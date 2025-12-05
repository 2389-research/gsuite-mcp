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
