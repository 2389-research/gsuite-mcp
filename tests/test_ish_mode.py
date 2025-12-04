# ABOUTME: Tests for ish fake Google API server integration
# ABOUTME: Validates ish mode configuration and fake credentials usage

import os
import pytest
from unittest.mock import Mock, patch, MagicMock
from gsuite_mcp.services.gmail import GmailService
from gsuite_mcp.auth.fake_credentials import FakeCredentials


class TestFakeCredentials:
    """Test fake credentials for ish mode."""

    def test_fake_credentials_with_token(self):
        """FakeCredentials should accept explicit token."""
        creds = FakeCredentials(token="user:alice")
        assert creds.token == "user:alice"
        assert creds.valid is True
        assert creds.expired is False

    def test_fake_credentials_with_user(self):
        """FakeCredentials should format token from username."""
        creds = FakeCredentials(user="bob")
        assert creds.token == "user:bob"

    def test_fake_credentials_from_env(self):
        """FakeCredentials should read ISH_USER from environment."""
        with patch.dict(os.environ, {"ISH_USER": "charlie"}):
            creds = FakeCredentials()
            assert creds.token == "user:charlie"

    def test_fake_credentials_default(self):
        """FakeCredentials should use default user if nothing provided."""
        with patch.dict(os.environ, {}, clear=True):
            creds = FakeCredentials()
            assert creds.token == "user:testuser"

    def test_fake_credentials_apply_headers(self):
        """FakeCredentials should apply Bearer token to headers."""
        creds = FakeCredentials(token="user:dave")
        headers = {}
        creds.apply(headers)
        assert headers['Authorization'] == 'Bearer user:dave'

    def test_fake_credentials_refresh_noop(self):
        """FakeCredentials refresh should be a no-op."""
        creds = FakeCredentials()
        creds.refresh(None)  # Should not raise


class TestGmailServiceIshMode:
    """Test Gmail service with ish mode enabled."""

    @patch('gsuite_mcp.services.gmail.build')
    def test_gmail_service_with_api_base_url(self, mock_build):
        """GmailService should use custom discovery URL when api_base_url provided."""
        creds = FakeCredentials(user="testuser")
        service = GmailService(
            credentials=creds,
            api_base_url="http://localhost:9000"
        )

        # Verify build was called with custom discoveryServiceUrl
        mock_build.assert_called_once()
        call_kwargs = mock_build.call_args[1]
        assert 'discoveryServiceUrl' in call_kwargs
        assert call_kwargs['discoveryServiceUrl'] == "http://localhost:9000/discovery/v1/apis/gmail/v1/rest"
        assert call_kwargs['static_discovery'] is False
        assert service._ish_mode is True
        assert service._api_base_url == "http://localhost:9000"

    @patch('gsuite_mcp.services.gmail.build')
    def test_gmail_service_with_auth_token(self, mock_build):
        """GmailService should create FakeCredentials from auth_token."""
        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:admin"
        )

        # Verify credentials were created
        mock_build.assert_called_once()
        call_args = mock_build.call_args
        credentials_arg = call_args[1]['credentials']
        assert isinstance(credentials_arg, FakeCredentials)
        assert credentials_arg.token == "user:admin"

    @patch('gsuite_mcp.services.gmail.build')
    def test_gmail_service_env_ish_mode(self, mock_build):
        """GmailService should auto-enable ish mode from ISH_MODE env var."""
        with patch.dict(os.environ, {
            "ISH_MODE": "true",
            "ISH_BASE_URL": "http://localhost:8888",
            "ISH_USER": "envuser"
        }):
            service = GmailService()

            # Verify ish mode was enabled from environment
            call_kwargs = mock_build.call_args[1]
            assert 'discoveryServiceUrl' in call_kwargs
            assert "http://localhost:8888" in call_kwargs['discoveryServiceUrl']
            assert service._ish_mode is True

    @patch('gsuite_mcp.services.gmail.build')
    def test_gmail_service_env_ish_mode_default_url(self, mock_build):
        """GmailService should use default ish URL if ISH_BASE_URL not set."""
        with patch.dict(os.environ, {"ISH_MODE": "true"}, clear=True):
            service = GmailService()

            # Verify default ish URL was used
            call_kwargs = mock_build.call_args[1]
            assert "http://localhost:9000" in call_kwargs['discoveryServiceUrl']

    @patch('gsuite_mcp.services.gmail.build')
    def test_gmail_service_production_mode(self, mock_build):
        """GmailService should work in production mode without ish settings."""
        from google.oauth2.credentials import Credentials

        # Create a real OAuth credential mock
        creds = Mock(spec=Credentials)
        service = GmailService(credentials=creds)

        # Verify standard build was used (no discoveryServiceUrl)
        call_kwargs = mock_build.call_args[1]
        assert 'discoveryServiceUrl' not in call_kwargs
        assert service._ish_mode is False
        assert service._api_base_url is None

    def test_gmail_service_requires_credentials_in_production(self):
        """GmailService should require credentials when not in ish mode."""
        with patch.dict(os.environ, {}, clear=True):
            with pytest.raises(ValueError, match="credentials are required"):
                GmailService()


class TestGmailServiceIshIntegration:
    """Integration tests for Gmail service with mocked ish responses."""

    @patch('gsuite_mcp.services.gmail.build')
    def test_list_messages_ish_mode(self, mock_build):
        """Test listing messages in ish mode."""
        # Setup mock service
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        # Mock ish server response
        mock_response = {
            'messages': [
                {'id': 'ish_msg_1', 'threadId': 'ish_thread_1'},
                {'id': 'ish_msg_2', 'threadId': 'ish_thread_2'},
            ]
        }
        mock_service.users().messages().list().execute.return_value = mock_response

        # Create service in ish mode
        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        # Call list_messages
        messages = service.list_messages(query="is:unread")

        # Verify results
        assert len(messages) == 2
        assert messages[0]['id'] == 'ish_msg_1'
        assert messages[1]['id'] == 'ish_msg_2'

    @patch('gsuite_mcp.services.gmail.build')
    def test_send_message_ish_mode(self, mock_build):
        """Test sending message in ish mode."""
        # Setup mock service
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        # Mock ish server response
        mock_response = {'id': 'ish_sent_msg_123', 'threadId': 'ish_thread_123'}
        mock_service.users().messages().send().execute.return_value = mock_response

        # Create service in ish mode
        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:sender"
        )

        # Send a message
        result = service.send_message(
            to="recipient@example.com",
            subject="Test from ish",
            body="This is a test message"
        )

        # Verify the message was sent
        assert result['id'] == 'ish_sent_msg_123'
        # Verify send was called with correct parameters
        send_call_args = mock_service.users().messages().send.call_args
        assert send_call_args[1]['userId'] == 'me'
        assert 'raw' in send_call_args[1]['body']

    @patch('gsuite_mcp.services.gmail.build')
    def test_get_message_ish_mode(self, mock_build):
        """Test getting message details in ish mode."""
        # Setup mock service
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        # Mock ish server response
        mock_response = {
            'id': 'ish_msg_456',
            'threadId': 'ish_thread_456',
            'payload': {
                'headers': [
                    {'name': 'Subject', 'value': 'Test Subject'},
                    {'name': 'From', 'value': 'sender@example.com'}
                ]
            }
        }
        mock_service.users().messages().get().execute.return_value = mock_response

        # Create service in ish mode
        service = GmailService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        # Get message
        message = service.get_message('ish_msg_456')

        # Verify message data
        assert message['id'] == 'ish_msg_456'
        assert message['payload']['headers'][0]['value'] == 'Test Subject'
