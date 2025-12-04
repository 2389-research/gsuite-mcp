# ABOUTME: Tests for Google Calendar service operations
# ABOUTME: Validates event CRUD, recurring events, and multi-calendar support

import os
import pytest
from datetime import datetime
from unittest.mock import Mock, patch, MagicMock
from gsuite_mcp.services.calendar import CalendarService
from gsuite_mcp.auth.fake_credentials import FakeCredentials


@pytest.fixture
def calendar_service():
    """Create Calendar service with mocked credentials."""
    with patch('gsuite_mcp.services.calendar.build') as mock_build:
        service = CalendarService(Mock())
        service._service = mock_build.return_value
        return service


def test_list_events(calendar_service):
    """List events should return upcoming events."""
    mock_response = {
        'items': [
            {
                'id': 'event1',
                'summary': 'Team Meeting',
                'start': {'dateTime': '2025-12-05T10:00:00Z'},
            },
            {
                'id': 'event2',
                'summary': 'Lunch',
                'start': {'dateTime': '2025-12-05T12:00:00Z'},
            },
        ]
    }
    calendar_service._service.events().list().execute.return_value = mock_response

    events = calendar_service.list_events(max_results=10)

    assert len(events) == 2
    assert events[0]['summary'] == 'Team Meeting'


def test_create_event(calendar_service):
    """Create event should create new calendar event."""
    mock_response = {
        'id': 'event3',
        'summary': 'New Meeting',
        'start': {'dateTime': '2025-12-06T14:00:00Z'},
        'end': {'dateTime': '2025-12-06T15:00:00Z'},
    }
    calendar_service._service.events().insert().execute.return_value = mock_response

    event = calendar_service.create_event(
        summary='New Meeting',
        start_time=datetime(2025, 12, 6, 14, 0, 0),
        end_time=datetime(2025, 12, 6, 15, 0, 0),
    )

    assert event['id'] == 'event3'
    assert event['summary'] == 'New Meeting'


def test_list_calendars(calendar_service):
    """List calendars should return all accessible calendars."""
    mock_response = {
        'items': [
            {
                'id': 'primary',
                'summary': 'Test User',
                'primary': True,
            },
            {
                'id': 'calendar2@group.calendar.google.com',
                'summary': 'Team Calendar',
                'primary': False,
            },
        ]
    }
    calendar_service._service.calendarList().list().execute.return_value = mock_response

    calendars = calendar_service.list_calendars()

    assert len(calendars) == 2
    assert calendars[0]['id'] == 'primary'
    assert calendars[1]['summary'] == 'Team Calendar'


def test_get_event(calendar_service):
    """Get event should return specific event by ID."""
    mock_response = {
        'id': 'event_abc',
        'summary': 'Important Meeting',
        'start': {'dateTime': '2025-12-07T09:00:00Z'},
        'end': {'dateTime': '2025-12-07T10:00:00Z'},
    }
    calendar_service._service.events().get().execute.return_value = mock_response

    event = calendar_service.get_event('event_abc')

    assert event['id'] == 'event_abc'
    assert event['summary'] == 'Important Meeting'


def test_update_event(calendar_service):
    """Update event should modify existing event."""
    # Mock get_event (called internally by update_event)
    existing_event = {
        'id': 'event_xyz',
        'summary': 'Old Title',
        'start': {'dateTime': '2025-12-08T10:00:00Z'},
        'end': {'dateTime': '2025-12-08T11:00:00Z'},
    }
    calendar_service._service.events().get().execute.return_value = existing_event

    updated_event = {
        'id': 'event_xyz',
        'summary': 'Updated Title',
        'start': {'dateTime': '2025-12-08T10:00:00Z'},
        'end': {'dateTime': '2025-12-08T11:00:00Z'},
    }
    calendar_service._service.events().update().execute.return_value = updated_event

    result = calendar_service.update_event('event_xyz', summary='Updated Title')

    assert result['summary'] == 'Updated Title'


def test_delete_event(calendar_service):
    """Delete event should remove event from calendar."""
    calendar_service._service.events().delete().execute.return_value = None

    # Should not raise exception
    calendar_service.delete_event('event_to_delete')


def test_respond_to_event(calendar_service):
    """Respond to event should update attendance status."""
    # Mock get_event
    existing_event = {
        'id': 'event_invite',
        'summary': 'Meeting Invitation',
        'attendees': [
            {'email': 'other@example.com', 'responseStatus': 'needsAction'},
            {'email': 'me@example.com', 'self': True, 'responseStatus': 'needsAction'},
        ],
    }
    calendar_service._service.events().get().execute.return_value = existing_event

    updated_event = existing_event.copy()
    updated_event['attendees'][1]['responseStatus'] = 'accepted'
    calendar_service._service.events().update().execute.return_value = updated_event

    result = calendar_service.respond_to_event('event_invite', 'accepted')

    assert result['attendees'][1]['responseStatus'] == 'accepted'


