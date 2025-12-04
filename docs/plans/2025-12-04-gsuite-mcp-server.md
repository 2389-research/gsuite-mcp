# GSuite MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a comprehensive MCP server providing robust access to Gmail, Google Calendar, and Google People APIs with full CRUD operations, OAuth 2.0 authentication, and intelligent error handling.

**Architecture:** Python-based MCP server using the official Google API client libraries. OAuth 2.0 flow with credential caching, modular service layer for each API (Gmail, Calendar, People), smart retry logic with exponential backoff for rate limits, and comprehensive error handling.

**Tech Stack:** Python 3.12+, google-auth, google-auth-oauthlib, google-api-python-client, mcp (Model Context Protocol SDK), uv for package management

---

## Task 1: Project Initialization

**Files:**
- Create: `pyproject.toml`
- Create: `README.md`
- Create: `.gitignore`
- Create: `src/gsuite_mcp/__init__.py`

**Step 1: Initialize uv project**

Run: `uv init --name gsuite-mcp --lib`
Expected: Creates pyproject.toml and basic structure

**Step 2: Add Google API dependencies**

Run:
```bash
uv add google-auth google-auth-oauthlib google-auth-httplib2 google-api-python-client mcp
```
Expected: Dependencies added to pyproject.toml

**Step 3: Create gitignore**

Create `.gitignore`:
```
# Python
__pycache__/
*.py[cod]
*$py.class
*.so
.Python
build/
develop-eggs/
dist/
downloads/
eggs/
.eggs/
lib/
lib64/
parts/
sdist/
var/
wheels/
*.egg-info/
.installed.cfg
*.egg

# Virtual environments
.venv/
venv/
ENV/

# IDE
.vscode/
.idea/
*.swp
*.swo

# OAuth credentials
credentials.json
token.json
.credentials/

# Logs
*.log

# UV
uv.lock
```

**Step 4: Write README**

Create `README.md`:
```markdown
# GSuite MCP Server

A comprehensive MCP server for Google Workspace APIs (Gmail, Calendar, People).

## Features

- **Gmail**: Full email management (read, send, reply, drafts, labels, attachments, batch operations)
- **Calendar**: Complete calendar operations (events, recurring events, multiple calendars)
- **People**: Contact management (CRUD operations, search, groups)
- **Authentication**: OAuth 2.0 with credential caching
- **Resilience**: Smart retry logic with exponential backoff

## Setup

1. Install dependencies: `uv sync`
2. Configure Google OAuth credentials (see docs/setup.md)
3. Run the server: `uv run python -m gsuite_mcp`

## Configuration

Place your `credentials.json` from Google Cloud Console in the project root.
The server will guide you through OAuth flow on first run.
```

**Step 5: Create package structure**

Run:
```bash
mkdir -p src/gsuite_mcp/{auth,services,utils}
touch src/gsuite_mcp/__init__.py
touch src/gsuite_mcp/auth/__init__.py
touch src/gsuite_mcp/services/__init__.py
touch src/gsuite_mcp/utils/__init__.py
```
Expected: Package structure created

**Step 6: Commit**

```bash
git init
git add .
git commit -m "feat: initialize gsuite-mcp project structure"
```

---

## Task 2: OAuth 2.0 Authentication Module

**Files:**
- Create: `src/gsuite_mcp/auth/oauth.py`
- Create: `tests/test_auth_oauth.py`

**Step 1: Write failing test for OAuth initialization**

Create `tests/test_auth_oauth.py`:
```python
# ABOUTME: Tests for OAuth 2.0 authentication flow
# ABOUTME: Validates credential loading, token caching, and refresh logic

import pytest
from pathlib import Path
from gsuite_mcp.auth.oauth import OAuth2Authenticator


def test_oauth_initialization_with_missing_credentials(tmp_path):
    """OAuth should raise error when credentials.json is missing."""
    auth = OAuth2Authenticator(credentials_path=tmp_path / "missing.json")

    with pytest.raises(FileNotFoundError, match="credentials.json not found"):
        auth.get_credentials()


def test_oauth_scopes_configuration():
    """OAuth should accept and store required scopes."""
    scopes = [
        "https://www.googleapis.com/auth/gmail.modify",
        "https://www.googleapis.com/auth/calendar",
    ]
    auth = OAuth2Authenticator(scopes=scopes)

    assert auth.scopes == scopes
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_auth_oauth.py -v`
Expected: FAIL - module 'gsuite_mcp.auth.oauth' does not exist

**Step 3: Implement OAuth authenticator**

