#!/usr/bin/env python3
"""
Yad2 Price List Scraper — extracts the base price (מחיר בסיס) for a car
from Yad2's official price list pages.

Uses undetected_chromedriver to bypass PerimeterX/Cloudflare anti-bot.

Usage:
    python yad2_price_scraper.py 103589 2019
    python yad2_price_scraper.py --url "https://www.yad2.co.il/price-list/sub-model/103589/2019"
    python yad2_price_scraper.py 103589 2019 --json
"""

import argparse
import json
import logging
import re
import sys
import time

try:
    import undetected_chromedriver as uc
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.webdriver.support import expected_conditions as EC
    from selenium.common.exceptions import TimeoutException, WebDriverException
except ImportError:
    print(
        "Missing dependencies. Install with:\n"
        "  pip install undetected-chromedriver selenium\n",
        file=sys.stderr,
    )
    sys.exit(1)

REAL_USER_AGENT = (
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
log = logging.getLogger("yad2-scraper")

CAPTCHA_INDICATORS = [
    "px-captcha",
    "challenge-running",
    "cf-challenge-running",
    "captcha-container",
    "human-challenge",
]

BLOCK_TITLE_KEYWORDS = [
    "403", "401", "access denied", "blocked", "just a moment",
    "are you for real",
]

BASE_URL = "https://www.yad2.co.il/price-list/sub-model"
PAGE_LOAD_TIMEOUT = 30
CONTENT_WAIT_TIMEOUT = 15
RETRY_ATTEMPTS = 2


def build_url(sub_model_id: int, year: int) -> str:
    return f"{BASE_URL}/{sub_model_id}/{year}"


def create_driver(*, headless: bool = True) -> uc.Chrome:
    """Create a stealth Chrome instance with anti-detection settings."""
    options = uc.ChromeOptions()
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-dev-shm-usage")
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument("--lang=he-IL,he")
    options.add_argument("--window-size=1920,1080")
    options.add_argument(f"--user-agent={REAL_USER_AGENT}")
    options.add_argument("--disable-extensions")

    # undetected_chromedriver has its own headless handling that patches
    # out the "HeadlessChrome" UA leak and navigator.webdriver flag.
    driver = uc.Chrome(options=options, headless=headless, version_main=None)
    driver.set_page_load_timeout(PAGE_LOAD_TIMEOUT)

    # Override UA at the CDP level too, to cover all request headers.
    driver.execute_cdp_cmd("Network.setUserAgentOverride", {
        "userAgent": REAL_USER_AGENT,
        "platform": "Linux",
    })
    driver.execute_cdp_cmd("Network.setExtraHTTPHeaders", {
        "headers": {"Accept-Language": "he-IL,he;q=0.9,en;q=0.8"}
    })
    return driver


def detect_block(driver) -> str | None:
    """Return a description of the block type, or None if page looks normal."""
    try:
        page_source = driver.page_source.lower()
    except WebDriverException:
        return "driver crashed or page unresponsive"

    for indicator in CAPTCHA_INDICATORS:
        if indicator in page_source:
            return f"CAPTCHA detected ({indicator})"

    title = driver.title.lower()
    for keyword in BLOCK_TITLE_KEYWORDS:
        if keyword in title:
            return f"block page detected (title contains '{keyword}')"

    return None


def extract_price_from_text(text: str) -> int | None:
    """Parse a Hebrew price string like '₪ 53,600' into 53600.

    Prefers the number immediately associated with the ₪ currency symbol.
    Falls back to the last numeric token if no currency-adjacent number is found.
    """
    # First try: number directly adjacent to ₪ (e.g. "₪ 53,600" or "53,600 ₪")
    currency_match = re.search(r"₪\s*([\d,]+)", text) or re.search(r"([\d,]+)\s*₪", text)
    if currency_match:
        val = int(currency_match.group(1).replace(",", ""))
        if val > 0:
            return val

    # Fallback: take the last numeric token (least likely to be a year/model)
    cleaned = text.replace(",", "").replace("₪", "").replace("\u200f", "").strip()
    tokens = re.findall(r"\d+", cleaned)
    if tokens:
        return int(tokens[-1])
    return None


def extract_base_price(driver) -> dict:
    """
    Extract the base price and car metadata from a loaded Yad2 price list page.
    Returns a dict with price, title, and raw text.
    """
    result = {
        "base_price": None,
        "title": None,
        "raw_price_text": None,
        "url": driver.current_url,
    }

    # Extract car title from the h1
    try:
        h1 = driver.find_element(By.CSS_SELECTOR, "h1")
        result["title"] = h1.text.strip()
    except Exception:
        pass

    # Strategy 1: Find the "מחיר בסיס" label and grab the adjacent price.
    # The page structure has a label "מחיר בסיס" followed by a price element.
    try:
        elements = driver.find_elements(By.XPATH, "//*[contains(text(), 'מחיר בסיס')]")
        for el in elements:
            parent = el.find_element(By.XPATH, "./..")
            price_text = parent.text
            price = extract_price_from_text(price_text)
            if price and price > 1000:
                result["base_price"] = price
                result["raw_price_text"] = price_text.strip()
                return result
    except Exception as e:
        log.debug("Strategy 1 (מחיר בסיס label) failed: %s", e)

    # Strategy 2: Look for price elements near the weighted-price section.
    try:
        price_elements = driver.find_elements(
            By.XPATH, "//*[contains(text(), '₪')]"
        )
        for el in price_elements:
            text = el.text.strip()
            price = extract_price_from_text(text)
            if price and price > 1000:
                result["base_price"] = price
                result["raw_price_text"] = text
                return result
    except Exception as e:
        log.debug("Strategy 2 (₪ text scan) failed: %s", e)

    # Strategy 3: Regex the entire page source for price patterns.
    try:
        source = driver.page_source
        matches = re.findall(r"₪\s*([\d,]+)", source)
        for m in matches:
            price = int(m.replace(",", ""))
            if price > 1000:
                result["base_price"] = price
                result["raw_price_text"] = f"₪ {m}"
                return result
    except Exception as e:
        log.debug("Strategy 3 (page source regex) failed: %s", e)

    return result


def _wait_for_challenge(driver, timeout: int = 20) -> bool:
    """
    If PerimeterX shows the "Are you for real?" challenge, wait for it
    to auto-resolve (it sometimes does after a JS proof-of-work).
    Returns True if the page eventually loaded, False if still blocked.
    """
    for _ in range(timeout // 2):
        block = detect_block(driver)
        if not block:
            return True
        time.sleep(2)
    return False


def _try_scrape(url: str, *, headless: bool) -> dict:
    """Single scrape attempt with the given headless setting."""
    driver = None
    try:
        mode = "headless" if headless else "headed"
        log.info("Opening browser (%s)...", mode)
        driver = create_driver(headless=headless)

        # Warm the session on the price-list homepage first to build
        # cookies and pass initial fingerprinting before the target page.
        log.info("Warming session on homepage...")
        driver.get("https://www.yad2.co.il/price-list")
        time.sleep(4)

        # Check if even the homepage is blocked.
        if detect_block(driver):
            log.info("Homepage challenge detected, waiting for auto-resolve...")
            if not _wait_for_challenge(driver):
                return {"error": "Blocked on homepage", "url": url, "base_price": None}
            time.sleep(2)

        log.info("Navigating to target page...")
        driver.get(url)

        # Wait for the SPA to render the price content.
        # The page loads an SPA shell ("Welcome to price-list!") first,
        # then fetches and renders the sub-model data asynchronously.
        try:
            WebDriverWait(driver, CONTENT_WAIT_TIMEOUT).until(
                lambda d: "מחיר בסיס" in d.page_source or "₪" in d.page_source
            )
            log.debug("Price content detected in page source")
        except TimeoutException:
            log.debug("Timed out waiting for price content to render")

        time.sleep(2)

        # Handle challenge on the target page.
        block = detect_block(driver)
        if block:
            log.info("Challenge on target page: %s — waiting...", block)
            if not _wait_for_challenge(driver):
                return {
                    "error": f"Blocked by anti-bot: {block}",
                    "url": url,
                    "base_price": None,
                }
            # Re-navigate after challenge clears.
            driver.get(url)
            time.sleep(4)

        result = extract_base_price(driver)
        if result["base_price"]:
            log.info(
                "Extracted base price: ₪%s (%s)",
                f"{result['base_price']:,}",
                result.get("title", "unknown"),
            )
        else:
            result["error"] = "Price element not found on page"
            current_url = driver.current_url
            title = driver.title
            log.warning(
                "Price not found — title=%r url=%s", title, current_url
            )
            # Check if we landed on a redirect/error page.
            if "validate" in current_url or "perfdrive" in current_url:
                result["error"] = "Redirected to bot validation page"
            elif not title or title == "":
                result["error"] = "Page failed to render (blank)"
        return result

    except TimeoutException:
        return {"error": "Page load timed out", "url": url, "base_price": None}
    except WebDriverException as e:
        return {"error": str(e), "url": url, "base_price": None}
    except Exception as e:
        return {"error": str(e), "url": url, "base_price": None}
    finally:
        if driver:
            try:
                driver.quit()
            except Exception:
                pass


def scrape_price(url: str, retries: int = RETRY_ATTEMPTS) -> dict:
    """Navigate to the URL and extract the base price with retry logic."""
    result = {"error": "no attempts made", "url": url, "base_price": None}

    for attempt in range(1, retries + 1):
        log.info("Attempt %d/%d — %s", attempt, retries, url)
        result = _try_scrape(url, headless=True)

        if result.get("base_price"):
            return result

        if attempt < retries:
            wait = 5 * attempt
            log.info("Waiting %ds before retry...", wait)
            time.sleep(wait)

    return result


def main():
    parser = argparse.ArgumentParser(
        description="Extract base price from Yad2 car price list",
        epilog="Example: %(prog)s 103589 2019",
    )
    parser.add_argument("sub_model_id", nargs="?", type=int, help="Yad2 sub-model ID")
    parser.add_argument("year", nargs="?", type=int, help="Car year")
    parser.add_argument("--url", type=str, help="Direct Yad2 price-list URL")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    def positive_int(value):
        ival = int(value)
        if ival < 1:
            raise argparse.ArgumentTypeError(f"retries must be >= 1, got {ival}")
        return ival

    parser.add_argument("--retries", type=positive_int, default=RETRY_ATTEMPTS)
    parser.add_argument("-v", "--verbose", action="store_true")
    args = parser.parse_args()

    if args.verbose:
        log.setLevel(logging.DEBUG)

    if args.url:
        url = args.url
    elif args.sub_model_id and args.year:
        url = build_url(args.sub_model_id, args.year)
    else:
        parser.error("Provide SUB_MODEL_ID YEAR or --url")
        return

    result = scrape_price(url, retries=args.retries)

    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
        if not result.get("base_price"):
            sys.exit(1)
    elif result.get("base_price"):
        print(result["base_price"])
    else:
        print(f"ERROR: {result.get('error', 'unknown')}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