class TestCalendarServiceIshMode:
    """Test Calendar service with ish mode enabled."""

    @patch('gsuite_mcp.services.calendar.build')
    def test_calendar_service_with_api_base_url(self, mock_build):
        """CalendarService should use custom discovery URL when api_base_url provided."""
        creds = FakeCredentials(user="testuser")
        service = CalendarService(
            credentials=creds,
            api_base_url="http://localhost:9000"
        )

        # Verify build was called with custom discoveryServiceUrl
        mock_build.assert_called_once()
        call_kwargs = mock_build.call_args[1]
        assert 'discoveryServiceUrl' in call_kwargs
        assert call_kwargs['discoveryServiceUrl'] == "http://localhost:9000/discovery/v1/apis/calendar/v3/rest"
        assert call_kwargs['static_discovery'] is False
        assert service._ish_mode is True
        assert service._api_base_url == "http://localhost:9000"

    @patch('gsuite_mcp.services.calendar.build')
    def test_calendar_service_with_auth_token(self, mock_build):
        """CalendarService should create FakeCredentials from auth_token."""
        service = CalendarService(
            api_base_url="http://localhost:9000",
            auth_token="user:admin"
        )

        # Verify credentials were created
        mock_build.assert_called_once()
        call_args = mock_build.call_args
        credentials_arg = call_args[1]['credentials']
        assert isinstance(credentials_arg, FakeCredentials)
        assert credentials_arg.token == "user:admin"

    @patch('gsuite_mcp.services.calendar.build')
    def test_calendar_service_env_ish_mode(self, mock_build):
        """CalendarService should auto-enable ish mode from ISH_MODE env var."""
        with patch.dict(os.environ, {
            "ISH_MODE": "true",
            "ISH_BASE_URL": "http://localhost:8888",
            "ISH_USER": "envuser"
        }):
            service = CalendarService()

            # Verify ish mode was enabled from environment
            call_kwargs = mock_build.call_args[1]
            assert 'discoveryServiceUrl' in call_kwargs
            assert "http://localhost:8888" in call_kwargs['discoveryServiceUrl']
            assert service._ish_mode is True

    @patch('gsuite_mcp.services.calendar.build')
    def test_calendar_service_production_mode(self, mock_build):
        """CalendarService should work in production mode without ish settings."""
        from google.oauth2.credentials import Credentials

        # Create a real OAuth credential mock
        creds = Mock(spec=Credentials)
        service = CalendarService(credentials=creds)

        # Verify standard build was used (no discoveryServiceUrl)
        call_kwargs = mock_build.call_args[1]
        assert 'discoveryServiceUrl' not in call_kwargs
        assert service._ish_mode is False
        assert service._api_base_url is None

    def test_calendar_service_requires_credentials_in_production(self):
        """CalendarService should require credentials when not in ish mode."""
        with patch.dict(os.environ, {}, clear=True):
            with pytest.raises(ValueError, match="credentials are required"):
                CalendarService()

    @patch('gsuite_mcp.services.calendar.build')
    def test_list_events_ish_mode(self, mock_build):
        """Test listing events in ish mode."""
        # Setup mock service
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        # Mock ish server response
        mock_response = {
            'items': [
                {
                    'id': 'ish_event_1',
                    'summary': 'Ish Meeting',
                    'start': {'dateTime': '2025-12-05T10:00:00Z'},
                },
            ]
        }
        mock_service.events().list().execute.return_value = mock_response

        # Create service in ish mode
        service = CalendarService(
            api_base_url="http://localhost:9000",
            auth_token="user:testuser"
        )

        # Call list_events
        events = service.list_events()

        # Verify results
        assert len(events) == 1
        assert events[0]['summary'] == 'Ish Meeting'

    @patch('gsuite_mcp.services.calendar.build')
    def test_create_event_ish_mode(self, mock_build):
        """Test creating event in ish mode."""
        # Setup mock service
        mock_service = MagicMock()
        mock_build.return_value = mock_service

        # Mock ish server response
        mock_response = {
            'id': 'ish_created_event',
            'summary': 'Ish Event',
            'start': {'dateTime': '2025-12-10T14:00:00Z'},
            'end': {'dateTime': '2025-12-10T15:00:00Z'},
        }
        mock_service.events().insert().execute.return_value = mock_response

        # Create service in ish mode
        service = CalendarService(
            api_base_url="http://localhost:9000",
            auth_token="user:creator"
        )

        # Create an event
        result = service.create_event(
            summary='Ish Event',
            start_time=datetime(2025, 12, 10, 14, 0, 0),
            end_time=datetime(2025, 12, 10, 15, 0, 0),
        )

        # Verify the event was created
        assert result['id'] == 'ish_created_event'
        assert result['summary'] == 'Ish Event'
