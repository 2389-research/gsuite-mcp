# ABOUTME: Tests for Gmail attachment operations
# ABOUTME: Validates attachment download and email sending with attachments

import pytest
from unittest.mock import patch, MagicMock
from pathlib import Path
import tempfile
import base64
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


class TestGetAttachment:
    """Tests for getting attachment data."""

    def test_get_attachment(self, gmail_ish_service):
        """Get attachment should download attachment data."""
        mock_response = {
            'data': 'base64encodeddata',
            'size': 1024
        }
        gmail_ish_service._service.users().messages().attachments().get().execute.return_value = mock_response

        attachment = gmail_ish_service.get_attachment('msg123', 'att456')

        assert attachment['data'] == 'base64encodeddata'
        assert attachment['size'] == 1024

        # Verify API was called correctly
        call_args = gmail_ish_service._service.users().messages().attachments().get.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['messageId'] == 'msg123'
        assert call_args[1]['id'] == 'att456'


class TestDownloadAttachment:
    """Tests for downloading attachments to disk."""

    def test_download_attachment(self, gmail_ish_service):
        """Download attachment should save data to file."""
        # Create test data
        test_content = b"This is test file content"
        encoded_data = base64.urlsafe_b64encode(test_content).decode()

        mock_response = {
            'data': encoded_data,
            'size': len(test_content)
        }
        gmail_ish_service._service.users().messages().attachments().get().execute.return_value = mock_response

        # Download to temporary file
        with tempfile.TemporaryDirectory() as tmpdir:
            output_path = Path(tmpdir) / "test_attachment.txt"
            result_path = gmail_ish_service.download_attachment('msg123', 'att456', output_path)

            # Verify file was created
            assert result_path.exists()
            assert result_path == output_path

            # Verify content
            with open(result_path, 'rb') as f:
                content = f.read()
            assert content == test_content

    def test_download_attachment_creates_parent_dirs(self, gmail_ish_service):
        """Download attachment should create parent directories if needed."""
        test_content = b"Test content"
        encoded_data = base64.urlsafe_b64encode(test_content).decode()

        mock_response = {
            'data': encoded_data,
            'size': len(test_content)
        }
        gmail_ish_service._service.users().messages().attachments().get().execute.return_value = mock_response

        with tempfile.TemporaryDirectory() as tmpdir:
            # Path with nested directories that don't exist
            output_path = Path(tmpdir) / "subdir1" / "subdir2" / "attachment.txt"
            result_path = gmail_ish_service.download_attachment('msg123', 'att456', output_path)

            assert result_path.exists()
            assert result_path.parent.exists()


class TestSendMessageWithAttachment:
    """Tests for sending emails with attachments."""

    def test_send_message_with_attachment(self, gmail_ish_service):
        """Send message with attachment should send email with file attached."""
        mock_sent = {'id': 'sent123', 'threadId': 'thread456'}
        gmail_ish_service._service.users().messages().send().execute.return_value = mock_sent

        # Create a temporary file to attach
        with tempfile.NamedTemporaryFile(mode='w', suffix='.txt', delete=False) as tmp:
            tmp.write("Test attachment content")
            tmp_path = Path(tmp.name)

        try:
            result = gmail_ish_service.send_message_with_attachment(
                to="recipient@example.com",
                subject="Test Subject",
                body="Test body",
                attachment_path=tmp_path
            )

            assert result['id'] == 'sent123'
            assert result['threadId'] == 'thread456'

            # Verify send was called
            call_args = gmail_ish_service._service.users().messages().send.call_args
            assert call_args[1]['userId'] == 'me'
            assert 'body' in call_args[1]
            assert 'raw' in call_args[1]['body']

        finally:
            tmp_path.unlink()

    def test_send_message_with_attachment_and_cc(self, gmail_ish_service):
        """Send message with attachment should support CC and BCC."""
        mock_sent = {'id': 'sent456', 'threadId': 'thread789'}
        gmail_ish_service._service.users().messages().send().execute.return_value = mock_sent

        with tempfile.NamedTemporaryFile(mode='w', suffix='.txt', delete=False) as tmp:
            tmp.write("Test content")
            tmp_path = Path(tmp.name)

        try:
            result = gmail_ish_service.send_message_with_attachment(
                to="recipient@example.com",
                subject="Test Subject",
                body="Test body",
                attachment_path=tmp_path,
                cc="cc@example.com",
                bcc="bcc@example.com"
            )

            assert result['id'] == 'sent456'

        finally:
            tmp_path.unlink()


class TestBatchModifyMessages:
    """Tests for batch modifying messages."""

    def test_batch_modify_messages(self, gmail_ish_service):
        """Batch modify should apply changes to multiple messages."""
        gmail_ish_service._service.users().messages().batchModify().execute.return_value = {}

        gmail_ish_service.batch_modify_messages(
            message_ids=['msg1', 'msg2', 'msg3'],
            add_labels=['STARRED'],
            remove_labels=['UNREAD']
        )

        call_args = gmail_ish_service._service.users().messages().batchModify.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['body']['ids'] == ['msg1', 'msg2', 'msg3']
        assert call_args[1]['body']['addLabelIds'] == ['STARRED']
        assert call_args[1]['body']['removeLabelIds'] == ['UNREAD']

    def test_batch_modify_messages_add_only(self, gmail_ish_service):
        """Batch modify should work with only add labels."""
        gmail_ish_service._service.users().messages().batchModify().execute.return_value = {}

        gmail_ish_service.batch_modify_messages(
            message_ids=['msg1', 'msg2'],
            add_labels=['IMPORTANT']
        )

        call_args = gmail_ish_service._service.users().messages().batchModify.call_args
        assert call_args[1]['body']['ids'] == ['msg1', 'msg2']
        assert call_args[1]['body']['addLabelIds'] == ['IMPORTANT']
        assert 'removeLabelIds' not in call_args[1]['body']

    def test_batch_modify_messages_remove_only(self, gmail_ish_service):
        """Batch modify should work with only remove labels."""
        gmail_ish_service._service.users().messages().batchModify().execute.return_value = {}

        gmail_ish_service.batch_modify_messages(
            message_ids=['msg1'],
            remove_labels=['SPAM']
        )

        call_args = gmail_ish_service._service.users().messages().batchModify.call_args
        assert call_args[1]['body']['ids'] == ['msg1']
        assert call_args[1]['body']['removeLabelIds'] == ['SPAM']
        assert 'addLabelIds' not in call_args[1]['body']


class TestBatchDeleteMessages:
    """Tests for batch deleting messages."""

    def test_batch_delete_messages(self, gmail_ish_service):
        """Batch delete should permanently delete multiple messages."""
        gmail_ish_service._service.users().messages().batchDelete().execute.return_value = {}

        gmail_ish_service.batch_delete_messages(['msg1', 'msg2', 'msg3'])

        call_args = gmail_ish_service._service.users().messages().batchDelete.call_args
        assert call_args[1]['userId'] == 'me'
        assert call_args[1]['body']['ids'] == ['msg1', 'msg2', 'msg3']

    def test_batch_delete_single_message(self, gmail_ish_service):
        """Batch delete should work with single message."""
        gmail_ish_service._service.users().messages().batchDelete().execute.return_value = {}

        gmail_ish_service.batch_delete_messages(['msg1'])

        call_args = gmail_ish_service._service.users().messages().batchDelete.call_args
        assert call_args[1]['body']['ids'] == ['msg1']
