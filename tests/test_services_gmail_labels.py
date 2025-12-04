# ABOUTME: Tests for Gmail label management
# ABOUTME: Validates label CRUD operations and application to messages

import pytest
from unittest.mock import patch, MagicMock
from gsuite_mcp.services.gmail import GmailService


@pytest.fixture
def gmail_ish_service():
    """Create Gmail service in ish mode with mocked build."""
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        # Store mock service for test access
        service._mock_service = mock_service

        return service


class TestListLabels:
    """Tests for listing labels."""

    def test_list_labels_basic(self, gmail_ish_service):
        """List labels should return all user labels."""
        mock_response = {
            'labels': [
                {'id': 'INBOX', 'name': 'INBOX', 'type': 'system'},
                {'id': 'Label_1', 'name': 'Work', 'type': 'user'},
                {'id': 'Label_2', 'name': 'Personal', 'type': 'user'},
            ]
        }
        gmail_ish_service._service.users().labels().list().execute.return_value = mock_response

        labels = gmail_ish_service.list_labels()

        assert len(labels) == 3
        assert labels[0]['name'] == 'INBOX'
        assert labels[1]['name'] == 'Work'
        assert labels[2]['name'] == 'Personal'

        # Verify list was called with correct parameters
        call_args = gmail_ish_service._service.users().labels().list.call_args
        assert call_args[1]['userId'] == 'me'

    def test_list_labels_empty(self, gmail_ish_service):
        """List labels should handle empty results."""
        mock_response = {'labels': []}
        gmail_ish_service._service.users().labels().list().execute.return_value = mock_response

        labels = gmail_ish_service.list_labels()

        assert len(labels) == 0


class TestGetLabel:
    """Tests for getting a specific label."""

    def test_get_label_basic(self, gmail_ish_service):
        """Get label should return label details."""
        mock_response = {
            'id': 'Label_1',
            'name': 'Work',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().get().execute.return_value = mock_response

        label = gmail_ish_service.get_label('Label_1')

        assert label['id'] == 'Label_1'
        assert label['name'] == 'Work'
        assert label['type'] == 'user'

        # Verify get was called with correct parameters
        call_args = gmail_ish_service._service.users().labels().get.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'Label_1'


class TestCreateLabel:
    """Tests for creating labels."""

    def test_create_label_basic(self, gmail_ish_service):
        """Create label should create new user label with defaults."""
        mock_response = {
            'id': 'Label_2',
            'name': 'Important',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().create().execute.return_value = mock_response

        label = gmail_ish_service.create_label('Important')

        assert label['name'] == 'Important'
        assert label['id'] == 'Label_2'

        # Verify create was called with correct parameters
        call_args = gmail_ish_service._service.users().labels().create.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['body']['name'] == 'Important'
        assert call_args[1]['body']['labelListVisibility'] == 'labelShow'
        assert call_args[1]['body']['messageListVisibility'] == 'show'

    def test_create_label_with_visibility(self, gmail_ish_service):
        """Create label should support custom visibility settings."""
        mock_response = {
            'id': 'Label_3',
            'name': 'Hidden',
            'type': 'user',
            'messageListVisibility': 'hide',
            'labelListVisibility': 'labelHide'
        }
        gmail_ish_service._service.users().labels().create().execute.return_value = mock_response

        label = gmail_ish_service.create_label(
            name='Hidden',
            label_list_visibility='labelHide',
            message_list_visibility='hide'
        )

        assert label['name'] == 'Hidden'
        assert label['messageListVisibility'] == 'hide'

        # Verify visibility settings were passed
        call_args = gmail_ish_service._service.users().labels().create.call_args
        assert call_args[1]['body']['labelListVisibility'] == 'labelHide'
        assert call_args[1]['body']['messageListVisibility'] == 'hide'


class TestUpdateLabel:
    """Tests for updating labels."""

    def test_update_label_name(self, gmail_ish_service):
        """Update label should change label name."""
        # Mock get_label response (current state)
        current_label = {
            'id': 'Label_1',
            'name': 'Work',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().get().execute.return_value = current_label

        # Mock update response
        updated_label = {
            'id': 'Label_1',
            'name': 'Professional',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().update().execute.return_value = updated_label

        result = gmail_ish_service.update_label('Label_1', name='Professional')

        assert result['name'] == 'Professional'
        assert result['id'] == 'Label_1'

        # Verify update was called with correct parameters
        call_args = gmail_ish_service._service.users().labels().update.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'Label_1'
        assert call_args[1]['body']['name'] == 'Professional'
        assert call_args[1]['body']['id'] == 'Label_1'

    def test_update_label_visibility(self, gmail_ish_service):
        """Update label should change visibility settings."""
        current_label = {
            'id': 'Label_2',
            'name': 'Important',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().get().execute.return_value = current_label

        updated_label = {
            'id': 'Label_2',
            'name': 'Important',
            'type': 'user',
            'messageListVisibility': 'hide',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().update().execute.return_value = updated_label

        result = gmail_ish_service.update_label('Label_2', message_list_visibility='hide')

        assert result['messageListVisibility'] == 'hide'

        # Verify visibility was updated
        call_args = gmail_ish_service._service.users().labels().update.call_args
        assert call_args[1]['body']['messageListVisibility'] == 'hide'

    def test_update_label_preserves_fields(self, gmail_ish_service):
        """Update label should preserve unchanged fields."""
        current_label = {
            'id': 'Label_3',
            'name': 'Projects',
            'type': 'user',
            'messageListVisibility': 'show',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().get().execute.return_value = current_label

        updated_label = {
            'id': 'Label_3',
            'name': 'Projects',
            'type': 'user',
            'messageListVisibility': 'hide',
            'labelListVisibility': 'labelShow'
        }
        gmail_ish_service._service.users().labels().update().execute.return_value = updated_label

        # Only update message visibility, name should be preserved
        result = gmail_ish_service.update_label('Label_3', message_list_visibility='hide')

        # Verify name was preserved from current label
        call_args = gmail_ish_service._service.users().labels().update.call_args
        assert call_args[1]['body']['name'] == 'Projects'


class TestDeleteLabel:
    """Tests for deleting labels."""

    def test_delete_label(self, gmail_ish_service):
        """Delete should remove a label."""
        gmail_ish_service._service.users().labels().delete().execute.return_value = None

        # Should not raise exception
        gmail_ish_service.delete_label('Label_1')

        # Verify delete was called with correct parameters
        call_args = gmail_ish_service._service.users().labels().delete.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'Label_1'
