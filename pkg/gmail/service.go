// ABOUTME: Gmail API service for email management
// ABOUTME: Handles messages, drafts, labels, and attachments

package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// ThreadingHeaders contains headers needed for proper email threading
type ThreadingHeaders struct {
	ThreadId   string // Original message's thread ID (required for Gmail API)
	MessageID  string // Original message's Message-ID header
	References string // References header (chain of message IDs)
	Subject    string // Original subject
	From       string // Original sender (for reply-to)
}

// GetMessageHeaders fetches a message and extracts threading headers
func (s *Service) GetMessageHeaders(ctx context.Context, messageID string) (*ThreadingHeaders, error) {
	var msg *gmail.Message

	err := retry.WithRetry(func() error {
		var err error
		// Fetch with metadata format to get headers efficiently
		msg, err = s.svc.Users.Messages.Get("me", messageID).
			Context(ctx).
			Format("metadata").
			MetadataHeaders("Message-ID", "References", "Subject", "From").
			Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to get message headers: %w", err)
	}

	headers := &ThreadingHeaders{
		ThreadId: msg.ThreadId, // Capture thread ID from message object
	}
	if msg.Payload != nil {
		for _, h := range msg.Payload.Headers {
			switch strings.ToLower(h.Name) {
			case "message-id":
				headers.MessageID = h.Value
			case "references":
				headers.References = h.Value
			case "subject":
				headers.Subject = h.Value
			case "from":
				headers.From = h.Value
			}
		}
	}

	return headers, nil
}

// SendMessage sends an email with automatic HTML detection
// If inReplyTo is provided (a message ID), threading headers are auto-fetched
func (s *Service) SendMessage(ctx context.Context, to, subject, body, inReplyTo, cc, bcc string) (*gmail.Message, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient address (to) cannot be empty")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	var inReplyToHeader, referencesHeader, threadId string

	// If replying, fetch original message headers for threading
	if inReplyTo != "" {
		headers, err := s.GetMessageHeaders(ctx, inReplyTo)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch original message for send reply: %w", err)
		}
		// Capture thread ID for Gmail API
		threadId = headers.ThreadId
		// Only set threading headers if the original message has a Message-ID
		if headers.MessageID != "" {
			inReplyToHeader = headers.MessageID
			referencesHeader = buildReferences(headers.MessageID, headers.References)
		}
		// Auto-prefix "Re: " if not already present
		subject = ensureReplySubject(subject)
	}

	var message string
	if isHTML(body) {
		message = buildHTMLMessage(to, cc, bcc, subject, body, inReplyToHeader, referencesHeader)
	} else {
		message = buildPlainTextMessage(to, cc, bcc, subject, body, inReplyToHeader, referencesHeader)
	}

	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	msg := &gmail.Message{
		Raw:      encoded,
		ThreadId: threadId, // Set thread ID for proper threading
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

func isHTML(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<!doctype") ||
		strings.Contains(lower, "<body") ||
		strings.Contains(lower, "<div") ||
		strings.Contains(lower, "<p>") ||
		strings.Contains(lower, "<br>") ||
		strings.Contains(lower, "<br/>") ||
		strings.Contains(lower, "<br />") ||
		strings.Contains(lower, "<span") ||
		strings.Contains(lower, "<a ") ||
		strings.Contains(lower, "<table")
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func buildPlainTextMessage(to, cc, bcc, subject, body, inReplyTo, references string) string {
	var headers strings.Builder
	headers.WriteString(fmt.Sprintf("To: %s\r\n", sanitizeHeader(to)))
	if cc != "" {
		headers.WriteString(fmt.Sprintf("Cc: %s\r\n", sanitizeHeader(cc)))
	}
	if bcc != "" {
		headers.WriteString(fmt.Sprintf("Bcc: %s\r\n", sanitizeHeader(bcc)))
	}
	headers.WriteString(fmt.Sprintf("Subject: %s\r\n", sanitizeHeader(subject)))
	if inReplyTo != "" {
		headers.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", sanitizeHeader(inReplyTo)))
	}
	if references != "" {
		headers.WriteString(fmt.Sprintf("References: %s\r\n", sanitizeHeader(references)))
	}
	headers.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	headers.WriteString("MIME-Version: 1.0\r\n")
	headers.WriteString("\r\n")
	headers.WriteString(body)
	return headers.String()
}

func buildHTMLMessage(to, cc, bcc, subject, body, inReplyTo, references string) string {
	var headers strings.Builder
	headers.WriteString(fmt.Sprintf("To: %s\r\n", sanitizeHeader(to)))
	if cc != "" {
		headers.WriteString(fmt.Sprintf("Cc: %s\r\n", sanitizeHeader(cc)))
	}
	if bcc != "" {
		headers.WriteString(fmt.Sprintf("Bcc: %s\r\n", sanitizeHeader(bcc)))
	}
	headers.WriteString(fmt.Sprintf("Subject: %s\r\n", sanitizeHeader(subject)))
	if inReplyTo != "" {
		headers.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", sanitizeHeader(inReplyTo)))
	}
	if references != "" {
		headers.WriteString(fmt.Sprintf("References: %s\r\n", sanitizeHeader(references)))
	}
	headers.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	headers.WriteString("MIME-Version: 1.0\r\n")
	headers.WriteString("\r\n")
	headers.WriteString(body)
	return headers.String()
}

// buildReferences constructs the References header for a reply
func buildReferences(originalMessageID, originalReferences string) string {
	if originalMessageID == "" {
		return originalReferences
	}
	if originalReferences != "" {
		return originalReferences + " " + originalMessageID
	}
	return originalMessageID
}

// ensureReplySubject adds "Re: " prefix if not already present
func ensureReplySubject(subject string) string {
	if strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

// CreateDraft creates a new draft email with automatic HTML detection
// If inReplyTo is provided (a message ID), threading headers are auto-fetched
func (s *Service) CreateDraft(ctx context.Context, to, subject, body, inReplyTo, cc, bcc string) (*gmail.Draft, error) {
	if to == "" {
		return nil, fmt.Errorf("recipient address (to) cannot be empty")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	var inReplyToHeader, referencesHeader, threadId string

	// If replying, fetch original message headers for threading
	if inReplyTo != "" {
		headers, err := s.GetMessageHeaders(ctx, inReplyTo)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch original message for draft reply: %w", err)
		}
		// Capture thread ID for Gmail API
		threadId = headers.ThreadId
		// Only set threading headers if the original message has a Message-ID
		if headers.MessageID != "" {
			inReplyToHeader = headers.MessageID
			referencesHeader = buildReferences(headers.MessageID, headers.References)
		}
		// Auto-prefix "Re: " if not already present
		subject = ensureReplySubject(subject)
	}

	var message string
	if isHTML(body) {
		message = buildHTMLMessage(to, cc, bcc, subject, body, inReplyToHeader, referencesHeader)
	} else {
		message = buildPlainTextMessage(to, cc, bcc, subject, body, inReplyToHeader, referencesHeader)
	}

	encoded := base64.URLEncoding.EncodeToString([]byte(message))

	draft := &gmail.Draft{
		Message: &gmail.Message{
			Raw:      encoded,
			ThreadId: threadId, // Set thread ID for proper threading
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

// ListDrafts lists draft messages
func (s *Service) ListDrafts(ctx context.Context, maxResults int64) ([]*gmail.Draft, error) {
	var result *gmail.ListDraftsResponse

	err := retry.WithRetry(func() error {
		call := s.svc.Users.Drafts.List("me").Context(ctx).MaxResults(maxResults)

		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to list drafts: %w", err)
	}

	return result.Drafts, nil
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

// GetProfile returns the authenticated user's email profile
func (s *Service) GetProfile(ctx context.Context) (*gmail.Profile, error) {
	var profile *gmail.Profile
	err := retry.WithRetry(func() error {
		var err error
		profile, err = s.svc.Users.GetProfile("me").Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to get profile: %w", err)
	}

	return profile, nil
}
