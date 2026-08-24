#!/usr/bin/env python3
"""按工作簿顺序把 XLSX 工作表提取为带行号的 TSV。"""

from __future__ import annotations

import argparse
from pathlib import Path
import posixpath
import re
import sys
import xml.etree.ElementTree as ET
import zipfile


SPREADSHEET_NAMESPACE = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"
OFFICE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"
)
PACKAGE_RELATIONSHIP_NAMESPACE = (
    "{http://schemas.openxmlformats.org/package/2006/relationships}"
)
CELL_REFERENCE_PATTERN = re.compile(r"^([A-Za-z]+)([1-9][0-9]*)$")
MAX_XML_BYTES = 64 * 1024 * 1024
MAX_ROWS = 10_000
MAX_COLUMNS = 1_000
EXCEL_MAX_ROW_NUMBER = 1_048_576
EXCEL_MAX_COLUMN_INDEX = 16_384
OLE_MAGIC = b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"


class SpreadsheetReadError(Exception):
    """表示无法安全读取 Excel 工作簿。"""


def _read_xml(archive: zipfile.ZipFile, part_name: str) -> ET.Element:
    try:
        info = archive.getinfo(part_name)
    except KeyError as error:
        raise SpreadsheetReadError(f"XLSX 缺少必要部分：{part_name}") from error
    if info.file_size > MAX_XML_BYTES:
        raise SpreadsheetReadError(f"XLSX XML 超过 64 MiB 安全上限：{part_name}")
    try:
        return ET.fromstring(archive.read(info))
    except ET.ParseError as error:
        raise SpreadsheetReadError(f"XLSX XML 无效（{part_name}）：{error}") from error


def _resolve_target(source_part: str, target: str) -> str:
    if target.startswith("/"):
        resolved = posixpath.normpath(target.lstrip("/"))
    else:
        resolved = posixpath.normpath(
            posixpath.join(posixpath.dirname(source_part), target)
        )
    if resolved == ".." or resolved.startswith("../"):
        raise SpreadsheetReadError(f"XLSX 关系目标越出容器根目录：{target}")
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


def _rich_text(node: ET.Element) -> str:
    return "".join(child.text or "" for child in node.iter(SPREADSHEET_NAMESPACE + "t"))


def _shared_strings(archive: zipfile.ZipFile) -> list[str]:
    part_name = "xl/sharedStrings.xml"
    if part_name not in archive.namelist():
        return []
    root = _read_xml(archive, part_name)
    return [_rich_text(item) for item in root.findall(SPREADSHEET_NAMESPACE + "si")]


def _column_index(reference: str) -> int:
    match = CELL_REFERENCE_PATTERN.fullmatch(reference)
    if match is None:
        raise SpreadsheetReadError(f"无效的单元格引用：{reference or '空值'}")
    result = 0
    for character in match.group(1).upper():
        result = result * 26 + ord(character) - ord("A") + 1
    if result > EXCEL_MAX_COLUMN_INDEX:
        raise SpreadsheetReadError(f"单元格列超出 Excel 上限：{reference}")
    return result


def _column_name(index: int) -> str:
    result = ""
    while index:
        index, remainder = divmod(index - 1, 26)
        result = chr(ord("A") + remainder) + result
    return result


def _cell_value(cell: ET.Element, shared_strings: list[str]) -> str:
    cell_type = cell.get("t", "")
    value_node = cell.find(SPREADSHEET_NAMESPACE + "v")
    raw_value = value_node.text or "" if value_node is not None else ""

    if cell_type == "s":
        try:
            shared_index = int(raw_value)
            if shared_index < 0:
                raise IndexError
            value = shared_strings[shared_index]
        except (ValueError, IndexError) as error:
            raise SpreadsheetReadError(
                f"共享字符串索引无效：{raw_value or '空值'}"
            ) from error
    elif cell_type == "inlineStr":
        inline = cell.find(SPREADSHEET_NAMESPACE + "is")
        value = _rich_text(inline) if inline is not None else ""
    elif cell_type == "b":
        value = "TRUE" if raw_value == "1" else "FALSE"
    elif cell_type == "e":
        value = f"#ERROR:{raw_value}"
    else:
        value = raw_value

    formula = cell.find(SPREADSHEET_NAMESPACE + "f")
    if formula is not None:
        expression = (formula.text or "").strip()
        return f"={expression} => {value}" if value else f"={expression}"
    return value


