"""Token-sync checker for kami templates.

Splits out from build.py: scans `:root { ... }` blocks across HTML templates
and `RGBColor(0xXX, 0xXX, 0xXX)` constants in the PPTX slide scripts, and
reports drift from `references/tokens.json` (the canonical color tokens).
"""
from __future__ import annotations

import json
import re
from pathlib import Path

from shared import ROOT, TEMPLATES, TOKENS_FILE, iter_template_files

ROOT_BLOCK = re.compile(r":root\s*\{([^}]*)\}", re.DOTALL)
CSS_VAR = re.compile(r"--([\w-]+)\s*:\s*([^;]+);")


def parse_root_vars(text: str) -> dict[str, str]:
    """Return {'--name': value} merged across every `:root { ... }` block.

    Scanning all blocks (not just the first) keeps a future dark-mode or print
    override inside a second `:root` from silently escaping the drift checks.
    """
    found: dict[str, str] = {}
    for block in ROOT_BLOCK.finditer(text):
        for m in CSS_VAR.finditer(block.group(1)):
            found[f"--{m.group(1)}"] = m.group(2).strip()
    return found
PY_RGB = re.compile(
    r"^([A-Z][A-Z_]+)\s*=\s*RGBColor\(\s*0x([0-9a-fA-F]{2})\s*,"
    r"\s*0x([0-9a-fA-F]{2})\s*,\s*0x([0-9a-fA-F]{2})\s*\)",
    re.MULTILINE,
)
# Python const name -> tokens.json key. Only constants that mirror a CSS token.
PY_TOKEN_MAP = {
    "PARCHMENT": "--parchment",
    "IVORY": "--ivory",
    "BRAND": "--brand",
    "NEAR_BLACK": "--near-black",
    "DARK_WARM": "--dark-warm",
    "CHARCOAL": "--charcoal",
    "OLIVE": "--olive",
    "STONE": "--stone",
}
MERMAID_THEME_FILE = ROOT / "references" / "mermaid-theme.json"
MERMAID_THEME_TOKEN_MAP = {
    "bg": "--parchment",
    "fg": "--near-black",
    "line": "--olive",
    "accent": "--brand",
    "muted": "--stone",
    "surface": "--ivory",
    "border": "--border",
}


def _mermaid_theme_drift(canonical: dict[str, str]) -> list[str]:
    if not MERMAID_THEME_FILE.exists():
        return [f"mermaid-theme.json not found at {MERMAID_THEME_FILE.relative_to(ROOT)}"]

    try:
        theme = json.loads(MERMAID_THEME_FILE.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        return [f"mermaid-theme.json is malformed: {exc}"]

    colors = theme.get("colors", {})
    roles = theme.get("roles", {})
    drift: list[str] = []
    for role, token in MERMAID_THEME_TOKEN_MAP.items():
        expected = canonical.get(token)
        actual = colors.get(role)
        if expected is None:
            drift.append(f"{role}: canonical token {token} missing")
            continue
        if actual is None:
            drift.append(f"{role}: color missing, expected {expected} from {token}")
        elif actual.lower() != expected.lower():
            drift.append(f"{role}: expected {expected} from {token}, got {actual}")

        role_doc = str(roles.get(role, ""))
        if token not in role_doc:
            drift.append(f"{role}: role doc should mention {token}")

    return drift


def sync_check(verbose: bool = False) -> int:
    if not TOKENS_FILE.exists():
        print(f"ERROR: tokens.json not found at {TOKENS_FILE.relative_to(ROOT)}")
        return 2

    try:
        canonical: dict[str, str] = json.loads(TOKENS_FILE.read_text())
    except json.JSONDecodeError as exc:
        print(f"ERROR: tokens.json is malformed: {exc}")
        return 2

    targets = iter_template_files(include_diagrams=True, include_marp_css=True)
    py_targets: list[Path] = list(TEMPLATES.glob("*.py"))
    if not targets and not py_targets:
        print("ERROR: no templates found to token-check (bad checkout?)")
        return 2

    drift: list[tuple[str, str, str, str]] = []  # (file, token, expected, actual)

    for path in targets:
        text = path.read_text(encoding="utf-8", errors="replace")
        found = parse_root_vars(text)
        if not found:
            if verbose:
                print(f"  (skip {path.name}: no :root block)")
            continue
        rel = path.relative_to(ROOT)
        for token, expected in canonical.items():
            actual = found.get(token)
            # Only flag if the template defines the token but with a wrong value.
            # Templates that don't use a token don't need to define it.
            if actual is not None and actual.lower() != expected.lower():
                drift.append((str(rel), token, expected, actual))

    for path in sorted(py_targets):
        text = path.read_text(encoding="utf-8", errors="replace")
        rel = path.relative_to(ROOT)
        for m in PY_RGB.finditer(text):
            name = m.group(1)
            token = PY_TOKEN_MAP.get(name)
            if token is None:
                continue
            expected = canonical.get(token)
            if expected is None:
                continue
            actual = f"#{m.group(2)}{m.group(3)}{m.group(4)}"
            if actual.lower() != expected.lower():
                drift.append((str(rel), token, expected, actual))

    theme_drift = _mermaid_theme_drift(canonical)

    if not drift and not theme_drift:
        scanned = len(targets) + len(py_targets)
        print(f"OK: tokens in sync across {scanned} template(s) and mermaid-theme.json")
        return 0

    if drift:
        print(f"\nERROR: [token-drift] {len(drift)}")
        for file, token, expected, actual in drift:
            print(f"  {file}: {token} expected {expected}, got {actual}")
    if theme_drift:
        print(f"\nERROR: [mermaid-theme-drift] {len(theme_drift)}")
        for issue in theme_drift:
            print(f"  {issue}")

    return 1
