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
