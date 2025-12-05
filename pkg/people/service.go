// ABOUTME: People API service for contact management
// ABOUTME: Handles contacts, searches, and person lookups

package people

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

// Service wraps People API operations
type Service struct {
	svc *people.Service
}

// NewService creates a new People service
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

	svc, err := people.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create People service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListContacts lists contacts from the user's contact list
func (s *Service) ListContacts(ctx context.Context, pageSize int64) ([]*people.Person, error) {
	call := s.svc.People.Connections.List("people/me").
		PersonFields("names,emailAddresses,phoneNumbers").
		PageSize(pageSize)

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list contacts: %w", err)
	}

	return result.Connections, nil
}

// SearchContacts searches for contacts matching the query
func (s *Service) SearchContacts(ctx context.Context, query string, pageSize int64) ([]*people.Person, error) {
	call := s.svc.People.SearchContacts().
		Query(query).
		ReadMask("names,emailAddresses,phoneNumbers").
		PageSize(pageSize)

	result, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to search contacts: %w", err)
	}

	// Extract Person objects from SearchResult
	var contacts []*people.Person
	for _, result := range result.Results {
		if result.Person != nil {
			contacts = append(contacts, result.Person)
		}
	}

	return contacts, nil
}

// GetPerson retrieves a specific person by resource name
func (s *Service) GetPerson(ctx context.Context, resourceName string) (*people.Person, error) {
	person, err := s.svc.People.Get(resourceName).
		PersonFields("names,emailAddresses,phoneNumbers,addresses,organizations").
		Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get person: %w", err)
	}
	return person, nil
}
