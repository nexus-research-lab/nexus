#!/usr/bin/env python3
"""PowerPoint 读取脚本的最小回归测试。"""

from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile


SCRIPT = Path(__file__).parents[1] / "scripts" / "read_presentation.py"
PRESENTATION_XML = """<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId1"/></p:sldIdLst>
</p:presentation>
"""
PRESENTATION_RELS = """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>
"""
SLIDE_TEMPLATE = """<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>{title}</a:t></a:r></a:p></p:txBody></p:sp>{table}</p:spTree></p:cSld>
</p:sld>
"""
TABLE_XML = """<a:graphicFrame><a:graphic><a:graphicData><a:tbl>
  <a:tr><a:tc><a:txBody><a:p><a:r><a:t>指标</a:t></a:r></a:p></a:txBody></a:tc>
  <a:tc><a:txBody><a:p><a:r><a:t>结果</a:t></a:r></a:p></a:txBody></a:tc></a:tr>
</a:tbl></a:graphicData></a:graphic></a:graphicFrame>"""
SLIDE_RELS = """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/>
</Relationships>
"""
NOTES_XML = """<?xml version="1.0" encoding="UTF-8"?>
<p:notes xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr/><p:cNvSpPr/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>只在备注里</a:t></a:r></a:p></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr/><p:cNvSpPr/><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>占位图</a:t></a:r></a:p></p:txBody></p:sp>
  </p:spTree></p:cSld>
</p:notes>
"""


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="nexus-pptx-test-") as directory:
        path = Path(directory) / "ordered.pptx"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("ppt/presentation.xml", PRESENTATION_XML)
            archive.writestr("ppt/_rels/presentation.xml.rels", PRESENTATION_RELS)
            archive.writestr(
                "ppt/slides/slide1.xml",
                SLIDE_TEMPLATE.format(title="第一页", table=""),
            )
            archive.writestr(
                "ppt/slides/slide2.xml",
                SLIDE_TEMPLATE.format(title="第二页", table=TABLE_XML),
            )
            archive.writestr("ppt/slides/_rels/slide2.xml.rels", SLIDE_RELS)
            archive.writestr("ppt/notesSlides/notesSlide1.xml", NOTES_XML)

        result = subprocess.run(
            [sys.executable, str(SCRIPT), str(path)],
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, result.stderr
        assert result.stdout.index("第二页") < result.stdout.index("第一页")
        assert "指标\t结果" in result.stdout
        assert "### 演讲者备注" in result.stdout
        assert "只在备注里" in result.stdout
        assert "占位图" not in result.stdout
    print("ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
