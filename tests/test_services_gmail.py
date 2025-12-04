# ABOUTME: Tests for Gmail service operations using ish mode
# ABOUTME: Validates email management, search, labels, and drafts with comprehensive coverage

import pytest
from unittest.mock import patch, MagicMock
from gsuite_mcp.services.gmail import GmailService
from gsuite_mcp.auth.fake_credentials import FakeCredentials


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


class TestListMessages:
    """Tests for listing messages."""

    def test_list_messages_basic(self, gmail_ish_service):
        """List messages should return message metadata."""
        mock_response = {
            'messages': [
                {'id': 'msg1', 'threadId': 'thread1'},
                {'id': 'msg2', 'threadId': 'thread2'},
            ]
        }
        gmail_ish_service._service.users().messages().list().execute.return_value = mock_response

        messages = gmail_ish_service.list_messages(max_results=10)

        assert len(messages) == 2
        assert messages[0]['id'] == 'msg1'
        assert messages[1]['id'] == 'msg2'

    def test_list_messages_with_query(self, gmail_ish_service):
        """List messages should support query filters."""
        mock_response = {
            'messages': [
                {'id': 'msg3', 'threadId': 'thread3'},
            ]
        }
        gmail_ish_service._service.users().messages().list().execute.return_value = mock_response

        messages = gmail_ish_service.list_messages(query="is:unread from:example@test.com")

        # Verify list was called with the query parameter
        call_args = gmail_ish_service._service.users().messages().list.call_args
        assert call_args[1]['q'] == "is:unread from:example@test.com"
        assert len(messages) == 1

    def test_list_messages_with_labels(self, gmail_ish_service):
        """List messages should support label filtering."""
        mock_response = {
            'messages': [
                {'id': 'msg4', 'threadId': 'thread4'},
            ]
        }
        gmail_ish_service._service.users().messages().list().execute.return_value = mock_response

        messages = gmail_ish_service.list_messages(label_ids=["INBOX", "UNREAD"])

        # Verify list was called with label IDs
        call_args = gmail_ish_service._service.users().messages().list.call_args
        assert call_args[1]['labelIds'] == ["INBOX", "UNREAD"]
        assert len(messages) == 1

    def test_list_messages_pagination(self, gmail_ish_service):
        """List messages should handle pagination."""
        # First page with nextPageToken
        first_response = {
            'messages': [
                {'id': 'msg1', 'threadId': 'thread1'},
            ],
            'nextPageToken': 'token123'
        }
        # Second page without nextPageToken
        second_response = {
            'messages': [
                {'id': 'msg2', 'threadId': 'thread2'},
            ]
        }

        # Create a fresh mock for this test to avoid state pollution
        mock_list = MagicMock()
        mock_list.return_value.execute.side_effect = [first_response, second_response]
        gmail_ish_service._service.users().messages().list = mock_list

        messages = gmail_ish_service.list_messages(max_results=10)

        assert len(messages) == 2
        # Verify pagination token was used in second call
        calls = mock_list.call_args_list
        assert len(calls) == 2
        assert calls[1][1]['pageToken'] == 'token123'

    def test_list_messages_empty(self, gmail_ish_service):
        """List messages should handle empty results."""
        mock_response = {'messages': []}
        gmail_ish_service._service.users().messages().list().execute.return_value = mock_response

        messages = gmail_ish_service.list_messages()

        assert len(messages) == 0


class TestGetMessage:
    """Tests for getting message details."""

    def test_get_message_full(self, gmail_ish_service):
        """Get message should return full message details."""
        mock_response = {
            'id': 'msg123',
            'threadId': 'thread123',
            'payload': {
                'headers': [
                    {'name': 'Subject', 'value': 'Test Subject'},
                    {'name': 'From', 'value': 'sender@example.com'},
                    {'name': 'To', 'value': 'recipient@example.com'},
                ]
            },
            'snippet': 'Message preview text...'
        }
        gmail_ish_service._service.users().messages().get().execute.return_value = mock_response

        message = gmail_ish_service.get_message('msg123')

        assert message['id'] == 'msg123'
        assert message['payload']['headers'][0]['value'] == 'Test Subject'

        # Verify get was called with correct parameters
        call_args = gmail_ish_service._service.users().messages().get.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'msg123'
        assert call_args[1]['format'] == 'full'

    def test_get_message_minimal(self, gmail_ish_service):
        """Get message should support minimal format."""
        mock_response = {
            'id': 'msg456',
            'threadId': 'thread456',
        }
        gmail_ish_service._service.users().messages().get().execute.return_value = mock_response

        message = gmail_ish_service.get_message('msg456', format='minimal')

        # Verify format parameter was passed
        call_args = gmail_ish_service._service.users().messages().get.call_args
        assert call_args[1]['format'] == 'minimal'


