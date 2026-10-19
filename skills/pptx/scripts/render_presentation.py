#!/usr/bin/env python3
"""用 LibreOffice 和 Poppler 渲染 PPTX，生成 PDF 与逐页 PNG 供视觉检查。"""

from __future__ import annotations

import argparse
from pathlib import Path
import os
import shutil
import subprocess
import sys
import tempfile


class PresentationRenderError(Exception):
    """表示 PPTX 视觉渲染失败。"""


def _find_command(env_name: str, names: tuple[str, ...]) -> str:
    configured = os.environ.get(env_name)
    if configured and Path(configured).is_file():
        return configured
    for name in names:
        if found := shutil.which(name):
            return found
    raise PresentationRenderError(
        f"找不到 {env_name} 对应的程序，请安装 {', '.join(names)} 或设置 {env_name}"
    )


def render_presentation(source: Path, output_dir: Path, dpi: int) -> tuple[Path, int]:
    source = source.expanduser().resolve()
    output_dir = output_dir.expanduser().resolve()
    if not source.is_file():
        raise PresentationRenderError(f"文件不存在：{source}")
    if dpi < 72 or dpi > 600:
        raise PresentationRenderError("--dpi 必须在 72 到 600 之间")
    if output_dir.exists() and any(output_dir.iterdir()):
        raise PresentationRenderError(f"拒绝写入非空目录：{output_dir}")
    output_dir.mkdir(parents=True, exist_ok=True)
    soffice = _find_command("SOFFICE", ("soffice", "libreoffice"))
    pdftoppm = _find_command("PDFTOPPM", ("pdftoppm",))
    pdf_path = output_dir / f"{source.stem}.pdf"
    profile = Path(tempfile.mkdtemp(prefix="nexus-pptx-profile-"))
    try:
        result = subprocess.run(
            [
                soffice,
                "--headless",
                f"-env:UserInstallation={profile.as_uri()}",
                "--convert-to",
                "pdf",
                "--outdir",
                str(output_dir),
                str(source),
            ],
            capture_output=True,
            text=True,
            timeout=180,
            check=False,
        )
        if result.returncode != 0 or not pdf_path.is_file():
            detail = (result.stderr or result.stdout).strip()[-800:]
            raise PresentationRenderError(f"LibreOffice 转换失败：{detail or '未生成 PDF'}")
        prefix = output_dir / "slide"
        result = subprocess.run(
            [pdftoppm, "-png", "-r", str(dpi), str(pdf_path), str(prefix)],
            capture_output=True,
            text=True,
            timeout=180,
            check=False,
        )
        if result.returncode != 0:
            detail = (result.stderr or result.stdout).strip()[-800:]
            raise PresentationRenderError(f"pdftoppm 渲染失败：{detail or '未知错误'}")
        images = sorted(output_dir.glob("slide-*.png"))
        if not images:
            raise PresentationRenderError("pdftoppm 未生成页面图片")
        return pdf_path, len(images)
    except (OSError, subprocess.SubprocessError) as error:
        raise PresentationRenderError(f"渲染过程失败：{error}") from error
    finally:
        shutil.rmtree(profile, ignore_errors=True)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="待渲染的 PPTX")
    parser.add_argument("--output-dir", required=True, type=Path, help="空的 QA 输出目录")
    parser.add_argument("--dpi", type=int, default=144, help="PNG 分辨率，默认 144")
    args = parser.parse_args()
    try:
        pdf_path, count = render_presentation(args.source, args.output_dir, args.dpi)
        print(f"pdf: {pdf_path}")
        print(f"slides: {count}")
        return 0
    except PresentationRenderError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
