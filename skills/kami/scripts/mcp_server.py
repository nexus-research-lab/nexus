#!/usr/bin/env python3
"""kami MCP server: render, check, and screenshot tools over stdio JSON-RPC.

Lets any MCP-capable agent use Kami as an engine (render + verify) without
reading SKILL.md: judgment stays in the skill prompt, execution lives here.
Render only trusted local HTML: referenced file, HTTP, and HTTPS resources load
with this process's filesystem and network permissions.
Zero third-party dependencies for the protocol itself; tools that need
weasyprint / pypdf / PyMuPDF surface the install hint as a tool error
instead of crashing the server.

Register with an MCP client, for example:
    claude mcp add kami -- python3 /absolute/path/to/kami/scripts/mcp_server.py

Tools:
    kami_templates   discover templates, diagram library, content schema types
    kami_render      render a filled HTML file to PDF (WeasyPrint + highlight)
    kami_check       run the matching deterministic checks for a file
    kami_screenshot  rasterize a PDF to page PNGs plus the review checklist

Transport: newline-delimited JSON-RPC 2.0 on stdin/stdout (MCP stdio).
All check output is captured; nothing but protocol frames touches stdout.
"""
from __future__ import annotations

import contextlib
import io
import json
import os
import sys
from pathlib import Path

from checks import (
    check_density,
    check_markdown_residue,
    check_orphans,
    check_placeholders,
)
from content import check_content
from optional_deps import MissingDepError
from render import render_pdf
from shared import (
    DIAGRAM_TEMPLATES,
    HTML_TEMPLATES,
    ROOT,
    SCREEN_TEMPLATES,
    content_schema_types,
    kami_version,
)
from visual import MAX_DPI, MIN_DPI, REVIEW_CHECKLIST, render_pages

PROTOCOL_VERSION = "2025-06-18"
SUPPORTED_PROTOCOL_VERSIONS = {"2024-11-05", "2025-03-26", "2025-06-18"}

TOOLS = [
    {
        "name": "kami_templates",
        "description": (
            "List Kami document templates, browser-only templates, the diagram "
            "library, and content schema types, with the reference docs to read "
            "before filling."
        ),
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "kami_render",
        "description": (
            "Render trusted local Kami HTML to PDF via WeasyPrint, with "
            "build-time code highlighting. Referenced file, HTTP, and HTTPS "
            "resources load with this process's permissions. Returns the PDF "
            "path and page count."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["html"],
            "properties": {
                "html": {"type": "string", "description": "Path to the filled HTML file"},
                "out": {"type": "string", "description": "Output PDF path (default: same stem .pdf)"},
            },
        },
    },
    {
        "name": "kami_check",
        "description": (
            "Run Kami's deterministic checks for a file. HTML: placeholders + "
            "markdown residue (+ content coverage when a content IR JSON is "
            "given). PDF: markdown residue + orphans + density. JSON: content "
            "IR schema validation. Returns the full report text."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["path"],
            "properties": {
                "path": {"type": "string", "description": "File to check (.html, .pdf, or content .json)"},
                "content": {"type": "string", "description": "Optional content IR JSON to verify coverage against an HTML file"},
            },
        },
    },
    {
        "name": "kami_screenshot",
        "description": (
            "Rasterize every PDF page to PNG for a perceptual review pass. "
            "Writes or replaces <pdf-stem>-visual/page-*.png, then returns the "
            "image paths and fixed review checklist; view every image against "
            "the checklist before shipping."
        ),
        "inputSchema": {
            "type": "object",
            "required": ["pdf"],
            "properties": {
                "pdf": {"type": "string", "description": "Path to the rendered PDF"},
                "dpi": {
                    "type": "integer",
                    "minimum": MIN_DPI,
                    "maximum": MAX_DPI,
                    "description": "Render DPI (default from checks thresholds)",
                },
            },
        },
    },
]


def _resolve(raw: str) -> Path:
    path = Path(raw).expanduser()
    return path if path.is_absolute() else (Path.cwd() / path)


def tool_templates(_args: dict) -> dict:
    return {
        "version": kami_version(),
        "document_templates": {name: spec.source for name, spec in HTML_TEMPLATES.items()},
        "screen_templates": dict(SCREEN_TEMPLATES),
        "diagram_templates": dict(DIAGRAM_TEMPLATES),
        "content_schema_types": content_schema_types(),
        "templates_dir": str(ROOT / "assets" / "templates"),
        "diagrams_dir": str(ROOT / "assets" / "diagrams"),
        "schemas_dir": str(ROOT / "references" / "schemas"),
        "read_before_filling": [
            str(ROOT / "SKILL.md"),
            str(ROOT / "CHEATSHEET.md"),
            str(ROOT / "references" / "design.md"),
            str(ROOT / "references" / "writing.md"),
            str(ROOT / "references" / "anti-patterns.md"),
        ],
    }


def tool_render(args: dict) -> dict:
    html_path = _resolve(args["html"])
    if not html_path.exists():
        raise FileNotFoundError(f"HTML file not found: {html_path}")
    if html_path.suffix.lower() not in {".html", ".htm"}:
        raise ValueError(f"input must be an HTML file: {html_path.name}")
    out = _resolve(args["out"]) if args.get("out") else html_path.with_suffix(".pdf")
    if out.suffix.lower() != ".pdf":
        raise ValueError(f"output must use a .pdf suffix: {out.name}")
    if out.is_symlink():
        raise ValueError("output PDF must not be a symbolic link")
    same_file = out.resolve(strict=False) == html_path.resolve(strict=False)
    if out.exists():
        try:
            same_file = same_file or os.path.samefile(html_path, out)
        except OSError:
            pass
    if same_file:
        raise ValueError("output PDF must not overwrite the source HTML")
    # render_pdf is the same pipeline build.py and verify.py use: highlight,
    # WeasyPrint, Kami metadata, page count.
    pages = render_pdf(html_path, out)
    return {"pdf": str(out), "pages": pages}


