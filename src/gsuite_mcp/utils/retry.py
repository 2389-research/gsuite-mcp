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