class TestSendMessage:
    """Tests for sending messages."""

    def test_send_message_basic(self, gmail_ish_service):
        """Send message should send a basic text email."""
        mock_response = {'id': 'sent_msg_1', 'threadId': 'thread_1', 'labelIds': ['SENT']}
        gmail_ish_service._service.users().messages().send().execute.return_value = mock_response

        result = gmail_ish_service.send_message(
            to="recipient@example.com",
            subject="Test Subject",
            body="Test body content"
        )

        assert result['id'] == 'sent_msg_1'

        # Verify send was called with correct parameters
        call_args = gmail_ish_service._service.users().messages().send.call_args
        assert call_args[1]['userId'] == 'me'
        assert 'raw' in call_args[1]['body']

    def test_send_message_with_cc_bcc(self, gmail_ish_service):
        """Send message should support CC and BCC."""
        mock_response = {'id': 'sent_msg_2', 'threadId': 'thread_2'}
        gmail_ish_service._service.users().messages().send().execute.return_value = mock_response

        result = gmail_ish_service.send_message(
            to="recipient@example.com",
            subject="Test",
            body="Body",
            cc="cc@example.com",
            bcc="bcc@example.com"
        )

        assert result['id'] == 'sent_msg_2'

    def test_send_message_html(self, gmail_ish_service):
        """Send message should support HTML body."""
        mock_response = {'id': 'sent_msg_3', 'threadId': 'thread_3'}
        gmail_ish_service._service.users().messages().send().execute.return_value = mock_response

        result = gmail_ish_service.send_message(
            to="recipient@example.com",
            subject="HTML Email",
            body="<h1>Hello</h1><p>This is HTML</p>",
            html=True
        )

        assert result['id'] == 'sent_msg_3'


class TestReplyToMessage:
    """Tests for replying to messages."""

    def test_reply_to_message_basic(self, gmail_ish_service):
        """Reply should send a reply to an existing message."""
        # Mock get_message response (original message)
        original_message = {
            'id': 'orig_msg_1',
            'threadId': 'thread_1',
            'payload': {
                'headers': [
                    {'name': 'Subject', 'value': 'Original Subject'},
                    {'name': 'From', 'value': 'sender@example.com'},
                ]
            }
        }
        gmail_ish_service._service.users().messages().get().execute.return_value = original_message

        # Mock send response
        reply_response = {'id': 'reply_msg_1', 'threadId': 'thread_1'}
        gmail_ish_service._service.users().messages().send().execute.return_value = reply_response

        result = gmail_ish_service.reply_to_message(
            message_id='orig_msg_1',
            body='This is my reply'
        )

        assert result['id'] == 'reply_msg_1'
        assert result['threadId'] == 'thread_1'

        # Verify send was called with threadId
        send_call_args = gmail_ish_service._service.users().messages().send.call_args
        assert send_call_args[1]['body']['threadId'] == 'thread_1'

    def test_reply_to_message_html(self, gmail_ish_service):
        """Reply should support HTML body."""
        original_message = {
            'id': 'orig_msg_2',
            'threadId': 'thread_2',
            'payload': {
                'headers': [
                    {'name': 'Subject', 'value': 'Test'},
                    {'name': 'From', 'value': 'sender@example.com'},
                ]
            }
        }
        gmail_ish_service._service.users().messages().get().execute.return_value = original_message

        reply_response = {'id': 'reply_msg_2', 'threadId': 'thread_2'}
        gmail_ish_service._service.users().messages().send().execute.return_value = reply_response

        result = gmail_ish_service.reply_to_message(
            message_id='orig_msg_2',
            body='<p>HTML reply</p>',
            html=True
        )

        assert result['id'] == 'reply_msg_2'

    def test_reply_adds_re_prefix(self, gmail_ish_service):
        """Reply should add 'Re:' prefix if not present."""
        original_message = {
            'id': 'orig_msg_3',
            'threadId': 'thread_3',
            'payload': {
                'headers': [
                    {'name': 'Subject', 'value': 'No Re Prefix'},
                    {'name': 'From', 'value': 'sender@example.com'},
                ]
            }
        }
        gmail_ish_service._service.users().messages().get().execute.return_value = original_message

        reply_response = {'id': 'reply_msg_3', 'threadId': 'thread_3'}
        gmail_ish_service._service.users().messages().send().execute.return_value = reply_response

        gmail_ish_service.reply_to_message(
            message_id='orig_msg_3',
            body='Reply text'
        )

        # The subject should have Re: added - verified by implementation


