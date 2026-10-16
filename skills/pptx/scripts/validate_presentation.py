#!/usr/bin/env python3
"""验证 PPTX 容器、XML 和内部关系，不替代 PowerPoint/LibreOffice 渲染检查。"""

from __future__ import annotations

import argparse
from pathlib import Path
import posixpath
import sys
import xml.etree.ElementTree as ET
import zipfile


PACKAGE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/package/2006/relationships}"
)
PRESENTATION_NAMESPACE = "{http://schemas.openxmlformats.org/presentationml/2006/main}"
OFFICE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"
)
MAX_XML_BYTES = 64 * 1024 * 1024
REQUIRED_PARTS = {
    "[Content_Types].xml",
    "_rels/.rels",
    "ppt/presentation.xml",
    "ppt/_rels/presentation.xml.rels",
}


class PresentationValidationError(Exception):
    """表示 PPTX 不能安全继续处理。"""


def _resolve_target(source_part: str, target: str) -> str:
    if target.startswith("/"):
        resolved = posixpath.normpath(target.lstrip("/"))
    else:
        resolved = posixpath.normpath(
            posixpath.join(posixpath.dirname(source_part), target)
        )
    if resolved == ".." or resolved.startswith("../"):
        raise PresentationValidationError(
            f"关系目标越出 PPTX 容器根目录：{source_part} -> {target}"
        )
    return resolved


def _read_xml(archive: zipfile.ZipFile, part: str) -> ET.Element:
    try:
        info = archive.getinfo(part)
    except KeyError as error:
        raise PresentationValidationError(f"缺少 XML 部件：{part}") from error
    if info.file_size > MAX_XML_BYTES:
        raise PresentationValidationError(f"XML 部件超过 64 MiB：{part}")
    try:
        return ET.fromstring(archive.read(info))
    except ET.ParseError as error:
        raise PresentationValidationError(f"XML 无效：{part}: {error}") from error


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
        if not relation_id or not target:
            raise PresentationValidationError(f"关系缺少 Id 或 Target：{part}")
        if relation.get("TargetMode") == "External":
            continue
        resolved = _resolve_target(source_part, target)
        if resolved not in archive.namelist():
            raise PresentationValidationError(
                f"关系目标不存在：{source_part} -> {resolved}"
            )
        result[relation_id] = (resolved, relation.get("Type", ""))
    return result


def validate_presentation(path: Path) -> None:
    if not path.is_file():
        raise PresentationValidationError(f"文件不存在：{path}")
    try:
        with zipfile.ZipFile(path) as archive:
            if archive.testzip() is not None:
                raise PresentationValidationError("ZIP 压缩数据校验失败")
            names = archive.namelist()
            duplicates = {name for name in names if names.count(name) > 1}
            if duplicates:
                raise PresentationValidationError(
                    "ZIP 含重复部件：" + ", ".join(sorted(duplicates))
                )
            missing = REQUIRED_PARTS - set(names)
            if missing:
                raise PresentationValidationError(
                    "PPTX 缺少必要部件：" + ", ".join(sorted(missing))
                )
            for name in names:
                if name.endswith(".xml") or name.endswith(".rels"):
                    _read_xml(archive, name)

            presentation = _read_xml(archive, "ppt/presentation.xml")
            relationships = _relationships(archive, "ppt/presentation.xml")
            slide_list = presentation.find(PRESENTATION_NAMESPACE + "sldIdLst")
            if slide_list is None:
                raise PresentationValidationError("presentation.xml 缺少 sldIdLst")
            for slide_id in slide_list.findall(PRESENTATION_NAMESPACE + "sldId"):
                relation_id = slide_id.get(
                    OFFICE_RELATIONSHIP_NAMESPACE + "id", ""
                )
                relation = relationships.get(relation_id)
                if relation is None or not relation[1].endswith("/slide"):
                    raise PresentationValidationError(
                        f"幻灯片关系无效：{relation_id or '缺少关系 ID'}"
                    )
    except (OSError, zipfile.BadZipFile) as error:
        raise PresentationValidationError(f"无法打开 PPTX：{error}") from error


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="待验证的 PPTX 文件")
    args = parser.parse_args()
    try:
        path = args.path.expanduser().resolve()
        validate_presentation(path)
        print(f"valid: {path}")
        return 0
    except PresentationValidationError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
