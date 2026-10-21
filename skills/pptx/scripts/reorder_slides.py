#!/usr/bin/env python3
"""按一基序号重排 PPTX 幻灯片，保留其余 OOXML 部件不变。"""

from __future__ import annotations

import argparse
from pathlib import Path
import shutil
import sys
import tempfile
import xml.etree.ElementTree as ET
import zipfile


PRESENTATION_NAMESPACE = "http://schemas.openxmlformats.org/presentationml/2006/main"


class SlideReorderError(Exception):
    """表示幻灯片顺序请求无效。"""


def _parse_order(value: str) -> list[int]:
    try:
        order = [int(item.strip()) for item in value.split(",") if item.strip()]
    except ValueError as error:
        raise SlideReorderError("--order 必须是逗号分隔的一基幻灯片序号") from error
    if not order or any(item < 1 for item in order):
        raise SlideReorderError("--order 必须至少包含一个正整数")
    if len(order) != len(set(order)):
        raise SlideReorderError("--order 不允许重复幻灯片")
    return order


def _reordered_presentation(payload: bytes, order: list[int]) -> bytes:
    ET.register_namespace("p", PRESENTATION_NAMESPACE)
    root = ET.fromstring(payload)
    slide_list = root.find("{" + PRESENTATION_NAMESPACE + "}sldIdLst")
    if slide_list is None:
        raise SlideReorderError("presentation.xml 缺少 sldIdLst")
    slides = list(slide_list)
    if sorted(order) != list(range(1, len(slides) + 1)):
        raise SlideReorderError(
            f"--order 必须恰好覆盖 1..{len(slides)}，当前为 {order}"
        )
    slide_list[:] = [slides[index - 1] for index in order]
    return ET.tostring(root, encoding="utf-8", xml_declaration=True)


def reorder_slides(source: Path, output: Path, order: list[int]) -> None:
    if not source.is_file():
        raise SlideReorderError(f"文件不存在：{source}")
    source = source.resolve()
    output = output.expanduser().resolve()
    if source == output:
        raise SlideReorderError("拒绝覆盖源文件，请指定新的输出路径")
    if output.exists():
        raise SlideReorderError(f"拒绝覆盖已有输出文件：{output}")
    output.parent.mkdir(parents=True, exist_ok=True)
    try:
        with zipfile.ZipFile(source) as archive:
            payload = archive.read("ppt/presentation.xml")
            updated = _reordered_presentation(payload, order)
            with tempfile.NamedTemporaryFile(
                dir=output.parent, prefix=f".{output.name}.", delete=False
            ) as temporary:
                temporary_path = Path(temporary.name)
            try:
                with zipfile.ZipFile(temporary_path, "w") as destination:
                    for info in archive.infolist():
                        data = updated if info.filename == "ppt/presentation.xml" else archive.read(info)
                        destination.writestr(info, data)
                shutil.move(temporary_path, output)
            finally:
                temporary_path.unlink(missing_ok=True)
    except KeyError as error:
        raise SlideReorderError("PPTX 缺少 ppt/presentation.xml") from error
    except (OSError, zipfile.BadZipFile, ET.ParseError) as error:
        raise SlideReorderError(f"无法重排 PPTX：{error}") from error


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="源 PPTX")
    parser.add_argument("--order", required=True, help="例如 3,1,2")
    parser.add_argument("--output", required=True, type=Path, help="新的 PPTX 路径")
    args = parser.parse_args()
    try:
        reorder_slides(
            args.source.expanduser(),
            args.output,
            _parse_order(args.order),
        )
        print(args.output.expanduser().resolve())
        return 0
    except SlideReorderError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
