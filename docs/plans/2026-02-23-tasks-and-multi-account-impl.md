# Google Tasks & Multi-Account Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Google Tasks CRUD support (8 tools, 3 resources, 2 prompts), then retrofit multi-account support across all services.

**Architecture:** Phase 1 adds `pkg/tasks/` following the existing service pattern (like `pkg/people/`). Phase 2 adds `pkg/accounts/` for account registry, refactors `pkg/server/` to resolve services per-account using an optional `account` parameter on every tool.

**Tech Stack:** Go 1.24, `google.golang.org/api/tasks/v1`, `github.com/mark3labs/mcp-go v0.43.2`

**Design doc:** `docs/plans/2026-02-23-tasks-and-multi-account-design.md`

---

## Phase 1: Google Tasks

### Task 1: Add Tasks API dependency and OAuth scope

**Files:**
- Modify: `pkg/auth/oauth.go:25-31` (add tasks scope)
- Modify: `go.mod` (add tasks dependency)

**Step 1: Add the tasks import and scope**

In `pkg/auth/oauth.go`, add the tasks import and scope:

```go
import (
	// ... existing imports ...
	"google.golang.org/api/tasks/v1"
)

var DefaultScopes = []string{
	gmail.GmailModifyScope,
	gmail.GmailLabelsScope,
	calendar.CalendarScope,
	people.ContactsScope,
	tasks.TasksScope,
}
```

**Step 2: Fetch the dependency**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go get google.golang.org/api/tasks/v1`

**Step 3: Run existing tests to verify nothing breaks**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/auth/...`
Expected: All PASS

**Step 4: Commit**

```bash
git add pkg/auth/oauth.go go.mod go.sum
git commit -m "feat: add Google Tasks OAuth scope"
```

---

### Task 2: Create Tasks service - NewService and ListTaskLists

**Files:**
- Create: `pkg/tasks/service.go`
- Create: `pkg/tasks/service_test.go`

**Step 1: Write the failing test**

Create `pkg/tasks/service_test.go`:

