# ABOUTME: Tests for Gmail service operations
# ABOUTME: Validates email management, search, labels, and drafts

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.services.gmail import GmailService


@pytest.fixture
def mock_credentials():
    """Mock OAuth credentials for testing."""
    return Mock()


@pytest.fixture
def gmail_service(mock_credentials):
    """Create Gmail service with mocked credentials."""
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        service = GmailService(mock_credentials)
        service._service = mock_build.return_value
        return service


def test_list_messages(gmail_service):
    """List messages should return message metadata."""
    mock_response = {
        'messages': [
            {'id': 'msg1', 'threadId': 'thread1'},
            {'id': 'msg2', 'threadId': 'thread2'},
        ]
    }
    gmail_service._service.users().messages().list().execute.return_value = mock_response

    messages = gmail_service.list_messages(max_results=10)

    assert len(messages) == 2
    assert messages[0]['id'] == 'msg1'


def test_list_messages_with_query(gmail_service):
    """List messages should support query filters."""
    mock_list = gmail_service._service.users().messages().list
    mock_list.return_value.execute.return_value = {'messages': []}

    gmail_service.list_messages(query="is:unread from:example@test.com")

    # Verify list was called with the query parameter
    calls = mock_list.call_args_list
    assert len(calls) == 1
    assert calls[0][1]['q'] == "is:unread from:example@test.com"
