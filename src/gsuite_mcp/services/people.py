# ABOUTME: Google People API service for contact management
# ABOUTME: Handles contact CRUD, search, and contact group operations

import logging
import os
from typing import List, Dict, Any, Optional, Union

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff
from gsuite_mcp.auth.fake_credentials import FakeCredentials

logger = logging.getLogger(__name__)


class PeopleService:
    """Service for Google People API operations.

    Supports both production Google APIs and ish fake server for testing.
    """

    def __init__(
        self,
        credentials: Optional[Union[Credentials, FakeCredentials]] = None,
        api_base_url: Optional[str] = None,
        auth_token: Optional[str] = None,
    ):
        """Initialize People service.

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
            discovery_url = f"{api_base_url}/discovery/v1/apis/people/v1/rest"
            logger.info(f"Initializing People service in ish mode with base URL: {api_base_url}")
            self._service = build(
                'people',
                'v1',
                credentials=credentials,
                discoveryServiceUrl=discovery_url,
                static_discovery=False,
            )
            self._ish_mode = True
            self._api_base_url = api_base_url
        else:
            # Production mode - use standard Google APIs
            self._service = build('people', 'v1', credentials=credentials)
            self._ish_mode = False
            self._api_base_url = None

    @retry_with_backoff
    def list_contacts(
        self,
        page_size: int = 100,
        page_token: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """List user's contacts.

        Args:
            page_size: Number of contacts per page
            page_token: Token for pagination

        Returns:
            List of contact data dictionaries
        """
        try:
            request_params = {
                'resourceName': 'people/me',
                'pageSize': page_size,
                'personFields': 'names,emailAddresses,phoneNumbers,addresses,organizations',
            }

            if page_token:
                request_params['pageToken'] = page_token

            response = (
                self._service.people()
                .connections()
                .list(**request_params)
                .execute()
            )

            return response.get('connections', [])

        except HttpError as error:
            logger.error(f"Error listing contacts: {error}")
            raise

    @retry_with_backoff
    def get_contact(self, resource_name: str) -> Dict[str, Any]:
        """Get a specific contact by resource name.

        Args:
            resource_name: Contact resource name (e.g., 'people/c123')

        Returns:
            Contact data dictionary
        """
        try:
            contact = (
                self._service.people()
                .get(
                    resourceName=resource_name,
                    personFields='names,emailAddresses,phoneNumbers,addresses,organizations,biographies',
                )
                .execute()
            )

            return contact

        except HttpError as error:
            logger.error(f"Error getting contact {resource_name}: {error}")
            raise

    @retry_with_backoff
    def create_contact(
        self,
        given_name: str,
        family_name: Optional[str] = None,
        email: Optional[str] = None,
        phone: Optional[str] = None,
        organization: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Create a new contact.

        Args:
            given_name: First name
            family_name: Last name
            email: Email address
            phone: Phone number
            organization: Organization/company name

        Returns:
            Created contact data
        """
        try:
            contact = {'names': [{'givenName': given_name}]}

            if family_name:
                contact['names'][0]['familyName'] = family_name

            if email:
                contact['emailAddresses'] = [{'value': email}]

            if phone:
                contact['phoneNumbers'] = [{'value': phone}]

            if organization:
                contact['organizations'] = [{'name': organization}]

            created_contact = (
                self._service.people()
                .createContact(body=contact)
                .execute()
            )

            logger.info(f"Contact created: {created_contact['resourceName']}")
            return created_contact

        except HttpError as error:
            logger.error(f"Error creating contact: {error}")
            raise

    @retry_with_backoff
    def update_contact(
        self,
        resource_name: str,
        given_name: Optional[str] = None,
        family_name: Optional[str] = None,
        email: Optional[str] = None,
        phone: Optional[str] = None,
        organization: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Update an existing contact.

        Args:
            resource_name: Contact resource name
            given_name: New first name
            family_name: New last name
            email: New email address
            phone: New phone number
            organization: New organization

        Returns:
            Updated contact data
        """
        try:
            # Get current contact
            contact = self.get_contact(resource_name)

            # Update fields
            if given_name or family_name:
                if 'names' not in contact:
                    contact['names'] = [{}]

                if given_name:
                    contact['names'][0]['givenName'] = given_name

                if family_name:
                    contact['names'][0]['familyName'] = family_name

            if email is not None:
                contact['emailAddresses'] = [{'value': email}] if email else []

            if phone is not None:
                contact['phoneNumbers'] = [{'value': phone}] if phone else []

            if organization is not None:
                contact['organizations'] = [{'name': organization}] if organization else []

            # Update with field mask
            update_mask = 'names,emailAddresses,phoneNumbers,organizations'

            updated_contact = (
                self._service.people()
                .updateContact(
                    resourceName=resource_name,
                    body=contact,
                    updatePersonFields=update_mask,
                )
                .execute()
            )

            logger.info(f"Contact {resource_name} updated")
            return updated_contact

        except HttpError as error:
            logger.error(f"Error updating contact {resource_name}: {error}")
            raise

    @retry_with_backoff
    def delete_contact(self, resource_name: str) -> None:
        """Delete a contact.

        Args:
            resource_name: Contact resource name to delete
        """
        try:
            self._service.people().deleteContact(
                resourceName=resource_name
            ).execute()

            logger.info(f"Contact {resource_name} deleted")

        except HttpError as error:
            logger.error(f"Error deleting contact {resource_name}: {error}")
            raise

    @retry_with_backoff
    def search_contacts(
        self,
        query: str,
        page_size: int = 30,
    ) -> List[Dict[str, Any]]:
        """Search contacts by name or email.

        Args:
            query: Search query
            page_size: Maximum results to return

        Returns:
            List of matching contacts
        """
        try:
            response = (
                self._service.people()
                .searchContacts(
                    query=query,
                    pageSize=page_size,
                    readMask='names,emailAddresses,phoneNumbers',
                )
                .execute()
            )

            return response.get('results', [])

        except HttpError as error:
            logger.error(f"Error searching contacts: {error}")
            raise

    @retry_with_backoff
    def list_contact_groups(self) -> List[Dict[str, Any]]:
        """List contact groups.

        Returns:
            List of contact group data
        """
        try:
            response = (
                self._service.contactGroups()
                .list()
                .execute()
            )

            return response.get('contactGroups', [])

        except HttpError as error:
            logger.error(f"Error listing contact groups: {error}")
            raise

    @retry_with_backoff
    def create_contact_group(self, name: str) -> Dict[str, Any]:
        """Create a new contact group.

        Args:
            name: Group name

        Returns:
            Created group data
        """
        try:
            group = {'contactGroup': {'name': name}}

            created_group = (
                self._service.contactGroups()
                .create(body=group)
                .execute()
            )

            logger.info(f"Contact group created: {created_group['resourceName']}")
            return created_group

        except HttpError as error:
            logger.error(f"Error creating contact group: {error}")
            raise

    @retry_with_backoff
    def add_contact_to_group(
        self,
        group_resource_name: str,
        contact_resource_names: List[str],
    ) -> Dict[str, Any]:
        """Add contacts to a group.

        Args:
            group_resource_name: Group resource name
            contact_resource_names: List of contact resource names to add

        Returns:
            Updated group data
        """
        try:
            body = {'resourceNamesToAdd': contact_resource_names}

            result = (
                self._service.contactGroups()
                .members()
                .modify(resourceName=group_resource_name, body=body)
                .execute()
            )

            logger.info(f"Added {len(contact_resource_names)} contacts to group")
            return result

        except HttpError as error:
            logger.error(f"Error adding contacts to group: {error}")
            raise
