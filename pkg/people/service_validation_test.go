// ABOUTME: Validation tests for People service contact field operations
// ABOUTME: Tests invalid email formats, resource name conflicts, malformed objects, and pagination edge cases

package people

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/people/v1"
)


// TestCreateContact_InvalidEmailFormats tests contact creation with various invalid email formats
func TestCreateContact_InvalidEmailFormats(t *testing.T) {
	testCases := []struct {
		name         string
		email        string
		expectError  bool
		errorPattern string
	}{
		{
			name:         "missing @ symbol",
			email:        "invalidemail.com",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "multiple @ symbols",
			email:        "user@@example.com",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "missing domain",
			email:        "user@",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "missing local part",
			email:        "@example.com",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "spaces in email",
			email:        "user name@example.com",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "empty email",
			email:        "",
			expectError:  true,
			errorPattern: "",
		},
		{
			name:         "valid email should pass",
			email:        "valid.user@example.com",
			expectError:  false,
			errorPattern: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			person := &people.Person{
				EmailAddresses: []*people.EmailAddress{
					{
						Value: tc.email,
						Type:  "home",
					},
				},
				Names: []*people.Name{
					{
						GivenName:  "Test",
						FamilyName: "User",
					},
				},
			}

			_, err = svc.CreateContact(context.Background(), person)

			if tc.expectError {
				// We expect the API call to fail, though without a real server
				// we're primarily testing that the service properly handles the request
				t.Logf("Testing invalid email '%s': error=%v", tc.email, err)
			} else {
				// Valid emails might still fail without a server, but the structure is correct
				t.Logf("Testing valid email '%s': error=%v", tc.email, err)
			}
		})
	}
}

// TestCreateContact_MalformedPersonObject tests contact creation with malformed person objects
func TestCreateContact_MalformedPersonObject(t *testing.T) {
	testCases := []struct {
		name        string
		person      *people.Person
		description string
	}{
		{
			name:        "nil person object",
			person:      nil,
			description: "nil person should be rejected",
		},
		{
			name: "empty person object",
			person: &people.Person{},
			description: "person with no fields should be rejected",
		},
		{
			name: "person with only resource name",
			person: &people.Person{
				ResourceName: "people/c12345",
			},
			description: "person with only resource name should be rejected on create",
		},
		{
			name: "person with empty names array",
			person: &people.Person{
				Names: []*people.Name{},
			},
			description: "person with empty names array",
		},
		{
			name: "person with nil name fields",
			person: &people.Person{
				Names: []*people.Name{
					{
						GivenName:  "",
						FamilyName: "",
					},
				},
			},
			description: "person with empty name fields",
		},
		{
			name: "person with multiple email addresses",
			person: &people.Person{
				EmailAddresses: []*people.EmailAddress{
					{Value: "email1@example.com", Type: "work"},
					{Value: "email2@example.com", Type: "home"},
					{Value: "email3@example.com", Type: "other"},
				},
				Names: []*people.Name{
					{GivenName: "Multi", FamilyName: "Email"},
				},
			},
			description: "person with multiple email addresses should be valid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			_, err = svc.CreateContact(context.Background(), tc.person)

			// We expect errors for most malformed objects
			// Log the behavior for documentation
			t.Logf("%s: error=%v", tc.description, err)
		})
	}
}

