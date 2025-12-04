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

    wrapped_func = retry_with_backoff(mock_func)
    result = wrapped_func()

    assert result == "success"
    assert mock_func.call_count == 3


def test_retry_exhausts_attempts():
    """Retry should raise after max attempts."""
    mock_func = Mock(side_effect=HttpError(Mock(status=429), b"Rate limit"))

    wrapped_func = retry_with_backoff(mock_func, max_retries=3)

    with pytest.raises(HttpError):
        wrapped_func()

    assert mock_func.call_count == 4  # initial + 3 retries


def test_retry_does_not_retry_client_errors():
    """Retry should not retry 4xx client errors (except 429)."""
    mock_func = Mock(side_effect=HttpError(Mock(status=400), b"Bad request"))

    wrapped_func = retry_with_backoff(mock_func)

    with pytest.raises(HttpError):
        wrapped_func()

    assert mock_func.call_count == 1  # no retries
