"""Fabiaoqing.com crawler using requests + BeautifulSoup."""

import hashlib
import os
import random
import re
import ssl
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.parse import urljoin, urlparse

import requests
from bs4 import BeautifulSoup
from requests.adapters import HTTPAdapter

from ..base import BaseCrawler, MemeItem, detect_format, generate_id_from_url
from ..staging import StagingManager, create_staging_item

# Request headers
HEADERS = {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
    "Accept-Encoding": "gzip, deflate, br",
    "Connection": "keep-alive",
}


class TLSAdapter(HTTPAdapter):
    """Custom adapter for TLS configuration."""

    def init_poolmanager(self, *args, **kwargs):
        context = ssl.create_default_context()
        context.set_ciphers("DEFAULT@SECLEVEL=1")
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.maximum_version = ssl.TLSVersion.TLSv1_2
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        kwargs["ssl_context"] = context
        return super().init_poolmanager(*args, **kwargs)

    def proxy_manager_for(self, *args, **kwargs):
        context = ssl.create_default_context()
        context.set_ciphers("DEFAULT@SECLEVEL=1")
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.maximum_version = ssl.TLSVersion.TLSv1_2
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        kwargs["ssl_context"] = context
        return super().proxy_manager_for(*args, **kwargs)


class FabiaoqingCrawler(BaseCrawler):
    """Crawler for fabiaoqing.com meme website."""

    BASE_URL = "https://fabiaoqing.com"
    LIST_URL = "https://fabiaoqing.com/biaoqing/lists/page/{page}.html"

    def __init__(
        self,
        rate_limit: float = 2.0,
        max_retries: int = 3,
        timeout: int = 15,
        threads: int = 5,
    ):
        super().__init__(rate_limit, max_retries, timeout)
        self.threads = threads
        self.session = self._create_session()

    @property
    def source_id(self) -> str:
        return "fabiaoqing"

    @property
    def display_name(self) -> str:
        return "发表情 (fabiaoqing.com)"

    def _create_session(self) -> requests.Session:
        """Create session with SSL fix."""
        import urllib3

        urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

        session = requests.Session()
        session.headers.update(HEADERS)
        session.headers["Referer"] = self.BASE_URL
        session.mount("https://", TLSAdapter())
        session.verify = False
        return session

    def _get_page(self, url: str) -> str | None:
        """Fetch page content with delay."""
        try:
            delay = 1.0 / self.rate_limit
            time.sleep(random.uniform(delay * 0.5, delay * 1.5))
            response = self.session.get(url, timeout=self.timeout)
            response.raise_for_status()
            response.encoding = response.apparent_encoding or "utf-8"
            return response.text
        except requests.RequestException as e:
            print(f"[ERROR] Failed to fetch {url}: {e}")
            return None

    def _parse_images(self, html: str) -> list[MemeItem]:
        """Parse meme images from HTML."""
        soup = BeautifulSoup(html, "html.parser")
        items = []
        found_urls = set()

        # Multiple selectors to find images
        selectors = [
            "img.ui.image.lazy",
            "img[data-original]",
            ".bqppdiv img",
            ".tagbqppdiv img",
            ".bqba img",
            "img.lazy",
            ".image img",
            "a.image img",
        ]

        for selector in selectors:
            for img in soup.select(selector):
                src = img.get("data-original") or img.get("src") or img.get("data-src")
                if not src:
                    continue

                # Filter placeholders and icons
                if "placeholder" in src.lower() or "icon" in src.lower():
                    continue
                if "lazyload" in src.lower() or "transparent.gif" in src.lower():
                    continue

                # Normalize URL
                if src.startswith("//"):
                    src = "https:" + src
                elif src.startswith("/"):
                    src = urljoin(self.BASE_URL, src)

                # Only keep image files
                if not any(
                    ext in src.lower() for ext in [".gif", ".jpg", ".jpeg", ".png", ".webp"]
                ):
                    continue

                if src in found_urls:
                    continue
                found_urls.add(src)

                # Extract metadata
                alt = img.get("alt", "")
                item_id = generate_id_from_url(src)
                fmt = detect_format(src)

                items.append(
                    MemeItem(
                        id=item_id,
                        image_url=src,
                        category=self._extract_category(alt) or "表情包",
                        tags=self._extract_tags(alt),
                        is_animated=fmt == "gif",
                        format=fmt,
                    )
                )

        return items

    def _extract_tags(self, text: str) -> list[str]:
        """Extract tags from alt text."""
        if not text:
            return []
        parts = re.split(r"[,，、\s_\-]+", text.strip())
        tags = []
        for part in parts:
            part = part.strip()
            if 2 <= len(part) <= 20 and not part.isdigit():
                tags.append(part)
        return tags[:5]

    def _extract_category(self, text: str) -> str | None:
        """Extract category from alt text."""
        if not text:
            return None
        patterns = [
            r"(熊猫头|蘑菇头|金馆长|张学友|姚明|暴漫|表情包)",
            r"^([^_\-\s]+?)(?:表情|系列)",
        ]
        for pattern in patterns:
            match = re.search(pattern, text)
            if match:
                return match.group(1)
        return None

    def _download_image(self, url: str) -> bytes | None:
        """Download image data."""
        try:
            response = self.session.get(url, timeout=self.timeout)
            response.raise_for_status()
            return response.content
        except Exception as e:
            print(f"[ERROR] Download failed {url}: {e}")
            return None

    async def crawl(
        self,
        staging: StagingManager,
        limit: int,
        cursor: str | None = None,
    ) -> tuple[int, str | None]:
        """Crawl memes from fabiaoqing.com.

        Args:
            staging: Staging manager instance.
            limit: Maximum number of items to crawl.
            cursor: Starting page number (default: "1").

        Returns:
            Tuple of (items_crawled, next_cursor).
        """
        # Get existing IDs
        existing_ids = await staging.get_existing_ids(self.source_id)
        print(f"[INFO] {len(existing_ids)} existing items in staging")

        # Start from cursor page or page 1
        current_page = int(cursor) if cursor else 1
        all_items: list[MemeItem] = []

        print(f"[INFO] Starting from page {current_page}...")

        # Collect images from list pages
        while len(all_items) < limit:
            url = self.LIST_URL.format(page=current_page)
            print(f"[INFO] Fetching page {current_page}: {url}")

            html = self._get_page(url)
            if not html:
                print(f"[WARN] Failed to fetch page {current_page}")
                break

            items = self._parse_images(html)
            if not items:
                print(f"[INFO] No more images found at page {current_page}")
                break

            # Filter out existing items
            new_items = [i for i in items if i.id not in existing_ids]
            for item in new_items:
                existing_ids.add(item.id)  # Prevent duplicates within this run
            all_items.extend(new_items)

            print(f"  Page {current_page}: found {len(items)}, new {len(new_items)}, total {len(all_items)}")
            current_page += 1

        # Limit to requested amount
        items_to_download = all_items[:limit]
        print(f"[INFO] Downloading {len(items_to_download)} images...")

        # Download and save images
        items_crawled = 0

        def process_item(item: MemeItem):
            data = self._download_image(item.image_url)
            if not data:
                return None
            return (item, data)

        with ThreadPoolExecutor(max_workers=self.threads) as executor:
            futures = {executor.submit(process_item, item): item for item in items_to_download}

            for future in as_completed(futures):
                result = future.result()
                if not result:
                    continue

                item, data = result

                # Save image
                filename = f"{item.id}.{item.format}"
                images_path = staging._get_images_path(self.source_id)
                images_path.mkdir(parents=True, exist_ok=True)

                file_path = images_path / filename
                with open(file_path, "wb") as f:
                    f.write(data)

                # Append to manifest
                staging_item = create_staging_item(
                    item_id=item.id,
                    filename=filename,
                    category=item.category,
                    tags=item.tags,
                    source_url=item.image_url,
                    is_animated=item.is_animated,
                    format=item.format,
                )

                manifest_path = staging._get_manifest_path(self.source_id)
                import json

                with open(manifest_path, "a", encoding="utf-8") as f:
                    f.write(json.dumps(staging_item.to_dict(), ensure_ascii=False) + "\n")

                items_crawled += 1
                size_kb = len(data) / 1024
                print(f"[OK] {filename} ({size_kb:.1f}KB)")

        # Return next page as cursor for continuation
        next_cursor = str(current_page) if items_crawled > 0 else None
        return items_crawled, next_cursor