def _sanitize_tsv(value: str) -> str:
    return (
        value.replace("\r\n", "\n")
        .replace("\r", "\n")
        .replace("\t", " ")
        .replace("\n", " ↩ ")
    )


def _workbook_sheets(archive: zipfile.ZipFile) -> list[tuple[str, str]]:
    workbook_part = "xl/workbook.xml"
    root = _read_xml(archive, workbook_part)
    relationships = _relationships(archive, workbook_part)
    sheets = root.find(SPREADSHEET_NAMESPACE + "sheets")
    if sheets is None:
        return []

    result: list[tuple[str, str]] = []
    for sheet in sheets.findall(SPREADSHEET_NAMESPACE + "sheet"):
        name = sheet.get("name", "").strip()
        relation_id = sheet.get(OFFICE_RELATIONSHIP_NAMESPACE + "id", "")
        relation = relationships.get(relation_id)
        if not name or relation is None or not relation[1].endswith("/worksheet"):
            raise SpreadsheetReadError(
                f"XLSX 工作表关系无效：{name or relation_id or '缺少名称和关系 ID'}"
            )
        result.append((name, relation[0]))
    return result


def _sheet_rows(
    root: ET.Element,
    shared_strings: list[str],
) -> list[tuple[int, dict[int, str]]]:
    sheet_data = root.find(SPREADSHEET_NAMESPACE + "sheetData")
    if sheet_data is None:
        return []

    result: list[tuple[int, dict[int, str]]] = []
    previous_row_number = 0
    for row in sheet_data.findall(SPREADSHEET_NAMESPACE + "row"):
        try:
            row_number = int(row.get("r", str(previous_row_number + 1)))
        except ValueError as error:
            raise SpreadsheetReadError(
                f"无效的工作表行号：{row.get('r', '')}"
            ) from error
        if not 1 <= row_number <= EXCEL_MAX_ROW_NUMBER:
            raise SpreadsheetReadError(f"无效的工作表行号：{row_number}")
        cells: dict[int, str] = {}
        next_column = 1
        for cell in row.findall(SPREADSHEET_NAMESPACE + "c"):
            reference = cell.get("r", "")
            column = _column_index(reference) if reference else next_column
            cells[column] = _cell_value(cell, shared_strings)
            next_column = column + 1
        if cells:
            result.append((row_number, cells))
        previous_row_number = row_number
    return result


def _format_sheet(
    name: str,
    rows: list[tuple[int, dict[int, str]]],
    max_rows: int,
    max_columns: int,
) -> str:
    selected_rows = rows[:max_rows]
    widest_column = max(
        (max(cells, default=0) for _, cells in selected_rows),
        default=0,
    )
    visible_columns = min(widest_column, max_columns)
    lines = [f"## 工作表：{name}"]
    if not selected_rows:
        lines.append("（无可读单元格）")
        return "\n\n".join(lines)

    header = ["行号"] + [_column_name(index) for index in range(1, visible_columns + 1)]
    table = ["\t".join(header)]
    for row_number, cells in selected_rows:
        table.append(
            "\t".join(
                [str(row_number)]
                + [
                    _sanitize_tsv(cells.get(index, ""))
                    for index in range(1, visible_columns + 1)
                ]
            )
        )
    fence = chr(96) * 3
    lines.append(f"{fence}tsv\n" + "\n".join(table) + f"\n{fence}")

    truncation: list[str] = []
    if len(rows) > max_rows:
        truncation.append(f"仅显示前 {max_rows} 个有数据的行，共 {len(rows)} 行")
    if any(max(cells, default=0) > max_columns for _, cells in rows):
        truncation.append(f"仅显示前 {max_columns} 列")
    if truncation:
        lines.append("> 已截断：" + "；".join(truncation) + "。")
    return "\n\n".join(lines)


