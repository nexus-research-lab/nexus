"""Content IR checks: schema validation and content-to-HTML coverage.

The content IR is a JSON file the agent writes before filling a template:

    {"type": "resume", "lang": "cn", "content": {...}}

`type` selects a contract from `references/schemas/<type>.json` (a lean JSON
Schema subset). Validation happens before layout, so structural defects
(too few metric cards, missing impact rows, an over-long tagline) are caught
as data problems instead of surfacing later as sparse or overflowing pages.

The coverage check runs after filling: every short atomic value from the
content file must appear in the filled HTML's visible text, which catches
silently dropped facts. Long prose fields are exempt because filling is
editorial, not verbatim.
"""
from __future__ import annotations

import json
import re
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

from checks import css_hidden_selectors, visible_html_text
from shared import ROOT, SCHEMAS_DIR, content_schema_types, rel_to_root

# Strings longer than this are treated as prose the agent may rephrase while
# filling; only shorter atomic values (names, metrics, dates) must survive
# verbatim into the rendered document.
COVERAGE_MAX_LEN = 80
MAX_COVERAGE_VALUES = 5000
MAX_COVERAGE_ISSUES = 200

_CJK = re.compile(r"[\u3000-\u9fff\uf900-\ufaff\u3040-\u30ff\uac00-\ud7af]")

_TYPE_CHECKS: dict[str, type | tuple[type, ...]] = {
    "object": dict,
    "array": list,
    "string": str,
    "number": (int, float),
    "integer": int,
    "boolean": bool,
}


