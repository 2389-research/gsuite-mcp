# ABOUTME: Tests for Google People API service
# ABOUTME: Validates contact CRUD, search, and group management

import pytest
from unittest.mock import patch, MagicMock
from gsuite_mcp.services.people import PeopleService
from gsuite_mcp.auth.fake_credentials import FakeCredentials


@pytest.fixture
def people_ish_service():
    """Create People service in ish mode with mocked build."""
    with patch('gsuite_mcp.services.people.build') as mock_build:
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        service = PeopleService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        # Store mock service for test access
        service._mock_service = mock_service

        return service


class TestListContacts:
    """Tests for listing contacts."""

    def test_list_contacts_basic(self, people_ish_service):
        """List contacts should return user's contacts."""
        mock_response = {
            'connections': [
                {
                    'resourceName': 'people/c123',
                    'names': [{'displayName': 'John Doe'}],
                    'emailAddresses': [{'value': 'john@example.com'}],
                },
                {
                    'resourceName': 'people/c456',
                    'names': [{'displayName': 'Jane Smith'}],
                    'emailAddresses': [{'value': 'jane@example.com'}],
                },
            ]
        }
        people_ish_service._service.people().connections().list().execute.return_value = mock_response

        contacts = people_ish_service.list_contacts()

        assert len(contacts) == 2
        assert contacts[0]['names'][0]['displayName'] == 'John Doe'
        assert contacts[1]['names'][0]['displayName'] == 'Jane Smith'

    def test_list_contacts_with_pagination(self, people_ish_service):
        """List contacts should support pagination."""
        mock_response = {
            'connections': [
                {
                    'resourceName': 'people/c123',
                    'names': [{'displayName': 'John Doe'}],
                },
            ]
        }
        people_ish_service._service.people().connections().list().execute.return_value = mock_response

        contacts = people_ish_service.list_contacts(page_size=50, page_token="token123")

        # Verify list was called with pagination parameters
        call_args = people_ish_service._service.people().connections().list.call_args
        assert call_args[1]['pageSize'] == 50
        assert call_args[1]['pageToken'] == "token123"
        assert len(contacts) == 1


class TestGetContact:
    """Tests for getting a specific contact."""

    def test_get_contact_basic(self, people_ish_service):
        """Get contact should return contact details."""
        mock_response = {
            'resourceName': 'people/c123',
            'names': [{'displayName': 'John Doe', 'givenName': 'John', 'familyName': 'Doe'}],
            'emailAddresses': [{'value': 'john@example.com'}],
            'phoneNumbers': [{'value': '+1234567890'}],
        }
        people_ish_service._service.people().get().execute.return_value = mock_response

        contact = people_ish_service.get_contact('people/c123')

        assert contact['resourceName'] == 'people/c123'
        assert contact['names'][0]['displayName'] == 'John Doe'
        assert contact['emailAddresses'][0]['value'] == 'john@example.com'


class TestCreateContact:
    """Tests for creating contacts."""

    def test_create_contact_basic(self, people_ish_service):
        """Create contact should create a new contact."""
        mock_response = {
            'resourceName': 'people/c789',
            'names': [{'givenName': 'Alice', 'familyName': 'Johnson'}],
            'emailAddresses': [{'value': 'alice@example.com'}],
        }
        people_ish_service._service.people().createContact().execute.return_value = mock_response

        contact = people_ish_service.create_contact(
            given_name='Alice',
            family_name='Johnson',
            email='alice@example.com'
        )

        assert contact['resourceName'] == 'people/c789'
        assert contact['names'][0]['givenName'] == 'Alice'

    def test_create_contact_with_all_fields(self, people_ish_service):
        """Create contact should support all optional fields."""
        mock_response = {
            'resourceName': 'people/c999',
            'names': [{'givenName': 'Bob', 'familyName': 'Smith'}],
            'emailAddresses': [{'value': 'bob@example.com'}],
            'phoneNumbers': [{'value': '+9876543210'}],
            'organizations': [{'name': 'Acme Corp'}],
        }
        people_ish_service._service.people().createContact().execute.return_value = mock_response

        contact = people_ish_service.create_contact(
            given_name='Bob',
            family_name='Smith',
            email='bob@example.com',
            phone='+9876543210',
            organization='Acme Corp'
        )

        assert contact['resourceName'] == 'people/c999'
        assert contact['phoneNumbers'][0]['value'] == '+9876543210'
        assert contact['organizations'][0]['name'] == 'Acme Corp'