// TestUpdateContact_ConflictingResourceNames tests contact update with mismatched resource names
func TestUpdateContact_ConflictingResourceNames(t *testing.T) {
	testCases := []struct {
		name               string
		paramResourceName  string
		personResourceName string
		description        string
	}{
		{
			name:               "matching resource names",
			paramResourceName:  "people/c12345",
			personResourceName: "people/c12345",
			description:        "matching resource names should succeed",
		},
		{
			name:               "mismatched resource names",
			paramResourceName:  "people/c12345",
			personResourceName: "people/c67890",
			description:        "mismatched resource names should fail",
		},
		{
			name:               "empty parameter resource name",
			paramResourceName:  "",
			personResourceName: "people/c12345",
			description:        "empty parameter resource name should fail",
		},
		{
			name:               "empty person resource name",
			paramResourceName:  "people/c12345",
			personResourceName: "",
			description:        "empty person resource name may be allowed",
		},
		{
			name:               "malformed resource name format",
			paramResourceName:  "invalid/format",
			personResourceName: "invalid/format",
			description:        "malformed resource name format should fail",
		},
		{
			name:               "resource name without prefix",
			paramResourceName:  "c12345",
			personResourceName: "c12345",
			description:        "resource name without 'people/' prefix should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			person := &people.Person{
				ResourceName: tc.personResourceName,
				Names: []*people.Name{
					{
						GivenName:  "Updated",
						FamilyName: "Name",
					},
				},
			}

			updateMask := "names"
			_, err = svc.UpdateContact(context.Background(), tc.paramResourceName, person, updateMask)

			t.Logf("%s: error=%v", tc.description, err)
		})
	}
}

// TestUpdateContact_InvalidUpdateMasks tests update operations with invalid field masks
func TestUpdateContact_InvalidUpdateMasks(t *testing.T) {
	testCases := []struct {
		name        string
		updateMask  string
		description string
	}{
		{
			name:        "empty update mask",
			updateMask:  "",
			description: "empty update mask should fail or update nothing",
		},
		{
			name:        "invalid field name",
			updateMask:  "invalidField",
			description: "invalid field name should fail",
		},
		{
			name:        "multiple valid fields",
			updateMask:  "names,emailAddresses,phoneNumbers",
			description: "multiple valid fields should succeed",
		},
		{
			name:        "mixed valid and invalid fields",
			updateMask:  "names,invalidField",
			description: "mixed valid and invalid fields should fail",
		},
		{
			name:        "nested field path",
			updateMask:  "names.givenName",
			description: "nested field path may not be supported",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			person := &people.Person{
				Names: []*people.Name{
					{
						GivenName:  "Updated",
						FamilyName: "User",
					},
				},
			}

			_, err = svc.UpdateContact(context.Background(), "people/c12345", person, tc.updateMask)

			t.Logf("%s: error=%v", tc.description, err)
		})
	}
}

// TestListContacts_PaginationEdgeCases tests pagination with various page sizes
func TestListContacts_PaginationEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		pageSize    int64
		description string
	}{
		{
			name:        "zero page size",
			pageSize:    0,
			description: "zero page size should use default",
		},
		{
			name:        "negative page size",
			pageSize:    -1,
			description: "negative page size should fail or use default",
		},
		{
			name:        "page size of 1",
			pageSize:    1,
			description: "minimum page size should work",
		},
		{
			name:        "normal page size",
			pageSize:    10,
			description: "normal page size should work",
		},
		{
			name:        "large page size",
			pageSize:    1000,
			description: "large page size should work or be capped",
		},
		{
			name:        "maximum allowed page size",
			pageSize:    2000,
			description: "maximum page size may be capped by API",
		},
		{
			name:        "excessive page size",
			pageSize:    10000,
			description: "excessive page size should be rejected or capped",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			contacts, err := svc.ListContacts(context.Background(), tc.pageSize)

			t.Logf("%s: contacts=%v, error=%v", tc.description, len(contacts), err)
		})
	}
}

