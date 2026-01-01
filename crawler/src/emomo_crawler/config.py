"""Configuration management for emomo-crawler."""

from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class StagingConfig:
    """Staging directory configuration.

    Attributes:
        base_path: Base path for the staging directory.
    """

    # Default to project root's data/staging (one level up from crawler/)
    base_path: Path = field(default_factory=lambda: Path(__file__).parent.parent.parent.parent / "data" / "staging")


@dataclass
class Config:
    """Main configuration.

    Attributes:
        staging: Staging configuration section.
    """

    staging: StagingConfig = field(default_factory=StagingConfig)


def get_default_config() -> Config:
    """Get default configuration.

    Returns:
        Default Config instance with standard paths.
    """
    return Config()
