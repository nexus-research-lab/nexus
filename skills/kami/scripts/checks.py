"""PDF-side and content-shape checks for kami documents.

Splits out from build.py:
  - check_placeholders: scan filled HTML for unreplaced `{{...}}` tokens.
  - check_orphans:      scan rendered PDFs for short trailing lines (typographic orphans).
  - check_density:      scan rendered PDFs for pages with too much trailing whitespace.
  - check_resume_balance: scan resume PDFs for exact two-page balance.
  - check_rhythm:       scan slides Python source for monotonous deck sequences.

Density scanning uses a parchment-aware pixel sweep. The hot path is
vectorized with NumPy when available and falls back to a pure-Python loop.
Thresholds and DPI live in `references/checks_thresholds.json`.
"""
from __future__ import annotations

import re
from html.parser import HTMLParser
from pathlib import Path

from optional_deps import MissingDepError, require_pymupdf, require_pypdf_reader
from shared import (
    PARCHMENT_RGB,
    ROOT,
    TEMPLATES,
    default_example_pdfs,
    load_checks_thresholds,
    pptx_targets,
    rel_to_root,
)

PLACEHOLDER = re.compile(r"\{\{[^}]+\}\}")
MARKDOWN_THEMATIC_BREAK = re.compile(r"^\s*[-*_]{3,}\s*$")
MARKDOWN_RESIDUE_MARKERS = (
    ("markdown thematic break", MARKDOWN_THEMATIC_BREAK),
    ("unconverted bold marker", re.compile(r"\*\*")),
    ("unconverted inline-code marker", re.compile(r"`")),
    # Em dash is banned in every deliverable (writing.md, anti-patterns #28).
    # It keeps leaking past the prose-level rule, so it is a machine gate here.
    ("em dash (banned punctuation)", re.compile("\u2014")),
)

# Parchment background RGB for pixel comparison (sourced from shared.PARCHMENT_RGB).
_BG_R, _BG_G, _BG_B = PARCHMENT_RGB
_BG_TOLERANCE = 10


def check_placeholders(paths: list[str]) -> int:
    if not paths:
        print("ERROR: provide at least one HTML file to scan")
        return 2

    failures = 0
    for raw in paths:
        path = Path(raw)
        if not path.is_absolute():
            path = ROOT / path
        if not path.exists():
            print(f"ERROR: {raw}: file not found")
            failures += 1
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        hits = list(dict.fromkeys(PLACEHOLDER.findall(text)))
        rel = rel_to_root(path)
        if hits:
            print(f"ERROR: {rel}: unfilled placeholder(s): {', '.join(hits)}")
            failures += 1
        else:
            print(f"OK: {rel}: no placeholders")

    return 0 if failures == 0 else 1


# ---------- markdown residue check ----------

def css_hidden_selectors(raw: str) -> tuple[set[str], set[str]]:
    """Return simple class and id selectors hidden by inline stylesheet rules.

    This intentionally recognizes only unambiguous single-element selectors;
    a full CSS cascade belongs to the renderer, not this zero-dependency check.
    """
    hidden_classes: set[str] = set()
    hidden_ids: set[str] = set()
    style_blocks = re.findall(r"<style\b[^>]*>(.*?)</style\s*>", raw, flags=re.I | re.S)
    for block in style_blocks:
        clean = re.sub(r"/\*.*?\*/", "", block, flags=re.S)
        for selectors, body in re.findall(r"([^{}]+)\{([^{}]*)\}", clean, flags=re.S):
            compact = re.sub(r"\s+", "", body.lower())
            if "display:none" not in compact and "visibility:hidden" not in compact:
                continue
            for selector in selectors.split(","):
                selector = selector.strip()
                class_match = re.fullmatch(r"(?:[a-zA-Z][\w-]*)?\.([\w-]+)", selector)
                id_match = re.fullmatch(r"(?:[a-zA-Z][\w-]*)?#([\w-]+)", selector)
                if class_match:
                    hidden_classes.add(class_match.group(1))
                elif id_match:
                    hidden_ids.add(id_match.group(1))
    return hidden_classes, hidden_ids