```go
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
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/tasks/...`
Expected: FAIL (package doesn't exist)

**Step 3: Write minimal implementation**

Create `pkg/tasks/service.go`:

```go
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
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/tasks/...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/tasks/service.go pkg/tasks/service_test.go
git commit -m "feat: add Tasks service with ListTaskLists"
```

---

### Task 3: Add remaining TaskList CRUD methods

**Files:**
- Modify: `pkg/tasks/service.go`

**Step 1: Add CreateTaskList, UpdateTaskList, DeleteTaskList**

Append to `pkg/tasks/service.go`:

```go
// CreateTaskList creates a new task list
func (s *Service) CreateTaskList(ctx context.Context, title string) (*tasks.TaskList, error) {
	var result *tasks.TaskList
	err := retry.WithRetry(func() error {
		var err error
		result, err = s.svc.Tasklists.Insert(&tasks.TaskList{
			Title: title,
		}).Context(ctx).Do()
		return err
	}, 3, time.Second)
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
```

**Step 2: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/tasks/...`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/tasks/service.go
git commit -m "feat: add TaskList CRUD methods"
```

---

### Task 4: Add Task CRUD methods

**Files:**
- Modify: `pkg/tasks/service.go`

**Step 1: Add ListTasks, CreateTask, UpdateTask, DeleteTask**

Append to `pkg/tasks/service.go`:

```go
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
	return result.Items, nil
}

// CreateTask creates a task in a task list
func (s *Service) CreateTask(ctx context.Context, tasklistID string, task *tasks.Task, parent string) (*tasks.Task, error) {
	if tasklistID == "" {
		tasklistID = "@default"
	}

	var result *tasks.Task
	err := retry.WithRetry(func() error {
		call := s.svc.Tasks.Insert(tasklistID, task).Context(ctx)
		if parent != "" {
			call = call.Parent(parent)
		}
		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)
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
```

**Step 2: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/tasks/...`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/tasks/service.go
git commit -m "feat: add Task CRUD methods"
```

---

### Task 5: Register Tasks tools in MCP server

**Files:**
- Modify: `pkg/server/server.go:6-25` (add import)
- Modify: `pkg/server/server.go:27-34` (add tasks field to Server struct)
- Modify: `pkg/server/server.go:36-98` (create tasks service in NewServer)
- Modify: `pkg/server/server.go:101-464` (register tasks tools in registerTools)

**Step 1: Write the failing test**

Add to `pkg/server/server_test.go` after existing tool name assertions:

```go
func TestServer_HasTasksTools(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	// Tasks tools
	assert.True(t, toolNames["tasks_list_tasklists"])
	assert.True(t, toolNames["tasks_create_tasklist"])
	assert.True(t, toolNames["tasks_update_tasklist"])
	assert.True(t, toolNames["tasks_delete_tasklist"])
	assert.True(t, toolNames["tasks_list_tasks"])
	assert.True(t, toolNames["tasks_create_task"])
	assert.True(t, toolNames["tasks_update_task"])
	assert.True(t, toolNames["tasks_delete_task"])
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/server/... -run TestServer_HasTasksTools`
Expected: FAIL (tools not registered)

**Step 3: Implement - add tasks to server**

In `pkg/server/server.go`:

1. Add import: `"github.com/harper/gsuite-mcp/pkg/tasks"`
2. Add field to Server struct: `tasks *tasks.Service`
3. In `NewServer`, after people service creation (line ~77), add:

```go
	tasksSvc, err := tasks.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tasks service: %w", err)
	}
```

4. Add `tasks: tasksSvc` to Server struct init (line ~83)
5. In `registerTools()`, before the auth tools section (line ~411), add:

```go
	// Tasks tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_list_tasklists",
		Description: "List all task lists",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleTasksListTaskLists)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_create_tasklist",
		Description: "Create a new task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"title": map[string]string{"type": "string", "description": "Title of the task list"},
			},
			Required: []string{"title"},
		},
	}, s.handleTasksCreateTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_update_tasklist",
		Description: "Update a task list's title",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list"},
				"title":       map[string]string{"type": "string", "description": "New title for the task list"},
			},
			Required: []string{"tasklist_id", "title"},
		},
	}, s.handleTasksUpdateTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_delete_tasklist",
		Description: "Delete a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list to delete"},
			},
			Required: []string{"tasklist_id"},
		},
	}, s.handleTasksDeleteTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_list_tasks",
		Description: "List tasks in a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id":    map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"show_completed": map[string]interface{}{"type": "boolean", "description": "Include completed tasks (default: false)"},
				"show_hidden":    map[string]interface{}{"type": "boolean", "description": "Include hidden tasks (default: false)"},
				"due_min":        map[string]string{"type": "string", "description": "Minimum due date (RFC 3339, e.g., 2026-01-01T00:00:00Z)"},
				"due_max":        map[string]string{"type": "string", "description": "Maximum due date (RFC 3339, e.g., 2026-12-31T23:59:59Z)"},
			},
		},
	}, s.handleTasksListTasks)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_create_task",
		Description: "Create a task in a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"title":       map[string]string{"type": "string", "description": "Title of the task"},
				"notes":       map[string]string{"type": "string", "description": "Additional notes for the task"},
				"due":         map[string]string{"type": "string", "description": "Due date (RFC 3339, e.g., 2026-03-01T00:00:00Z)"},
				"parent":      map[string]string{"type": "string", "description": "Parent task ID for creating a subtask"},
			},
			Required: []string{"title"},
		},
	}, s.handleTasksCreateTask)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_update_task",
		Description: "Update a task's title, notes, due date, or status",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"task_id":     map[string]string{"type": "string", "description": "ID of the task to update"},
				"title":       map[string]string{"type": "string", "description": "New title"},
				"notes":       map[string]string{"type": "string", "description": "New notes"},
				"due":         map[string]string{"type": "string", "description": "New due date (RFC 3339)"},
				"status":      map[string]string{"type": "string", "description": "Task status: 'needsAction' or 'completed'"},
			},
			Required: []string{"task_id"},
		},
	}, s.handleTasksUpdateTask)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_delete_task",
		Description: "Delete a task",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"task_id":     map[string]string{"type": "string", "description": "ID of the task to delete"},
			},
			Required: []string{"task_id"},
		},
	}, s.handleTasksDeleteTask)
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/server/... -run TestServer_HasTasksTools`
Expected: FAIL (handlers don't exist yet - that's Task 6)

---

### Task 6: Implement Tasks tool handlers

**Files:**
- Modify: `pkg/server/server.go` (add handler functions and response types)

**Step 1: Add response types**

Add near the existing response types (after line ~494):

```go
// ListTaskListsResponse wraps task lists for MCP
type ListTaskListsResponse struct {
	TaskLists []*googletasks.TaskList `json:"task_lists"`
	Count     int                     `json:"count"`
}

// ListTasksResponse wraps tasks for MCP
type ListTasksResponse struct {
	Tasks []*googletasks.Task `json:"tasks"`
	Count int                 `json:"count"`
}
```

(Add `googletasks "google.golang.org/api/tasks/v1"` to the imports at the top)

**Step 2: Add handler functions**

```go
func (s *Server) handleTasksListTaskLists(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskLists, err := s.tasks.ListTaskLists(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(ListTaskListsResponse{
		TaskLists: taskLists,
		Count:     len(taskLists),
	})
}

func (s *Server) handleTasksCreateTaskList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, _ := req.RequireString("title")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}
	taskList, err := s.tasks.CreateTaskList(ctx, title)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(taskList)
}

func (s *Server) handleTasksUpdateTaskList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasklistID, _ := req.RequireString("tasklist_id")
	if tasklistID == "" {
		return mcp.NewToolResultError("tasklist_id is required"), nil
	}
	title, _ := req.RequireString("title")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}
	taskList, err := s.tasks.UpdateTaskList(ctx, tasklistID, title)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(taskList)
}

func (s *Server) handleTasksDeleteTaskList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasklistID, _ := req.RequireString("tasklist_id")
	if tasklistID == "" {
		return mcp.NewToolResultError("tasklist_id is required"), nil
	}
	err := s.tasks.DeleteTaskList(ctx, tasklistID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "deleted", "tasklist_id": tasklistID})
}

func (s *Server) handleTasksListTasks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasklistID := req.GetString("tasklist_id", "@default")
	showCompleted := req.GetBool("show_completed", false)
	showHidden := req.GetBool("show_hidden", false)
	dueMin := req.GetString("due_min", "")
	dueMax := req.GetString("due_max", "")

	tasks, err := s.tasks.ListTasks(ctx, tasklistID, showCompleted, showHidden, dueMin, dueMax)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(ListTasksResponse{
		Tasks: tasks,
		Count: len(tasks),
	})
}

func (s *Server) handleTasksCreateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, _ := req.RequireString("title")
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	tasklistID := req.GetString("tasklist_id", "@default")
	notes := req.GetString("notes", "")
	due := req.GetString("due", "")
	parent := req.GetString("parent", "")

	task := &googletasks.Task{
		Title: title,
		Notes: notes,
	}
	if due != "" {
		task.Due = due
	}

	result, err := s.tasks.CreateTask(ctx, tasklistID, task, parent)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(result)
}

func (s *Server) handleTasksUpdateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := req.RequireString("task_id")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	tasklistID := req.GetString("tasklist_id", "@default")

	task := &googletasks.Task{}
	if title := req.GetString("title", ""); title != "" {
		task.Title = title
	}
	if notes := req.GetString("notes", ""); notes != "" {
		task.Notes = notes
	}
	if due := req.GetString("due", ""); due != "" {
		task.Due = due
	}
	if status := req.GetString("status", ""); status != "" {
		task.Status = status
	}

	result, err := s.tasks.UpdateTask(ctx, tasklistID, taskID, task)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(result)
}

func (s *Server) handleTasksDeleteTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := req.RequireString("task_id")
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}
	tasklistID := req.GetString("tasklist_id", "@default")

	err := s.tasks.DeleteTask(ctx, tasklistID, taskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "deleted", "task_id": taskID})
}
```

**Step 3: Run all tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/server/server.go pkg/server/server_test.go
git commit -m "feat: register Tasks tools and implement handlers"
```

---

### Task 7: Add Tasks resources

**Files:**
- Modify: `pkg/server/server.go` (add to registerResources)

**Step 1: Find registerResources and add tasks resources**

Locate the `registerResources()` function in `pkg/server/server.go`. Add after the existing resources:

```go
	// Tasks resources
	s.mcp.AddResource(mcp.Resource{
		URI:         "gsuite://tasks/today",
		Name:        "Today's Tasks",
		Description: "Tasks due today",
		MIMEType:    "application/json",
	}, s.handleTasksTodayResource)

	s.mcp.AddResource(mcp.Resource{
		URI:         "gsuite://tasks/upcoming",
		Name:        "Upcoming Tasks",
		Description: "Tasks due in the next 7 days",
		MIMEType:    "application/json",
	}, s.handleTasksUpcomingResource)

	s.mcp.AddResource(mcp.Resource{
		URI:         "gsuite://tasks/overdue",
		Name:        "Overdue Tasks",
		Description: "Overdue incomplete tasks",
		MIMEType:    "application/json",
	}, s.handleTasksOverdueResource)
```

**Step 2: Implement resource handlers**

```go
func (s *Server) handleTasksTodayResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Format(time.RFC3339)

	tasks, err := s.tasks.ListTasks(ctx, "@default", false, false, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	return resourcesToJSON(req.Params.URI, ListTasksResponse{Tasks: tasks, Count: len(tasks)})
}

func (s *Server) handleTasksUpcomingResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)
	endOfWeek := now.AddDate(0, 0, 7).Format(time.RFC3339)

	tasks, err := s.tasks.ListTasks(ctx, "@default", false, false, startOfDay, endOfWeek)
	if err != nil {
		return nil, err
	}
	return resourcesToJSON(req.Params.URI, ListTasksResponse{Tasks: tasks, Count: len(tasks)})
}

