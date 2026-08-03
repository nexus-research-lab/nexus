#!/usr/bin/env python3
"""把用户工作目录中的 Kami HTML 渲染为指定 PDF。"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from optional_deps import MissingDepError
from render import render_pdf


SKILL_ROOT = Path(__file__).resolve().parent.parent


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Render a copied Kami HTML document to a PDF outside the built-in Skill."
    )
    parser.add_argument("input", type=Path, help="filled HTML document")
    parser.add_argument("--output", type=Path, help="target PDF path")
    return parser.parse_args()


def ensure_external_output(path: Path) -> None:
    """禁止普通文档任务污染平台共享的内置 Skill。"""
    resolved = path.resolve()
    try:
        resolved.relative_to(SKILL_ROOT)
    except ValueError:
        return
    raise ValueError(f"output must be outside the built-in Skill: {resolved}")


def main() -> int:
    args = parse_args()
    source = args.input.resolve()
    output = (args.output or source.with_suffix(".pdf")).resolve()

    if not source.is_file():
        print(f"ERROR: input HTML not found: {source}", file=sys.stderr)
        return 2

    try:
        ensure_external_output(output)
        pages = render_pdf(source, output)
    except (MissingDepError, OSError, ValueError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1

    print(f"OK: {output} ({pages} page(s))")
    return 0


if __name__ == "__main__":
    sys.exit(main())
