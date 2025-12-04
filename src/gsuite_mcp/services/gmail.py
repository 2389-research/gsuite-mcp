# ABOUTME: Gmail API service for email management operations
# ABOUTME: Handles reading, sending, searching, labels, drafts, and attachments

import base64
import logging
from typing import List, Dict, Any, Optional
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.mime.base import MIMEBase
from email import encoders

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff

logger = logging.getLogger(__name__)


class GmailService:
    """Service for Gmail API operations."""

    def __init__(self, credentials: Credentials):
        """Initialize Gmail service.

        Args:
            credentials: Valid OAuth2 credentials
        """
        self._service = build('gmail', 'v1', credentials=credentials)
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

    @staticmethod
    def _get_header(headers: List[Dict[str, str]], name: str) -> str:
        """Extract header value by name."""
        for header in headers:
            if header['name'].lower() == name.lower():
                return header['value']
        return ""