class _VisibleTextParser(HTMLParser):
    """Extract visible text from filled HTML while skipping code-like blocks."""

    _SKIP_TAGS = {"code", "head", "noscript", "pre", "script", "style", "template"}
    _VOID_TAGS = {
        "area", "base", "br", "col", "embed", "hr", "img", "input",
        "link", "meta", "param", "source", "track", "wbr",
    }

    def __init__(self, hidden_classes: set[str], hidden_ids: set[str]) -> None:
        super().__init__(convert_charrefs=True)
        self._hidden_classes = hidden_classes
        self._hidden_ids = hidden_ids
        self._skip_depth = 0
        self._skip_stack: list[bool] = []
        self.parts: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attrs_map = {name.lower(): (value or "") for name, value in attrs}
        style = re.sub(r"\s+", "", attrs_map.get("style", "").lower())
        classes = set(attrs_map.get("class", "").split())
        hidden = (
            tag in self._SKIP_TAGS
            or "hidden" in attrs_map
            or attrs_map.get("aria-hidden", "").lower() == "true"
            or bool(classes & self._hidden_classes)
            or attrs_map.get("id", "") in self._hidden_ids
            or "display:none" in style
            or "visibility:hidden" in style
        )
        if tag not in self._VOID_TAGS:
            self._skip_stack.append(hidden)
        if hidden:
            self._skip_depth += 1
            if tag in self._VOID_TAGS:
                self._skip_depth -= 1

    def handle_endtag(self, tag: str) -> None:
        hidden = self._skip_stack.pop() if self._skip_stack else False
        if hidden and self._skip_depth > 0:
            self._skip_depth -= 1

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        # Self-closing hidden/input nodes contain no visible text and must not
        # leave the parser in a skipped state for the following sibling.
        return

    def handle_data(self, data: str) -> None:
        if self._skip_depth == 0:
            self.parts.append(data)


def visible_html_text(raw: str) -> str:
    parser = _VisibleTextParser(*css_hidden_selectors(raw))
    parser.feed(raw)
    return "\n".join(parser.parts)


def _markdown_residue_issues(text: str, *, page: int | None = None) -> list[str]:
    issues: list[str] = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        for label, pattern in MARKDOWN_RESIDUE_MARKERS:
            if pattern.search(line):
                where = f"p{page}" if page is not None else f"line {line_no}"
                sample = " ".join(line.strip().split())[:80]
                issues.append(f"{where}: {label}: {sample!r}")
    return issues


def _markdown_text_chunks(path: Path) -> tuple[list[tuple[int | None, str]], str | None]:
    suffix = path.suffix.lower()
    if suffix == ".pdf":
        try:
            PdfReader = require_pypdf_reader()
        except MissingDepError as exc:
            return [], str(exc)
        try:
            reader = PdfReader(str(path))
        except Exception as exc:
            return [], f"could not read PDF text: {exc}"
        return [
            (index, page.extract_text() or "")
            for index, page in enumerate(reader.pages, start=1)
        ], None

    raw = path.read_text(encoding="utf-8", errors="replace")
    if suffix in {".html", ".htm"}:
        return [(None, visible_html_text(raw))], None
    return [(None, raw)], None