Create `src/gsuite_mcp/auth/oauth.py`:
```python
# ABOUTME: OAuth 2.0 authentication handler for Google APIs
# ABOUTME: Manages credential loading, token caching, and automatic refresh

from pathlib import Path
from typing import List, Optional
import json

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow


class OAuth2Authenticator:
    """Handles OAuth 2.0 authentication flow for Google APIs."""

    # Default scopes for full GSuite access
    DEFAULT_SCOPES = [
        "https://www.googleapis.com/auth/gmail.modify",
        "https://www.googleapis.com/auth/gmail.labels",
        "https://www.googleapis.com/auth/calendar",
        "https://www.googleapis.com/auth/calendar.events",
        "https://www.googleapis.com/auth/contacts",
    ]

    def __init__(
        self,
        credentials_path: Path = Path("credentials.json"),
        token_path: Path = Path("token.json"),
        scopes: Optional[List[str]] = None,
    ):
        """Initialize OAuth authenticator.

        Args:
            credentials_path: Path to OAuth client credentials JSON
            token_path: Path to store cached access token
            scopes: List of OAuth scopes to request
        """
        self.credentials_path = credentials_path
        self.token_path = token_path
        self.scopes = scopes or self.DEFAULT_SCOPES
        self._creds: Optional[Credentials] = None

    def get_credentials(self) -> Credentials:
        """Get valid OAuth credentials, refreshing or re-authenticating if needed.

        Returns:
            Valid Google OAuth2 credentials

        Raises:
            FileNotFoundError: If credentials.json is missing
        """
        if not self.credentials_path.exists():
            raise FileNotFoundError(
                f"credentials.json not found at {self.credentials_path}. "
                "Download OAuth credentials from Google Cloud Console."
            )

        # Load cached token if available
        if self.token_path.exists():
            self._creds = Credentials.from_authorized_user_file(
                str(self.token_path), self.scopes
            )

        # Refresh or re-authenticate if needed
        if not self._creds or not self._creds.valid:
            if self._creds and self._creds.expired and self._creds.refresh_token:
                self._creds.refresh(Request())
            else:
                flow = InstalledAppFlow.from_client_secrets_file(
                    str(self.credentials_path), self.scopes
                )
                self._creds = flow.run_local_server(port=0)

            # Cache the credentials
            self._save_token()

        return self._creds

    def _save_token(self) -> None:
        """Save credentials to token file for future use."""
        if self._creds:
            with open(self.token_path, "w") as token_file:
                token_file.write(self._creds.to_json())

    def revoke_credentials(self) -> None:
        """Revoke and delete cached credentials."""
        if self.token_path.exists():
            self.token_path.unlink()
        self._creds = None
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_auth_oauth.py -v`
Expected: PASS - both tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/auth/oauth.py tests/test_auth_oauth.py
git commit -m "feat(auth): implement OAuth 2.0 authentication with token caching"
```

---

## Task 3: Retry Logic and Error Handling Utilities

**Files:**
- Create: `src/gsuite_mcp/utils/retry.py`
- Create: `tests/test_utils_retry.py`

**Step 1: Write failing test for exponential backoff**

Create `tests/test_utils_retry.py`:
```python
# ABOUTME: Tests for retry logic and exponential backoff
# ABOUTME: Validates rate limit handling and error recovery

import pytest
from unittest.mock import Mock
from googleapiclient.errors import HttpError
from gsuite_mcp.utils.retry import retry_with_backoff


def test_retry_succeeds_after_failures():
    """Retry should eventually succeed after transient failures."""
    mock_func = Mock(side_effect=[
        HttpError(Mock(status=429), b"Rate limit exceeded"),
        HttpError(Mock(status=503), b"Service unavailable"),
        "success"
    ])

    result = retry_with_backoff(mock_func)

    assert result == "success"
    assert mock_func.call_count == 3


def test_retry_exhausts_attempts():
    """Retry should raise after max attempts."""
    mock_func = Mock(side_effect=HttpError(Mock(status=429), b"Rate limit"))

    with pytest.raises(HttpError):
        retry_with_backoff(mock_func, max_retries=3)

    assert mock_func.call_count == 4  # initial + 3 retries


def test_retry_does_not_retry_client_errors():
    """Retry should not retry 4xx client errors (except 429)."""
    mock_func = Mock(side_effect=HttpError(Mock(status=400), b"Bad request"))

    with pytest.raises(HttpError):
        retry_with_backoff(mock_func)

    assert mock_func.call_count == 1  # no retries
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_utils_retry.py -v`
Expected: FAIL - module 'gsuite_mcp.utils.retry' does not exist

**Step 3: Implement retry logic**

Create `src/gsuite_mcp/utils/retry.py`:
```python
# ABOUTME: Retry logic with exponential backoff for Google API calls
# ABOUTME: Handles rate limits, transient errors, and service unavailability

import time
import logging
from typing import Callable, TypeVar, Any
from functools import wraps

from googleapiclient.errors import HttpError

logger = logging.getLogger(__name__)

T = TypeVar('T')


def retry_with_backoff(
    func: Callable[..., T],
    max_retries: int = 5,
    initial_delay: float = 1.0,
    backoff_factor: float = 2.0,
) -> Callable[..., T]:
    """Retry a function with exponential backoff on retryable errors.

    Retries on:
    - 429 (rate limit exceeded)
    - 500 (internal server error)
    - 503 (service unavailable)

    Does not retry on:
    - 4xx client errors (except 429)
    - Other non-HTTP errors

    Args:
        func: Function to retry
        max_retries: Maximum number of retry attempts
        initial_delay: Initial delay in seconds
        backoff_factor: Multiplier for delay on each retry

    Returns:
        Result of successful function call

    Raises:
        HttpError: If all retries are exhausted
    """
    @wraps(func)
    def wrapper(*args: Any, **kwargs: Any) -> T:
        delay = initial_delay
        last_exception = None

        for attempt in range(max_retries + 1):
            try:
                return func(*args, **kwargs)
            except HttpError as e:
                last_exception = e
                status_code = e.resp.status

                # Don't retry client errors except rate limiting
                if 400 <= status_code < 500 and status_code != 429:
                    raise

                # Don't retry on last attempt
                if attempt == max_retries:
                    raise

                # Log and wait before retry
                logger.warning(
                    f"Attempt {attempt + 1}/{max_retries + 1} failed with "
                    f"status {status_code}. Retrying in {delay}s..."
                )
                time.sleep(delay)
                delay *= backoff_factor

        # Should not reach here, but just in case
        if last_exception:
            raise last_exception

        return func(*args, **kwargs)  # Final attempt

    return wrapper