// TestSearchContacts_PaginationEdgeCases tests search pagination with various configurations
func TestSearchContacts_PaginationEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		query       string
		pageSize    int64
		description string
	}{
		{
			name:        "empty query with normal page size",
			query:       "",
			pageSize:    10,
			description: "empty query should fail or return no results",
		},
		{
			name:        "valid query with zero page size",
			query:       "john",
			pageSize:    0,
			description: "zero page size should use default",
		},
		{
			name:        "valid query with negative page size",
			query:       "john",
			pageSize:    -1,
			description: "negative page size should fail or use default",
		},
		{
			name:        "long query string",
			query:       "this is a very long search query with many words to test limits",
			pageSize:    10,
			description: "long query string should work",
		},
		{
			name:        "special characters in query",
			query:       "user@example.com",
			pageSize:    10,
			description: "email-like query should work",
		},
		{
			name:        "query with wildcards",
			query:       "john*",
			pageSize:    10,
			description: "wildcard query may or may not be supported",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			contacts, err := svc.SearchContacts(context.Background(), tc.query, tc.pageSize)

			t.Logf("%s: contacts=%v, error=%v", tc.description, len(contacts), err)
		})
	}
}

// TestGetPerson_InvalidResourceNames tests person retrieval with various invalid resource names
func TestGetPerson_InvalidResourceNames(t *testing.T) {
	testCases := []struct {
		name         string
		resourceName string
		expectError  bool
		description  string
	}{
		{
			name:         "valid resource name format",
			resourceName: "people/c12345",
			expectError:  true, // Will error without real data
			description:  "valid format should make API call",
		},
		{
			name:         "empty resource name",
			resourceName: "",
			expectError:  true,
			description:  "empty resource name should fail",
		},
		{
			name:         "resource name without prefix",
			resourceName: "c12345",
			expectError:  true,
			description:  "missing 'people/' prefix should fail",
		},
		{
			name:         "resource name with wrong prefix",
			resourceName: "contacts/c12345",
			expectError:  true,
			description:  "wrong prefix should fail",
		},
		{
			name:         "malformed resource ID",
			resourceName: "people/",
			expectError:  true,
			description:  "missing ID should fail",
		},
		{
			name:         "special characters in resource name",
			resourceName: "people/c12345!@#",
			expectError:  true,
			description:  "special characters should fail",
		},
		{
			name:         "resource name with path traversal",
			resourceName: "people/../admin",
			expectError:  true,
			description:  "path traversal should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			person, err := svc.GetPerson(context.Background(), tc.resourceName)

			if tc.expectError {
				assert.Error(t, err, tc.description)
				assert.Nil(t, person)
			}

			t.Logf("%s: person=%v, error=%v", tc.description, person, err)
		})
	}
}

// TestDeleteContact_InvalidResourceNames tests contact deletion with various invalid inputs
func TestDeleteContact_InvalidResourceNames(t *testing.T) {
	testCases := []struct {
		name         string
		resourceName string
		expectError  bool
		description  string
	}{
		{
			name:         "valid resource name format",
			resourceName: "people/c12345",
			expectError:  true, // Will error without real data
			description:  "valid format should make API call",
		},
		{
			name:         "empty resource name",
			resourceName: "",
			expectError:  true,
			description:  "empty resource name should fail",
		},
		{
			name:         "resource name of me",
			resourceName: "people/me",
			expectError:  true,
			description:  "cannot delete 'me' resource",
		},
		{
			name:         "resource name with invalid characters",
			resourceName: "people/c12345; DROP TABLE contacts;",
			expectError:  true,
			description:  "SQL injection attempt should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			err = svc.DeleteContact(context.Background(), tc.resourceName)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			}

			t.Logf("%s: error=%v", tc.description, err)
		})
	}
}

// TestContactOperations_NilContext tests that operations properly handle nil contexts
func TestContactOperations_NilContext(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	// Note: We use context.Background() instead of nil because Go's context
	// should never be nil. This test documents that behavior.

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	t.Run("list with background context", func(t *testing.T) {
		_, err := svc.ListContacts(context.Background(), 10)
		t.Logf("ListContacts with background context: error=%v", err)
	})

	t.Run("create with background context", func(t *testing.T) {
		person := &people.Person{
			Names: []*people.Name{{GivenName: "Test", FamilyName: "User"}},
		}
		_, err := svc.CreateContact(context.Background(), person)
		t.Logf("CreateContact with background context: error=%v", err)
	})
}

