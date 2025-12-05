// ABOUTME: Gmail API service for email management
// ABOUTME: Handles messages, drafts, labels, and attachments

package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

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
	call := s.svc.Users.Messages.List("me").MaxResults(maxResults)

	if query != "" {
		call = call.Q(query)
	}

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list messages: %w", err)
	}

	return result.Messages, nil
}

// GetMessage retrieves a specific message
func (s *Service) GetMessage(ctx context.Context, messageID string) (*gmail.Message, error) {
	msg, err := s.svc.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get message: %w", err)
	}
	return msg, nil
}

// SendMessage sends an email
func (s *Service) SendMessage(ctx context.Context, to, subject, body string) (*gmail.Message, error) {
	message := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	msg := &gmail.Message{
		Raw: encoded,
	}

	sent, err := s.svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to send message: %w", err)
	}

	return sent, nil
}
