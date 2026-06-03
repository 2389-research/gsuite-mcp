// ABOUTME: Verifies non-idempotent Gmail operations are not retried.
// ABOUTME: A send that hit a 503 must reach the server exactly once, never duplicated.

package gmail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/harper/gsuite-mcp/pkg/auth"
)

func TestSendMessageNotRetriedOn503(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"unavailable"}}`))
	}))
	defer srv.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", srv.URL)

	ctx := context.Background()
	svc, err := NewService(ctx, auth.NewFakeClient("test@example.com"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, _ = svc.SendMessage(ctx, "to@example.com", "subj", "body", "", "", "")

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("send must hit the server exactly once (no retry), got %d", got)
	}
}