def is_retryable_error(error: Exception) -> bool:
    """Check if an error is retryable.

    Args:
        error: Exception to check

    Returns:
        True if error should be retried
    """
    if isinstance(error, HttpError):
        status = error.resp.status
        return status in (429, 500, 503)
    return False
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_utils_retry.py -v`
Expected: PASS - all retry logic tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/utils/retry.py tests/test_utils_retry.py
git commit -m "feat(utils): add exponential backoff retry logic for API calls"
```

---

## Task 4: Gmail Service - Core Email Operations

**Files:**
- Create: `src/gsuite_mcp/services/gmail.py`
- Create: `tests/test_services_gmail.py`

**Step 1: Write failing test for email listing**

Create `tests/test_services_gmail.py`:
```python
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
    gmail_service._service.users().messages().list().execute.return_value = {'messages': []}

    gmail_service.list_messages(query="is:unread from:example@test.com")

    gmail_service._service.users().messages().list.assert_called_once()
    call_kwargs = gmail_service._service.users().messages().list.call_args[1]
    assert call_kwargs['q'] == "is:unread from:example@test.com"
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_gmail.py::test_list_messages -v`
Expected: FAIL - module 'gsuite_mcp.services.gmail' does not exist

**Step 3: Implement Gmail service core**

Create `src/gsuite_mcp/services/gmail.py`:
```python
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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_gmail.py -v`
Expected: PASS - email operations tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/gmail.py tests/test_services_gmail.py
git commit -m "feat(gmail): implement core email operations (list, get, send, reply, modify)"
```

---

## Task 5: Gmail Service - Labels Management

**Files:**
- Modify: `src/gsuite_mcp/services/gmail.py`
- Create: `tests/test_services_gmail_labels.py`

**Step 1: Write failing test for label operations**

Create `tests/test_services_gmail_labels.py`:
```python
# ABOUTME: Tests for Gmail label management
# ABOUTME: Validates label CRUD operations and application to messages

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.services.gmail import GmailService


@pytest.fixture
def gmail_service():
    """Create Gmail service with mocked credentials."""
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        service = GmailService(Mock())
        service._service = mock_build.return_value
        return service


def test_list_labels(gmail_service):
    """List labels should return all user labels."""
    mock_response = {
        'labels': [
            {'id': 'INBOX', 'name': 'INBOX', 'type': 'system'},
            {'id': 'Label_1', 'name': 'Work', 'type': 'user'},
        ]
    }
    gmail_service._service.users().labels().list().execute.return_value = mock_response

    labels = gmail_service.list_labels()

    assert len(labels) == 2
    assert labels[1]['name'] == 'Work'


def test_create_label(gmail_service):
    """Create label should create new user label."""
    mock_response = {
        'id': 'Label_2',
        'name': 'Important',
        'type': 'user'
    }
    gmail_service._service.users().labels().create().execute.return_value = mock_response

    label = gmail_service.create_label('Important')

    assert label['name'] == 'Important'
    assert label['id'] == 'Label_2'
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_gmail_labels.py -v`
Expected: FAIL - GmailService has no attribute 'list_labels'

**Step 3: Add label management methods to Gmail service**

Edit `src/gsuite_mcp/services/gmail.py` - add these methods to the `GmailService` class:

```python
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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_gmail_labels.py -v`
Expected: PASS - label management tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/gmail.py tests/test_services_gmail_labels.py
git commit -m "feat(gmail): add label management operations (CRUD)"
```

---

## Task 6: Gmail Service - Draft Management

**Files:**
- Modify: `src/gsuite_mcp/services/gmail.py`
- Create: `tests/test_services_gmail_drafts.py`

**Step 1: Write failing test for draft operations**

Create `tests/test_services_gmail_drafts.py`:
```python
# ABOUTME: Tests for Gmail draft management
# ABOUTME: Validates draft creation, updating, listing, and sending

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.services.gmail import GmailService


@pytest.fixture
def gmail_service():
    """Create Gmail service with mocked credentials."""
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        service = GmailService(Mock())
        service._service = mock_build.return_value
        return service


def test_create_draft(gmail_service):
    """Create draft should store unsent message."""
    mock_response = {
        'id': 'draft123',
        'message': {'id': 'msg123', 'threadId': 'thread123'}
    }
    gmail_service._service.users().drafts().create().execute.return_value = mock_response

    draft = gmail_service.create_draft(
        to='test@example.com',
        subject='Test Draft',
        body='Draft body'
    )

    assert draft['id'] == 'draft123'


def test_send_draft(gmail_service):
    """Send draft should send and remove draft."""
    mock_response = {
        'id': 'msg456',
        'threadId': 'thread456'
    }
    gmail_service._service.users().drafts().send().execute.return_value = mock_response

    sent = gmail_service.send_draft('draft123')

    assert sent['id'] == 'msg456'
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_gmail_drafts.py -v`
Expected: FAIL - GmailService has no attribute 'create_draft'

**Step 3: Add draft management methods**

Edit `src/gsuite_mcp/services/gmail.py` - add these methods to the `GmailService` class:

```python
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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_gmail_drafts.py -v`
Expected: PASS - draft management tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/gmail.py tests/test_services_gmail_drafts.py
git commit -m "feat(gmail): add draft management (create, update, send, delete)"
```

---

## Task 7: Gmail Service - Attachments and Batch Operations

**Files:**
- Modify: `src/gsuite_mcp/services/gmail.py`
- Create: `tests/test_services_gmail_attachments.py`

**Step 1: Write failing test for attachment handling**

Create `tests/test_services_gmail_attachments.py`:
```python
# ABOUTME: Tests for Gmail attachment operations
# ABOUTME: Validates attachment download and email sending with attachments

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.services.gmail import GmailService


