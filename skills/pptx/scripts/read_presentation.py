#!/usr/bin/env python3
"""按演示文稿顺序提取 PPTX 正文、表格和演讲者备注。"""

from __future__ import annotations

import argparse
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
OLE_MAGIC = b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"


class PresentationReadError(Exception):
    """表示无法安全读取 PowerPoint 演示文稿。"""


def _read_xml(archive: zipfile.ZipFile, part_name: str) -> ET.Element:
    try:
        info = archive.getinfo(part_name)
    except KeyError as error:
        raise PresentationReadError(f"PPTX 缺少必要部分：{part_name}") from error
    if info.file_size > MAX_XML_BYTES:
        raise PresentationReadError(f"PPTX XML 超过 64 MiB 安全上限：{part_name}")
    try:
        return ET.fromstring(archive.read(info))
    except ET.ParseError as error:
        raise PresentationReadError(f"PPTX XML 无效（{part_name}）：{error}") from error


def _resolve_target(source_part: str, target: str) -> str:
    if target.startswith("/"):
        resolved = posixpath.normpath(target.lstrip("/"))
    else:
        resolved = posixpath.normpath(
            posixpath.join(posixpath.dirname(source_part), target)
        )
    if resolved == ".." or resolved.startswith("../"):
        raise PresentationReadError(f"PPTX 关系目标越出容器根目录：{target}")
    return resolved


def _relationships(
    archive: zipfile.ZipFile,
    source_part: str,
) -> dict[str, tuple[str, str]]:
    source_dir = posixpath.dirname(source_part)
    rels_name = posixpath.join(
        source_dir,
        "_rels",
        posixpath.basename(source_part) + ".rels",
    )
    if rels_name not in archive.namelist():
        return {}
    root = _read_xml(archive, rels_name)
    result: dict[str, tuple[str, str]] = {}
    for relation in root.findall(PACKAGE_RELATIONSHIP_NAMESPACE + "Relationship"):
        relation_id = relation.get("Id", "").strip()
        target = relation.get("Target", "").strip()
        if not relation_id or not target or relation.get("TargetMode") == "External":
            continue
        result[relation_id] = (
            _resolve_target(source_part, target),
            relation.get("Type", ""),
        )
    return result


def _paragraph_text(paragraph: ET.Element) -> str:
    parts: list[str] = []
    for node in paragraph.iter():
        if node.tag == DRAWING_NAMESPACE + "t":
            parts.append(node.text or "")
        elif node.tag == DRAWING_NAMESPACE + "tab":
            parts.append("\t")
        elif node.tag in {
            DRAWING_NAMESPACE + "br",
            DRAWING_NAMESPACE + "cr",
        }:
            parts.append("\n")
    return "".join(parts).strip()


def _extract_content(root: ET.Element) -> tuple[list[str], list[list[list[str]]]]:
    table_paragraphs = {
        id(paragraph)
        for table in root.iter(DRAWING_NAMESPACE + "tbl")
        for paragraph in table.iter(DRAWING_NAMESPACE + "p")
    }
    paragraphs = [
        text
        for paragraph in root.iter(DRAWING_NAMESPACE + "p")
        if id(paragraph) not in table_paragraphs
        if (text := _paragraph_text(paragraph))
    ]
    tables: list[list[list[str]]] = []
    for table in root.iter(DRAWING_NAMESPACE + "tbl"):
        rows: list[list[str]] = []
        for row in table.findall(DRAWING_NAMESPACE + "tr"):
            cells = [
                " ".join(
                    text
                    for paragraph in cell.iter(DRAWING_NAMESPACE + "p")
                    if (text := _paragraph_text(paragraph))
                ).replace("\t", " ")
                for cell in row.findall(DRAWING_NAMESPACE + "tc")
            ]
            if any(cells):
                rows.append(cells)
        if rows:
            tables.append(rows)
    return paragraphs, tables


def _extract_notes(root: ET.Element) -> list[str]:
    lines: list[str] = []
    ignored_placeholder_types = {"dt", "ftr", "hdr", "sldImg", "sldNum"}
    for shape in root.iter(PRESENTATION_NAMESPACE + "sp"):
        placeholder = shape.find(
            ".//" + PRESENTATION_NAMESPACE + "nvPr/" + PRESENTATION_NAMESPACE + "ph"
        )
        if (
            placeholder is not None
            and placeholder.get("type") in ignored_placeholder_types
        ):
            continue
        for paragraph in shape.iter(DRAWING_NAMESPACE + "p"):
            if text := _paragraph_text(paragraph):
                lines.append(text)
    return lines