func (s *Server) handleTasksOverdueResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	pastDate := "1970-01-01T00:00:00Z"
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format(time.RFC3339)

	tasks, err := s.tasks.ListTasks(ctx, "@default", false, false, pastDate, startOfDay)
	if err != nil {
		return nil, err
	}
	return resourcesToJSON(req.Params.URI, ListTasksResponse{Tasks: tasks, Count: len(tasks)})
}
```

Note: Check how existing resources serialize JSON - there may be a helper like `resourcesToJSON`. If not, follow the exact pattern used by existing resource handlers (likely `mcp.NewResourceContents` or similar).

**Step 3: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/server/server.go
git commit -m "feat: add Tasks resources (today, upcoming, overdue)"
```

---

### Task 8: Add Tasks prompts

**Files:**
- Modify: `pkg/server/server.go` (add to registerPrompts)

**Step 1: Find registerPrompts and add tasks prompts**

```go
	// Tasks prompts
	s.mcp.AddPrompt(mcp.Prompt{
		Name:        "task_review",
		Description: "Review and organize pending tasks across all task lists",
		Arguments: []mcp.PromptArgument{
			{Name: "focus", Description: "Area to focus on (e.g., 'overdue', 'today', 'this week')", Required: false},
		},
	}, s.handleTaskReviewPrompt)

	s.mcp.AddPrompt(mcp.Prompt{
		Name:        "plan_tasks",
		Description: "Break down a goal into actionable tasks",
		Arguments: []mcp.PromptArgument{
			{Name: "goal", Description: "The goal to break down into tasks", Required: true},
			{Name: "tasklist", Description: "Task list to add tasks to (default: primary list)", Required: false},
		},
	}, s.handlePlanTasksPrompt)
```