def check_markdown_residue(paths: list[str]) -> int:
    """Scan filled HTML/PDF outputs for visible raw Markdown markers.

    This catches common hand-conversion misses such as literal `---`, `**bold**`,
    and inline-code backticks leaking into the final PDF.
    """
    if not paths:
        paths = default_example_pdfs()
        if not paths:
            print("ERROR: no files to scan")
            return 2

    failures = 0
    scanned = 0
    for raw in paths:
        path = Path(raw)
        if not path.is_absolute():
            path = ROOT / path
        rel = rel_to_root(path)
        if not path.exists():
            print(f"ERROR: {raw}: file not found")
            failures += 1
            continue

        chunks, error = _markdown_text_chunks(path)
        if error:
            print(f"ERROR: {rel}: {error}")
            failures += 1
            continue

        scanned += 1
        issues: list[str] = []
        for page, text in chunks:
            issues.extend(_markdown_residue_issues(text, page=page))
        if issues:
            failures += 1
            print(f"ERROR: {rel}: markdown residue found")
            for issue in issues:
                print(f"  {issue}")
        else:
            print(f"OK: {rel}: no markdown residue")

    if scanned == 0:
        print("ERROR: no files scanned")
        return 2
    return 0 if failures == 0 else 1


# ---------- orphan check ----------

def _orphan_last_line(text: str, max_words: int, max_chars: int) -> str | None:
    """Return a block's last line if it is an orphan, else None.

    A block orphans when it has 2+ lines and the trailing line is short by
    both word count (<= max_words) and length (< max_chars). Pure so the
    predicate is unit-testable without a rendered PDF.
    """
    lines = text.strip().splitlines()
    if len(lines) < 2:
        return None
    last = lines[-1].strip()
    if len(last.split()) <= max_words and len(last) < max_chars:
        return last
    return None


def check_orphans(paths: list[str]) -> int:
    """Scan PDF for text blocks whose last line has <= max_words and < max_chars."""
    try:
        fitz = require_pymupdf()
    except MissingDepError as exc:
        print(f"ERROR: {exc}")
        return 2

    if not paths:
        paths = default_example_pdfs()
        if not paths:
            print("ERROR: no PDF files to scan")
            return 2

    orphan_cfg = load_checks_thresholds()["orphan"]
    max_words = int(orphan_cfg["max_words"])
    max_chars = int(orphan_cfg["max_chars"])

    total = 0
    missing = 0
    scanned = 0
    for raw in paths:
        path = Path(raw)
        if not path.exists():
            print(f"ERROR: {raw}: not found")
            missing += 1
            continue
        try:
            doc = fitz.open(str(path))
        except Exception as exc:
            print(f"ERROR: {raw}: could not read PDF: {exc}")
            missing += 1
            continue
        scanned += 1
        rel = rel_to_root(path)
        for page_num in range(len(doc)):
            page = doc[page_num]
            blocks = page.get_text("blocks")
            for bx0, by0, bx1, by1, text, block_no, block_type in blocks:
                if block_type != 0:  # text blocks only
                    continue
                last = _orphan_last_line(text, max_words, max_chars)
                if last is not None:
                    total += 1
                    print(f"  {rel} p{page_num + 1}: orphan: \"{last}\" ({len(last.split())} word(s), {len(last)} chars)")
        doc.close()

    if scanned == 0:
        print(f"ERROR: no PDFs scanned ({missing} missing)")
        return 2

    if total == 0 and missing == 0:
        print(f"OK: no orphans found across {scanned} PDF(s)")
        return 0

    if total:
        print(f"\n{total} orphan(s) found across {scanned} PDF(s)")
    if missing:
        print(f"{missing} input(s) missing")
    return 1


# ---------- density check ----------

def _last_content_y(samples: bytes, w: int, h: int, stride: int, n: int) -> int:
    """Return the highest y row index that contains non-parchment content.

    Uses numpy when available (vectorized scan, ~50-100x faster on multi-page
    PDFs); falls back to a pure Python loop otherwise. Both paths sample every
    fourth column for parity, so the result is identical.
    """
    try:
        import numpy as np
    except ImportError:
        last_y = 0
        for y in range(h - 1, -1, -1):
            row_start = y * stride
            is_bg = True
            for x in range(0, w, 4):
                offset = row_start + x * n
                if (abs(samples[offset] - _BG_R) > _BG_TOLERANCE
                        or abs(samples[offset + 1] - _BG_G) > _BG_TOLERANCE
                        or abs(samples[offset + 2] - _BG_B) > _BG_TOLERANCE):
                    is_bg = False
                    break
            if not is_bg:
                last_y = y
                break
        return last_y

    arr = np.frombuffer(samples, dtype=np.uint8).reshape((h, stride))
    pixels = arr[:, : w * n].reshape((h, w, n))
    rgb = pixels[:, ::4, :3].astype(np.int16)
    bg = np.array([_BG_R, _BG_G, _BG_B], dtype=np.int16)
    row_is_bg = (np.abs(rgb - bg).max(axis=2) <= _BG_TOLERANCE).all(axis=1)
    non_bg = np.where(~row_is_bg)[0]
    return int(non_bg[-1]) if non_bg.size else 0