def read_spreadsheet(
    path: Path,
    sheet_name: str | None,
    max_rows: int,
    max_columns: int,
) -> str:
    if not path.is_file():
        raise SpreadsheetReadError(f"文件不存在：{path}")
    if not zipfile.is_zipfile(path):
        try:
            with path.open("rb") as source:
                is_legacy = source.read(len(OLE_MAGIC)) == OLE_MAGIC
        except OSError as error:
            raise SpreadsheetReadError(f"无法读取文件：{error}") from error
        if is_legacy or path.suffix.lower() == ".xls":
            raise SpreadsheetReadError("旧版 XLS 无法直接读取，请另存为 XLSX 或 CSV")
        raise SpreadsheetReadError("文件不是有效的 XLSX/OOXML 工作簿")

    try:
        with zipfile.ZipFile(path) as archive:
            sheets = _workbook_sheets(archive)
            if sheet_name is not None:
                sheets = [sheet for sheet in sheets if sheet[0] == sheet_name]
                if not sheets:
                    raise SpreadsheetReadError(f"工作表不存在：{sheet_name}")
            shared_strings = _shared_strings(archive)
            sections = [f"# {path.name}"]
            for name, part_name in sheets:
                rows = _sheet_rows(_read_xml(archive, part_name), shared_strings)
                sections.append(_format_sheet(name, rows, max_rows, max_columns))
            if not sheets:
                sections.append("（工作簿不含工作表）")
            return "\n\n".join(sections)
    except (OSError, RuntimeError, zipfile.BadZipFile) as error:
        raise SpreadsheetReadError(f"无法打开 XLSX：{error}") from error


def _bounded_positive(value: str, maximum: int, label: str) -> int:
    try:
        parsed = int(value)
    except ValueError as error:
        raise argparse.ArgumentTypeError(f"{label}必须是整数") from error
    if not 1 <= parsed <= maximum:
        raise argparse.ArgumentTypeError(f"{label}必须在 1 到 {maximum} 之间")
    return parsed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", type=Path, help="XLSX 文件路径")
    parser.add_argument("--sheet", help="只读取名称完全匹配的工作表")
    parser.add_argument(
        "--max-rows",
        type=lambda value: _bounded_positive(value, MAX_ROWS, "最大行数"),
        default=200,
        help=f"每张表最多输出的有数据行数（默认 200，上限 {MAX_ROWS}）",
    )
    parser.add_argument(
        "--max-columns",
        type=lambda value: _bounded_positive(value, MAX_COLUMNS, "最大列数"),
        default=50,
        help=f"每张表最多输出的列数（默认 50，上限 {MAX_COLUMNS}）",
    )
    parser.add_argument("--output", type=Path, help="写入新的 UTF-8 Markdown 文件")
    args = parser.parse_args()
    try:
        text = read_spreadsheet(
            args.path.expanduser().resolve(),
            args.sheet,
            args.max_rows,
            args.max_columns,
        )
        if args.output:
            output = args.output.expanduser().resolve()
            output.parent.mkdir(parents=True, exist_ok=True)
            try:
                with output.open("x", encoding="utf-8") as destination:
                    destination.write(text)
            except FileExistsError as error:
                raise SpreadsheetReadError(f"拒绝覆盖已有输出文件：{output}") from error
            except OSError as error:
                raise SpreadsheetReadError(f"无法写入输出文件：{error}") from error
            print(output)
        else:
            print(text)
        return 0
    except SpreadsheetReadError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
