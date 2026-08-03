"""Artifact production primitives shared by build, verify, and the MCP server.

One home for the render pipeline (read HTML, highlight code blocks, WeasyPrint
to PDF, stamp Kami metadata, count pages) and the PPTX fallback build. Before
this module existed the pipeline lived in build.py and was duplicated by
verify.py (through injected callbacks, to dodge a circular import) and the MCP
server (through a late import). Keeping it here, below build.py in the import
graph, dissolves both workarounds.
"""
from __future__ import annotations

import functools
import os
import subprocess
import sys
from pathlib import Path

from highlight import highlight_code_blocks
from optional_deps import (
    MissingDepError,
    require_pypdf_reader,
    require_pypdf_writer,
    require_weasyprint_html,
)
from shared import EXAMPLES, TEMPLATES, pptx_targets


@functools.lru_cache(maxsize=1)
def infer_author() -> str:
    """Infer author name from git config or environment.

    Priority:
    1. git config user.name
    2. KAMI_AUTHOR env var
    3. fallback to "Kami"

    Cached so a full build doesn't shell out for every PDF target.
    """
    try:
        result = subprocess.run(
            ["git", "config", "user.name"],
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
    except FileNotFoundError:
        pass

    if env_author := os.environ.get("KAMI_AUTHOR"):
        return env_author

    return "Kami"


def set_pdf_metadata(pdf_path: Path, author: str | None = None) -> None:
    """Set PDF metadata using pypdf, only if placeholders are still present."""
    try:
        PdfReader = require_pypdf_reader()
        PdfWriter = require_pypdf_writer()
    except MissingDepError:
        return

    if not pdf_path.exists():
        return

    reader = PdfReader(str(pdf_path))

    existing = reader.metadata or {}
    needs_update = False
    metadata = dict(existing)

    if author and existing.get("/Author"):
        author_value = str(existing["/Author"])
        if "{{" in author_value and "}}" in author_value:
            metadata["/Author"] = author
            needs_update = True

    if metadata.get("/Producer") != "Kami":
        metadata["/Producer"] = "Kami"
        needs_update = True
    if metadata.get("/Creator") != "Kami":
        metadata["/Creator"] = "Kami"
        needs_update = True

    if not needs_update:
        return

    writer = PdfWriter()
    for page in reader.pages:
        writer.add_page(page)

    writer.add_metadata(metadata)

    with open(pdf_path, "wb") as f:
        writer.write(f)


def render_pdf(src: Path, out: Path) -> int:
    """Render an HTML file to PDF and return its page count.

    The full pipeline every caller must agree on: build-time code highlighting,
    WeasyPrint with base_url at the source directory, Kami PDF metadata, page
    count via pypdf. Raises MissingDepError when weasyprint/pypdf are absent;
    callers decide how to report it.
    """
    HTML = require_weasyprint_html()
    PdfReader = require_pypdf_reader()

    html_text = highlight_code_blocks(src.read_text(encoding="utf-8"))
    out.parent.mkdir(parents=True, exist_ok=True)
    HTML(string=html_text, base_url=str(src.parent)).write_pdf(str(out))
    set_pdf_metadata(out, author=infer_author())
    return len(PdfReader(str(out)).pages)


def build_slides(name: str = "slides") -> bool:
    """Run a python-pptx slide script from the shared registry; True on success."""
    source = pptx_targets().get(name)
    if source is None:
        print(f"ERROR: {name}: unknown slides target")
        return False
    src = TEMPLATES / source
    if not src.exists():
        print(f"ERROR: {name}: source not found ({src})")
        return False

    EXAMPLES.mkdir(parents=True, exist_ok=True)
    out = EXAMPLES / f"{name}.pptx"
    # Pass --out so the slides script writes directly to the target path. Older
    # slides.py defaults to 'output.pptx' in cwd; new copies accept --out.
    result = subprocess.run(
        [sys.executable, str(src), "--out", str(out)],
        cwd=str(src.parent),
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(f"ERROR: {name}: {result.stderr.strip() or 'script failed'}")
        return False
    if out.exists():
        print(f"OK: {name}: generated {out.name}")
        return True
    print(f"ERROR: {name}: {out.name} not produced")
    return False
