#!/usr/bin/env python3
"""Capture the isolated Nexus failure-recovery visual review gallery."""

from __future__ import annotations

import argparse
from pathlib import Path
from urllib.parse import urlencode

from playwright.sync_api import sync_playwright


STORIES = (
    "feedback-not-applied",
    "feedback-accepted",
    "feedback-committed-refresh",
    "feedback-outcome-unknown",
    "resource-load-failed",
    "resource-stale-snapshot",
    "conversation-delivery-unknown",
    "editor-conflict",
    "editor-outcome-unknown",
    "provider-persist-unknown",
    "destructive-outcome-unknown",
)

VIEWPORTS = {
    "desktop": {"width": 1180, "height": 820},
    "mobile": {"width": 390, "height": 844},
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:4173/visual-review/failure-recovery/",
    )
    parser.add_argument(
        "--output-dir",
        default=str(
            Path(__file__).resolve().parents[2]
            / "docs"
            / "reviews"
            / "failure-recovery-visuals"
        ),
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    captured = 0

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(
            executable_path=playwright.chromium.executable_path,
            headless=True,
        )
        try:
            for viewport_name, viewport in VIEWPORTS.items():
                context = browser.new_context(
                    color_scheme="light",
                    device_scale_factor=1,
                    locale="zh-CN",
                    reduced_motion="reduce",
                    viewport=viewport,
                )
                context.add_init_script(
                    "localStorage.setItem('nexus-locale', 'zh');"
                    "localStorage.setItem('nexus-theme', 'light');"
                )
                page = context.new_page()
                console_errors: list[str] = []
                page.on(
                    "console",
                    lambda message: console_errors.append(message.text)
                    if message.type == "error"
                    else None,
                )
                page.on("pageerror", lambda error: console_errors.append(str(error)))
                for story in STORIES:
                    query = urlencode({"story": story})
                    page.goto(f"{args.base_url}?{query}", wait_until="networkidle")
                    page.locator(
                        f'[data-review-ready="true"][data-review-story="{story}"]'
                    ).wait_for(state="visible")
                    page.evaluate("document.fonts.ready")
                    page.locator(".review-stage").screenshot(
                        animations="disabled",
                        path=output_dir / f"{viewport_name}--{story}.png",
                    )
                    captured += 1
                if console_errors:
                    raise RuntimeError(
                        f"{viewport_name} gallery emitted browser errors: "
                        + " | ".join(console_errors)
                    )
                context.close()
        finally:
            browser.close()

    print(f"captured {captured} screenshots in {output_dir}")


if __name__ == "__main__":
    main()