**Step 2: Implement prompt handlers**

```go
func (s *Server) handleTaskReviewPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	focus := req.Params.Arguments["focus"]
	if focus == "" {
		focus = "all pending"
	}
	return &mcp.GetPromptResult{
		Description: "Review and organize tasks",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Please review my %s tasks. Use tasks_list_tasklists to see all my lists, then tasks_list_tasks to check each list. Summarize what's overdue, what's due today, and what's coming up. Suggest which tasks to prioritize.", focus),
				},
			},
		},
	}, nil
}

func (s *Server) handlePlanTasksPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	goal := req.Params.Arguments["goal"]
	tasklist := req.Params.Arguments["tasklist"]
	tasklistNote := ""
	if tasklist != "" {
		tasklistNote = fmt.Sprintf(" Add them to the '%s' task list.", tasklist)
	}
	return &mcp.GetPromptResult{
		Description: "Plan tasks for a goal",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Help me break down this goal into actionable tasks: %s\n\nCreate specific, concrete tasks with clear titles and due dates where appropriate. Use tasks_create_task to add each one.%s", goal, tasklistNote),
				},
			},
		},
	}, nil
}
```

**Step 3: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/server/server.go
git commit -m "feat: add Tasks prompts (task_review, plan_tasks)"
```

---

### Task 9: Update ABOUTME comments and docs

**Files:**
- Modify: `pkg/server/server.go:1-2` (update ABOUTME)
- Modify: `README.md` (add Tasks section)

**Step 1: Update server ABOUTME**

Change line 2 of `pkg/server/server.go` from:
```
// ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools
```
to:
```
// ABOUTME: Exposes Gmail, Calendar, People, and Tasks services as MCP tools
```

**Step 2: Add Tasks section to README.md**

Add a Tasks section following the existing pattern for Gmail/Calendar/People.

**Step 3: Run full test suite**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/server/server.go README.md
git commit -m "docs: add Tasks to ABOUTME and README"
```