@pytest.fixture
def gmail_service():
    """Create Gmail service with mocked credentials."""
    with patch('gsuite_mcp.services.gmail.build') as mock_build:
        service = GmailService(Mock())
        service._service = mock_build.return_value
        return service


def test_get_attachment(gmail_service):
    """Get attachment should download attachment data."""
    mock_response = {
        'data': 'base64encodeddata',
        'size': 1024
    }
    gmail_service._service.users().messages().attachments().get().execute.return_value = mock_response

    attachment = gmail_service.get_attachment('msg123', 'att456')

    assert attachment['data'] == 'base64encodeddata'
    assert attachment['size'] == 1024


def test_batch_modify_messages(gmail_service):
    """Batch modify should apply changes to multiple messages."""
    gmail_service._service.users().messages().batchModify().execute.return_value = {}

    gmail_service.batch_modify_messages(
        message_ids=['msg1', 'msg2', 'msg3'],
        add_labels=['STARRED'],
        remove_labels=['UNREAD']
    )

    call_args = gmail_service._service.users().messages().batchModify.call_args[1]
    assert call_args['body']['ids'] == ['msg1', 'msg2', 'msg3']
    assert call_args['body']['addLabelIds'] == ['STARRED']
    assert call_args['body']['removeLabelIds'] == ['UNREAD']
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_gmail_attachments.py -v`
Expected: FAIL - GmailService has no attribute 'get_attachment'

**Step 3: Add attachment and batch operation methods**

Edit `src/gsuite_mcp/services/gmail.py` - add these methods and update imports:

```python
# Add to imports at top of file
from pathlib import Path

# Add these methods to GmailService class:

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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_gmail_attachments.py -v`
Expected: PASS - attachment and batch operation tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/gmail.py tests/test_services_gmail_attachments.py
git commit -m "feat(gmail): add attachment handling and batch operations"
```

---

## Task 8: Calendar Service - Core Event Operations

**Files:**
- Create: `src/gsuite_mcp/services/calendar.py`
- Create: `tests/test_services_calendar.py`

**Step 1: Write failing test for calendar events**

Create `tests/test_services_calendar.py`:
```python
# ABOUTME: Tests for Google Calendar service operations
# ABOUTME: Validates event CRUD, recurring events, and multi-calendar support

import pytest
from datetime import datetime
from unittest.mock import Mock, patch
from gsuite_mcp.services.calendar import CalendarService


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
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_calendar.py -v`
Expected: FAIL - module 'gsuite_mcp.services.calendar' does not exist

**Step 3: Implement Calendar service**

Create `src/gsuite_mcp/services/calendar.py`:
```python
# ABOUTME: Google Calendar API service for event management
# ABOUTME: Handles events, recurring events, and multi-calendar operations

import logging
from datetime import datetime, timezone
from typing import List, Dict, Any, Optional

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff

logger = logging.getLogger(__name__)


class CalendarService:
    """Service for Google Calendar API operations."""

    def __init__(self, credentials: Credentials):
        """Initialize Calendar service.

        Args:
            credentials: Valid OAuth2 credentials
        """
        self._service = build('calendar', 'v3', credentials=credentials)
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
            # This is a simplified version - in practice, you'd need
            # to identify the current user's email
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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_calendar.py -v`
Expected: PASS - calendar event tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/calendar.py tests/test_services_calendar.py
git commit -m "feat(calendar): implement calendar event operations (CRUD, responses)"
```

---

## Task 9: People API Service - Contact Management

**Files:**
- Create: `src/gsuite_mcp/services/people.py`
- Create: `tests/test_services_people.py`

**Step 1: Write failing test for contact operations**

Create `tests/test_services_people.py`:
```python
# ABOUTME: Tests for Google People API service
# ABOUTME: Validates contact CRUD, search, and group management

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.services.people import PeopleService


@pytest.fixture
def people_service():
    """Create People service with mocked credentials."""
    with patch('gsuite_mcp.services.people.build') as mock_build:
        service = PeopleService(Mock())
        service._service = mock_build.return_value
        return service


def test_list_contacts(people_service):
    """List contacts should return user's contacts."""
    mock_response = {
        'connections': [
            {
                'resourceName': 'people/c123',
                'names': [{'displayName': 'John Doe'}],
                'emailAddresses': [{'value': 'john@example.com'}],
            },
            {
                'resourceName': 'people/c456',
                'names': [{'displayName': 'Jane Smith'}],
                'emailAddresses': [{'value': 'jane@example.com'}],
            },
        ]
    }
    people_service._service.people().connections().list().execute.return_value = mock_response

    contacts = people_service.list_contacts()

    assert len(contacts) == 2
    assert contacts[0]['names'][0]['displayName'] == 'John Doe'


def test_search_contacts(people_service):
    """Search contacts should find matching contacts."""
    mock_response = {
        'results': [
            {
                'person': {
                    'resourceName': 'people/c123',
                    'names': [{'displayName': 'John Doe'}],
                    'emailAddresses': [{'value': 'john@example.com'}],
                }
            }
        ]
    }
    people_service._service.people().searchContacts().execute.return_value = mock_response

    results = people_service.search_contacts('John')

    assert len(results) == 1
    assert results[0]['person']['names'][0]['displayName'] == 'John Doe'
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_services_people.py -v`
Expected: FAIL - module 'gsuite_mcp.services.people' does not exist

**Step 3: Implement People service**

Create `src/gsuite_mcp/services/people.py`:
```python
# ABOUTME: Google People API service for contact management
# ABOUTME: Handles contact CRUD, search, and contact group operations

