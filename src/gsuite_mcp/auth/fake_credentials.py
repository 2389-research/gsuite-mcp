# ABOUTME: Fake credentials implementation for testing with ish server
# ABOUTME: Bypasses OAuth validation and uses simple Bearer token authentication

import os
from typing import Optional


class FakeCredentials:
    """Fake credentials for testing with ish fake Google API server.

    This class mimics the interface of google.oauth2.credentials.Credentials
    but uses simple Bearer token authentication instead of OAuth.
    """

    def __init__(self, token: Optional[str] = None, user: Optional[str] = None):
        """Initialize fake credentials.

        Args:
            token: Bearer token in format "user:username" or just username
            user: Username for ish authentication (alternative to token)
        """
        if token:
            self.token = token
        elif user:
            self.token = f"user:{user}"
        else:
            # Try to get from environment
            ish_user = os.getenv("ISH_USER")
            if ish_user:
                self.token = f"user:{ish_user}"
            else:
                self.token = "user:testuser"

        self.valid = True
        self.expired = False

    def refresh(self, request):
        """Mock refresh - does nothing for fake credentials."""
        pass

    def apply(self, headers, token=None):
        """Apply credentials to request headers.

        Args:
            headers: Dictionary of HTTP headers to modify
            token: Optional token override
        """
        headers['Authorization'] = f'Bearer {self.token}'

    def before_request(self, request, method, url, headers):
        """Apply credentials before HTTP request.

        Args:
            request: HTTP request object
            method: HTTP method
            url: Request URL
            headers: Request headers dictionary
        """
        self.apply(headers)