def _density_bucket(empty: float, warn_pct: float, sparse_pct: float) -> str:
    """Categorize a page by its trailing-whitespace fraction.

    Pure so `scan_density` and its tests share one decision. A test that
    reimplements these comparisons would stay green if the real operators
    drifted (`>` to `>=`, or warn/sparse swapped); calling this keeps the
    assertion anchored to production logic.
    """
    if empty > sparse_pct:
        return "SPARSE"
    if empty > warn_pct:
        return "WARN"
    return "OK"


def scan_density(paths: list[str], scan_single_page: bool = False) -> tuple[int, int, int, int] | None:
    """Scan PDFs and print SPARSE/WARN lines.

    Returns (sparse, warn, missing, scanned), or None if PyMuPDF is missing.
    Thresholds (warn_pct, sparse_pct, dpi) come from
    references/checks_thresholds.json. Public: verify.py runs the same scan
    as its advisory pass.

    Page 1 is skipped as a cover exemption, which left a one-page document with
    no scanned page at all: the single-page templates (one-pager, letter) were
    silently exempt from the only layout gate that reads a rendered page. When
    `scan_single_page` is set, a one-page PDF has that page scanned instead.
    It stays opt-in because the no-arg repo sweep reads example PDFs built from
    unfilled placeholder skeletons, which are sparse by construction and would
    turn the sweep permanently red without describing a real defect.
    """
    try:
        fitz = require_pymupdf()
    except MissingDepError as exc:
        print(f"ERROR: {exc}")
        return None

    density_cfg = load_checks_thresholds()["density"]
    warn_pct = float(density_cfg["warn_pct"])
    sparse_pct = float(density_cfg["sparse_pct"])
    dpi = int(density_cfg["dpi"])

    sparse = 0
    warn = 0
    missing = 0
    scanned = 0
    for raw in paths:
        path = Path(raw)
        if not path.exists():
            print(f"ERROR: {raw}: not found")
            missing += 1
            continue
        try:
            doc = fitz.open(str(path))
        except Exception as exc:
            print(f"ERROR: {raw}: could not read PDF: {exc}")
            missing += 1
            continue
        scanned += 1
        rel = rel_to_root(path)
        single_page = len(doc) == 1
        for page_num in range(len(doc)):
            if page_num == 0 and not (single_page and scan_single_page):
                continue
            page = doc[page_num]
            pix = page.get_pixmap(dpi=dpi)
            w, h = pix.width, pix.height
            if h == 0:
                continue
            last_content_y = _last_content_y(pix.samples, w, h, pix.stride, pix.n)

            empty = (h - last_content_y) / h
            bucket = _density_bucket(empty, warn_pct, sparse_pct)
            if bucket == "SPARSE":
                print(f"  SPARSE: {rel} p{page_num + 1}: {empty:.0%} trailing whitespace")
                sparse += 1
            elif bucket == "WARN":
                print(f"  WARN: {rel} p{page_num + 1}: {empty:.0%} trailing whitespace")
                warn += 1
        doc.close()
    return sparse, warn, missing, scanned


