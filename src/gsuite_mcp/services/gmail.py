# ABOUTME: Gmail API service for email management operations
# ABOUTME: Handles reading, sending, searching, labels, drafts, and attachments

import base64
import logging
import os
from pathlib import Path
from typing import List, Dict, Any, Optional, Union
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.mime.base import MIMEBase
from email import encoders

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff
from gsuite_mcp.auth.fake_credentials import FakeCredentials

logger = logging.getLogger(__name__)


class GmailService:
    """Service for Gmail API operations.

    Supports both production Google APIs and ish fake server for testing.
    """

    def __init__(
        self,
        credentials: Optional[Union[Credentials, FakeCredentials]] = None,
        api_base_url: Optional[str] = None,
        auth_token: Optional[str] = None,
    ):
        """Initialize Gmail service.

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
            discovery_url = f"{api_base_url}/discovery/v1/apis/gmail/v1/rest"
            logger.info(f"Initializing Gmail service in ish mode with base URL: {api_base_url}")
            self._service = build(
                'gmail',
                'v1',
                credentials=credentials,
                discoveryServiceUrl=discovery_url,
                static_discovery=False,
            )
            self._ish_mode = True
            self._api_base_url = api_base_url
        else:
            # Production mode - use standard Google APIs
            self._service = build('gmail', 'v1', credentials=credentials)
            self._ish_mode = False
            self._api_base_url = None

        self._user_id = 'me'

    @retry_with_backoff
    def list_messages(
        self,
        query: str = "",
        max_results: int = 100,
        label_ids: Optional[List[str]] = None,
    ) -> List[Dict[str, Any]]:
        """List messages matching query.

        Args:
            query: Gmail search query (e.g., "is:unread from:example@test.com")
            max_results: Maximum number of messages to return
            label_ids: Filter by label IDs (e.g., ["INBOX", "UNREAD"])

        Returns:
            List of message metadata dictionaries
        """
        try:
            request_params = {
                'userId': self._user_id,
                'maxResults': max_results,
            }

            if query:
                request_params['q'] = query

            if label_ids:
                request_params['labelIds'] = label_ids

            response = self._service.users().messages().list(**request_params).execute()

            messages = response.get('messages', [])

            # Handle pagination if needed
            while 'nextPageToken' in response and len(messages) < max_results:
                request_params['pageToken'] = response['nextPageToken']
                response = self._service.users().messages().list(**request_params).execute()
                messages.extend(response.get('messages', []))

            return messages[:max_results]

        except HttpError as error:
            logger.error(f"Error listing messages: {error}")
            raise

    @retry_with_backoff
    def get_message(
        self,
        message_id: str,
        format: str = "full",
    ) -> Dict[str, Any]:
        """Get a specific message by ID.

        Args:
            message_id: The message ID
            format: Message format ('minimal', 'full', 'raw', 'metadata')

        Returns:
            Message data dictionary
        """
        try:
            message = (
                self._service.users()
                .messages()
                .get(userId=self._user_id, id=message_id, format=format)
                .execute()
            )
            return message

        except HttpError as error:
            logger.error(f"Error getting message {message_id}: {error}")
            raise

    @retry_with_backoff
    def send_message(
        self,
        to: str,
        subject: str,
        body: str,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, Any]:
        """Send an email message.

        Args:
            to: Recipient email address
            subject: Email subject
            body: Email body content
            cc: CC recipients (comma-separated)
            bcc: BCC recipients (comma-separated)
            html: If True, body is HTML; otherwise plain text

        Returns:
            Sent message metadata
        """
        try:
            message = self._create_message(
                to=to,
                subject=subject,
                body=body,
                cc=cc,
                bcc=bcc,
                html=html,
            )

            sent_message = (
                self._service.users()
                .messages()
                .send(userId=self._user_id, body=message)
                .execute()
            )

            logger.info(f"Message sent successfully. ID: {sent_message['id']}")
            return sent_message

        except HttpError as error:
            logger.error(f"Error sending message: {error}")
            raise

    def _create_message(
        self,
        to: str,
        subject: str,
        body: str,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, str]:
        """Create a message for sending.

        Args:
            to: Recipient email
            subject: Email subject
            body: Email body
            cc: CC recipients
            bcc: BCC recipients
            html: Whether body is HTML

        Returns:
            Message dictionary ready for API
        """
        if html:
            message = MIMEMultipart('alternative')
            part = MIMEText(body, 'html')
            message.attach(part)
        else:
            message = MIMEText(body)

        message['to'] = to
        message['subject'] = subject

        if cc:
            message['cc'] = cc

        if bcc:
            message['bcc'] = bcc

        raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
        return {'raw': raw_message}

    @retry_with_backoff
    def reply_to_message(
        self,
        message_id: str,
        body: str,
        html: bool = False,
    ) -> Dict[str, Any]:
        """Reply to an existing message.

        Args:
            message_id: ID of message to reply to
            body: Reply body content
            html: If True, body is HTML

        Returns:
            Sent reply metadata
        """
        try:
            # Get original message to extract headers
            original = self.get_message(message_id, format='metadata')
            headers = original['payload']['headers']

            # Extract necessary headers
            to = self._get_header(headers, 'From')
            subject = self._get_header(headers, 'Subject')
            thread_id = original['threadId']

            # Add "Re:" prefix if not present
            if not subject.lower().startswith('re:'):
                subject = f"Re: {subject}"

            # Create reply message
            if html:
                message = MIMEMultipart('alternative')
                part = MIMEText(body, 'html')
                message.attach(part)
            else:
                message = MIMEText(body)

            message['to'] = to
            message['subject'] = subject
            message['In-Reply-To'] = message_id
            message['References'] = message_id

            raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()

            # Send with threadId to maintain conversation
            reply = (
                self._service.users()
                .messages()
                .send(
                    userId=self._user_id,
                    body={'raw': raw_message, 'threadId': thread_id}
                )
                .execute()
            )

            logger.info(f"Reply sent successfully. ID: {reply['id']}")
            return reply

        except HttpError as error:
            logger.error(f"Error replying to message {message_id}: {error}")
            raise

    @retry_with_backoff
    def modify_message(
        self,
        message_id: str,
        add_labels: Optional[List[str]] = None,
        remove_labels: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """Modify labels on a message.

        Args:
            message_id: Message ID to modify
            add_labels: Label IDs to add (e.g., ["UNREAD", "STARRED"])
            remove_labels: Label IDs to remove

        Returns:
            Modified message metadata
        """
        try:
            body = {}

            if add_labels:
                body['addLabelIds'] = add_labels

            if remove_labels:
                body['removeLabelIds'] = remove_labels

            modified = (
                self._service.users()
                .messages()
                .modify(userId=self._user_id, id=message_id, body=body)
                .execute()
            )

            return modified

        except HttpError as error:
            logger.error(f"Error modifying message {message_id}: {error}")
            raise

    def mark_as_read(self, message_id: str) -> Dict[str, Any]:
        """Mark a message as read."""
        return self.modify_message(message_id, remove_labels=['UNREAD'])

    def mark_as_unread(self, message_id: str) -> Dict[str, Any]:
        """Mark a message as unread."""
        return self.modify_message(message_id, add_labels=['UNREAD'])

    def star_message(self, message_id: str) -> Dict[str, Any]:
        """Star a message."""
        return self.modify_message(message_id, add_labels=['STARRED'])

    def unstar_message(self, message_id: str) -> Dict[str, Any]:
        """Unstar a message."""
        return self.modify_message(message_id, remove_labels=['STARRED'])

    def archive_message(self, message_id: str) -> Dict[str, Any]:
        """Archive a message (remove from inbox)."""
        return self.modify_message(message_id, remove_labels=['INBOX'])

    @retry_with_backoff
    def delete_message(self, message_id: str) -> None:
        """Permanently delete a message.

        Args:
            message_id: Message ID to delete
        """
        try:
            self._service.users().messages().delete(
                userId=self._user_id,
                id=message_id
            ).execute()

            logger.info(f"Message {message_id} deleted successfully")

        except HttpError as error:
            logger.error(f"Error deleting message {message_id}: {error}")
            raise

    @retry_with_backoff
    def trash_message(self, message_id: str) -> Dict[str, Any]:
        """Move a message to trash.

        Args:
            message_id: Message ID to trash

        Returns:
            Trashed message metadata
        """
        try:
            trashed = (
                self._service.users()
                .messages()
                .trash(userId=self._user_id, id=message_id)
                .execute()
            )

            return trashed

        except HttpError as error:
            logger.error(f"Error trashing message {message_id}: {error}")
            raise

    @retry_with_backoff
    def list_labels(self) -> List[Dict[str, Any]]:
        """List all labels in the user's mailbox.

        Returns:
            List of label dictionaries
        """
        try:
            response = self._service.users().labels().list(userId=self._user_id).execute()
            return response.get('labels', [])

        except HttpError as error:
            logger.error(f"Error listing labels: {error}")
            raise

    @retry_with_backoff
    def get_label(self, label_id: str) -> Dict[str, Any]:
        """Get a specific label by ID.

        Args:
            label_id: The label ID

        Returns:
            Label data dictionary
        """
        try:
            label = (
                self._service.users()
                .labels()
                .get(userId=self._user_id, id=label_id)
                .execute()
            )
            return label

        except HttpError as error:
            logger.error(f"Error getting label {label_id}: {error}")
            raise

    @retry_with_backoff
    def create_label(
        self,
        name: str,
        label_list_visibility: str = "labelShow",
        message_list_visibility: str = "show",
    ) -> Dict[str, Any]:
        """Create a new label.

        Args:
            name: Label name
            label_list_visibility: Visibility in label list ('labelShow', 'labelHide')
            message_list_visibility: Visibility in message list ('show', 'hide')

        Returns:
            Created label data
        """
        try:
            label_object = {
                'name': name,
                'labelListVisibility': label_list_visibility,
                'messageListVisibility': message_list_visibility,
            }

            created_label = (
                self._service.users()
                .labels()
                .create(userId=self._user_id, body=label_object)
                .execute()
            )

            logger.info(f"Label '{name}' created with ID: {created_label['id']}")
            return created_label

        except HttpError as error:
            logger.error(f"Error creating label '{name}': {error}")
            raise

    @retry_with_backoff
    def update_label(
        self,
        label_id: str,
        name: Optional[str] = None,
        label_list_visibility: Optional[str] = None,
        message_list_visibility: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Update an existing label.

        Args:
            label_id: Label ID to update
            name: New label name
            label_list_visibility: New label list visibility
            message_list_visibility: New message list visibility

        Returns:
            Updated label data
        """
        try:
            # Get current label to preserve unchanged fields
            current_label = self.get_label(label_id)

            label_object = {
                'name': name or current_label['name'],
                'id': label_id,
            }

            if label_list_visibility:
                label_object['labelListVisibility'] = label_list_visibility

            if message_list_visibility:
                label_object['messageListVisibility'] = message_list_visibility

            updated_label = (
                self._service.users()
                .labels()
                .update(userId=self._user_id, id=label_id, body=label_object)
                .execute()
            )

            logger.info(f"Label {label_id} updated successfully")
            return updated_label

        except HttpError as error:
            logger.error(f"Error updating label {label_id}: {error}")
            raise

    @retry_with_backoff
    def delete_label(self, label_id: str) -> None:
        """Delete a label.

        Args:
            label_id: Label ID to delete
        """
        try:
            self._service.users().labels().delete(
                userId=self._user_id,
                id=label_id
            ).execute()

            logger.info(f"Label {label_id} deleted successfully")

        except HttpError as error:
            logger.error(f"Error deleting label {label_id}: {error}")
            raise

    @retry_with_backoff
    def list_drafts(self, max_results: int = 100) -> List[Dict[str, Any]]:
        """List draft messages.

        Args:
            max_results: Maximum number of drafts to return

        Returns:
            List of draft metadata
        """
        try:
            response = (
                self._service.users()
                .drafts()
                .list(userId=self._user_id, maxResults=max_results)
                .execute()
            )

            return response.get('drafts', [])

        except HttpError as error:
            logger.error(f"Error listing drafts: {error}")
            raise

    @retry_with_backoff
    def get_draft(self, draft_id: str) -> Dict[str, Any]:
        """Get a specific draft by ID.

        Args:
            draft_id: The draft ID

        Returns:
            Draft data including message content
        """
        try:
            draft = (
                self._service.users()
                .drafts()
                .get(userId=self._user_id, id=draft_id)
                .execute()
            )
            return draft

        except HttpError as error:
            logger.error(f"Error getting draft {draft_id}: {error}")
            raise

    @retry_with_backoff
    def create_draft(
        self,
        to: str,
        subject: str,
        body: str,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, Any]:
        """Create a draft message.

        Args:
            to: Recipient email address
            subject: Email subject
            body: Email body content
            cc: CC recipients
            bcc: BCC recipients
            html: If True, body is HTML

        Returns:
            Created draft metadata
        """
        try:
            message = self._create_message(
                to=to,
                subject=subject,
                body=body,
                cc=cc,
                bcc=bcc,
                html=html,
            )

            draft = (
                self._service.users()
                .drafts()
                .create(userId=self._user_id, body={'message': message})
                .execute()
            )

            logger.info(f"Draft created successfully. ID: {draft['id']}")
            return draft

        except HttpError as error:
            logger.error(f"Error creating draft: {error}")
            raise

    @retry_with_backoff
    def update_draft(
        self,
        draft_id: str,
        to: str,
        subject: str,
        body: str,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, Any]:
        """Update an existing draft.

        Args:
            draft_id: Draft ID to update
            to: Recipient email address
            subject: Email subject
            body: Email body content
            cc: CC recipients
            bcc: BCC recipients
            html: If True, body is HTML

        Returns:
            Updated draft metadata
        """
        try:
            message = self._create_message(
                to=to,
                subject=subject,
                body=body,
                cc=cc,
                bcc=bcc,
                html=html,
            )

            draft = (
                self._service.users()
                .drafts()
                .update(
                    userId=self._user_id,
                    id=draft_id,
                    body={'message': message}
                )
                .execute()
            )

            logger.info(f"Draft {draft_id} updated successfully")
            return draft

        except HttpError as error:
            logger.error(f"Error updating draft {draft_id}: {error}")
            raise

    @retry_with_backoff
    def delete_draft(self, draft_id: str) -> None:
        """Delete a draft.

        Args:
            draft_id: Draft ID to delete
        """
        try:
            self._service.users().drafts().delete(
                userId=self._user_id,
                id=draft_id
            ).execute()

            logger.info(f"Draft {draft_id} deleted successfully")

        except HttpError as error:
            logger.error(f"Error deleting draft {draft_id}: {error}")
            raise

    @retry_with_backoff
    def send_draft(self, draft_id: str) -> Dict[str, Any]:
        """Send a draft message.

        Args:
            draft_id: Draft ID to send

        Returns:
            Sent message metadata
        """
        try:
            sent_message = (
                self._service.users()
                .drafts()
                .send(userId=self._user_id, body={'id': draft_id})
                .execute()
            )

            logger.info(f"Draft {draft_id} sent successfully. Message ID: {sent_message['id']}")
            return sent_message

        except HttpError as error:
            logger.error(f"Error sending draft {draft_id}: {error}")
            raise

    @retry_with_backoff
    def get_attachment(
        self,
        message_id: str,
        attachment_id: str,
    ) -> Dict[str, Any]:
        """Get a message attachment.

        Args:
            message_id: Message ID containing the attachment
            attachment_id: Attachment ID

        Returns:
            Attachment data (base64 encoded)
        """
        try:
            attachment = (
                self._service.users()
                .messages()
                .attachments()
                .get(userId=self._user_id, messageId=message_id, id=attachment_id)
                .execute()
            )

            return attachment

        except HttpError as error:
            logger.error(f"Error getting attachment {attachment_id}: {error}")
            raise

    def download_attachment(
        self,
        message_id: str,
        attachment_id: str,
        output_path: Path,
    ) -> Path:
        """Download attachment to file.

        Args:
            message_id: Message ID containing the attachment
            attachment_id: Attachment ID
            output_path: Path to save attachment

        Returns:
            Path to saved file
        """
        attachment = self.get_attachment(message_id, attachment_id)
        data = base64.urlsafe_b64decode(attachment['data'])

        output_path = Path(output_path)
        output_path.parent.mkdir(parents=True, exist_ok=True)

        with open(output_path, 'wb') as f:
            f.write(data)

        logger.info(f"Attachment saved to {output_path}")
        return output_path

    @retry_with_backoff
    def send_message_with_attachment(
        self,
        to: str,
        subject: str,
        body: str,
        attachment_path: Path,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, Any]:
        """Send an email with an attachment.

        Args:
            to: Recipient email address
            subject: Email subject
            body: Email body content
            attachment_path: Path to file to attach
            cc: CC recipients
            bcc: BCC recipients
            html: If True, body is HTML

        Returns:
            Sent message metadata
        """
        try:
            message = self._create_message_with_attachment(
                to=to,
                subject=subject,
                body=body,
                attachment_path=attachment_path,
                cc=cc,
                bcc=bcc,
                html=html,
            )

            sent_message = (
                self._service.users()
                .messages()
                .send(userId=self._user_id, body=message)
                .execute()
            )

            logger.info(f"Message with attachment sent. ID: {sent_message['id']}")
            return sent_message

        except HttpError as error:
            logger.error(f"Error sending message with attachment: {error}")
            raise

    def _create_message_with_attachment(
        self,
        to: str,
        subject: str,
        body: str,
        attachment_path: Path,
        cc: Optional[str] = None,
        bcc: Optional[str] = None,
        html: bool = False,
    ) -> Dict[str, str]:
        """Create a message with attachment.

        Args:
            to: Recipient email
            subject: Email subject
            body: Email body
            attachment_path: Path to attachment file
            cc: CC recipients
            bcc: BCC recipients
            html: Whether body is HTML

        Returns:
            Message dictionary ready for API
        """
        message = MIMEMultipart()
        message['to'] = to
        message['subject'] = subject

        if cc:
            message['cc'] = cc
        if bcc:
            message['bcc'] = bcc

        # Add body
        body_part = MIMEText(body, 'html' if html else 'plain')
        message.attach(body_part)

        # Add attachment
        attachment_path = Path(attachment_path)

        with open(attachment_path, 'rb') as f:
            attachment = MIMEBase('application', 'octet-stream')
            attachment.set_payload(f.read())
            encoders.encode_base64(attachment)
            attachment.add_header(
                'Content-Disposition',
                f'attachment; filename= {attachment_path.name}'
            )
            message.attach(attachment)

        raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
        return {'raw': raw_message}

    @retry_with_backoff
    def batch_modify_messages(
        self,
        message_ids: List[str],
        add_labels: Optional[List[str]] = None,
        remove_labels: Optional[List[str]] = None,
    ) -> None:
        """Modify labels on multiple messages at once.

        Args:
            message_ids: List of message IDs to modify
            add_labels: Label IDs to add
            remove_labels: Label IDs to remove
        """
        try:
            body = {'ids': message_ids}

            if add_labels:
                body['addLabelIds'] = add_labels

            if remove_labels:
                body['removeLabelIds'] = remove_labels

            self._service.users().messages().batchModify(
                userId=self._user_id,
                body=body
            ).execute()

            logger.info(f"Batch modified {len(message_ids)} messages")

        except HttpError as error:
            logger.error(f"Error batch modifying messages: {error}")
            raise

    @retry_with_backoff
    def batch_delete_messages(self, message_ids: List[str]) -> None:
        """Permanently delete multiple messages at once.

        Args:
            message_ids: List of message IDs to delete
        """
        try:
            self._service.users().messages().batchDelete(
                userId=self._user_id,
                body={'ids': message_ids}
            ).execute()

            logger.info(f"Batch deleted {len(message_ids)} messages")

        except HttpError as error:
            logger.error(f"Error batch deleting messages: {error}")
            raise

    @staticmethod
    def _get_header(headers: List[Dict[str, str]], name: str) -> str:
        """Extract header value by name."""
        for header in headers:
            if header['name'].lower() == name.lower():
                return header['value']
        return ""