---

## Phase 2: Multi-Account

### Task 10: Create accounts package - Registry and config loading

**Files:**
- Create: `pkg/accounts/registry.go`
- Create: `pkg/accounts/registry_test.go`

**Step 1: Write the failing test**

Create `pkg/accounts/registry_test.go`:

```go
// ABOUTME: Tests for account registry
// ABOUTME: Validates config loading, account resolution, and fallback behavior

package accounts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRegistry_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")
	os.WriteFile(configPath, []byte(`{
		"default": "work",
		"accounts": {
			"work": {"token_path": "/tmp/work.json"},
			"personal": {"token_path": "/tmp/personal.json"}
		}
	}`), 0600)

	reg, err := LoadRegistry(configPath)
	require.NoError(t, err)
	assert.Equal(t, "work", reg.DefaultAlias)
	assert.Len(t, reg.Accounts, 2)
}

func TestLoadRegistry_FileNotFound(t *testing.T) {
	reg, err := LoadRegistry("/nonexistent/accounts.json")
	require.NoError(t, err)
	assert.Equal(t, "default", reg.DefaultAlias)
	assert.Len(t, reg.Accounts, 1)
}

func TestRegistry_Resolve_Default(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	acct, err := reg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "work", acct.Alias)
}

func TestRegistry_Resolve_ByAlias(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	acct, err := reg.Resolve("personal")
	require.NoError(t, err)
	assert.Equal(t, "personal", acct.Alias)
}

func TestRegistry_Resolve_Unknown(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work": {Alias: "work", TokenPath: "/tmp/work.json"},
		},
	}

	_, err := reg.Resolve("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown account")
	assert.Contains(t, err.Error(), "work")
}

func TestRegistry_ListAccounts(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	aliases := reg.ListAccounts()
	assert.Len(t, aliases, 2)
	assert.Contains(t, aliases, "work")
	assert.Contains(t, aliases, "personal")
}

func TestRegistry_AddAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg := &Registry{
		configPath:   configPath,
		DefaultAlias: "default",
		Accounts:     map[string]*Account{},
	}

	err := reg.AddAccount("work", "/tmp/work.json")
	require.NoError(t, err)
	assert.Len(t, reg.Accounts, 1)

	// Verify file was written
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/accounts/...`
Expected: FAIL (package doesn't exist)

**Step 3: Write the implementation**

Create `pkg/accounts/registry.go`:

```go
// ABOUTME: Account registry for multi-account Google OAuth support
// ABOUTME: Loads account config, resolves aliases, manages token paths

package accounts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Account represents a single Google account with its token path
type Account struct {
	Alias     string `json:"-"`
	TokenPath string `json:"token_path"`
	Client    *http.Client `json:"-"`
}

// Registry manages multiple Google accounts
type Registry struct {
	configPath   string
	DefaultAlias string              `json:"default"`
	Accounts     map[string]*Account `json:"accounts"`
}

// configFile is the JSON structure on disk
type configFile struct {
	Default  string                    `json:"default"`
	Accounts map[string]*accountEntry  `json:"accounts"`
}

type accountEntry struct {
	TokenPath string `json:"token_path"`
}

// LoadRegistry loads accounts from a config file, or creates a single-account
// fallback registry if the file doesn't exist
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file - create a single-account fallback
			return &Registry{
				configPath:   path,
				DefaultAlias: "default",
				Accounts: map[string]*Account{
					"default": {Alias: "default"},
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to read accounts config: %w", err)
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse accounts config: %w", err)
	}

	reg := &Registry{
		configPath:   path,
		DefaultAlias: cfg.Default,
		Accounts:     make(map[string]*Account, len(cfg.Accounts)),
	}

	for alias, entry := range cfg.Accounts {
		reg.Accounts[alias] = &Account{
			Alias:     alias,
			TokenPath: entry.TokenPath,
		}
	}

	return reg, nil
}

// Resolve returns the account for the given alias, or the default if alias is empty
func (r *Registry) Resolve(alias string) (*Account, error) {
	if alias == "" {
		alias = r.DefaultAlias
	}

	acct, ok := r.Accounts[alias]
	if !ok {
		available := r.ListAccounts()
		return nil, fmt.Errorf("unknown account '%s'. Available accounts: %s", alias, strings.Join(available, ", "))
	}
	return acct, nil
}

// GetDefault returns the default account
func (r *Registry) GetDefault() (*Account, error) {
	return r.Resolve("")
}

// ListAccounts returns all account aliases sorted alphabetically
func (r *Registry) ListAccounts() []string {
	aliases := make([]string, 0, len(r.Accounts))
	for alias := range r.Accounts {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

// AddAccount adds a new account to the registry and persists the config
func (r *Registry) AddAccount(alias, tokenPath string) error {
	r.Accounts[alias] = &Account{
		Alias:     alias,
		TokenPath: tokenPath,
	}
	return r.save()
}

// save persists the registry to disk
func (r *Registry) save() error {
	cfg := configFile{
		Default:  r.DefaultAlias,
		Accounts: make(map[string]*accountEntry, len(r.Accounts)),
	}
	for alias, acct := range r.Accounts {
		cfg.Accounts[alias] = &accountEntry{
			TokenPath: acct.TokenPath,
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal accounts config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(r.configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(r.configPath, data, 0600)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/accounts/...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/accounts/registry.go pkg/accounts/registry_test.go
git commit -m "feat: add account registry for multi-account support"
```

---

### Task 11: Add GetAccountsConfigPath to auth/paths.go

**Files:**
- Modify: `pkg/auth/paths.go` (add accounts config path function)
- Modify: `pkg/auth/paths_test.go` (add test)

**Step 1: Write the failing test**

Add to `pkg/auth/paths_test.go`:

```go
func TestGetAccountsConfigPath(t *testing.T) {
	t.Run("uses environment variable when set", func(t *testing.T) {
		t.Setenv("GSUITE_MCP_ACCOUNTS_PATH", "/custom/accounts.json")
		assert.Equal(t, "/custom/accounts.json", GetAccountsConfigPath())
	})

	t.Run("falls back to XDG config", func(t *testing.T) {
		// Unset any override
		t.Setenv("GSUITE_MCP_ACCOUNTS_PATH", "")
		path := GetAccountsConfigPath()
		assert.Contains(t, path, "gsuite-mcp")
		assert.Contains(t, path, "accounts.json")
	})
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/auth/... -run TestGetAccountsConfigPath`
Expected: FAIL

**Step 3: Implement**

Add to `pkg/auth/paths.go`:

```go
const defaultAccounts = "accounts.json"

// GetAccountsConfigPath returns the path to the accounts config file
func GetAccountsConfigPath() string {
	if p := os.Getenv("GSUITE_MCP_ACCOUNTS_PATH"); p != "" {
		return p
	}
	return filepath.Join(getConfigDir(), appName, defaultAccounts)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/auth/... -run TestGetAccountsConfigPath`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/auth/paths.go pkg/auth/paths_test.go
git commit -m "feat: add GetAccountsConfigPath for multi-account config"
```

---

### Task 12: Refactor Server struct for multi-account

**Files:**
- Modify: `pkg/server/server.go` (Server struct, NewServer, resolveServices helper)

This is the core refactor. The Server struct changes from holding one instance of each service to holding a registry and a map of per-account services.

**Step 1: Write the failing test**

Add to `pkg/server/server_test.go`:

```go
func TestServer_ResolveServices_Default(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// Default account should resolve without error
	svc, err := srv.resolveServices(context.Background(), "")
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.Gmail)
	assert.NotNil(t, svc.Calendar)
	assert.NotNil(t, svc.People)
	assert.NotNil(t, svc.Tasks)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/server/... -run TestServer_ResolveServices`
Expected: FAIL

**Step 3: Implement the refactor**

Update `pkg/server/server.go`:

1. Replace the Server struct:

```go
// AccountServices holds all Google service instances for a single account
type AccountServices struct {
	Gmail    *gmail.Service
	Calendar *calendar.Service
	People   *people.Service
	Tasks    *tasks.Service
}

// Server is the MCP server for GSuite APIs
type Server struct {
	registry *accounts.Registry
	services map[string]*AccountServices
	mcp      *server.MCPServer
	auth     *auth.Authenticator
}
```

2. Update `NewServer` to:
   - Load the account registry (or create fallback)
   - In ISH_MODE: create a single "default" account with fake client
   - In real mode: for each account in the registry, create an Authenticator per token path
   - Store services lazily (empty map initially), create on first use

3. Add `resolveServices` helper:

```go
func (s *Server) resolveServices(ctx context.Context, alias string) (*AccountServices, error) {
	acct, err := s.registry.Resolve(alias)
	if err != nil {
		return nil, err
	}

	if svc, ok := s.services[acct.Alias]; ok {
		return svc, nil
	}

	// Lazy init services for this account
	client := acct.Client
	if client == nil {
		client = &http.Client{}
	}

	gmailSvc, err := gmail.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service for account '%s': %w", acct.Alias, err)
	}
	calendarSvc, err := calendar.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service for account '%s': %w", acct.Alias, err)
	}
	peopleSvc, err := people.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create People service for account '%s': %w", acct.Alias, err)
	}
	tasksSvc, err := tasks.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tasks service for account '%s': %w", acct.Alias, err)
	}

	svc := &AccountServices{
		Gmail:    gmailSvc,
		Calendar: calendarSvc,
		People:   peopleSvc,
		Tasks:    tasksSvc,
	}
	s.services[acct.Alias] = svc
	return svc, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./pkg/server/... -run TestServer_ResolveServices`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS (existing tests still work via default account)

**Step 6: Commit**

```bash
git add pkg/server/server.go pkg/server/server_test.go
git commit -m "refactor: restructure Server for multi-account service resolution"
```

---

### Task 13: Update all tool handlers to use resolveServices

**Files:**
- Modify: `pkg/server/server.go` (all ~32 handler functions)

This is a mechanical change. Every handler that currently accesses `s.gmail`, `s.calendar`, `s.people`, or `s.tasks` must instead:

1. Extract `account` param: `account := req.GetString("account", "")`
2. Resolve services: `svc, err := s.resolveServices(ctx, account)`
3. Use `svc.Gmail`, `svc.Calendar`, `svc.People`, `svc.Tasks` instead of `s.gmail`, etc.

Also add the `account` property to every tool's InputSchema:

```go
"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
```

**Step 1: Write a test that verifies the account parameter is accepted**

Add to `pkg/server/server_test.go`:

```go
func TestServer_ToolsAcceptAccountParam(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	// Every non-auth tool should have an "account" property
	for _, tool := range tools {
		if strings.HasPrefix(tool.Name, "auth_") || tool.Name == "accounts_list" {
			continue
		}
		props := tool.InputSchema.Properties
		_, hasAccount := props["account"]
		assert.True(t, hasAccount, "tool %s should have 'account' property", tool.Name)
	}
}
```

**Step 2: Update all tool registrations to include account param**

Add `"account": map[string]string{...}` to every tool's Properties map (Gmail, Calendar, People, Tasks).

**Step 3: Update all handler functions**

For each handler, replace direct service access with resolved services. Example transformation:

Before:
```go
func (s *Server) handleGmailListMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messages, err := s.gmail.ListMessages(ctx, query, maxResults)
```

After:
```go
func (s *Server) handleGmailListMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := req.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	messages, err := svc.Gmail.ListMessages(ctx, query, maxResults)
```

Apply this pattern to all handlers:
- `handleGmailListMessages`, `handleGmailGetMessage`, `handleGmailSendMessage`, `handleGmailCreateDraft`, `handleGmailSendDraft`, `handleGmailModifyLabels`, `handleGmailTrashMessage`, `handleGmailDeleteMessage`
- `handleCalendarListEvents`, `handleCalendarGetEvent`, `handleCalendarCreateEvent`, `handleCalendarUpdateEvent`, `handleCalendarDeleteEvent`
- `handlePeopleListContacts`, `handlePeopleSearchContacts`, `handlePeopleGetContact`, `handlePeopleCreateContact`, `handlePeopleUpdateContact`, `handlePeopleDeleteContact`
- `handleTasksListTaskLists`, `handleTasksCreateTaskList`, `handleTasksUpdateTaskList`, `handleTasksDeleteTaskList`, `handleTasksListTasks`, `handleTasksCreateTask`, `handleTasksUpdateTask`, `handleTasksDeleteTask`

**Step 4: Run all tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/server/server.go pkg/server/server_test.go
git commit -m "feat: add account parameter to all tool handlers"
```