def check_density(paths: list[str]) -> int:
    """Scan PDF pages for sparse content (large trailing whitespace from
    break-inside:avoid pushing content to the next page)."""
    explicit = bool(paths)
    if not paths:
        paths = default_example_pdfs()
        if not paths:
            print("ERROR: no PDF files to scan")
            return 2

    result = scan_density(paths, scan_single_page=explicit)
    if result is None:
        return 2
    sparse, warn, missing, scanned = result

    if scanned == 0:
        print(f"ERROR: no PDFs scanned ({missing} missing)")
        return 2

    total = sparse + warn
    if total == 0 and missing == 0:
        print(f"OK: no density issues across {scanned} PDF(s)")
        return 0

    if total:
        print(f"\n{total} density warning(s) across {scanned} PDF(s)")
    if missing:
        print(f"{missing} input(s) missing")
    return 1


# ---------- resume balance check ----------

def _resume_balance_issues(
    fills: list[float],
    page_count: int,
    min_fill: float,
    max_fill: float,
    max_gap: float,
) -> list[str]:
    """Return human-readable resume balance failures for unit tests and CLI."""
    issues: list[str] = []
    if page_count != 2:
        issues.append(f"{page_count} pages (expected 2)")

    for index, fill in enumerate(fills[:2], start=1):
        if fill < min_fill:
            issues.append(f"p{index} fill {fill:.0%} below {min_fill:.0%}")
        elif fill > max_fill:
            issues.append(f"p{index} fill {fill:.0%} above {max_fill:.0%}")

    if len(fills) >= 2:
        gap = abs(fills[0] - fills[1])
        if gap > max_gap:
            issues.append(f"page fill gap {gap:.0%} above {max_gap:.0%}")

    return issues


def _resume_page_fills(path: Path, dpi: int) -> tuple[list[float], int] | None:
    try:
        fitz = require_pymupdf()
    except MissingDepError as exc:
        print(f"ERROR: {exc}")
        return None

    try:
        doc = fitz.open(str(path))
    except Exception as exc:
        print(f"ERROR: {path}: could not read PDF: {exc}")
        return None
    fills: list[float] = []
    for page in doc:
        pix = page.get_pixmap(dpi=dpi)
        if pix.height == 0:
            fills.append(0.0)
            continue
        last_content_y = _last_content_y(pix.samples, pix.width, pix.height, pix.stride, pix.n)
        fills.append((last_content_y + 1) / pix.height)
    page_count = len(doc)
    doc.close()
    return fills, page_count


def check_resume_balance(paths: list[str]) -> int:
    """Require resume PDFs to be exactly 2 pages with balanced content fill."""
    if not paths:
        print("ERROR: provide at least one filled resume PDF to scan")
        return 2

    # Resolve the dependency once up front so a missing PyMuPDF stays a
    # tooling error (exit 2) while a single unreadable PDF inside the loop
    # tallies as missing and lets the remaining files still get scanned.
    try:
        require_pymupdf()
    except MissingDepError as exc:
        print(f"ERROR: {exc}")
        return 2

    cfg = load_checks_thresholds()["resume_balance"]
    min_fill = float(cfg["min_fill_pct"])
    max_fill = float(cfg["max_fill_pct"])
    max_gap = float(cfg["max_gap_pct"])
    dpi = int(cfg["dpi"])

    failures = 0
    missing = 0
    scanned = 0
    for raw in paths:
        path = Path(raw)
        if not path.exists():
            print(f"ERROR: {raw}: not found")
            missing += 1
            continue

        result = _resume_page_fills(path, dpi)
        if result is None:
            missing += 1
            continue

        fills, page_count = result
        scanned += 1
        rel = rel_to_root(path)
        fill_text = " / ".join(f"{fill:.0%}" for fill in fills)
        issues = _resume_balance_issues(fills, page_count, min_fill, max_fill, max_gap)
        if issues:
            failures += 1
            print(f"ERROR: {rel}: {fill_text} ({'; '.join(issues)})")
        else:
            gap = abs(fills[0] - fills[1])
            print(f"OK: {rel}: 2 pages, fill {fill_text}, gap {gap:.0%}")

    if scanned == 0:
        print(f"ERROR: no PDFs scanned ({missing} missing)")
        return 2
    if missing:
        print(f"{missing} input(s) missing")
    return 0 if failures == 0 and missing == 0 else 1