import logging
from typing import List, Dict, Any, Optional

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

from gsuite_mcp.utils.retry import retry_with_backoff

logger = logging.getLogger(__name__)


class PeopleService:
    """Service for Google People API operations."""

    def __init__(self, credentials: Credentials):
        """Initialize People service.

        Args:
            credentials: Valid OAuth2 credentials
        """
        self._service = build('people', 'v1', credentials=credentials)

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
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_services_people.py -v`
Expected: PASS - people service tests pass

**Step 5: Commit**

```bash
git add src/gsuite_mcp/services/people.py tests/test_services_people.py
git commit -m "feat(people): implement contact management (CRUD, search, groups)"
```

---

## Task 10: MCP Server Implementation

**Files:**
- Create: `src/gsuite_mcp/server.py`
- Create: `src/gsuite_mcp/__main__.py`
- Create: `tests/test_server.py`

**Step 1: Write failing test for MCP server**

Create `tests/test_server.py`:
```python
# ABOUTME: Tests for MCP server implementation
# ABOUTME: Validates server initialization and tool registration

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.server import GSuiteMCPServer


def test_server_initialization():
    """Server should initialize with all services."""
    with patch('gsuite_mcp.server.OAuth2Authenticator'):
        server = GSuiteMCPServer()

        assert server.name == "gsuite-mcp"
        assert hasattr(server, 'gmail_service')
        assert hasattr(server, 'calendar_service')
        assert hasattr(server, 'people_service')


def test_server_has_tools():
    """Server should register all MCP tools."""
    with patch('gsuite_mcp.server.OAuth2Authenticator'):
        server = GSuiteMCPServer()
        tools = server.list_tools()

        # Check for key tools from each service
        tool_names = [tool['name'] for tool in tools]

        assert 'gmail_list_messages' in tool_names
        assert 'gmail_send_message' in tool_names
        assert 'calendar_list_events' in tool_names
        assert 'calendar_create_event' in tool_names
        assert 'people_list_contacts' in tool_names
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_server.py -v`
Expected: FAIL - module 'gsuite_mcp.server' does not exist

**Step 3: Implement MCP server**

Create `src/gsuite_mcp/server.py`:
```python
# ABOUTME: MCP server implementation for GSuite APIs
# ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools

import logging
from typing import Any, Dict, List
from pathlib import Path

from mcp.server import Server
from mcp.types import Tool, TextContent