def _slide_parts(archive: zipfile.ZipFile) -> list[str]:
    presentation_part = "ppt/presentation.xml"
    root = _read_xml(archive, presentation_part)
    relationships = _relationships(archive, presentation_part)
    slide_list = root.find(PRESENTATION_NAMESPACE + "sldIdLst")
    if slide_list is None:
        return []

    result: list[str] = []
    for slide in slide_list.findall(PRESENTATION_NAMESPACE + "sldId"):
        relation_id = slide.get(OFFICE_RELATIONSHIP_NAMESPACE + "id", "")
        relation = relationships.get(relation_id)
        if relation is None or not relation[1].endswith("/slide"):
            raise PresentationReadError(
                f"PPTX 幻灯片关系无效：{relation_id or '缺少关系 ID'}"
            )
        result.append(relation[0])
    return result


def _notes_part(
    archive: zipfile.ZipFile,
    slide_part: str,
) -> str | None:
    for target, relation_type in _relationships(archive, slide_part).values():
        if relation_type.endswith("/notesSlide"):
            return target
    return None


def _format_table(rows: list[list[str]]) -> str:
    width = max(len(row) for row in rows)
    normalized = [row + [""] * (width - len(row)) for row in rows]
    return "\n".join(
        "\t".join(cell.replace("\n", " ") for cell in row) for row in normalized
    )


def read_presentation(path: Path) -> str:
    if not path.is_file():
        raise PresentationReadError(f"文件不存在：{path}")
    if not zipfile.is_zipfile(path):
        try:
            with path.open("rb") as source:
                is_legacy = source.read(len(OLE_MAGIC)) == OLE_MAGIC
        except OSError as error:
            raise PresentationReadError(f"无法读取文件：{error}") from error
        if is_legacy or path.suffix.lower() == ".ppt":
            raise PresentationReadError("旧版 PPT 无法直接读取，请另存为 PPTX 或 PDF")
        raise PresentationReadError("文件不是有效的 PPTX/OOXML 演示文稿")

    try:
        with zipfile.ZipFile(path) as archive:
            slides = _slide_parts(archive)
            sections = [f"# {path.name}"]
            for index, slide_part in enumerate(slides, start=1):
                paragraphs, tables = _extract_content(_read_xml(archive, slide_part))
                notes: list[str] = []
                sections.append(f"## 幻灯片 {index}")
                sections.extend(paragraphs)
                for table_index, table in enumerate(tables, start=1):
                    sections.append(f"### 表格 {table_index}\n\n{_format_table(table)}")
                notes_part = _notes_part(archive, slide_part)
                if notes_part:
                    notes = _extract_notes(_read_xml(archive, notes_part))
                    if notes:
                        sections.append("### 演讲者备注\n\n" + "\n".join(notes))
                if not paragraphs and not tables and not notes:
                    sections.append("（未提取到可验证文字）")
            if not slides:
                sections.append("（演示文稿不含幻灯片）")
            return "\n\n".join(sections)
    except (OSError, RuntimeError, zipfile.BadZipFile) as error:
        raise PresentationReadError(f"无法打开 PPTX：{error}") from error


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="PPTX 文件路径")
    parser.add_argument("--output", type=Path, help="写入新的 UTF-8 Markdown 文件")
    args = parser.parse_args()
    try:
        text = read_presentation(args.path.expanduser().resolve())
        if args.output:
            output = args.output.expanduser().resolve()
            output.parent.mkdir(parents=True, exist_ok=True)
            try:
                with output.open("x", encoding="utf-8") as destination:
                    destination.write(text)
            except FileExistsError as error:
                raise PresentationReadError(
                    f"拒绝覆盖已有输出文件：{output}"
                ) from error
            except OSError as error:
                raise PresentationReadError(f"无法写入输出文件：{error}") from error
            print(output)
        else:
            print(text)
        return 0
    except PresentationReadError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
