"""Command line interface for emomo-crawler."""

import asyncio
from pathlib import Path

import click
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table

from .config import get_default_config
from .sources import FabiaoqingCrawler
from .staging import StagingManager

console = Console()

# Available crawlers
CRAWLERS = {
    "fabiaoqing": FabiaoqingCrawler,
}


def get_staging_manager(staging_path: str | None = None) -> StagingManager:
    """Get staging manager with configured path."""
    config = get_default_config()
    path = Path(staging_path) if staging_path else config.staging.base_path
    return StagingManager(path)


def get_crawler(source: str, **kwargs):
    """Get crawler instance by source name."""
    if source not in CRAWLERS:
        raise click.BadParameter(
            f"Unknown source: {source}. Available: {', '.join(CRAWLERS.keys())}"
        )
    return CRAWLERS[source](**kwargs)


@click.group()
@click.version_option()
def cli():
    """Emomo Crawler - Meme crawler for emomo project."""
    pass


@cli.command()
@click.option(
    "--source",
    "-s",
    required=True,
    type=click.Choice(list(CRAWLERS.keys())),
    help="Crawler source to use.",
)
@click.option(
    "--limit",
    "-l",
    default=100,
    type=int,
    help="Maximum number of items to crawl.",
)
@click.option(
    "--cursor",
    "-c",
    default=None,
    help="Starting cursor (e.g., page number).",
)
@click.option(
    "--staging-path",
    default=None,
    help="Custom staging directory path.",
)
@click.option(
    "--rate-limit",
    default=2.0,
    type=float,
    help="Requests per second.",
)
@click.option(
    "--threads",
    "-t",
    default=5,
    type=int,
    help="Number of download threads.",
)
def crawl(
    source: str,
    limit: int,
    cursor: str | None,
    staging_path: str | None,
    rate_limit: float,
    threads: int,
):
    """Crawl memes from a source and save to staging area."""
    staging = get_staging_manager(staging_path)
    crawler = get_crawler(source, rate_limit=rate_limit, threads=threads)

    console.print(f"[bold blue]Starting crawler:[/] {crawler.display_name}")
    console.print(f"[dim]Limit: {limit}, Rate: {rate_limit} req/s[/]")
    if cursor:
        console.print(f"[dim]Starting from cursor: {cursor}[/]")

    async def run_crawl():
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        ) as progress:
            task = progress.add_task(f"Crawling {source}...", total=None)

            items_crawled, next_cursor = await crawler.crawl(
                staging=staging,
                limit=limit,
                cursor=cursor,
            )

            progress.update(task, completed=True)

            return items_crawled, next_cursor

    items_crawled, next_cursor = asyncio.run(run_crawl())

    console.print()
    console.print(f"[bold green]Crawled {items_crawled} items[/]")

    if next_cursor:
        console.print(f"[dim]Next cursor: {next_cursor}[/]")
        console.print(
            f"[dim]Continue with: emomo-crawler crawl -s {source} -c {next_cursor}[/]"
        )
    else:
        console.print("[dim]No more items to crawl.[/]")


@cli.group()
def staging():
    """Manage the staging area."""
    pass


@staging.command("list")
@click.option(
    "--staging-path",
    default=None,
    help="Custom staging directory path.",
)
def staging_list(staging_path: str | None):
    """List all sources in the staging area."""
    staging = get_staging_manager(staging_path)
    sources = staging.list_sources()

    if not sources:
        console.print("[yellow]No sources in staging area.[/]")
        return

    table = Table(title="Staging Sources")
    table.add_column("Source", style="cyan")
    table.add_column("Path", style="dim")

    for source_id in sources:
        path = staging._get_source_path(source_id)
        table.add_row(source_id, str(path))

    console.print(table)


@staging.command("stats")
@click.option(
    "--source",
    "-s",
    required=True,
    help="Source to get stats for.",
)
@click.option(
    "--staging-path",
    default=None,
    help="Custom staging directory path.",
)
def staging_stats(source: str, staging_path: str | None):
    """Show statistics for a staging source."""
    staging = get_staging_manager(staging_path)

    async def get_stats():
        return await staging.get_stats(source)

    stats = asyncio.run(get_stats())

    console.print(f"[bold]Statistics for [cyan]{source}[/cyan][/]")
    console.print()

    # Summary table
    summary = Table(show_header=False)
    summary.add_column("Metric", style="dim")
    summary.add_column("Value", style="bold")

    summary.add_row("Total Images", str(stats.total_images))
    summary.add_row("Total Size", f"{stats.total_size_bytes / 1024 / 1024:.2f} MB")
    console.print(summary)

    # Categories table
    if stats.categories:
        console.print()
        cat_table = Table(title="Categories")
        cat_table.add_column("Category", style="cyan")
        cat_table.add_column("Count", justify="right")

        for cat, count in sorted(stats.categories.items(), key=lambda x: -x[1])[:10]:
            cat_table.add_row(cat, str(count))

        console.print(cat_table)

    # Formats table
    if stats.formats:
        console.print()
        fmt_table = Table(title="Formats")
        fmt_table.add_column("Format", style="cyan")
        fmt_table.add_column("Count", justify="right")

        for fmt, count in sorted(stats.formats.items(), key=lambda x: -x[1]):
            fmt_table.add_row(fmt, str(count))

        console.print(fmt_table)


@staging.command("clean")
@click.option(
    "--source",
    "-s",
    required=True,
    help="Source to clean.",
)
@click.option(
    "--staging-path",
    default=None,
    help="Custom staging directory path.",
)
@click.option(
    "--yes",
    "-y",
    is_flag=True,
    help="Skip confirmation.",
)
def staging_clean(source: str, staging_path: str | None, yes: bool):
    """Clean the staging area for a source."""
    staging = get_staging_manager(staging_path)

    if source not in staging.list_sources():
        console.print(f"[yellow]Source '{source}' not found in staging.[/]")
        return

    if not yes:
        if not click.confirm(f"Are you sure you want to delete staging for '{source}'?"):
            console.print("[dim]Cancelled.[/]")
            return

    staging.clean(source)
    console.print(f"[green]Cleaned staging for '{source}'.[/]")


@staging.command("clean-all")
@click.option(
    "--staging-path",
    default=None,
    help="Custom staging directory path.",
)
@click.option(
    "--yes",
    "-y",
    is_flag=True,
    help="Skip confirmation.",
)
def staging_clean_all(staging_path: str | None, yes: bool):
    """Clean the entire staging area."""
    staging = get_staging_manager(staging_path)

    if not yes:
        if not click.confirm("Are you sure you want to delete ALL staging data?"):
            console.print("[dim]Cancelled.[/]")
            return

    staging.clean_all()
    console.print("[green]Cleaned all staging data.[/]")


if __name__ == "__main__":
    cli()
