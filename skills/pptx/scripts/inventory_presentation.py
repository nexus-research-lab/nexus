#!/usr/bin/env python3
"""输出 PPTX 的页面、形状、文本和占位符清单，供模板选择和修改前审计。"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import posixpath
import sys
import xml.etree.ElementTree as ET
import zipfile


DRAWING_NAMESPACE = "{http://schemas.openxmlformats.org/drawingml/2006/main}"
PRESENTATION_NAMESPACE = "{http://schemas.openxmlformats.org/presentationml/2006/main}"
OFFICE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"
)
PACKAGE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/package/2006/relationships}"
)
MAX_XML_BYTES = 64 * 1024 * 1024


class PresentationInventoryError(Exception):
    """表示无法读取 PPTX 清单。"""


def _read_xml(archive: zipfile.ZipFile, part: str) -> ET.Element:
    try:
        info = archive.getinfo(part)
    except KeyError as error:
        raise PresentationInventoryError(f"缺少 XML 部件：{part}") from error
    if info.file_size > MAX_XML_BYTES:
        raise PresentationInventoryError(f"XML 部件超过 64 MiB：{part}")
    try:
        return ET.fromstring(archive.read(info))
    except ET.ParseError as error:
        raise PresentationInventoryError(f"XML 无效：{part}: {error}") from error


def _relationship_part(source_part: str) -> str:
    return posixpath.join(
        posixpath.dirname(source_part),
        "_rels",
        posixpath.basename(source_part) + ".rels",
    )


def _relationships(
    archive: zipfile.ZipFile,
    source_part: str,
) -> dict[str, tuple[str, str]]:
    part = _relationship_part(source_part)
    if part not in archive.namelist():
        return {}
    root = _read_xml(archive, part)
    result: dict[str, tuple[str, str]] = {}
    for relation in root.findall(PACKAGE_RELATIONSHIP_NAMESPACE + "Relationship"):
        relation_id = relation.get("Id", "")
        target = relation.get("Target", "")
        if not relation_id or not target or relation.get("TargetMode") == "External":
            continue
        if target.startswith("/"):
            resolved = posixpath.normpath(target.lstrip("/"))
        else:
            resolved = posixpath.normpath(
                posixpath.join(posixpath.dirname(source_part), target)
            )
        if resolved == ".." or resolved.startswith("../"):
            raise PresentationInventoryError(f"关系目标越出容器根目录：{target}")
        result[relation_id] = (resolved, relation.get("Type", ""))
    return result


def _slide_parts(archive: zipfile.ZipFile) -> list[str]:
    root = _read_xml(archive, "ppt/presentation.xml")
    relationships = _relationships(archive, "ppt/presentation.xml")
    slide_list = root.find(PRESENTATION_NAMESPACE + "sldIdLst")
    if slide_list is None:
        return []
    result: list[str] = []
    for slide in slide_list.findall(PRESENTATION_NAMESPACE + "sldId"):
        relation_id = slide.get(OFFICE_RELATIONSHIP_NAMESPACE + "id", "")
        relation = relationships.get(relation_id)
        if relation is None or not relation[1].endswith("/slide"):
            raise PresentationInventoryError(
                f"幻灯片关系无效：{relation_id or '缺少关系 ID'}"
            )
        result.append(relation[0])
    return result


def _text(root: ET.Element) -> list[str]:
    return [
        text
        for node in root.iter(DRAWING_NAMESPACE + "t")
        if (text := "".join(node.itertext()).strip())
    ]


def _shape_kind(node: ET.Element) -> str:
    return node.tag.rsplit("}", 1)[-1]


def _shape_inventory(node: ET.Element) -> dict[str, object]:
    name_node = node.find(".//" + PRESENTATION_NAMESPACE + "cNvPr")
    placeholder = node.find(".//" + PRESENTATION_NAMESPACE + "ph")
    texts = _text(node)
    entry: dict[str, object] = {
        "kind": _shape_kind(node),
        "name": name_node.get("name", "") if name_node is not None else "",
        "id": name_node.get("id", "") if name_node is not None else "",
        "text": texts,
    }
    if placeholder is not None:
        entry["placeholder"] = {
            key: placeholder.get(key, "")
            for key in ("type", "idx", "orient", "sz")
            if placeholder.get(key) is not None
        }
    xfrm = node.find(".//" + DRAWING_NAMESPACE + "xfrm")
    if xfrm is not None:
        off = xfrm.find(DRAWING_NAMESPACE + "off")
        ext = xfrm.find(DRAWING_NAMESPACE + "ext")
        if off is not None and ext is not None:
            entry["bounds_emu"] = {
                "x": int(off.get("x", "0")),
                "y": int(off.get("y", "0")),
                "width": int(ext.get("cx", "0")),
                "height": int(ext.get("cy", "0")),
            }
    return entry


def _notes_part(archive: zipfile.ZipFile, slide_part: str) -> str | None:
    for target, relation_type in _relationships(archive, slide_part).values():
        if relation_type.endswith("/notesSlide"):
            return target
    return None


def inventory_presentation(path: Path) -> dict[str, object]:
    if not path.is_file():
        raise PresentationInventoryError(f"文件不存在：{path}")
    try:
        with zipfile.ZipFile(path) as archive:
            presentation = _read_xml(archive, "ppt/presentation.xml")
            size = presentation.find(PRESENTATION_NAMESPACE + "sldSz")
            slide_entries: list[dict[str, object]] = []
            for index, part in enumerate(_slide_parts(archive), start=1):
                slide = _read_xml(archive, part)
                shape_nodes = [
                    node
                    for node in slide.iter()
                    if node.tag
                    in {
                        PRESENTATION_NAMESPACE + "sp",
                        PRESENTATION_NAMESPACE + "pic",
                        PRESENTATION_NAMESPACE + "graphicFrame",
                        PRESENTATION_NAMESPACE + "cxnSp",
                        PRESENTATION_NAMESPACE + "grpSp",
                    }
                ]
                notes_part = _notes_part(archive, part)
                notes = _text(_read_xml(archive, notes_part)) if notes_part else []
                texts = _text(slide)
                slide_entries.append(
                    {
                        "index": index,
                        "part": part,
                        "title": texts[0] if texts else "",
                        "text": texts,
                        "notes": notes,
                        "shape_count": len(shape_nodes),
                        "shapes": [_shape_inventory(node) for node in shape_nodes],
                    }
                )
            return {
                "file": str(path),
                "slide_count": len(slide_entries),
                "slide_size_emu": (
                    {
                        "width": int(size.get("cx", "0")),
                        "height": int(size.get("cy", "0")),
                    }
                    if size is not None
                    else None
                ),
                "slides": slide_entries,
            }
    except (OSError, zipfile.BadZipFile) as error:
        raise PresentationInventoryError(f"无法打开 PPTX：{error}") from error


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="待分析的 PPTX 文件")
    parser.add_argument("--output", type=Path, help="写入新的 UTF-8 JSON 文件")
    args = parser.parse_args()
    try:
        result = inventory_presentation(args.path.expanduser().resolve())
        payload = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
        if args.output:
            output = args.output.expanduser().resolve()
            output.parent.mkdir(parents=True, exist_ok=True)
            try:
                with output.open("x", encoding="utf-8") as destination:
                    destination.write(payload)
            except FileExistsError as error:
                raise PresentationInventoryError(
                    f"拒绝覆盖已有输出文件：{output}"
                ) from error
            print(output)
        else:
            print(payload, end="")
        return 0
    except PresentationInventoryError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