# ---------- rhythm check ----------

# Layout functions that count as "divider" slides (break monotony).
_DIVIDER_FUNCS = {"chapter_slide"}
# Layout functions that count as "density variation" slides.
_DENSITY_VARIATION_FUNCS = {"quote_slide", "metrics_slide"}
# Layout function call pattern in slides.py source.
_SLIDE_CALL = re.compile(r"^\s*(\w+_slide)\s*\(")


def _parse_slide_sequence(src: Path) -> list[str]:
    """Return the ordered list of slide-function names called in main()."""
    text = src.read_text(encoding="utf-8", errors="replace")
    in_main = False
    sequence: list[str] = []
    for line in text.splitlines():
        if re.match(r"^def main\s*\(", line):
            in_main = True
            continue
        if in_main and re.match(r"^def \w", line):
            break
        if in_main:
            m = _SLIDE_CALL.match(line)
            if m:
                sequence.append(m.group(1))
    return sequence


def _rhythm_issues(seq: list[str], max_content_run: int, divider_min_deck_size: int) -> list[str]:
    """Return the rhythm warnings for one parsed slide sequence.

    Pure so the three monotony rules are unit-testable without rendering a
    deck, matching the `_resume_balance_issues` seam.
    """
    issues: list[str] = []

    # Rule 1: no run of more than `max_content_run` consecutive content_slides.
    run = 0
    max_run = 0
    for fn in seq:
        if fn == "content_slide":
            run += 1
            max_run = max(max_run, run)
        else:
            run = 0
    if max_run > max_content_run:
        issues.append(f"longest content_slide run is {max_run} (limit {max_content_run})")

    # Rule 2: large decks need at least one chapter_slide divider.
    if len(seq) >= divider_min_deck_size and not any(fn in _DIVIDER_FUNCS for fn in seq):
        issues.append(f"{len(seq)} slides with no chapter_slide divider")

    # Rule 3: deck must contain at least one density-variation slide.
    if not any(fn in _DENSITY_VARIATION_FUNCS for fn in seq):
        issues.append("no quote_slide or metrics_slide for density variation")

    return issues


def check_rhythm(targets: list[str]) -> int:
    """Scan slide templates for monotony: too many consecutive content_slides,
    missing dividers, and missing density variation.

    Targets come from the shared PPTX registry; thresholds from
    references/checks_thresholds.json.
    """
    slide_sources = pptx_targets()
    names = targets if targets else list(slide_sources)
    failures = 0
    rhythm_cfg = load_checks_thresholds()["rhythm"]
    max_content_run = int(rhythm_cfg["max_content_run"])
    divider_min_deck_size = int(rhythm_cfg["divider_min_deck_size"])

    for name in names:
        source = slide_sources.get(name)
        if source is None:
            print(f"ERROR: {name}: not a known slides target")
            failures += 1
            continue
        src = TEMPLATES / source
        if not src.exists():
            print(f"ERROR: {name}: source not found ({src})")
            failures += 1
            continue

        seq = _parse_slide_sequence(src)
        if not seq:
            print(f"ERROR: {name}: no slide calls found in main() (deck unparseable)")
            failures += 1
            continue

        issues = _rhythm_issues(seq, max_content_run, divider_min_deck_size)

        if issues:
            # These fail the run (exit 1), so label them ERROR; WARN is
            # reserved for advisory output that does not gate the build.
            for issue in issues:
                print(f"ERROR: {name}: {issue}")
            failures += 1
        else:
            content_run = 0
            max_run = 0
            for fn in seq:
                content_run = content_run + 1 if fn == "content_slide" else 0
                max_run = max(max_run, content_run)
            print(f"OK: {name}: rhythm ok ({len(seq)} slides, max run {max_run})")

    return 0 if failures == 0 else 1
