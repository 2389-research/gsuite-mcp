# ABOUTME: Integration tests for full workflows
# ABOUTME: Tests realistic end-to-end scenarios across services

import pytest
from datetime import datetime, timedelta
from unittest.mock import patch, Mock


@pytest.mark.integration
class TestEmailWorkflow:
    """Test realistic email workflows."""

    def test_receive_and_reply_workflow(self, gmail_service):
        """Test receiving email and replying."""
        # Mock listing unread messages
        gmail_service._service.users().messages().list().execute.return_value = {
            'messages': [{'id': 'msg123', 'threadId': 'thread123'}]
        }

        # Mock getting message
        gmail_service._service.users().messages().get().execute.return_value = {
            'id': 'msg123',
            'threadId': 'thread123',
            'payload': {
                'headers': [
                    {'name': 'From', 'value': 'sender@example.com'},
                    {'name': 'Subject', 'value': 'Test'},
                ]
            }
        }

        # Mock sending reply
        gmail_service._service.users().messages().send().execute.return_value = {
            'id': 'reply456'
        }

        # Execute workflow
        unread = gmail_service.list_messages(query="is:unread")
        assert len(unread) == 1

        message = gmail_service.get_message(unread[0]['id'])
        assert message['id'] == 'msg123'

        reply = gmail_service.reply_to_message('msg123', 'Thanks!')
        assert reply['id'] == 'reply456'


@pytest.mark.integration
class TestCalendarWorkflow:
    """Test realistic calendar workflows."""

    def test_create_and_update_event(self, calendar_service):
        """Test creating and updating calendar event."""
        # Mock event creation
        calendar_service._service.events().insert().execute.return_value = {
            'id': 'event123',
            'summary': 'Meeting',
        }

        # Mock event update
        calendar_service._service.events().get().execute.return_value = {
            'id': 'event123',
            'summary': 'Meeting',
        }

        calendar_service._service.events().update().execute.return_value = {
            'id': 'event123',
            'summary': 'Updated Meeting',
        }

        # Execute workflow
        start = datetime.now()
        end = start + timedelta(hours=1)

        event = calendar_service.create_event(
            summary='Meeting',
            start_time=start,
            end_time=end,
        )
        assert event['id'] == 'event123'

        updated = calendar_service.update_event(
            event_id='event123',
            summary='Updated Meeting',
        )
        assert updated['summary'] == 'Updated Meeting'


@pytest.mark.integration
class TestContactEmailWorkflow:
    """Test workflows combining contacts and email."""

    def test_find_contact_and_email(self, people_service, gmail_service):
        """Test finding contact and sending email."""
        # Mock contact search
        people_service._service.people().searchContacts().execute.return_value = {
            'results': [{
                'person': {
                    'resourceName': 'people/c123',
                    'names': [{'displayName': 'John Doe'}],
                    'emailAddresses': [{'value': 'john@example.com'}],
                }
            }]
        }

        # Mock sending email
        gmail_service._service.users().messages().send().execute.return_value = {
            'id': 'msg456'
        }

        # Execute workflow
        results = people_service.search_contacts('John')
        assert len(results) == 1

        contact = results[0]['person']
        email = contact['emailAddresses'][0]['value']

        sent = gmail_service.send_message(
            to=email,
            subject='Hello',
            body='Hi John!',
        )
        assert sent['id'] == 'msg456'


@pytest.fixture
def gmail_service():
    """Mock Gmail service."""
    from gsuite_mcp.services.gmail import GmailService
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        service = GmailService(Mock())
        service._service = mock_build.return_value
        return service


@pytest.fixture
def calendar_service():
    """Mock Calendar service."""
    from gsuite_mcp.services.calendar import CalendarService
    with patch('gsuite_mcp.services.calendar.build') as mock_build:
        service = CalendarService(Mock())
        service._service = mock_build.return_value
        return service


@pytest.fixture
def people_service():
    """Mock People service."""
    from gsuite_mcp.services.people import PeopleService
    with patch('gsuite_mcp.services.people.build') as mock_build:
        service = PeopleService(Mock())
        service._service = mock_build.return_value
        return service
