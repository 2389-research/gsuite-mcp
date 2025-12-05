// ABOUTME: Gmail API service for email management
// ABOUTME: Handles messages, drafts, labels, and attachments

package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/harper/gsuite-mcp/pkg/retry"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Service wraps Gmail API operations
type Service struct {
	svc *gmail.Service
}

// NewService creates a new Gmail service
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

	svc, err := gmail.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListMessages lists messages matching query
func (s *Service) ListMessages(ctx context.Context, query string, maxResults int64) ([]*gmail.Message, error) {
	var result *gmail.ListMessagesResponse

	err := retry.WithRetry(func() error {
		call := s.svc.Users.Messages.List("me").Context(ctx).MaxResults(maxResults)

		if query != "" {
			call = call.Q(query)
		}

		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to list messages: %w", err)
	}

	return result.Messages, nil
}

// GetMessage retrieves a specific message
func (s *Service) GetMessage(ctx context.Context, messageID string) (*gmail.Message, error) {
	var msg *gmail.Message

	err := retry.WithRetry(func() error {
		var err error
		msg, err = s.svc.Users.Messages.Get("me", messageID).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to get message: %w", err)
	}
	return msg, nil
}

// SendMessage sends an email
func (s *Service) SendMessage(ctx context.Context, to, subject, body string) (*gmail.Message, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient address (to) cannot be empty")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	message := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	msg := &gmail.Message{
		Raw: encoded,
	}

	var sent *gmail.Message
	err := retry.WithRetry(func() error {
		var err error
		sent, err = s.svc.Users.Messages.Send("me", msg).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to send message: %w", err)
	}

	return sent, nil
}

// CreateDraft creates a new draft email
func (s *Service) CreateDraft(ctx context.Context, to, subject, body string) (*gmail.Draft, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient address (to) cannot be empty")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	message := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	draft := &gmail.Draft{
		Message: &gmail.Message{
			Raw: encoded,
		},
	}

	var created *gmail.Draft
	err := retry.WithRetry(func() error {
		var err error
		created, err = s.svc.Users.Drafts.Create("me", draft).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to create draft: %w", err)
	}

	return created, nil
}

// SendDraft sends an existing draft
func (s *Service) SendDraft(ctx context.Context, draftID string) (*gmail.Message, error) {
	draft := &gmail.Draft{
		Id: draftID,
	}

	var sent *gmail.Message
	err := retry.WithRetry(func() error {
		var err error
		sent, err = s.svc.Users.Drafts.Send("me", draft).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to send draft: %w", err)
	}

	return sent, nil
}

// ModifyLabels adds or removes labels from a message
func (s *Service) ModifyLabels(ctx context.Context, messageID string, addLabels, removeLabels []string) (*gmail.Message, error) {
	req := &gmail.ModifyMessageRequest{
		AddLabelIds:    addLabels,
		RemoveLabelIds: removeLabels,
	}

	var modified *gmail.Message
	err := retry.WithRetry(func() error {
		var err error
		modified, err = s.svc.Users.Messages.Modify("me", messageID, req).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to modify labels: %w", err)
	}

	return modified, nil
}

// DeleteMessage permanently deletes a message
func (s *Service) DeleteMessage(ctx context.Context, messageID string) error {
	err := retry.WithRetry(func() error {
		return s.svc.Users.Messages.Delete("me", messageID).Context(ctx).Do()
	}, 3, time.Second)

	if err != nil {
		return fmt.Errorf("unable to delete message: %w", err)
	}

	return nil
}

// TrashMessage moves a message to trash
func (s *Service) TrashMessage(ctx context.Context, messageID string) (*gmail.Message, error) {
	var trashed *gmail.Message
	err := retry.WithRetry(func() error {
		var err error
		trashed, err = s.svc.Users.Messages.Trash("me", messageID).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to trash message: %w", err)
	}

	return trashed, nil
}
