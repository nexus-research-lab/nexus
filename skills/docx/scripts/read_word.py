#!/usr/bin/env python3
"""按真实文件格式提取 Word 文档文本。"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
import zipfile


OLE_MAGIC = b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"
WORD_NAMESPACE = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
MAX_DOCUMENT_XML_BYTES = 64 * 1024 * 1024


class WordReadError(Exception):
    """表示无法安全读取 Word 文档。"""


def _node_text(node: ET.Element) -> str:
    parts: list[str] = []
    for child in node.iter():
        if child.tag == WORD_NAMESPACE + "t":
            parts.append(child.text or "")
        elif child.tag == WORD_NAMESPACE + "tab":
            parts.append("\t")
        elif child.tag in {WORD_NAMESPACE + "br", WORD_NAMESPACE + "cr"}:
            parts.append("\n")
    return "".join(parts).strip()


def _parse_document_xml(payload: bytes) -> str:
    try:
        root = ET.fromstring(payload)
    except ET.ParseError as error:
        raise WordReadError(f"DOCX 正文 XML 无效：{error}") from error

    body = root.find(WORD_NAMESPACE + "body")
    if body is None:
        raise WordReadError("DOCX 缺少正文结构")

    lines: list[str] = []
    for child in body:
        if child.tag == WORD_NAMESPACE + "p":
            text = _node_text(child)
            if text:
                lines.append(text)
            continue
        if child.tag == WORD_NAMESPACE + "tbl":
            for row in child.iter(WORD_NAMESPACE + "tr"):
                cells = [
                    _node_text(cell).replace("\n", " ")
                    for cell in row.findall(WORD_NAMESPACE + "tc")
                ]
                if any(cells):
                    lines.append("\t".join(cells))
            continue
        for paragraph in child.iter(WORD_NAMESPACE + "p"):
            text = _node_text(paragraph)
            if text:
                lines.append(text)
    return "\n".join(lines)


def _read_docx(path: Path) -> str:
    try:
        with zipfile.ZipFile(path) as archive:
            try:
                info = archive.getinfo("word/document.xml")
            except KeyError as error:
                raise WordReadError("ZIP 文件不是有效 DOCX：缺少 word/document.xml") from error
            if info.file_size > MAX_DOCUMENT_XML_BYTES:
                raise WordReadError("DOCX 正文超过 64 MiB 安全上限")
            return _parse_document_xml(archive.read(info))
    except (OSError, zipfile.BadZipFile) as error:
        raise WordReadError(f"无法打开 DOCX：{error}") from error


def _prefer_console_soffice(path: str) -> str:
    if path.lower().endswith("soffice.exe"):
        console_path = path[:-4] + ".com"
        if os.path.isfile(console_path):
            return console_path
    return path


def _find_soffice() -> str | None:
    configured = os.environ.get("SOFFICE")
    if configured and os.path.isfile(configured):
        return _prefer_console_soffice(configured)
    for command in ("soffice", "libreoffice", "soffice.com", "soffice.exe"):
        if found := shutil.which(command):
            return _prefer_console_soffice(found)
    for candidate in (
        "/Applications/LibreOffice.app/Contents/MacOS/soffice",
        "/usr/bin/soffice",
        "/usr/bin/libreoffice",
        r"C:\Program Files\LibreOffice\program\soffice.exe",
        r"C:\Program Files (x86)\LibreOffice\program\soffice.exe",
    ):
        if os.path.isfile(candidate):
            return _prefer_console_soffice(candidate)
    return None


def _convert_legacy_with_soffice(path: Path, executable: str) -> str:
    with tempfile.TemporaryDirectory(prefix="nexus-word-") as directory:
        root = Path(directory)
        source = root / "source.doc"
        output = root / "output"
        profile = root / "profile"
        output.mkdir()
        profile.mkdir()
        shutil.copyfile(path, source)
        result = subprocess.run(
            [
                executable,
                "--headless",
                f"-env:UserInstallation={profile.as_uri()}",
                "--convert-to",
                "docx",
                "--outdir",
                str(output),
                str(source),
            ],
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
        converted = output / "source.docx"
        if result.returncode != 0 or not converted.is_file():
            detail = (result.stderr or result.stdout).strip()[-500:]
            raise WordReadError(f"LibreOffice 转换失败：{detail or '未生成 DOCX'}")
        return _read_docx(converted)


def _read_legacy_with_textutil(path: Path) -> str:
    executable = Path("/usr/bin/textutil")
    if sys.platform != "darwin" or not executable.is_file():
        raise WordReadError("macOS textutil 不可用")
    result = subprocess.run(
        [str(executable), "-convert", "txt", "-stdout", str(path)],
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    if result.returncode != 0:
        raise WordReadError(f"textutil 转换失败：{result.stderr.strip()[-500:]}")
    return result.stdout.strip()


def _read_legacy(path: Path) -> str:
    errors: list[str] = []
    if executable := _find_soffice():
        try:
            return _convert_legacy_with_soffice(path, executable)
        except (OSError, subprocess.SubprocessError, WordReadError) as error:
            errors.append(str(error))
    try:
        return _read_legacy_with_textutil(path)
    except (OSError, subprocess.SubprocessError, WordReadError) as error:
        errors.append(str(error))
    detail = "；".join(errors)
    raise WordReadError(
        "旧版 Word 文档需要 LibreOffice（soffice/libreoffice），或在 macOS 使用 "
        f"textutil。也可以把文件另存为真正的 DOCX/PDF。{detail}"
    )


def read_word(path: Path) -> str:
    if not path.is_file():
        raise WordReadError(f"文件不存在：{path}")
    if zipfile.is_zipfile(path):
        return _read_docx(path)
    try:
        with path.open("rb") as source:
            magic = source.read(len(OLE_MAGIC))
    except OSError as error:
        raise WordReadError(f"无法读取文件：{error}") from error
    if magic == OLE_MAGIC:
        return _read_legacy(path)
    raise WordReadError("文件既不是标准 DOCX，也不是旧版 DOC/OLE 文档")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="Word 文件路径")
    parser.add_argument("--output", type=Path, help="写入新的 UTF-8 文本文件")
    args = parser.parse_args()
    try:
        text = read_word(args.path.expanduser().resolve())
        if args.output:
            output = args.output.expanduser().resolve()
            output.parent.mkdir(parents=True, exist_ok=True)
            try:
                with output.open("x", encoding="utf-8") as destination:
                    destination.write(text)
            except FileExistsError as error:
                raise WordReadError(f"拒绝覆盖已有输出文件：{output}") from error
            except OSError as error:
                raise WordReadError(f"无法写入输出文件：{error}") from error
            print(output)
        else:
            print(text)
        if not text.strip():
            print("警告：未提取到文本，文档可能仅包含扫描图片。", file=sys.stderr)
        return 0
    except WordReadError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
