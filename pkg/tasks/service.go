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
	items := result.Items
	if items == nil {
		items = []*tasks.TaskList{}
	}
	return items, nil
}

// CreateTaskList creates a new task list
func (s *Service) CreateTaskList(ctx context.Context, title string) (*tasks.TaskList, error) {
	// Task list creates are not retried: a retry after a request that succeeded but
	// whose response was lost would create a duplicate task list.
	result, err := s.svc.Tasklists.Insert(&tasks.TaskList{
		Title: title,
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create task list: %w", err)
	}
	return result, nil
}

// UpdateTaskList updates a task list's title
func (s *Service) UpdateTaskList(ctx context.Context, tasklistID, title string) (*tasks.TaskList, error) {
	var result *tasks.TaskList
	err := retry.WithRetry(func() error {
		var err error
		result, err = s.svc.Tasklists.Patch(tasklistID, &tasks.TaskList{
			Title: title,
		}).Context(ctx).Do()
		return err
	}, 3, time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to update task list: %w", err)
	}
	return result, nil
}

// DeleteTaskList deletes a task list
func (s *Service) DeleteTaskList(ctx context.Context, tasklistID string) error {
	return retry.WithRetry(func() error {
		return s.svc.Tasklists.Delete(tasklistID).Context(ctx).Do()
	}, 3, time.Second)
}

// ListTasks returns tasks from a task list
func (s *Service) ListTasks(ctx context.Context, tasklistID string, showCompleted, showHidden bool, dueMin, dueMax string) ([]*tasks.Task, error) {
	if tasklistID == "" {
		tasklistID = "@default"
	}

	var result *tasks.Tasks
	err := retry.WithRetry(func() error {
		call := s.svc.Tasks.List(tasklistID).Context(ctx).MaxResults(100)
		call = call.ShowCompleted(showCompleted)
		call = call.ShowHidden(showHidden)
		if dueMin != "" {
			call = call.DueMin(dueMin)
		}
		if dueMax != "" {
			call = call.DueMax(dueMax)
		}
		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	items := result.Items
	if items == nil {
		items = []*tasks.Task{}
	}
	return items, nil
}

// CreateTask creates a task in a task list
func (s *Service) CreateTask(ctx context.Context, tasklistID string, task *tasks.Task, parent string) (*tasks.Task, error) {
	if tasklistID == "" {
		tasklistID = "@default"
	}

	// Task creates are not retried: a retry after a request that succeeded but
	// whose response was lost would create a duplicate task.
	call := s.svc.Tasks.Insert(tasklistID, task).Context(ctx)
	if parent != "" {
		call = call.Parent(parent)
	}
	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return result, nil
}

// UpdateTask updates a task
func (s *Service) UpdateTask(ctx context.Context, tasklistID, taskID string, task *tasks.Task) (*tasks.Task, error) {
	if tasklistID == "" {
		tasklistID = "@default"
	}

	var result *tasks.Task
	err := retry.WithRetry(func() error {
		var err error
		result, err = s.svc.Tasks.Patch(tasklistID, taskID, task).Context(ctx).Do()
		return err
	}, 3, time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}
	return result, nil
}

// DeleteTask deletes a task
func (s *Service) DeleteTask(ctx context.Context, tasklistID, taskID string) error {
	if tasklistID == "" {
		tasklistID = "@default"
	}

	return retry.WithRetry(func() error {
		return s.svc.Tasks.Delete(tasklistID, taskID).Context(ctx).Do()
	}, 3, time.Second)
}
