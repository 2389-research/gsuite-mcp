# ABOUTME: Google Calendar API service for event management
# ABOUTME: Handles events, recurring events, and multi-calendar operations

import logging
import os
from datetime import datetime, timezone
from typing import List, Dict, Any, Optional, Union

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff
from gsuite_mcp.auth.fake_credentials import FakeCredentials

logger = logging.getLogger(__name__)


class CalendarService:
    """Service for Google Calendar API operations.

    Supports both production Google APIs and ish fake server for testing.
    """

    def __init__(
        self,
        credentials: Optional[Union[Credentials, FakeCredentials]] = None,
        api_base_url: Optional[str] = None,
        auth_token: Optional[str] = None,
    ):
        """Initialize Calendar service.

        Args:
            credentials: Valid OAuth2 credentials or FakeCredentials for ish mode
            api_base_url: Optional custom API base URL (for ish mode)
            auth_token: Optional Bearer token for simple auth (for ish mode)

        Environment variables for ish mode:
            ISH_MODE: Set to "true" to enable ish mode
            ISH_BASE_URL: Base URL for ish server (default: http://localhost:9000)
            ISH_USER: Username for ish authentication (default: testuser)
        """
        # Check for ish mode via environment
        ish_mode = os.getenv("ISH_MODE", "").lower() == "true"
        if ish_mode and not api_base_url:
            api_base_url = os.getenv("ISH_BASE_URL", "http://localhost:9000")

        # Create fake credentials if in ish mode and credentials not provided
        if api_base_url and not credentials and not auth_token:
            credentials = FakeCredentials()
        elif api_base_url and auth_token and not credentials:
            credentials = FakeCredentials(token=auth_token)

        if not credentials:
            raise ValueError("credentials are required unless using ish mode")

        # Build the service with custom discovery URL if in ish mode
        if api_base_url:
            # In ish mode, use custom base URL for discovery
            discovery_url = f"{api_base_url}/discovery/v1/apis/calendar/v3/rest"
            logger.info(f"Initializing Calendar service in ish mode with base URL: {api_base_url}")
            self._service = build(
                'calendar',
                'v3',
                credentials=credentials,
                discoveryServiceUrl=discovery_url,
                static_discovery=False,
            )
            self._ish_mode = True
            self._api_base_url = api_base_url
        else:
            # Production mode - use standard Google APIs
            self._service = build('calendar', 'v3', credentials=credentials)
            self._ish_mode = False
            self._api_base_url = None

        self._primary_calendar = 'primary'

    @retry_with_backoff
    def list_calendars(self) -> List[Dict[str, Any]]:
        """List all calendars accessible to the user.

        Returns:
            List of calendar metadata
        """
        try:
            response = self._service.calendarList().list().execute()
            return response.get('items', [])

        except HttpError as error:
            logger.error(f"Error listing calendars: {error}")
            raise

    @retry_with_backoff
    def list_events(
        self,
        calendar_id: str = 'primary',
        max_results: int = 100,
        time_min: Optional[datetime] = None,
        time_max: Optional[datetime] = None,
        query: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """List events in a calendar.

        Args:
            calendar_id: Calendar ID ('primary' for main calendar)
            max_results: Maximum number of events to return
            time_min: Lower bound for event start time
            time_max: Upper bound for event start time
            query: Free text search query

        Returns:
            List of event dictionaries
        """
        try:
            request_params = {
                'calendarId': calendar_id,
                'maxResults': max_results,
                'singleEvents': True,
                'orderBy': 'startTime',
            }

            if time_min:
                request_params['timeMin'] = time_min.isoformat()
            else:
                # Default to now
                request_params['timeMin'] = datetime.now(timezone.utc).isoformat()

            if time_max:
                request_params['timeMax'] = time_max.isoformat()

            if query:
                request_params['q'] = query

            response = self._service.events().list(**request_params).execute()
            return response.get('items', [])

        except HttpError as error:
            logger.error(f"Error listing events: {error}")
            raise

    @retry_with_backoff
    def get_event(
        self,
        event_id: str,
        calendar_id: str = 'primary',
    ) -> Dict[str, Any]:
        """Get a specific event by ID.

        Args:
            event_id: Event ID
            calendar_id: Calendar ID

        Returns:
            Event data dictionary
        """
        try:
            event = (
                self._service.events()
                .get(calendarId=calendar_id, eventId=event_id)
                .execute()
            )
            return event

        except HttpError as error:
            logger.error(f"Error getting event {event_id}: {error}")
            raise

    @retry_with_backoff
    def create_event(
        self,
        summary: str,
        start_time: datetime,
        end_time: datetime,
        calendar_id: str = 'primary',
        description: Optional[str] = None,
        location: Optional[str] = None,
        attendees: Optional[List[str]] = None,
        recurrence: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """Create a new calendar event.

        Args:
            summary: Event title
            start_time: Event start time
            end_time: Event end time
            calendar_id: Calendar ID to create event in
            description: Event description
            location: Event location
            attendees: List of attendee email addresses
            recurrence: Recurrence rules (RFC5545 format)

        Returns:
            Created event data
        """
        try:
            event = {
                'summary': summary,
                'start': {
                    'dateTime': start_time.isoformat(),
                    'timeZone': 'UTC',
                },
                'end': {
                    'dateTime': end_time.isoformat(),
                    'timeZone': 'UTC',
                },
            }

            if description:
                event['description'] = description

            if location:
                event['location'] = location

            if attendees:
                event['attendees'] = [{'email': email} for email in attendees]

            if recurrence:
                event['recurrence'] = recurrence

            created_event = (
                self._service.events()
                .insert(calendarId=calendar_id, body=event)
                .execute()
            )

            logger.info(f"Event created: {created_event['id']}")
            return created_event

        except HttpError as error:
            logger.error(f"Error creating event: {error}")
            raise

    @retry_with_backoff
    def update_event(
        self,
        event_id: str,
        calendar_id: str = 'primary',
        summary: Optional[str] = None,
        start_time: Optional[datetime] = None,
        end_time: Optional[datetime] = None,
        description: Optional[str] = None,
        location: Optional[str] = None,
        attendees: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """Update an existing event.

        Args:
            event_id: Event ID to update
            calendar_id: Calendar ID
            summary: New event title
            start_time: New start time
            end_time: New end time
            description: New description
            location: New location
            attendees: New attendee list

        Returns:
            Updated event data
        """
        try:
            # Get current event
            event = self.get_event(event_id, calendar_id)

            # Update fields
            if summary:
                event['summary'] = summary

            if start_time:
                event['start'] = {
                    'dateTime': start_time.isoformat(),
                    'timeZone': 'UTC',
                }

            if end_time:
                event['end'] = {
                    'dateTime': end_time.isoformat(),
                    'timeZone': 'UTC',
                }

            if description is not None:
                event['description'] = description

            if location is not None:
                event['location'] = location

            if attendees is not None:
                event['attendees'] = [{'email': email} for email in attendees]

            updated_event = (
                self._service.events()
                .update(calendarId=calendar_id, eventId=event_id, body=event)
                .execute()
            )

            logger.info(f"Event {event_id} updated")
            return updated_event

        except HttpError as error:
            logger.error(f"Error updating event {event_id}: {error}")
            raise

    @retry_with_backoff
    def delete_event(
        self,
        event_id: str,
        calendar_id: str = 'primary',
    ) -> None:
        """Delete an event.

        Args:
            event_id: Event ID to delete
            calendar_id: Calendar ID
        """
        try:
            self._service.events().delete(
                calendarId=calendar_id,
                eventId=event_id
            ).execute()

            logger.info(f"Event {event_id} deleted")

        except HttpError as error:
            logger.error(f"Error deleting event {event_id}: {error}")
            raise

    @retry_with_backoff
    def respond_to_event(
        self,
        event_id: str,
        response: str,
        calendar_id: str = 'primary',
    ) -> Dict[str, Any]:
        """Respond to an event invitation.

        Args:
            event_id: Event ID
            response: Response status ('accepted', 'declined', 'tentative')
            calendar_id: Calendar ID

        Returns:
            Updated event data
        """
        try:
            # Get current event
            event = self.get_event(event_id, calendar_id)

            # Find and update user's attendance status
            if 'attendees' in event:
                for attendee in event['attendees']:
                    if attendee.get('self', False):
                        attendee['responseStatus'] = response

            updated_event = (
                self._service.events()
                .update(calendarId=calendar_id, eventId=event_id, body=event)
                .execute()
            )

            logger.info(f"Responded '{response}' to event {event_id}")
            return updated_event

        except HttpError as error:
            logger.error(f"Error responding to event {event_id}: {error}")
            raise