---

### Task 14: Update auth tools for multi-account

**Files:**
- Modify: `pkg/server/server.go` (auth handlers)

**Step 1: Update auth tool registrations to accept account param**

Add `account` property to `auth_init`, `auth_complete`, `auth_status`, `auth_info`, `auth_revoke`.

**Step 2: Update auth handlers**

Each auth handler needs to resolve which account it's operating on. The authenticator creation needs to use the account's token path.

Key changes:
- `handleAuthInit`: Accept `account` param. If account doesn't exist in registry, auto-create it with a new token path. Return auth URL for that account.
- `handleAuthComplete`: Accept `account` param. Exchange code and save token to that account's path.
- `handleAuthStatus`: If no `account` param, show status for ALL accounts. Otherwise show status for the specified account.
- `handleAuthInfo`: Same as status - show per-account or all.
- `handleAuthRevoke`: Accept `account` param. Revoke token for that specific account.

**Step 3: Add accounts_list tool**

Register a new tool:

```go
s.mcp.AddTool(mcp.Tool{
	Name:        "accounts_list",
	Description: "List all configured accounts and their authentication status",
	InputSchema: mcp.ToolInputSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
	},
}, s.handleAccountsList)
```

Handler:

```go
func (s *Server) handleAccountsList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	type AccountInfo struct {
		Alias         string `json:"alias"`
		IsDefault     bool   `json:"is_default"`
		Authenticated bool   `json:"authenticated"`
		TokenPath     string `json:"token_path"`
	}

	var accounts []AccountInfo
	for _, alias := range s.registry.ListAccounts() {
		acct, _ := s.registry.Resolve(alias)
		authenticated := false
		if acct.TokenPath != "" {
			if _, err := os.Stat(acct.TokenPath); err == nil {
				authenticated = true
			}
		}
		accounts = append(accounts, AccountInfo{
			Alias:         alias,
			IsDefault:     alias == s.registry.DefaultAlias,
			Authenticated: authenticated,
			TokenPath:     acct.TokenPath,
		})
	}

	return mcp.NewToolResultJSON(map[string]interface{}{
		"accounts": accounts,
		"count":    len(accounts),
	})
}
```

