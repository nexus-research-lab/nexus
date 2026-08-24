#!/usr/bin/env python3
"""Excel 读取脚本的最小回归测试。"""

from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile


SCRIPT = Path(__file__).parents[1] / "scripts" / "read_spreadsheet.py"
WORKBOOK_XML = """<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="汇总" sheetId="1" r:id="rId2"/><sheet name="明细" sheetId="2" r:id="rId1"/></sheets>
</workbook>
"""
WORKBOOK_RELS = """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
</Relationships>
"""
SHARED_STRINGS_XML = """<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>收入</t></si><si><r><t>本</t></r><r><t>季</t></r></si>
</sst>
"""
SUMMARY_XML = """<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
  <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="inlineStr"><is><t>金额</t></is></c></row>
  <row r="2"><c r="A2" t="s"><v>1</v></c><c r="B2"><f>SUM(B3:B4)</f><v>300</v></c><c r="C2" t="b"><v>1</v></c></row>
  <row r="9"><c r="A9"><v>900</v></c></row>
</sheetData></worksheet>
"""
DETAIL_XML = """<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
  <row r="1"><c r="A1" t="inlineStr"><is><t>明细值</t></is></c></row>
</sheetData></worksheet>
"""


def run(*arguments: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *arguments],
        capture_output=True,
        text=True,
        check=False,
    )


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="nexus-xlsx-test-") as directory:
        path = Path(directory) / "ordered.xlsx"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("xl/workbook.xml", WORKBOOK_XML)
            archive.writestr("xl/_rels/workbook.xml.rels", WORKBOOK_RELS)
            archive.writestr("xl/sharedStrings.xml", SHARED_STRINGS_XML)
            archive.writestr("xl/worksheets/sheet1.xml", DETAIL_XML)
            archive.writestr("xl/worksheets/sheet2.xml", SUMMARY_XML)

        result = run(str(path), "--max-rows", "2")
        assert result.returncode == 0, result.stderr
        assert result.stdout.index("工作表：汇总") < result.stdout.index("工作表：明细")
        assert "收入\t金额" in result.stdout
        assert "本季\t=SUM(B3:B4) => 300\tTRUE" in result.stdout
        assert "仅显示前 2 个有数据的行，共 3 行" in result.stdout

        selected = run(str(path), "--sheet", "明细")
        assert selected.returncode == 0, selected.stderr
        assert "明细值" in selected.stdout
        assert "工作表：汇总" not in selected.stdout
    print("ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
