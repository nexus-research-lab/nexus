#!/usr/bin/env python3
"""Re-theme and normalize a beautiful-mermaid SVG into a Kami-styled, WeasyPrint-safe SVG.

beautiful-mermaid (https://github.com/lukilabs/beautiful-mermaid) hangs its seven
color roles on CSS custom properties on the root ``<svg>`` and derives the rest with
``color-mix(in srgb, ...)``. WeasyPrint's inline-SVG renderer does not resolve
``color-mix()``, does not reliably cascade SVG ``<style>`` custom properties, and
should not fetch a runtime web font. This script:

  1. **Re-themes** the SVG to the Kami palette by overriding the seven root color
     roles with ``references/mermaid-theme.json`` values, so the diagram looks like
     Kami no matter which theme it was generated with.
  2. **Resolves** every ``var()`` and ``color-mix(in srgb, ...)`` to a static hex, so
     colors land on inline presentation attributes WeasyPrint renders directly.
  3. **Fixes fonts**: strips beautiful-mermaid's Google-Fonts ``@import`` and rewrites
     the (mis-quoted) ``font-family`` to the Kami serif stack (with CJK fallback).

No Node, no network, pure stdlib. Generate the SVG anywhere beautiful-mermaid runs
(e.g. https://agents.craft.do/mermaid or your own one-off script), then run this.

Scope: graph-type diagrams (flowchart / state / sequence / class / ER) whose colors
flow from the seven root roles. xychart-beta styles via ``<style>`` class selectors,
which WeasyPrint will not apply to inline SVG, so charts stay browser-only; use
Kami's hand-drawn bar/line/donut/candlestick/waterfall diagrams for PDF. See
references/mermaid.md.

Usage:
    python3 scripts/mermaid_normalize.py raw.svg            # cleaned SVG to stdout
    python3 scripts/mermaid_normalize.py raw.svg -o out.svg
    cat raw.svg | python3 scripts/mermaid_normalize.py -    # read from stdin
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

_ROOT = Path(__file__).resolve().parent.parent
_THEME_FILE = _ROOT / "references" / "mermaid-theme.json"

# Fallbacks if references/mermaid-theme.json is missing. Mirror that file and
# references/design.md.
_DEFAULT_FONT_STACK = (
    'Charter, Georgia, "TsangerJinKai02", "Source Han Serif SC", '
    '"Noto Serif CJK SC", serif'
)
_DEFAULT_COLORS = {
    "--bg": "#f5f4ed", "--fg": "#141413", "--line": "#504e49",
    "--accent": "#1B365D", "--muted": "#6b6a64", "--surface": "#faf9f5",
    "--border": "#e8e6dc",
}

# A handful of CSS named colors beautiful-mermaid may emit. Hex is preferred.
_NAMED = {"white": (255, 255, 255), "black": (0, 0, 0), "transparent": None}


def _load_theme() -> tuple[dict[str, str], str]:
    """Return (kami color-role overrides, font stack) from the theme file."""
    try:
        data = json.loads(_THEME_FILE.read_text(encoding="utf-8"))
        colors = {f"--{k}": v for k, v in data.get("colors", {}).items()}
        font = data.get("cssFontStack") or _DEFAULT_FONT_STACK
        if colors:
            return colors, font
    except (OSError, ValueError):
        pass
    return dict(_DEFAULT_COLORS), _DEFAULT_FONT_STACK


# ---------------------------------------------------------------------------
# color helpers
# ---------------------------------------------------------------------------

def _parse_hex(value: str) -> tuple[int, int, int]:
    h = value.strip().lstrip("#")
    if len(h) == 3:
        h = "".join(c * 2 for c in h)
    if len(h) != 6:
        raise ValueError(f"not a hex color: {value!r}")
    return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)


def _to_hex(rgb: tuple[float, float, float]) -> str:
    return "#" + "".join(f"{max(0, min(255, round(c))):02x}" for c in rgb)


def _mix_srgb(c1: tuple[int, int, int], p1: float,
              c2: tuple[int, int, int], p2: float) -> tuple[float, float, float]:
    """color-mix(in srgb, c1 p1%, c2 p2%): linear blend in gamma sRGB.

    Percentages are normalized to sum to 100 (per CSS Color 4).
    """
    total = p1 + p2 or 1.0
    w1, w2 = p1 / total, p2 / total
    return tuple(a * w1 + b * w2 for a, b in zip(c1, c2))


# ---------------------------------------------------------------------------
# balanced-paren parsing
# ---------------------------------------------------------------------------

def _match_paren(s: str, open_idx: int) -> int:
    """Given index of a '(', return index of its matching ')'."""
    depth = 0
    for i in range(open_idx, len(s)):
        if s[i] == "(":
            depth += 1
        elif s[i] == ")":
            depth -= 1
            if depth == 0:
                return i
    raise ValueError("unbalanced parentheses")


def _split_top(s: str, sep: str = ",") -> list[str]:
    """Split on `sep` at paren depth 0 only."""
    parts, depth, start = [], 0, 0
    for i, ch in enumerate(s):
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        elif ch == sep and depth == 0:
            parts.append(s[start:i])
            start = i + 1
    parts.append(s[start:])
    return [p.strip() for p in parts]


class _Resolver:
    """Resolves a CSS color value to a static hex using a custom-property map."""

    def __init__(self, raw_defs: dict[str, str]):
        self._raw = raw_defs

    def hex_of(self, value: str) -> str | None:
        rgb = self._rgb(value)
        return None if rgb is None else _to_hex(rgb)

    def var_map(self) -> dict[str, str]:
        out: dict[str, str] = {}
        for name in self._raw:
            h = self.hex_of(f"var({name})")
            if h is not None:
                out[name] = h
        return out

    def _rgb(self, value: str, _seen: frozenset[str] = frozenset()) -> tuple[int, int, int] | None:
        value = value.strip().rstrip(";").strip()
        if not value:
            return None
        if value.startswith("#"):
            return _parse_hex(value)
        low = value.lower()
        if low in _NAMED:
            return _NAMED[low]
        if value.startswith("var(") and value.endswith(")"):
            inner = value[4:_match_paren(value, 3)]
            args = _split_top(inner)
            name = args[0].strip()
            fallback = args[1] if len(args) > 1 else None
            if name in _seen:
                return None  # cycle guard
            if name in self._raw:
                return self._rgb(self._raw[name], _seen | {name})
            if fallback is not None:
                return self._rgb(fallback, _seen | {name})
            return None
        if value.startswith("color-mix(") and value.endswith(")"):
            inner = value[len("color-mix("):_match_paren(value, len("color-mix"))]
            args = _split_top(inner)
            # args[0] is the color space, e.g. "in srgb"
            if not args or "srgb" not in args[0]:
                raise ValueError(f"unsupported color-mix space: {args[0] if args else '?'}")
            ops = [self._parse_operand(a, _seen) for a in args[1:3]]
            (c1, p1), (c2, p2) = ops[0], ops[1]
            if p1 is None and p2 is None:
                p1 = p2 = 50.0
            elif p1 is None:
                p1 = max(0.0, 100.0 - p2)
            elif p2 is None:
                p2 = max(0.0, 100.0 - p1)
            if c1 is None or c2 is None:
                return None
            return tuple(round(x) for x in _mix_srgb(c1, p1, c2, p2))
        return None

    def _parse_operand(self, token: str,
                       _seen: frozenset[str]) -> tuple[tuple[int, int, int] | None, float | None]:
        """Parse a color-mix operand 'COLOR' or 'COLOR P%'."""
        token = token.strip()
        pct = None
        m = re.search(r"\s(\d+(?:\.\d+)?)%\s*$", " " + token)
        if m:
            pct = float(m.group(1))
            token = token[: token.rfind(m.group(1) + "%")].strip()
        return self._rgb(token, _seen), pct


# ---------------------------------------------------------------------------
# SVG transforms
# ---------------------------------------------------------------------------

_STYLE_RE = re.compile(r"<style[^>]*>(.*?)</style>", re.DOTALL)
_CUSTOM_PROP_RE = re.compile(r"(--[\w-]+)\s*:\s*([^;]*?(?:\([^)]*\)[^;]*?)*);")
_IMPORT_RE = re.compile(r"@import\s+url\([^)]*\)\s*;", re.IGNORECASE)
_FONT_FAMILY_RE = re.compile(r"font-family\s*:\s*[^;}]+")


def _collect_raw_defs(svg: str, overrides: dict[str, str]) -> dict[str, str]:
    """Gather custom-property defs from the root <svg style> and <style> svg{} rules.

    `overrides` (the Kami color roles) replace any same-named root role, re-theming
    the diagram regardless of the theme it was generated with.
    """
    raw: dict[str, str] = {}
    m = re.search(r"<svg\b[^>]*\bstyle=\"([^\"]*)\"", svg)
    if m:
        for prop in m.group(1).split(";"):
            if ":" in prop:
                k, v = prop.split(":", 1)
                k = k.strip()
                if k.startswith("--"):
                    raw[k] = v.strip()
    for sm in _STYLE_RE.finditer(svg):
        for pm in _CUSTOM_PROP_RE.finditer(sm.group(1)):
            raw[pm.group(1)] = pm.group(2).strip()
    raw.update(overrides)  # Kami palette wins
    return raw


def _resolve_functions(text: str, resolver: _Resolver) -> str:
    """Replace every var()/color-mix() in `text` with a static hex, innermost-first.

    Raises if any function cannot be resolved to a color: in well-formed
    beautiful-mermaid output every var()/color-mix() resolves, so an unresolved
    one means the input structure changed. Failing loudly beats silently emitting
    an invalid color that renders as a black or invisible diagram.
    """
    while True:
        starts = [m.start() for m in re.finditer(r"\b(?:var|color-mix)\(", text)]
        if not starts:
            break
        # rightmost opening = guaranteed leaf (no nested var/color-mix after it)
        start = max(starts)
        open_paren = text.index("(", start)
        close = _match_paren(text, open_paren)
        expr = text[start:close + 1]
        hexval = resolver.hex_of(expr)
        if hexval is None:
            raise ValueError(
                f"could not resolve {expr!r} to a color; the input may not be "
                "beautiful-mermaid output or uses an unsupported structure"
            )
        text = text[:start] + hexval + text[close + 1:]
    return text


def _assert_beautiful_mermaid(svg: str) -> None:
    """Raise unless the SVG carries beautiful-mermaid's root color-role props.

    beautiful-mermaid v1.x defines --bg / --fg (plus the optional roles) as inline
    custom properties on the root <svg>. Their absence means the input came from a
    different renderer whose structure this normalizer does not understand, so we
    fail loudly here instead of silently producing unresolved colors downstream.
    Verified against beautiful-mermaid v1.1.3; see references/mermaid.md.
    """
    m = re.search(r"<svg\b[^>]*\bstyle=\"([^\"]*)\"", svg)
    style = m.group(1) if m else ""
    if "--bg" not in style or "--fg" not in style:
        raise ValueError(
            "input does not look like beautiful-mermaid output: the root <svg> is "
            "missing the --bg/--fg color roles (verified against v1.1.3)"
        )


def normalize(svg: str, theme: dict[str, str] | None = None,
              font_stack: str | None = None) -> str:
    """Return a Kami-re-themed, WeasyPrint-safe copy of a beautiful-mermaid SVG."""
    if theme is None or font_stack is None:
        default_colors, default_font = _load_theme()
        theme = theme or default_colors
        font_stack = font_stack or default_font

    _assert_beautiful_mermaid(svg)
    raw_defs = _collect_raw_defs(svg, theme)
    resolver = _Resolver(raw_defs)

    out = _resolve_functions(svg, resolver)
    out = _IMPORT_RE.sub("", out)
    out = _FONT_FAMILY_RE.sub(f"font-family: {font_stack}", out)

    # The derived custom props were inlined into presentation attributes, so the
    # <style> svg{} rules and the root role decls are now dead. Strip them, keeping
    # only live rules (e.g. the rewritten font-family).
    def _clean_style(m: re.Match[str]) -> str:
        body = m.group(1)
        body = re.sub(r"--[\w-]+\s*:\s*#[0-9a-fA-F]{3,8}\s*;", "", body)
        body = re.sub(r"[\w*.#-]+\s*\{\s*(?:/\*.*?\*/\s*)?\}", "", body, flags=re.DOTALL)
        body = re.sub(r"\n\s*\n+", "\n", body).strip()
        return f"<style>\n  {body}\n</style>" if body else ""

    out = _STYLE_RE.sub(_clean_style, out)

    def _clean_root_style(m: re.Match[str]) -> str:
        kept = [d.strip() for d in m.group(1).split(";")
                if d.strip() and not d.strip().startswith("--")]
        return f' style="{";".join(kept)}"' if kept else ""

    out = re.sub(r'\s+style="([^"]*)"', _clean_root_style, out, count=1)
    return out


def main(argv: list[str]) -> int:
    if len(argv) == 1:
        print(__doc__)
        return 0

    parser = argparse.ArgumentParser(
        description="Re-theme a beautiful-mermaid SVG into a Kami, WeasyPrint-safe SVG.",
    )
    parser.add_argument("src", help="Input SVG path, or '-' to read from stdin")
    parser.add_argument("-o", "--output", help="Output SVG path (default: stdout)")
    args = parser.parse_args(argv[1:])

    try:
        raw = sys.stdin.read() if args.src == "-" else Path(args.src).read_text(encoding="utf-8")
        result = normalize(raw)
        if args.output:
            Path(args.output).write_text(result, encoding="utf-8")
            print(f"OK: wrote {args.output}")
        else:
            sys.stdout.write(result)
    except (OSError, ValueError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