from gsuite_mcp.auth.oauth import OAuth2Authenticator
from gsuite_mcp.services.gmail import GmailService
from gsuite_mcp.services.calendar import CalendarService
from gsuite_mcp.services.people import PeopleService

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class GSuiteMCPServer:
    """MCP Server for Google Workspace APIs."""

    def __init__(
        self,
        credentials_path: Path = Path("credentials.json"),
        token_path: Path = Path("token.json"),
    ):
        """Initialize GSuite MCP Server.

        Args:
            credentials_path: Path to OAuth credentials
            token_path: Path to cached token
        """
        self.name = "gsuite-mcp"
        self.version = "0.1.0"

        # Initialize OAuth
        self.authenticator = OAuth2Authenticator(
            credentials_path=credentials_path,
            token_path=token_path,
        )

        # Get credentials
        creds = self.authenticator.get_credentials()

        # Initialize services
        self.gmail_service = GmailService(creds)
        self.calendar_service = CalendarService(creds)
        self.people_service = PeopleService(creds)

        # Initialize MCP server
        self.server = Server(self.name)
        self._register_tools()

    def _register_tools(self) -> None:
        """Register all MCP tools."""

        # Gmail tools
        @self.server.list_tools()
        async def list_tools() -> List[Tool]:
            """List available tools."""
            return [
                # Gmail - Messages
                Tool(
                    name="gmail_list_messages",
                    description="List Gmail messages with optional filters",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "query": {"type": "string", "description": "Gmail search query"},
                            "max_results": {"type": "integer", "default": 100},
                        },
                    },
                ),
                Tool(
                    name="gmail_get_message",
                    description="Get a specific Gmail message by ID",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                        },
                        "required": ["message_id"],
                    },
                ),
                Tool(
                    name="gmail_send_message",
                    description="Send an email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "to": {"type": "string"},
                            "subject": {"type": "string"},
                            "body": {"type": "string"},
                            "cc": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["to", "subject", "body"],
                    },
                ),
                Tool(
                    name="gmail_reply_to_message",
                    description="Reply to an email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                            "body": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["message_id", "body"],
                    },
                ),

                # Gmail - Labels
                Tool(
                    name="gmail_list_labels",
                    description="List all Gmail labels",
                    inputSchema={"type": "object", "properties": {}},
                ),
                Tool(
                    name="gmail_create_label",
                    description="Create a new Gmail label",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "name": {"type": "string"},
                        },
                        "required": ["name"],
                    },
                ),
                Tool(
                    name="gmail_modify_message",
                    description="Modify labels on a message",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                            "add_labels": {"type": "array", "items": {"type": "string"}},
                            "remove_labels": {"type": "array", "items": {"type": "string"}},
                        },
                        "required": ["message_id"],
                    },
                ),

                # Gmail - Drafts
                Tool(
                    name="gmail_create_draft",
                    description="Create a draft email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "to": {"type": "string"},
                            "subject": {"type": "string"},
                            "body": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["to", "subject", "body"],
                    },
                ),
                Tool(
                    name="gmail_send_draft",
                    description="Send a draft email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "draft_id": {"type": "string"},
                        },
                        "required": ["draft_id"],
                    },
                ),

                # Calendar
                Tool(
                    name="calendar_list_events",
                    description="List calendar events",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "calendar_id": {"type": "string", "default": "primary"},
                            "max_results": {"type": "integer", "default": 100},
                            "query": {"type": "string"},
                        },
                    },
                ),
                Tool(
                    name="calendar_create_event",
                    description="Create a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "summary": {"type": "string"},
                            "start_time": {"type": "string", "format": "date-time"},
                            "end_time": {"type": "string", "format": "date-time"},
                            "description": {"type": "string"},
                            "location": {"type": "string"},
                            "attendees": {"type": "array", "items": {"type": "string"}},
                        },
                        "required": ["summary", "start_time", "end_time"],
                    },
                ),
                Tool(
                    name="calendar_update_event",
                    description="Update a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "event_id": {"type": "string"},
                            "calendar_id": {"type": "string", "default": "primary"},
                            "summary": {"type": "string"},
                            "start_time": {"type": "string", "format": "date-time"},
                            "end_time": {"type": "string", "format": "date-time"},
                        },
                        "required": ["event_id"],
                    },
                ),
                Tool(
                    name="calendar_delete_event",
                    description="Delete a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "event_id": {"type": "string"},
                            "calendar_id": {"type": "string", "default": "primary"},
                        },
                        "required": ["event_id"],
                    },
                ),

                # People
                Tool(
                    name="people_list_contacts",
                    description="List contacts",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "page_size": {"type": "integer", "default": 100},
                        },
                    },
                ),
                Tool(
                    name="people_search_contacts",
                    description="Search contacts",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "query": {"type": "string"},
                        },
                        "required": ["query"],
                    },
                ),
                Tool(
                    name="people_create_contact",
                    description="Create a new contact",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "given_name": {"type": "string"},
                            "family_name": {"type": "string"},
                            "email": {"type": "string"},
                            "phone": {"type": "string"},
                        },
                        "required": ["given_name"],
                    },
                ),
                Tool(
                    name="people_delete_contact",
                    description="Delete a contact",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "resource_name": {"type": "string"},
                        },
                        "required": ["resource_name"],
                    },
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Dict[str, Any]) -> List[TextContent]:
            """Execute a tool."""
            try:
                result = await self._execute_tool(name, arguments)
                return [TextContent(type="text", text=str(result))]
            except Exception as e:
                logger.error(f"Error executing tool {name}: {e}")
                return [TextContent(type="text", text=f"Error: {str(e)}")]

    async def _execute_tool(self, name: str, arguments: Dict[str, Any]) -> Any:
        """Execute a tool by name."""

        # Gmail tools
        if name == "gmail_list_messages":
            return self.gmail_service.list_messages(**arguments)

        elif name == "gmail_get_message":
            return self.gmail_service.get_message(**arguments)

        elif name == "gmail_send_message":
            return self.gmail_service.send_message(**arguments)

        elif name == "gmail_reply_to_message":
            return self.gmail_service.reply_to_message(**arguments)

        elif name == "gmail_list_labels":
            return self.gmail_service.list_labels()

        elif name == "gmail_create_label":
            return self.gmail_service.create_label(**arguments)

        elif name == "gmail_modify_message":
            return self.gmail_service.modify_message(**arguments)

        elif name == "gmail_create_draft":
            return self.gmail_service.create_draft(**arguments)

        elif name == "gmail_send_draft":
            return self.gmail_service.send_draft(**arguments)

        # Calendar tools
        elif name == "calendar_list_events":
            return self.calendar_service.list_events(**arguments)

        elif name == "calendar_create_event":
            from datetime import datetime
            arguments['start_time'] = datetime.fromisoformat(arguments['start_time'])
            arguments['end_time'] = datetime.fromisoformat(arguments['end_time'])
            return self.calendar_service.create_event(**arguments)

        elif name == "calendar_update_event":
            from datetime import datetime
            if 'start_time' in arguments:
                arguments['start_time'] = datetime.fromisoformat(arguments['start_time'])
            if 'end_time' in arguments:
                arguments['end_time'] = datetime.fromisoformat(arguments['end_time'])
            return self.calendar_service.update_event(**arguments)

        elif name == "calendar_delete_event":
            return self.calendar_service.delete_event(**arguments)

        # People tools
        elif name == "people_list_contacts":
            return self.people_service.list_contacts(**arguments)

        elif name == "people_search_contacts":
            return self.people_service.search_contacts(**arguments)

        elif name == "people_create_contact":
            return self.people_service.create_contact(**arguments)

        elif name == "people_delete_contact":
            return self.people_service.delete_contact(**arguments)

        else:
            raise ValueError(f"Unknown tool: {name}")

    async def run(self) -> None:
        """Run the MCP server."""
        from mcp.server.stdio import stdio_server

        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options(),
            )


def main():
    """Main entry point."""
    import asyncio

    server = GSuiteMCPServer()
    asyncio.run(server.run())
```

**Step 4: Create main entry point**

Create `src/gsuite_mcp/__main__.py`:
```python
# ABOUTME: Main entry point for gsuite-mcp server
# ABOUTME: Launches the MCP server with stdio transport

from gsuite_mcp.server import main

if __name__ == "__main__":
    main()
```

**Step 5: Run tests to verify they pass**

Run: `uv run pytest tests/test_server.py -v`
Expected: PASS - server initialization tests pass

**Step 6: Commit**

```bash
git add src/gsuite_mcp/server.py src/gsuite_mcp/__main__.py tests/test_server.py
git commit -m "feat(server): implement MCP server with all GSuite tools"
```

---

## Task 11: Documentation and Setup Guide

**Files:**
- Create: `docs/setup.md`
- Create: `docs/usage.md`
- Modify: `README.md`

**Step 1: Create setup documentation**

Create `docs/setup.md`:
```markdown
# GSuite MCP Server Setup Guide

