# ABOUTME: Tests for Gmail draft management using ish mode
# ABOUTME: Validates draft creation, updating, listing, sending, and deletion

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


class TestListDrafts:
    """Tests for listing drafts."""

    def test_list_drafts_basic(self, gmail_ish_service):
        """List drafts should return draft metadata."""
        mock_response = {
            'drafts': [
                {'id': 'draft1', 'message': {'id': 'msg1', 'threadId': 'thread1'}},
                {'id': 'draft2', 'message': {'id': 'msg2', 'threadId': 'thread2'}},
            ]
        }
        gmail_ish_service._service.users().drafts().list().execute.return_value = mock_response

        drafts = gmail_ish_service.list_drafts(max_results=10)

        assert len(drafts) == 2
        assert drafts[0]['id'] == 'draft1'
        assert drafts[1]['id'] == 'draft2'

    def test_list_drafts_empty(self, gmail_ish_service):
        """List drafts should handle empty results."""
        mock_response = {}
        gmail_ish_service._service.users().drafts().list().execute.return_value = mock_response

        drafts = gmail_ish_service.list_drafts()

        assert len(drafts) == 0

    def test_list_drafts_with_max_results(self, gmail_ish_service):
        """List drafts should respect max_results parameter."""
        mock_response = {
            'drafts': [
                {'id': 'draft1', 'message': {'id': 'msg1'}},
            ]
        }
        gmail_ish_service._service.users().drafts().list().execute.return_value = mock_response

        drafts = gmail_ish_service.list_drafts(max_results=5)

        # Verify list was called with maxResults parameter
        call_args = gmail_ish_service._service.users().drafts().list.call_args
        assert call_args[1]['maxResults'] == 5
        assert len(drafts) == 1


class TestGetDraft:
    """Tests for getting draft details."""

    def test_get_draft_basic(self, gmail_ish_service):
        """Get draft should return draft details."""
        mock_response = {
            'id': 'draft123',
            'message': {
                'id': 'msg123',
                'threadId': 'thread123',
                'payload': {
                    'headers': [
                        {'name': 'Subject', 'value': 'Draft Subject'},
                        {'name': 'To', 'value': 'recipient@example.com'},
                    ]
                },
                'snippet': 'Draft preview...'
            }
        }
        gmail_ish_service._service.users().drafts().get().execute.return_value = mock_response

        draft = gmail_ish_service.get_draft('draft123')

        assert draft['id'] == 'draft123'
        assert draft['message']['id'] == 'msg123'

        # Verify get was called with correct parameters
        call_args = gmail_ish_service._service.users().drafts().get.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'draft123'


class TestCreateDraft:
    """Tests for creating drafts."""

    def test_create_draft_basic(self, gmail_ish_service):
        """Create draft should store unsent message."""
        mock_response = {
            'id': 'draft123',
            'message': {'id': 'msg123', 'threadId': 'thread123'}
        }
        gmail_ish_service._service.users().drafts().create().execute.return_value = mock_response

        draft = gmail_ish_service.create_draft(
            to='test@example.com',
            subject='Test Draft',
            body='Draft body'
        )

        assert draft['id'] == 'draft123'

        # Verify create was called
        call_args = gmail_ish_service._service.users().drafts().create.call_args
        assert call_args[1]['userId'] == 'me'
        assert 'body' in call_args[1]

    def test_create_draft_with_cc_bcc(self, gmail_ish_service):
        """Create draft should support CC and BCC."""
        mock_response = {
            'id': 'draft456',
            'message': {'id': 'msg456', 'threadId': 'thread456'}
        }
        gmail_ish_service._service.users().drafts().create().execute.return_value = mock_response

        draft = gmail_ish_service.create_draft(
            to='to@example.com',
            subject='Test',
            body='Body',
            cc='cc@example.com',
            bcc='bcc@example.com'
        )

        assert draft['id'] == 'draft456'

    def test_create_draft_html(self, gmail_ish_service):
        """Create draft should support HTML body."""
        mock_response = {
            'id': 'draft789',
            'message': {'id': 'msg789'}
        }
        gmail_ish_service._service.users().drafts().create().execute.return_value = mock_response

        draft = gmail_ish_service.create_draft(
            to='test@example.com',
            subject='HTML Draft',
            body='<h1>Hello</h1><p>HTML content</p>',
            html=True
        )

        assert draft['id'] == 'draft789'


class TestUpdateDraft:
    """Tests for updating drafts."""

    def test_update_draft_basic(self, gmail_ish_service):
        """Update draft should modify existing draft."""
        mock_response = {
            'id': 'draft123',
            'message': {'id': 'msg123_updated', 'threadId': 'thread123'}
        }
        gmail_ish_service._service.users().drafts().update().execute.return_value = mock_response

        draft = gmail_ish_service.update_draft(
            draft_id='draft123',
            to='updated@example.com',
            subject='Updated Subject',
            body='Updated body'
        )

        assert draft['id'] == 'draft123'

        # Verify update was called with correct parameters
        call_args = gmail_ish_service._service.users().drafts().update.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'draft123'
        assert 'body' in call_args[1]

    def test_update_draft_html(self, gmail_ish_service):
        """Update draft should support HTML body."""
        mock_response = {
            'id': 'draft456',
            'message': {'id': 'msg456_updated'}
        }
        gmail_ish_service._service.users().drafts().update().execute.return_value = mock_response

        draft = gmail_ish_service.update_draft(
            draft_id='draft456',
            to='test@example.com',
            subject='Updated',
            body='<p>Updated HTML</p>',
            html=True
        )

        assert draft['id'] == 'draft456'


class TestDeleteDraft:
    """Tests for deleting drafts."""

    def test_delete_draft(self, gmail_ish_service):
        """Delete should permanently remove a draft."""
        gmail_ish_service._service.users().drafts().delete().execute.return_value = None

        # Should not raise exception
        gmail_ish_service.delete_draft('draft_to_delete')

        # Verify delete was called
        call_args = gmail_ish_service._service.users().drafts().delete.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'draft_to_delete'


class TestSendDraft:
    """Tests for sending drafts."""

    def test_send_draft_basic(self, gmail_ish_service):
        """Send draft should send and remove draft."""
        mock_response = {
            'id': 'msg456',
            'threadId': 'thread456',
            'labelIds': ['SENT']
        }
        gmail_ish_service._service.users().drafts().send().execute.return_value = mock_response

        sent = gmail_ish_service.send_draft('draft123')

        assert sent['id'] == 'msg456'
        assert sent['threadId'] == 'thread456'

        # Verify send was called with draft ID
        call_args = gmail_ish_service._service.users().drafts().send.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['body']['id'] == 'draft123'