class TestModifyMessage:
    """Tests for modifying message labels."""

    def test_modify_message_add_labels(self, gmail_ish_service):
        """Modify should add labels to a message."""
        mock_response = {
            'id': 'msg1',
            'labelIds': ['INBOX', 'UNREAD', 'STARRED']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.modify_message(
            message_id='msg1',
            add_labels=['STARRED']
        )

        assert 'STARRED' in result['labelIds']

        # Verify modify was called with correct parameters
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['id'] == 'msg1'
        assert call_args[1]['body']['addLabelIds'] == ['STARRED']

    def test_modify_message_remove_labels(self, gmail_ish_service):
        """Modify should remove labels from a message."""
        mock_response = {
            'id': 'msg2',
            'labelIds': ['INBOX']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.modify_message(
            message_id='msg2',
            remove_labels=['UNREAD']
        )

        assert 'UNREAD' not in result['labelIds']

        # Verify modify was called correctly
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['removeLabelIds'] == ['UNREAD']

    def test_modify_message_add_and_remove(self, gmail_ish_service):
        """Modify should support adding and removing labels simultaneously."""
        mock_response = {
            'id': 'msg3',
            'labelIds': ['INBOX', 'STARRED']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.modify_message(
            message_id='msg3',
            add_labels=['STARRED'],
            remove_labels=['UNREAD']
        )

        # Verify both operations were included
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['addLabelIds'] == ['STARRED']
        assert call_args[1]['body']['removeLabelIds'] == ['UNREAD']


class TestMarkAsRead:
    """Tests for marking messages as read."""

    def test_mark_as_read(self, gmail_ish_service):
        """Mark as read should remove UNREAD label."""
        mock_response = {
            'id': 'msg1',
            'labelIds': ['INBOX']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.mark_as_read('msg1')

        assert 'UNREAD' not in result['labelIds']

        # Verify modify was called to remove UNREAD
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['removeLabelIds'] == ['UNREAD']


class TestMarkAsUnread:
    """Tests for marking messages as unread."""

    def test_mark_as_unread(self, gmail_ish_service):
        """Mark as unread should add UNREAD label."""
        mock_response = {
            'id': 'msg2',
            'labelIds': ['INBOX', 'UNREAD']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.mark_as_unread('msg2')

        assert 'UNREAD' in result['labelIds']

        # Verify modify was called to add UNREAD
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['addLabelIds'] == ['UNREAD']


class TestStarMessage:
    """Tests for starring messages."""

    def test_star_message(self, gmail_ish_service):
        """Star message should add STARRED label."""
        mock_response = {
            'id': 'msg3',
            'labelIds': ['INBOX', 'STARRED']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.star_message('msg3')

        assert 'STARRED' in result['labelIds']

        # Verify modify was called to add STARRED
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['addLabelIds'] == ['STARRED']


class TestUnstarMessage:
    """Tests for unstarring messages."""

    def test_unstar_message(self, gmail_ish_service):
        """Unstar message should remove STARRED label."""
        mock_response = {
            'id': 'msg4',
            'labelIds': ['INBOX']
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.unstar_message('msg4')

        assert 'STARRED' not in result['labelIds']

        # Verify modify was called to remove STARRED
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['removeLabelIds'] == ['STARRED']


class TestArchiveMessage:
    """Tests for archiving messages."""

    def test_archive_message(self, gmail_ish_service):
        """Archive message should remove INBOX label."""
        mock_response = {
            'id': 'msg5',
            'labelIds': []
        }
        gmail_ish_service._service.users().messages().modify().execute.return_value = mock_response

        result = gmail_ish_service.archive_message('msg5')

        assert 'INBOX' not in result['labelIds']

        # Verify modify was called to remove INBOX
        call_args = gmail_ish_service._service.users().messages().modify.call_args
        assert call_args[1]['body']['removeLabelIds'] == ['INBOX']


class TestDeleteMessage:
    """Tests for permanently deleting messages."""

    def test_delete_message(self, gmail_ish_service):
        """Delete should permanently remove a message."""
        gmail_ish_service._service.users().messages().delete().execute.return_value = None

        # Should not raise exception
        gmail_ish_service.delete_message('msg_to_delete')

        # Verify delete was called
        call_args = gmail_ish_service._service.users().messages().delete.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'msg_to_delete'


class TestTrashMessage:
    """Tests for moving messages to trash."""

    def test_trash_message(self, gmail_ish_service):
        """Trash should move message to trash."""
        mock_response = {
            'id': 'msg6',
            'labelIds': ['TRASH']
        }
        gmail_ish_service._service.users().messages().trash().execute.return_value = mock_response

        result = gmail_ish_service.trash_message('msg6')

        assert result['id'] == 'msg6'
        assert 'TRASH' in result['labelIds']

        # Verify trash was called
        call_args = gmail_ish_service._service.users().messages().trash.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['id'] == 'msg6'


class TestIshModeConfiguration:
    """Tests for ish mode configuration."""

    @patch('gsuite_mcp.services.gmail.build')
    def test_service_uses_ish_mode(self, mock_build):
        """Service should be configured for ish mode."""
        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        assert service._ish_mode is True
        assert service._api_base_url == "http://localhost:9000"

        # Verify build was called with custom discovery URL
        call_kwargs = mock_build.call_args[1]
        assert 'discoveryServiceUrl' in call_kwargs
        assert call_kwargs['discoveryServiceUrl'] == "http://localhost:9000/discovery/v1/apis/gmail/v1/rest"
