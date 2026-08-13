#!/usr/bin/env python3
"""Word 读取脚本的最小回归测试。"""

from pathlib import Path
import subprocess
import sys
import tempfile
import zipfile


SCRIPT = Path(__file__).parents[1] / "scripts" / "read_word.py"
DOCUMENT_XML = """<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>季度报告</w:t></w:r></w:p>
    <w:tbl><w:tr>
      <w:tc><w:p><w:r><w:t>收入</w:t></w:r></w:p></w:tc>
      <w:tc><w:p><w:r><w:t>100</w:t></w:r></w:p></w:tc>
    </w:tr></w:tbl>
  </w:body>
</w:document>
"""


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="nexus-docx-test-") as directory:
        path = Path(directory) / "renamed.doc"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("word/document.xml", DOCUMENT_XML)
        result = subprocess.run(
            [sys.executable, str(SCRIPT), str(path)],
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, result.stderr
        assert "季度报告" in result.stdout
        assert "收入\t100" in result.stdout
    print("ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
