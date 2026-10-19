#!/usr/bin/env python3
"""PPTX inventory、关系校验和页面重排工具的回归测试。"""

from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile


ROOT = Path(__file__).parents[1]
INVENTORY = ROOT / "scripts" / "inventory_presentation.py"
REORDER = ROOT / "scripts" / "reorder_slides.py"
VALIDATE = ROOT / "scripts" / "validate_presentation.py"
PRESENTATION_NAMESPACE = "http://schemas.openxmlformats.org/presentationml/2006/main"
DRAWING_NAMESPACE = "http://schemas.openxmlformats.org/drawingml/2006/main"
RELATIONSHIP_NAMESPACE = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PACKAGE_NAMESPACE = "http://schemas.openxmlformats.org/package/2006/relationships"


def _fixture(path: Path) -> None:
    content_types = """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/slides/slide3.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>"""
    root_relationships = f"""<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="{PACKAGE_NAMESPACE}">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>"""
    presentation = f"""<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="{PRESENTATION_NAMESPACE}" xmlns:r="{RELATIONSHIP_NAMESPACE}">
  <p:sldSz cx="12192000" cy="6858000"/>
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId1"/><p:sldId id="257" r:id="rId2"/><p:sldId id="258" r:id="rId3"/>
  </p:sldIdLst>
</p:presentation>"""
    presentation_relationships = f"""<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="{PACKAGE_NAMESPACE}">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide3.xml"/>
</Relationships>"""

    def slide(title: str) -> str:
        return f"""<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="{PRESENTATION_NAMESPACE}" xmlns:a="{DRAWING_NAMESPACE}">
  <p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="标题"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
      <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>{title}</a:t></a:r></a:p></p:txBody>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>"""

    with zipfile.ZipFile(path, "w") as archive:
        archive.writestr("[Content_Types].xml", content_types)
        archive.writestr("_rels/.rels", root_relationships)
        archive.writestr("ppt/presentation.xml", presentation)
        archive.writestr("ppt/_rels/presentation.xml.rels", presentation_relationships)
        for index, title in enumerate(("第一页", "第二页", "第三页"), start=1):
            archive.writestr(f"ppt/slides/slide{index}.xml", slide(title))


def _run(script: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(script), *args],
        capture_output=True,
        text=True,
        check=False,
    )


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="nexus-pptx-tools-test-") as directory:
        root = Path(directory)
        source = root / "source.pptx"
        reordered = root / "reordered.pptx"
        inventory = root / "inventory.json"
        _fixture(source)

        result = _run(VALIDATE, str(source))
        assert result.returncode == 0, result.stderr

        result = _run(INVENTORY, str(source), "--output", str(inventory))
        assert result.returncode == 0, result.stderr
        value = json.loads(inventory.read_text(encoding="utf-8"))
        assert value["slide_count"] == 3
        assert value["slides"][0]["title"] == "第一页"
        assert value["slides"][0]["shapes"][0]["placeholder"]["type"] == "title"

        result = _run(
            REORDER,
            str(source),
            "--order",
            "3,1,2",
            "--output",
            str(reordered),
        )
        assert result.returncode == 0, result.stderr
        result = _run(VALIDATE, str(reordered))
        assert result.returncode == 0, result.stderr
        result = _run(ROOT / "scripts" / "read_presentation.py", str(reordered))
        assert result.returncode == 0, result.stderr
        assert result.stdout.index("第三页") < result.stdout.index("第一页")
    print("ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