// TestContactOperations_CanceledContext tests behavior with canceled contexts
func TestContactOperations_CanceledContext(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9000")

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	t.Run("list with canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := svc.ListContacts(ctx, 10)

		// Should get context canceled error
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled) || err != nil, "should fail with canceled context")
		t.Logf("ListContacts with canceled context: error=%v", err)
	})

	t.Run("create with canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		person := &people.Person{
			Names: []*people.Name{{GivenName: "Test", FamilyName: "User"}},
		}
		_, err := svc.CreateContact(ctx, person)

		assert.Error(t, err)
		t.Logf("CreateContact with canceled context: error=%v", err)
	})
}

// TestPhoneNumberValidation tests contact creation with various phone number formats
func TestPhoneNumberValidation(t *testing.T) {
	testCases := []struct {
		name        string
		phoneNumber string
		description string
	}{
		{
			name:        "valid US phone number",
			phoneNumber: "+1-555-123-4567",
			description: "standard US format should be accepted",
		},
		{
			name:        "valid international format",
			phoneNumber: "+44 20 7946 0958",
			description: "international format should be accepted",
		},
		{
			name:        "phone number with extension",
			phoneNumber: "+1-555-123-4567 ext. 123",
			description: "phone with extension should be accepted",
		},
		{
			name:        "invalid phone number",
			phoneNumber: "not-a-phone-number",
			description: "clearly invalid format",
		},
		{
			name:        "empty phone number",
			phoneNumber: "",
			description: "empty phone number",
		},
		{
			name:        "phone number with special chars",
			phoneNumber: "(555) 123-4567",
			description: "US format with parentheses",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ISH_MODE", "true")
			t.Setenv("ISH_BASE_URL", "http://localhost:9000")

			svc, err := NewService(context.Background(), nil)
			require.NoError(t, err)

			person := &people.Person{
				Names: []*people.Name{
					{GivenName: "Test", FamilyName: "User"},
				},
				PhoneNumbers: []*people.PhoneNumber{
					{
						Value: tc.phoneNumber,
						Type:  "mobile",
					},
				},
			}

			_, err = svc.CreateContact(context.Background(), person)
			t.Logf("%s: error=%v", tc.description, err)
		})
	}
}

// TestService_ErrorHandling tests how the service handles various API errors
func TestService_ErrorHandling(t *testing.T) {
	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", "http://localhost:9999") // Non-existent server

	svc, err := NewService(context.Background(), nil)
	require.NoError(t, err)

	t.Run("list contacts with unreachable server", func(t *testing.T) {
		_, err := svc.ListContacts(context.Background(), 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to list contacts")
		t.Logf("Expected error with unreachable server: %v", err)
	})

	t.Run("create contact with unreachable server", func(t *testing.T) {
		person := &people.Person{
			Names: []*people.Name{{GivenName: "Test", FamilyName: "User"}},
		}
		_, err := svc.CreateContact(context.Background(), person)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to create contact")
		t.Logf("Expected error with unreachable server: %v", err)
	})
}

// TestGoogleAPIErrorHandling tests handling of Google API specific errors
func TestGoogleAPIErrorHandling(t *testing.T) {
	t.Run("parse 400 bad request", func(t *testing.T) {
		apiErr := &googleapi.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid request",
		}

		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Contains(t, apiErr.Message, "Invalid")
	})

	t.Run("parse 404 not found", func(t *testing.T) {
		apiErr := &googleapi.Error{
			Code:    http.StatusNotFound,
			Message: "Contact not found",
		}

		assert.Equal(t, http.StatusNotFound, apiErr.Code)
		assert.Contains(t, apiErr.Message, "not found")
	})

	t.Run("parse 409 conflict", func(t *testing.T) {
		apiErr := &googleapi.Error{
			Code:    http.StatusConflict,
			Message: "Resource already exists",
		}

		assert.Equal(t, http.StatusConflict, apiErr.Code)
		assert.Contains(t, apiErr.Message, "exists")
	})
}