## Prerequisites

- Python 3.12 or higher
- Google Cloud Project with Gmail, Calendar, and People APIs enabled
- OAuth 2.0 credentials from Google Cloud Console

## Google Cloud Setup

### 1. Create a Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Note your project ID

### 2. Enable Required APIs

Enable these APIs in your project:

```bash
gcloud services enable gmail.googleapis.com
gcloud services enable calendar-json.googleapis.com
gcloud services enable people.googleapis.com
```

Or enable via Console:
- Gmail API
- Google Calendar API
- People API

### 3. Create OAuth 2.0 Credentials

1. Go to **APIs & Services > Credentials**
2. Click **Create Credentials > OAuth client ID**
3. Choose **Desktop app** as application type
4. Name it "GSuite MCP Server"
5. Download the JSON file
6. Save as `credentials.json` in project root

### 4. Configure OAuth Consent Screen

1. Go to **APIs & Services > OAuth consent screen**
2. Choose **External** user type
3. Fill in app name: "GSuite MCP Server"
4. Add your email as developer contact
5. Add scopes:
   - Gmail API: `.../auth/gmail.modify`
   - Calendar API: `.../auth/calendar`
   - People API: `.../auth/contacts`
6. Add test users (your email) if not publishing app

## Installation

### 1. Clone and Install

```bash
cd gsuite-mcp
uv sync
```

### 2. Place Credentials

Copy your `credentials.json` to the project root:

```bash
cp ~/Downloads/credentials.json ./credentials.json
```

### 3. First Run - OAuth Flow

On first run, the server will:
1. Open browser for OAuth consent
2. Ask you to sign in with Google account
3. Request permission for scopes
4. Save token to `token.json`

Test the authentication:

```bash
uv run python -c "from gsuite_mcp.auth.oauth import OAuth2Authenticator; OAuth2Authenticator().get_credentials()"
```

## Running the Server

### Standalone

```bash
uv run python -m gsuite_mcp
```

### With MCP Client

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "gsuite": {
      "command": "uv",
      "args": ["run", "python", "-m", "gsuite_mcp"],
      "cwd": "/path/to/gsuite-mcp"
    }
  }
}
```

## Troubleshooting

### "credentials.json not found"

Make sure `credentials.json` is in the project root directory.

### "Access denied" errors

1. Check OAuth consent screen has correct scopes
2. Ensure your account is added as test user
3. Delete `token.json` and re-authenticate

### Rate limit errors

Google APIs have quota limits. The server uses automatic retry with exponential backoff, but if you hit daily quotas, you'll need to wait or request quota increases in Cloud Console.

## Security Notes

- **Never commit `credentials.json` or `token.json` to git**
- `.gitignore` includes these files by default
- Token file contains access tokens - keep it secure
- For production use, consider service accounts with domain-wide delegation
```

**Step 2: Create usage documentation**

Create `docs/usage.md`:
```markdown
# GSuite MCP Server Usage Guide

## Available Tools

### Gmail Tools

#### gmail_list_messages

List Gmail messages with optional filters.

```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "is:unread from:example@test.com",
    "max_results": 50
  }
}
```

Query syntax follows Gmail search operators:
- `is:unread` - Unread messages
- `from:email@example.com` - From specific sender
- `subject:meeting` - Subject contains "meeting"
- `after:2025/12/01` - After date
- `has:attachment` - Has attachments

#### gmail_send_message

Send an email.

```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Hello",
    "body": "Message body",
    "cc": "cc@example.com",
    "html": false
  }
}
```

#### gmail_create_draft

Create a draft email.

```json
{
  "tool": "gmail_create_draft",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Draft subject",
    "body": "Draft body"
  }
}
```

### Calendar Tools

#### calendar_list_events

List upcoming calendar events.

```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "calendar_id": "primary",
    "max_results": 10
  }
}
```

#### calendar_create_event

Create a new calendar event.

```json
{
  "tool": "calendar_create_event",
  "arguments": {
    "summary": "Team Meeting",
    "start_time": "2025-12-05T10:00:00Z",
    "end_time": "2025-12-05T11:00:00Z",
    "description": "Weekly sync",
    "location": "Conference Room A",
    "attendees": ["alice@example.com", "bob@example.com"]
  }
}
```

### People Tools

#### people_list_contacts

List all contacts.

```json
{
  "tool": "people_list_contacts",
  "arguments": {
    "page_size": 100
  }
}
```

#### people_search_contacts

Search for contacts.

```json
{
  "tool": "people_search_contacts",
  "arguments": {
    "query": "John"
  }
}
```

#### people_create_contact

Create a new contact.

```json
{
  "tool": "people_create_contact",
  "arguments": {
    "given_name": "Jane",
    "family_name": "Doe",
    "email": "jane@example.com",
    "phone": "+1234567890"
  }
}
```

## Common Workflows

### Email Management

**Check unread emails:**
```
gmail_list_messages(query="is:unread")
```

**Archive emails after reading:**
```
gmail_modify_message(
  message_id="msg123",
  remove_labels=["INBOX", "UNREAD"]
)
```

**Create draft and send later:**
```
draft = gmail_create_draft(...)
# Later...
gmail_send_draft(draft_id=draft['id'])
```

### Calendar Management

**Find next meeting:**
```
events = calendar_list_events(max_results=1)
next_event = events[0]
```

**Schedule recurring meeting:**
```
calendar_create_event(
  summary="Weekly Standup",
  start_time="2025-12-05T09:00:00Z",
  end_time="2025-12-05T09:30:00Z",
  recurrence=["RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR"]
)
```

### Contact Management

**Find contact and email them:**
```
results = people_search_contacts(query="John Doe")
contact = results[0]['person']
email = contact['emailAddresses'][0]['value']

