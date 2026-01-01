"""Staging area management for crawled memes."""

import json
import shutil
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass
class StagingItem:
    """A staged meme item.

    Attributes:
        id: Unique identifier for the item.
        filename: Filename of the stored image.
        category: Category name.
        tags: List of tag strings.
        source_url: Original image URL.
        is_animated: Whether the image is animated.
        format: Image format (jpg, png, gif, webp).
        crawled_at: ISO timestamp of when the item was crawled.
    """

    id: str
    filename: str
    category: str
    tags: list[str]
    source_url: str
    is_animated: bool
    format: str
    crawled_at: str

    def to_dict(self) -> dict:
        """Convert the staging item to a dictionary.

        Returns:
            Dictionary representation of the staging item.
        """
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> "StagingItem":
        """Create a StagingItem from a dictionary.

        Args:
            data: Dictionary containing staging item fields.

        Returns:
            StagingItem instance built from the dictionary.
        """
        return cls(**data)


@dataclass
class StagingStats:
    """Statistics for a staging source.

    Attributes:
        source_id: Source identifier.
        total_images: Total number of staged images.
        total_size_bytes: Total size of staged images in bytes.
        categories: Count of items per category.
        formats: Count of items per image format.
    """

    source_id: str
    total_images: int
    total_size_bytes: int
    categories: dict[str, int]
    formats: dict[str, int]


class StagingManager:
    """Manages the staging area for crawled memes.

    Attributes:
        base_path: Base path for the staging directory.
    """

    def __init__(self, base_path: Path | str):
        """Initialize the staging manager.

        Args:
            base_path: Base path for the staging directory.

        Returns:
            None.
        """
        self.base_path = Path(base_path)

    def _get_source_path(self, source_id: str) -> Path:
        """Get the path for a specific source.

        Args:
            source_id: The source identifier.

        Returns:
            Path to the source directory.
        """
        return self.base_path / source_id

    def _get_images_path(self, source_id: str) -> Path:
        """Get the images directory for a specific source.

        Args:
            source_id: The source identifier.

        Returns:
            Path to the images directory.
        """
        return self._get_source_path(source_id) / "images"

    def _get_manifest_path(self, source_id: str) -> Path:
        """Get the manifest file path for a specific source.

        Args:
            source_id: The source identifier.

        Returns:
            Path to the manifest.jsonl file.
        """
        return self._get_source_path(source_id) / "manifest.jsonl"

    def ensure_directories(self, source_id: str) -> None:
        """Ensure the staging directories exist for a source.

        Args:
            source_id: The source identifier.

        Returns:
            None.
        """
        images_path = self._get_images_path(source_id)
        images_path.mkdir(parents=True, exist_ok=True)

    def save_image(
        self,
        source_id: str,
        item_id: str,
        image_data: bytes,
        format: str,
    ) -> str:
        """Save an image to the staging area.

        Args:
            source_id: The source identifier.
            item_id: Unique ID for the image.
            image_data: Raw image bytes.
            format: Image format (jpg, png, gif, webp).

        Returns:
            The filename of the saved image.
        """
        self.ensure_directories(source_id)

        filename = f"{item_id}.{format}"
        file_path = self._get_images_path(source_id) / filename

        with open(file_path, "wb") as f:
            f.write(image_data)

        return filename

    def append_manifest(self, source_id: str, item: StagingItem) -> None:
        """Append an item to the manifest file.

        Args:
            source_id: The source identifier.
            item: The staging item to append.

        Returns:
            None.
        """
        self.ensure_directories(source_id)

        manifest_path = self._get_manifest_path(source_id)
        line = json.dumps(item.to_dict(), ensure_ascii=False) + "\n"

        with open(manifest_path, "a", encoding="utf-8") as f:
            f.write(line)

    def read_manifest(self, source_id: str) -> list[StagingItem]:
        """Read all items from the manifest.

        Args:
            source_id: The source identifier.

        Returns:
            List of staging items.
        """
        manifest_path = self._get_manifest_path(source_id)

        if not manifest_path.exists():
            return []

        items = []
        with open(manifest_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line:
                    data = json.loads(line)
                    items.append(StagingItem.from_dict(data))

        return items

    async def get_existing_ids(self, source_id: str) -> set[str]:
        """Get set of existing item IDs to avoid duplicates.

        Args:
            source_id: The source identifier.

        Returns:
            Set of existing item IDs.
        """
        items = self.read_manifest(source_id)
        return {item.id for item in items}

    def list_sources(self) -> list[str]:
        """List all source directories in the staging area.

        Returns:
            List of source IDs.
        """
        if not self.base_path.exists():
            return []

        return [
            d.name
            for d in self.base_path.iterdir()
            if d.is_dir() and (d / "manifest.jsonl").exists()
        ]

    async def get_stats(self, source_id: str) -> StagingStats:
        """Get statistics for a staging source.

        Args:
            source_id: The source identifier.

        Returns:
            Statistics for the source.
        """
        items = self.read_manifest(source_id)
        images_path = self._get_images_path(source_id)

        total_size = 0
        categories: dict[str, int] = {}
        formats: dict[str, int] = {}

        for item in items:
            # Count categories
            categories[item.category] = categories.get(item.category, 0) + 1

            # Count formats
            formats[item.format] = formats.get(item.format, 0) + 1

            # Calculate size
            image_path = images_path / item.filename
            if image_path.exists():
                total_size += image_path.stat().st_size

        return StagingStats(
            source_id=source_id,
            total_images=len(items),
            total_size_bytes=total_size,
            categories=categories,
            formats=formats,
        )

    def clean(self, source_id: str) -> None:
        """Clean the staging area for a source.

        Args:
            source_id: The source identifier.

        Returns:
            None.
        """
        source_path = self._get_source_path(source_id)
        if source_path.exists():
            shutil.rmtree(source_path)

    def clean_all(self) -> None:
        """Clean all staging sources.

        Returns:
            None.
        """
        if self.base_path.exists():
            shutil.rmtree(self.base_path)


def create_staging_item(
    item_id: str,
    filename: str,
    category: str,
    tags: list[str],
    source_url: str,
    is_animated: bool,
    format: str,
) -> StagingItem:
    """Create a staging item with current timestamp.

    Args:
        item_id: Unique ID for the item.
        filename: Saved filename.
        category: Category name.
        tags: List of tags.
        source_url: Original source URL.
        is_animated: Whether the image is animated.
        format: Image format.

    Returns:
        A new StagingItem.
    """
    return StagingItem(
        id=item_id,
        filename=filename,
        category=category,
        tags=tags,
        source_url=source_url,
        is_animated=is_animated,
        format=format,
        crawled_at=datetime.now(timezone.utc).isoformat(),
    )
