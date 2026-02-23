// ABOUTME: Tasks API service for task and task list management
// ABOUTME: Handles CRUD operations for Google Tasks

package tasks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/harper/gsuite-mcp/pkg/retry"
	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

// Service wraps Tasks API operations
type Service struct {
	svc *tasks.Service
}

// NewService creates a new Tasks service
func NewService(ctx context.Context, client *http.Client) (*Service, error) {
	opts := []option.ClientOption{}

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		baseURL := os.Getenv("ISH_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
		opts = append(opts, option.WithEndpoint(baseURL))
		opts = append(opts, option.WithoutAuthentication())
	}

	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}

	svc, err := tasks.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Tasks service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListTaskLists returns all task lists for the authenticated user
func (s *Service) ListTaskLists(ctx context.Context) ([]*tasks.TaskList, error) {
	var result *tasks.TaskLists
	err := retry.WithRetry(func() error {
		var err error
		result, err = s.svc.Tasklists.List().Context(ctx).MaxResults(100).Do()
		return err
	}, 3, time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}
	return result.Items, nil
}
