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
