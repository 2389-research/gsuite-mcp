// ABOUTME: People API service for contact management
// ABOUTME: Handles contacts, searches, and person lookups

package people

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/harper/gsuite-mcp/pkg/retry"
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
	var result *people.ListConnectionsResponse

	err := retry.WithRetry(func() error {
		call := s.svc.People.Connections.List("people/me").
			Context(ctx).
			PersonFields("names,emailAddresses,phoneNumbers").
			PageSize(pageSize)

		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to list contacts: %w", err)
	}

	return result.Connections, nil
}

// SearchContacts searches for contacts matching the query
func (s *Service) SearchContacts(ctx context.Context, query string, pageSize int64) ([]*people.Person, error) {
	var result *people.SearchResponse

	err := retry.WithRetry(func() error {
		call := s.svc.People.SearchContacts().
			Context(ctx).
			Query(query).
			ReadMask("names,emailAddresses,phoneNumbers").
			PageSize(pageSize)

		var err error
		result, err = call.Do()
		return err
	}, 3, time.Second)

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
	var person *people.Person

	err := retry.WithRetry(func() error {
		var err error
		person, err = s.svc.People.Get(resourceName).
			Context(ctx).
			PersonFields("names,emailAddresses,phoneNumbers,addresses,organizations").
			Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to get person: %w", err)
	}
	return person, nil
}

// CreateContact creates a new contact
func (s *Service) CreateContact(ctx context.Context, person *people.Person) (*people.Person, error) {
	var created *people.Person

	err := retry.WithRetry(func() error {
		var err error
		created, err = s.svc.People.CreateContact(person).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to create contact: %w", err)
	}

	return created, nil
}

// UpdateContact updates an existing contact
func (s *Service) UpdateContact(ctx context.Context, resourceName string, person *people.Person, updateMask string) (*people.Person, error) {
	var updated *people.Person

	err := retry.WithRetry(func() error {
		var err error
		updated, err = s.svc.People.UpdateContact(resourceName, person).
			Context(ctx).
			UpdatePersonFields(updateMask).
			Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to update contact: %w", err)
	}

	return updated, nil
}

// DeleteContact deletes a contact
func (s *Service) DeleteContact(ctx context.Context, resourceName string) error {
	err := retry.WithRetry(func() error {
		_, callErr := s.svc.People.DeleteContact(resourceName).Context(ctx).Do()
		return callErr
	}, 3, time.Second)

	if err != nil {
		return fmt.Errorf("unable to delete contact: %w", err)
	}

	return nil
}