class _HtmlAttributeParser(HTMLParser):
    """Collect resource-bearing HTML attributes for asset coverage checks."""

    _RESOURCE_ATTRS = {
        "audio": {"src"},
        "image": {"href"},
        "img": {"src", "srcset"},
        "source": {"src", "srcset"},
        "use": {"href"},
        "video": {"poster", "src"},
    }
    _SKIP_TAGS = {"head", "noscript", "script", "style", "template"}
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
        self.values: set[str] = set()

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        tag = tag.lower()
        attrs_map = {name.lower(): (value or "") for name, value in attrs}
        style = re.sub(r"\s+", "", attrs_map.get("style", "").lower())
        hidden = (
            tag in self._SKIP_TAGS
            or "hidden" in attrs_map
            or attrs_map.get("aria-hidden", "").lower() == "true"
            or bool(set(attrs_map.get("class", "").split()) & self._hidden_classes)
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
        if self._skip_depth:
            return
        allowed = self._RESOURCE_ATTRS.get(tag, set())
        for name, value in attrs:
            if name.lower() not in allowed or not value:
                continue
            if name.lower() == "srcset":
                self.values.update(part.strip().split()[0] for part in value.split(",") if part.strip())
            else:
                self.values.add(value.strip())

    def handle_endtag(self, _tag: str) -> None:
        hidden = self._skip_stack.pop() if self._skip_stack else False
        if hidden and self._skip_depth > 0:
            self._skip_depth -= 1

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        self.handle_starttag(tag, attrs)
        if tag.lower() not in self._VOID_TAGS:
            self.handle_endtag(tag)


def load_schema(doc_type: str) -> dict:
    path = SCHEMAS_DIR / f"{doc_type}.json"
    if not path.exists():
        known = ", ".join(content_schema_types()) or "none"
        raise FileNotFoundError(f"no schema for type {doc_type!r} (known: {known})")
    return json.loads(path.read_text(encoding="utf-8"))


def _type_ok(value, expected: str) -> bool:
    py = _TYPE_CHECKS.get(expected)
    if py is None:
        return True
    if isinstance(value, bool) and expected in ("number", "integer"):
        return False
    return isinstance(value, py)


def validate_node(value, schema: dict, path: str = "content") -> list[str]:
    """Validate `value` against a JSON Schema subset; return issue strings.

    Supported keywords: type, required, properties, additionalProperties
    (False only), items, minItems, maxItems, minLength, maxLength, enum.
    `$comment` and `description` carry authoring guidance and are ignored.
    """
    issues: list[str] = []

    if "enum" in schema and value not in schema["enum"]:
        allowed = ", ".join(repr(v) for v in schema["enum"])
        issues.append(f"{path}: {value!r} not in ({allowed})")
        return issues

    expected = schema.get("type")
    if expected and not _type_ok(value, expected):
        issues.append(f"{path}: expected {expected}, got {type(value).__name__}")
        return issues

    if isinstance(value, str):
        n = len(value.strip())
        if "minLength" in schema and n < schema["minLength"]:
            issues.append(f"{path}: too short ({n} < {schema['minLength']} chars)")
        if "maxLength" in schema and n > schema["maxLength"]:
            issues.append(f"{path}: too long ({n} > {schema['maxLength']} chars)")

    elif isinstance(value, list):
        n = len(value)
        if "minItems" in schema and n < schema["minItems"]:
            issues.append(f"{path}: too few items ({n} < {schema['minItems']})")
        if "maxItems" in schema and n > schema["maxItems"]:
            issues.append(f"{path}: too many items ({n} > {schema['maxItems']})")
        item_schema = schema.get("items")
        if isinstance(item_schema, dict):
            for i, item in enumerate(value):
                issues.extend(validate_node(item, item_schema, f"{path}[{i}]"))

    elif isinstance(value, dict):
        props = schema.get("properties", {})
        for key in schema.get("required", []):
            if key not in value:
                issues.append(f"{path}: missing required field {key!r}")
        if schema.get("additionalProperties") is False:
            for key in value:
                if key not in props:
                    issues.append(f"{path}: unknown field {key!r}")
        for key, sub in props.items():
            if key in value and isinstance(sub, dict):
                issues.extend(validate_node(value[key], sub, f"{path}.{key}"))

    return issues


def validate_content_file(data) -> tuple[str | None, list[str]]:
    """Validate a parsed content IR envelope. Returns (doc_type, issues)."""
    if not isinstance(data, dict):
        return None, ["content file must be a JSON object"]
    doc_type = data.get("type")
    if not isinstance(doc_type, str) or doc_type not in content_schema_types():
        known = ", ".join(content_schema_types()) or "none"
        return None, [f"top-level 'type' must be one of: {known}"]
    body = data.get("content")
    if not isinstance(body, dict):
        return doc_type, ["top-level 'content' must be an object"]
    return doc_type, validate_node(body, load_schema(doc_type))


# ---------- coverage: content values must survive into the filled HTML ----------

def _normalize(text: str, *, cjk: bool) -> str:
    """Collapse whitespace; for CJK values drop it entirely.

    Filling may legitimately insert spaces between CJK and Latin runs, so
    CJK needles compare whitespace-free.
    """
    if cjk:
        return re.sub(r"\s+", "", text)
    return " ".join(text.split())


def _contains_atomic(haystack: str, needle: str) -> bool:
    """Match an atomic value without accepting it inside a larger token.

    ASCII letters and digits get explicit boundaries, so `62%` cannot pass
    against `162%` and `Ada` cannot pass against `Adams`. CJK edges stay
    unbounded because adjacent Han characters are normal sentence flow.
    """
    if not needle:
        return True
    left = r"(?<![A-Za-z0-9_])" if re.match(r"[A-Za-z0-9_]", needle[0]) else ""
    right = r"(?![A-Za-z0-9_])" if re.match(r"[A-Za-z0-9_]", needle[-1]) else ""
    return re.search(left + re.escape(needle) + right, haystack) is not None


def html_resource_attributes(raw: str) -> set[str]:
    parser = _HtmlAttributeParser(*css_hidden_selectors(raw))
    parser.feed(raw)
    return parser.values


def _asset_present(needle: str, attributes: set[str]) -> bool:
    expected = unquote(urlsplit(needle).path).lstrip("./")
    for raw in attributes:
        actual = unquote(urlsplit(raw).path).lstrip("./")
        if actual == expected or actual.endswith(f"/{expected}"):
            return True
    return False


def _leaf_values(node, path: str):
    if isinstance(node, dict):
        for key, sub in node.items():
            yield from _leaf_values(sub, f"{path}.{key}")
    elif isinstance(node, list):
        for i, sub in enumerate(node):
            yield from _leaf_values(sub, f"{path}[{i}]")
    else:
        yield path, node


def coverage_issues(
    content: dict,
    html_text: str,
    html_attributes: set[str] | None = None,
) -> tuple[list[str], int, int]:
    """Return (issues, checked, skipped) for content-to-HTML coverage.

    Only short atomic values are held verbatim; image paths and long prose
    are skipped (skipped counts the prose fields).
    """
    plain = _normalize(html_text, cjk=False)
    plain_cjk = _normalize(html_text, cjk=True)
    issues: list[str] = []
    checked = skipped = 0

    for index, (path, value) in enumerate(_leaf_values(content, "content")):
        if index >= MAX_COVERAGE_VALUES:
            issues.append(f"content: too many atomic values to check (limit {MAX_COVERAGE_VALUES})")
            break
        if len(issues) >= MAX_COVERAGE_ISSUES:
            issues.append(
                f"content: coverage issue limit reached ({MAX_COVERAGE_ISSUES}); "
                "remaining values not checked"
            )
            break
        if isinstance(value, bool) or value is None:
            continue
        needle = str(value).strip()
        if not needle:
            continue
        if isinstance(value, str):
            if len(needle) > COVERAGE_MAX_LEN:
                skipped += 1
                continue
            # Asset paths are consumed by attributes, not visible text. Direct
            # text-only callers may omit the attribute set; the real CLI always
            # provides it and therefore proves required images were embedded.
            if re.search(r"\.image(s\[\d+\])?$", path) or re.search(r"\.(png|jpe?g|svg|webp)$", needle, re.I):
                if html_attributes is not None:
                    checked += 1
                    if not _asset_present(needle, html_attributes):
                        issues.append(f"{path}: asset not found in document attributes: {needle!r}")
                continue
        checked += 1
        cjk = bool(_CJK.search(needle))
        normalized = _normalize(needle, cjk=cjk)
        present = _contains_atomic(plain_cjk if cjk else plain, normalized)
        # Whitespace-stripped fallback: markup can split a compact value across
        # sibling nodes ("<span>62</span><span>%</span>" extracts as "62\n%"),
        # which whitespace collapse alone cannot rejoin. Do not apply this to
        # space-separated Latin values: `12 34` is not the same fact as `1234`.
        if not present and (cjk or not re.search(r"\s", needle)):
            present = _contains_atomic(plain_cjk, _normalize(needle, cjk=True))
        if not present:
            issues.append(f"{path}: value not found in document text: {needle!r}")

    return issues, checked, skipped


def check_content(paths: list[str]) -> int:
    """CLI: --check-content content.json [filled.html]

    Validates the content IR against its schema; with a filled HTML file,
    also verifies every short atomic value made it into the visible text.
    """
    args = [p for p in paths if not p.startswith("-")]
    if not args or len(args) > 2:
        known = ", ".join(content_schema_types()) or "none"
        print("ERROR: usage: --check-content content.json [filled.html]")
        print(f"  known types: {known}")
        return 2

    content_path = Path(args[0])
    if not content_path.is_absolute():
        content_path = ROOT / content_path
    rel = rel_to_root(content_path)
    if not content_path.exists():
        print(f"ERROR: {args[0]}: file not found")
        return 2
    try:
        data = json.loads(content_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        print(f"ERROR: {rel}: invalid JSON: {exc}")
        return 1

    doc_type, issues = validate_content_file(data)
    if issues:
        print(f"ERROR: {rel}: {len(issues)} schema issue(s)")
        for issue in issues:
            print(f"  {issue}")
        return 1
    print(f"OK: {rel}: valid {doc_type} content")

    if len(args) == 1:
        return 0

    html_path = Path(args[1])
    if not html_path.is_absolute():
        html_path = ROOT / html_path
    html_rel = rel_to_root(html_path)
    if not html_path.exists():
        print(f"ERROR: {args[1]}: file not found")
        return 2
    html_raw = html_path.read_text(encoding="utf-8", errors="replace")
    html_text = visible_html_text(html_raw)
    missing, checked, skipped = coverage_issues(
        data["content"], html_text, html_resource_attributes(html_raw)
    )
    if missing:
        print(f"ERROR: {html_rel}: {len(missing)} content value(s) missing from document")
        for issue in missing:
            print(f"  {issue}")
        return 1
    note = f" ({skipped} prose field(s) not held verbatim)" if skipped else ""
    print(f"OK: {html_rel}: all {checked} atomic content values present{note}")
    return 0
