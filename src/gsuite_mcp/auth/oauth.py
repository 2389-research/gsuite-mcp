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