def _run_check(fn, argv: list[str]) -> tuple[int, str]:
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer):
        code = fn(argv)
    return code, buffer.getvalue().rstrip()


def tool_check(args: dict) -> dict:
    path = _resolve(args["path"])
    if not path.exists():
        raise FileNotFoundError(f"file not found: {path}")

    suffix = path.suffix.lower()
    reports: list[str] = []
    worst = 0
    if suffix in {".html", ".htm"}:
        checks = [(check_placeholders, [str(path)]), (check_markdown_residue, [str(path)])]
        if args.get("content"):
            checks.append((check_content, [str(_resolve(args["content"])), str(path)]))
    elif suffix == ".pdf":
        checks = [
            (check_markdown_residue, [str(path)]),
            (check_orphans, [str(path)]),
            (check_density, [str(path)]),
        ]
    elif suffix == ".json":
        checks = [(check_content, [str(path)])]
    else:
        raise ValueError(f"unsupported file type: {path.name} (expected .html, .pdf, or .json)")

    for fn, argv in checks:
        code, report = _run_check(fn, argv)
        worst = max(worst, code)
        reports.append(report)
    return {"exit_code": worst, "ok": worst == 0, "report": "\n".join(reports)}


def tool_screenshot(args: dict) -> dict:
    pdf = _resolve(args["pdf"])
    if not pdf.exists():
        raise FileNotFoundError(f"PDF not found: {pdf}")
    dpi = args.get("dpi")
    if dpi is not None and (isinstance(dpi, bool) or not isinstance(dpi, int)):
        raise ValueError("dpi must be an integer")
    pages = render_pages(pdf, dpi=dpi)
    return {
        "pages": [str(p) for p in pages],
        "review_checklist": list(REVIEW_CHECKLIST),
        "instruction": "View every page image against the checklist before shipping.",
    }


TOOL_HANDLERS = {
    "kami_templates": tool_templates,
    "kami_render": tool_render,
    "kami_check": tool_check,
    "kami_screenshot": tool_screenshot,
}


def _send(payload: dict) -> None:
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def _reply(msg_id, result: dict) -> None:
    _send({"jsonrpc": "2.0", "id": msg_id, "result": result})


def _reply_error(msg_id, code: int, message: str) -> None:
    _send({"jsonrpc": "2.0", "id": msg_id, "error": {"code": code, "message": message}})


def _tool_result(msg_id, payload: dict, *, is_error: bool = False) -> None:
    text = json.dumps(payload, ensure_ascii=False, indent=2)
    _reply(msg_id, {"content": [{"type": "text", "text": text}], "isError": is_error})


def handle(msg: dict) -> None:
    method = msg.get("method")
    msg_id = msg.get("id")
    if msg.get("jsonrpc") != "2.0" or not isinstance(method, str):
        _reply_error(msg_id, -32600, "invalid request")
        return
    raw_params = msg.get("params")
    if raw_params is None:
        params = {}
    elif isinstance(raw_params, dict):
        params = raw_params
    else:
        if msg_id is not None:
            _reply_error(msg_id, -32602, "params must be an object")
        return

    if method == "initialize":
        client_version = params.get("protocolVersion")
        agreed = client_version if client_version in SUPPORTED_PROTOCOL_VERSIONS else PROTOCOL_VERSION
        _reply(msg_id, {
            "protocolVersion": agreed,
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "kami", "version": kami_version()},
        })
        return
    if msg_id is None:
        return  # notifications (initialized, cancelled, ...) need no reply
    if method == "ping":
        _reply(msg_id, {})
        return
    if method == "tools/list":
        _reply(msg_id, {"tools": TOOLS})
        return
    if method == "tools/call":
        name = params.get("name")
        arguments = params.get("arguments")
        if not isinstance(name, str) or (arguments is not None and not isinstance(arguments, dict)):
            _reply_error(msg_id, -32602, "tool name must be a string and arguments must be an object")
            return
        handler = TOOL_HANDLERS.get(name)
        if handler is None:
            _reply_error(msg_id, -32602, f"unknown tool: {name}")
            return
        try:
            # Capture stray prints so nothing corrupts the protocol stream.
            with contextlib.redirect_stdout(io.StringIO()):
                result = handler(arguments or {})
            _tool_result(msg_id, result)
        except MissingDepError as exc:
            _tool_result(msg_id, {"error": str(exc)}, is_error=True)
        except Exception as exc:
            _tool_result(msg_id, {"error": f"{type(exc).__name__}: {exc}"}, is_error=True)
        return
    _reply_error(msg_id, -32601, f"method not found: {method}")


def main() -> int:
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            _reply_error(None, -32700, "parse error")
            continue
        if not isinstance(msg, dict):
            _reply_error(None, -32600, "invalid request")
            continue
        try:
            handle(msg)
        except Exception:
            # A malformed request must never terminate the stdio server or
            # prevent the client from sending the next frame.
            _reply_error(msg.get("id"), -32603, "internal error")
    return 0


if __name__ == "__main__":
    sys.exit(main())