**Step 4: Run all tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/server/server.go
git commit -m "feat: make auth tools account-aware, add accounts_list tool"
```

---

### Task 15: Update CLI commands for multi-account

**Files:**
- Modify: `cmd/gsuite-mcp/main.go` (add --account flag to setup, test, whoami)

**Step 1: Add account flag parsing**

Update the CLI to accept `--account <alias>` for `setup`, `test`, and `whoami` commands. When specified, use that account's token path instead of the default.

**Step 2: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/gsuite-mcp/main.go
git commit -m "feat: add --account flag to CLI commands"
```

---

### Task 16: Update resources for multi-account

**Files:**
- Modify: `pkg/server/server.go` (resource handlers)

Resource handlers currently use the default account. Since resources don't have parameters in MCP, they'll continue to use the default account. Document this limitation.

**Step 1: Ensure resource handlers use resolveServices with empty alias**

Update any resource handler that directly accesses `s.gmail`, `s.calendar`, `s.people`, `s.tasks` to use `s.resolveServices(ctx, "")` instead.

**Step 2: Run tests**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/server/server.go
git commit -m "refactor: update resource handlers to use account resolution"
```

---

### Task 17: Update docs and bump version

**Files:**
- Modify: `README.md` (multi-account section)
- Modify: `docs/setup.md` (multi-account setup)
- Modify: `cmd/gsuite-mcp/main.go:19` (version bump)

**Step 1: Add multi-account documentation**

Add a section to README.md explaining:
- How to set up multiple accounts
- The accounts.json config format
- How to use the `account` parameter in tool calls
- How auth works per-account

**Step 2: Bump version**

Change version in `cmd/gsuite-mcp/main.go` from `"1.0.3"` to `"1.1.0"`.

**Step 3: Run full test suite one final time**

Run: `cd /Users/harper/Public/src/2389/gsuite-mcp && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add README.md docs/setup.md cmd/gsuite-mcp/main.go
git commit -m "docs: add multi-account documentation, bump to v1.1.0"
```