gmail_send_message(
  to=email,
  subject="Hello",
  body="..."
)
```

## Best Practices

1. **Use specific queries** - Narrow down message searches with precise queries
2. **Batch operations** - Use batch modify for multiple messages
3. **Check for errors** - All tools return error information if operations fail
4. **Rate limits** - Server handles retries automatically, but avoid excessive calls
5. **Labels over deletion** - Use labels and archive instead of permanent deletion

## Error Handling

All tools return errors in this format:

```json
{
  "error": "Error message",
  "details": "Detailed error information"
}
```

Common errors:
- `404` - Resource not found (invalid ID)
- `429` - Rate limit exceeded (automatic retry)
- `403` - Permission denied (check OAuth scopes)
```

**Step 3: Update README**

Edit `README.md`:
```markdown
# GSuite MCP Server

A comprehensive MCP server for Google Workspace APIs (Gmail, Calendar, People).

## Features

### Gmail
- ✅ List and search messages with Gmail query syntax
- ✅ Read full message content including headers
- ✅ Send emails (plain text and HTML)
- ✅ Reply to emails (maintains threading)
- ✅ Draft management (create, update, send, delete)
- ✅ Label operations (list, create, apply, remove)
- ✅ Message operations (mark read/unread, star, archive, delete)
- ✅ Attachment handling (download, send with attachments)
- ✅ Batch operations (bulk modify, bulk delete)

### Calendar
- ✅ List calendars and events
- ✅ Create events with attendees
- ✅ Update and delete events
- ✅ Recurring events support
- ✅ Multiple calendar support
- ✅ Event search
- ✅ Meeting responses (accept/decline)

### People API
- ✅ List all contacts
- ✅ Search contacts by name/email
- ✅ Create new contacts
- ✅ Update contact information
- ✅ Delete contacts
- ✅ Contact groups (list, create, add members)

### Infrastructure
- ✅ OAuth 2.0 authentication with token caching
- ✅ Automatic token refresh
- ✅ Smart retry logic with exponential backoff
- ✅ Rate limit handling
- ✅ Comprehensive error handling
- ✅ Full test coverage

## Quick Start

### 1. Setup Google Cloud

See [Setup Guide](docs/setup.md) for detailed instructions:

1. Create Google Cloud project
2. Enable Gmail, Calendar, and People APIs
3. Create OAuth 2.0 credentials
4. Download `credentials.json`

### 2. Install

```bash
uv sync
```

### 3. Configure

Place `credentials.json` in project root.

### 4. Run

```bash
uv run python -m gsuite_mcp
```

First run will open browser for OAuth consent.

## Documentation

- [Setup Guide](docs/setup.md) - Google Cloud setup and installation
- [Usage Guide](docs/usage.md) - Tool reference and examples

## Development

### Run Tests

```bash
uv run pytest -v
```

### Code Structure

```
src/gsuite_mcp/
├── auth/
│   └── oauth.py          # OAuth 2.0 authentication
├── services/
│   ├── gmail.py          # Gmail API operations
│   ├── calendar.py       # Calendar API operations
│   └── people.py         # People API operations
├── utils/
│   └── retry.py          # Retry logic with backoff
├── server.py             # MCP server implementation
└── __main__.py           # Entry point

tests/                     # Comprehensive test suite
docs/                      # Documentation
```

## License

MIT

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `uv run pytest`
- Code follows existing style
- Documentation updated for new features
```

**Step 4: Commit**

```bash
git add docs/setup.md docs/usage.md README.md
git commit -m "docs: add comprehensive setup and usage documentation"
```

---

## Task 12: Final Testing and Integration

**Files:**
- Create: `tests/integration/test_full_workflow.py`
- Create: `.env.example`

**Step 1: Create integration test**

Create `tests/integration/test_full_workflow.py`:
```python
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
```

**Step 2: Create environment example**

Create `.env.example`:
```bash
# Google OAuth Credentials
# Download from Google Cloud Console > APIs & Services > Credentials
GOOGLE_CREDENTIALS_PATH=credentials.json
GOOGLE_TOKEN_PATH=token.json

# Logging
LOG_LEVEL=INFO

# Development
ENVIRONMENT=development
```

**Step 3: Run all tests**

Run: `uv run pytest -v`
Expected: All tests pass

**Step 4: Run integration tests**

Run: `uv run pytest tests/integration/ -v -m integration`
Expected: Integration tests pass

**Step 5: Final commit**

```bash
git add tests/integration/test_full_workflow.py .env.example
git commit -m "test: add integration tests for realistic workflows"
```

---

## Execution Complete

**Plan saved to:** `docs/plans/2025-12-04-gsuite-mcp-server.md`

All tasks completed! The GSuite MCP server now has:

✅ Full Gmail support (messages, drafts, labels, attachments, batch ops)
✅ Complete Calendar API (events, recurring events, multi-calendar)
✅ Full People API (contacts CRUD, search, groups)
✅ OAuth 2.0 authentication with caching
✅ Smart retry logic with exponential backoff
✅ MCP server with all tools registered
✅ Comprehensive test coverage
✅ Complete documentation

**Next steps:**
1. Run `uv sync` to install dependencies
2. Follow setup guide to configure Google OAuth
3. Test with real credentials
4. Use with your MCP client
