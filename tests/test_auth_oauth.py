# ABOUTME: Tests for OAuth 2.0 authentication flow
# ABOUTME: Validates credential loading, token caching, and refresh logic

import pytest
from pathlib import Path
from unittest.mock import Mock, patch, mock_open
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


def test_token_loading_from_cache(tmp_path):
    """OAuth should load valid token from cache when available."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    # Create dummy credentials file
    creds_path.write_text('{"installed": {"client_id": "test"}}')
    token_path.write_text('{"token": "test_token"}')

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)

    # Mock the Credentials class to return valid credentials
    mock_creds = Mock()
    mock_creds.valid = True
    mock_creds.expired = False

    with patch('gsuite_mcp.auth.oauth.Credentials.from_authorized_user_file', return_value=mock_creds):
        result = auth.get_credentials()

        assert result == mock_creds
        assert auth._creds == mock_creds


def test_token_refresh_on_expiration(tmp_path):
    """OAuth should refresh expired token when refresh_token is available."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    # Create dummy credentials file
    creds_path.write_text('{"installed": {"client_id": "test"}}')
    token_path.write_text('{"token": "expired_token"}')

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)

    # Mock expired credentials with refresh token
    mock_creds = Mock()
    mock_creds.valid = False
    mock_creds.expired = True
    mock_creds.refresh_token = "refresh_token_123"
    mock_creds.to_json.return_value = '{"token": "refreshed_token"}'

    with patch('gsuite_mcp.auth.oauth.Credentials.from_authorized_user_file', return_value=mock_creds):
        with patch('gsuite_mcp.auth.oauth.Request') as mock_request:
            with patch('builtins.open', mock_open()) as mocked_file:
                result = auth.get_credentials()

                # Verify refresh was called
                mock_creds.refresh.assert_called_once()

                # Verify token was saved
                mocked_file.assert_called_once_with(token_path, 'w')

                assert result == mock_creds


def test_token_save_after_authentication(tmp_path):
    """OAuth should save token to file after successful authentication."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    # Create dummy credentials file
    creds_path.write_text('{"installed": {"client_id": "test"}}')

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)

    # Mock new authentication flow
    mock_creds = Mock()
    mock_creds.valid = True
    mock_creds.to_json.return_value = '{"token": "new_token"}'

    mock_flow = Mock()
    mock_flow.run_local_server.return_value = mock_creds

    with patch('gsuite_mcp.auth.oauth.InstalledAppFlow.from_client_secrets_file', return_value=mock_flow):
        with patch('builtins.open', mock_open()) as mocked_file:
            result = auth.get_credentials()

            # Verify flow was initiated
            mock_flow.run_local_server.assert_called_once_with(port=0)

            # Verify token was saved
            mocked_file.assert_called_once_with(token_path, 'w')
            handle = mocked_file()
            handle.write.assert_called_once_with('{"token": "new_token"}')

            assert result == mock_creds


def test_revoke_credentials_deletes_token_file(tmp_path):
    """OAuth revoke should delete token file and clear cached credentials."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    # Create dummy files
    creds_path.write_text('{"installed": {"client_id": "test"}}')
    token_path.write_text('{"token": "test_token"}')

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)
    auth._creds = Mock()

    # Verify token file exists
    assert token_path.exists()

    # Revoke credentials
    auth.revoke_credentials()

    # Verify token file was deleted
    assert not token_path.exists()
    assert auth._creds is None


def test_revoke_credentials_when_no_token_file(tmp_path):
    """OAuth revoke should handle case when token file doesn't exist."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)
    auth._creds = Mock()

    # Verify token file doesn't exist
    assert not token_path.exists()

    # Revoke should not raise error
    auth.revoke_credentials()

    assert auth._creds is None


def test_reauthentication_without_refresh_token(tmp_path):
    """OAuth should re-authenticate when token is invalid and no refresh_token."""
    creds_path = tmp_path / "credentials.json"
    token_path = tmp_path / "token.json"

    # Create dummy credentials file
    creds_path.write_text('{"installed": {"client_id": "test"}}')
    token_path.write_text('{"token": "invalid_token"}')

    auth = OAuth2Authenticator(credentials_path=creds_path, token_path=token_path)

    # Mock invalid credentials without refresh token
    mock_old_creds = Mock()
    mock_old_creds.valid = False
    mock_old_creds.expired = True
    mock_old_creds.refresh_token = None

    # Mock new credentials from flow
    mock_new_creds = Mock()
    mock_new_creds.valid = True
    mock_new_creds.to_json.return_value = '{"token": "new_token"}'

    mock_flow = Mock()
    mock_flow.run_local_server.return_value = mock_new_creds

    with patch('gsuite_mcp.auth.oauth.Credentials.from_authorized_user_file', return_value=mock_old_creds):
        with patch('gsuite_mcp.auth.oauth.InstalledAppFlow.from_client_secrets_file', return_value=mock_flow):
            with patch('builtins.open', mock_open()) as mocked_file:
                result = auth.get_credentials()

                # Verify new flow was initiated (not refresh)
                mock_flow.run_local_server.assert_called_once_with(port=0)
                mock_old_creds.refresh.assert_not_called()

                # Verify token was saved
                mocked_file.assert_called_once_with(token_path, 'w')

                assert result == mock_new_creds


def test_default_scopes_are_set():
    """OAuth should use default scopes when none provided."""
    auth = OAuth2Authenticator()

    assert auth.scopes == OAuth2Authenticator.DEFAULT_SCOPES
    assert "https://www.googleapis.com/auth/gmail.modify" in auth.scopes
    assert "https://www.googleapis.com/auth/calendar" in auth.scopes
    assert "https://www.googleapis.com/auth/contacts" in auth.scopes