class TestUpdateContact:
    """Tests for updating contacts."""

    def test_update_contact_basic(self, people_ish_service):
        """Update contact should modify existing contact."""
        # Mock get_contact for retrieving current data
        mock_get_response = {
            'resourceName': 'people/c123',
            'names': [{'givenName': 'John', 'familyName': 'Doe'}],
            'emailAddresses': [{'value': 'john@example.com'}],
            'etag': 'etag123'
        }
        # Mock updateContact for the update operation
        mock_update_response = {
            'resourceName': 'people/c123',
            'names': [{'givenName': 'John', 'familyName': 'Smith'}],
            'emailAddresses': [{'value': 'john@example.com'}],
            'etag': 'etag124'
        }

        people_ish_service._service.people().get().execute.return_value = mock_get_response
        people_ish_service._service.people().updateContact().execute.return_value = mock_update_response

        contact = people_ish_service.update_contact(
            'people/c123',
            family_name='Smith'
        )

        assert contact['resourceName'] == 'people/c123'
        assert contact['names'][0]['familyName'] == 'Smith'


class TestDeleteContact:
    """Tests for deleting contacts."""

    def test_delete_contact_basic(self, people_ish_service):
        """Delete contact should remove contact."""
        people_ish_service._service.people().deleteContact().execute.return_value = {}

        # Should not raise an exception
        people_ish_service.delete_contact('people/c123')

        # Verify deleteContact was called with correct resource name
        call_args = people_ish_service._service.people().deleteContact.call_args
        assert call_args[1]['resourceName'] == 'people/c123'


class TestSearchContacts:
    """Tests for searching contacts."""

    def test_search_contacts_basic(self, people_ish_service):
        """Search contacts should find matching contacts."""
        mock_response = {
            'results': [
                {
                    'person': {
                        'resourceName': 'people/c123',
                        'names': [{'displayName': 'John Doe'}],
                        'emailAddresses': [{'value': 'john@example.com'}],
                    }
                }
            ]
        }
        people_ish_service._service.people().searchContacts().execute.return_value = mock_response

        results = people_ish_service.search_contacts('John')

        assert len(results) == 1
        assert results[0]['person']['names'][0]['displayName'] == 'John Doe'

    def test_search_contacts_with_page_size(self, people_ish_service):
        """Search contacts should support custom page size."""
        mock_response = {'results': []}
        people_ish_service._service.people().searchContacts().execute.return_value = mock_response

        people_ish_service.search_contacts('test', page_size=50)

        # Verify searchContacts was called with correct page size
        call_args = people_ish_service._service.people().searchContacts.call_args
        assert call_args[1]['pageSize'] == 50


class TestContactGroups:
    """Tests for contact group operations."""

    def test_list_contact_groups(self, people_ish_service):
        """List contact groups should return user's groups."""
        mock_response = {
            'contactGroups': [
                {
                    'resourceName': 'contactGroups/g123',
                    'name': 'Friends',
                    'memberCount': 5,
                },
                {
                    'resourceName': 'contactGroups/g456',
                    'name': 'Colleagues',
                    'memberCount': 10,
                },
            ]
        }
        people_ish_service._service.contactGroups().list().execute.return_value = mock_response

        groups = people_ish_service.list_contact_groups()

        assert len(groups) == 2
        assert groups[0]['name'] == 'Friends'
        assert groups[1]['name'] == 'Colleagues'

    def test_create_contact_group(self, people_ish_service):
        """Create contact group should create a new group."""
        mock_response = {
            'resourceName': 'contactGroups/g789',
            'name': 'Family',
            'memberCount': 0,
        }
        people_ish_service._service.contactGroups().create().execute.return_value = mock_response

        group = people_ish_service.create_contact_group('Family')

        assert group['resourceName'] == 'contactGroups/g789'
        assert group['name'] == 'Family'

    def test_add_contact_to_group(self, people_ish_service):
        """Add contact to group should add members."""
        mock_response = {
            'resourceName': 'contactGroups/g123',
            'name': 'Friends',
            'memberCount': 7,
        }
        people_ish_service._service.contactGroups().members().modify().execute.return_value = mock_response

        result = people_ish_service.add_contact_to_group(
            'contactGroups/g123',
            ['people/c123', 'people/c456']
        )

        assert result['memberCount'] == 7
        # Verify modify was called with correct parameters
        call_args = people_ish_service._service.contactGroups().members().modify.call_args
        assert call_args[1]['resourceName'] == 'contactGroups/g123'
        assert 'resourceNamesToAdd' in call_args[1]['body']
