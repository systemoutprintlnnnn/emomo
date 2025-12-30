"""Base crawler class and common utilities."""

import hashlib
from abc import ABC, abstractmethod
from dataclasses import dataclass

from .staging import StagingManager


@dataclass
class MemeItem:
    """Represents a meme item to be crawled."""

    id: str
    image_url: str
    category: str
    tags: list[str]
    is_animated: bool
    format: str  # jpg, png, gif, webp


def generate_id_from_url(url: str) -> str:
    """Generate a unique ID from URL.

    Args:
        url: The URL to generate ID from.

    Returns:
        A unique ID string.
    """
    return hashlib.md5(url.encode()).hexdigest()[:16]


def detect_format(url: str, content_type: str | None = None) -> str:
    """Detect image format from URL or content type.

    Args:
        url: Image URL.
        content_type: Optional content-type header.

    Returns:
        Image format (jpg, png, gif, webp).
    """
    url_lower = url.lower()

    if ".gif" in url_lower:
        return "gif"
    elif ".png" in url_lower:
        return "png"
    elif ".webp" in url_lower:
        return "webp"
    elif ".jpg" in url_lower or ".jpeg" in url_lower:
        return "jpg"

    if content_type:
        if "gif" in content_type:
            return "gif"
        elif "png" in content_type:
            return "png"
        elif "webp" in content_type:
            return "webp"

    return "jpg"  # Default


class BaseCrawler(ABC):
    """Abstract base class for meme crawlers."""

    def __init__(
        self,
        rate_limit: float = 2.0,
        max_retries: int = 3,
        timeout: int = 30,
    ):
        """Initialize the crawler.

        Args:
            rate_limit: Requests per second.
            max_retries: Maximum retry attempts.
            timeout: Request timeout in seconds.
        """
        self.rate_limit = rate_limit
        self.max_retries = max_retries
        self.timeout = timeout

    @property
    @abstractmethod
    def source_id(self) -> str:
        """Return the unique source identifier."""
        pass

    @property
    @abstractmethod
    def display_name(self) -> str:
        """Return a human-readable name for this source."""
        pass

    @abstractmethod
    async def crawl(
        self,
        staging: StagingManager,
        limit: int,
        cursor: str | None = None,
    ) -> tuple[int, str | None]:
        """Crawl memes and save to staging area.

        Args:
            staging: Staging manager instance.
            limit: Maximum number of items to crawl.
            cursor: Optional cursor for pagination (e.g., page number).

        Returns:
            Tuple of (items_crawled, next_cursor).
            next_cursor is None if no more items.
        """
        pass
