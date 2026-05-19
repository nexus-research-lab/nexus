#!/usr/bin/env python3
"""移除纯色 chroma key 背景，并输出带 alpha 通道的 PNG/WebP。"""

from __future__ import annotations

import argparse
from pathlib import Path
from statistics import median
from typing import Tuple

Color = Tuple[int, int, int]


def parse_hex_color(value: str) -> Color:
    raw = value.strip().removeprefix("#")
    if len(raw) != 6:
        raise SystemExit("--key-color 必须是 #00ff00 这种 6 位 RGB 十六进制颜色")
    return int(raw[0:2], 16), int(raw[2:4], 16), int(raw[4:6], 16)


def load_pillow():
    try:
        from PIL import Image
    except ImportError as exc:
        raise SystemExit("需要先安装 Pillow：python -m pip install pillow") from exc
    return Image


def distance(left: Color, right: Color) -> int:
    return max(abs(left[0] - right[0]), abs(left[1] - right[1]), abs(left[2] - right[2]))


def sample_border_key(image) -> Color:
    width, height = image.size
    pixels = image.load()
    band = max(1, min(width, height, 6))
    samples: list[Color] = []

    for x in range(width):
        for y in range(band):
            samples.append(pixels[x, y][:3])
            samples.append(pixels[x, height - 1 - y][:3])
    for y in range(height):
        for x in range(band):
            samples.append(pixels[x, y][:3])
            samples.append(pixels[width - 1 - x, y][:3])

    return (
        round(median(sample[0] for sample in samples)),
        round(median(sample[1] for sample in samples)),
        round(median(sample[2] for sample in samples)),
    )


def soft_alpha(value: int, transparent_threshold: float, opaque_threshold: float) -> int:
    if value <= transparent_threshold:
        return 0
    if value >= opaque_threshold:
        return 255
    ratio = (float(value) - transparent_threshold) / (opaque_threshold - transparent_threshold)
    smoothed = ratio * ratio * (3 - 2 * ratio)
    return max(0, min(255, round(255 * smoothed)))


def remove_key(args: argparse.Namespace) -> None:
    Image = load_pillow()
    source = Path(args.input)
    output = Path(args.out)
    if not source.exists():
        raise SystemExit(f"输入图片不存在：{source}")
    if output.suffix.lower() not in {".png", ".webp"}:
        raise SystemExit("--out 必须是 .png 或 .webp，才能保留透明通道")

    with Image.open(source) as original:
        image = original.convert("RGBA")

    key = sample_border_key(image) if args.auto_key == "border" else parse_hex_color(args.key_color)
    pixels = image.load()
    width, height = image.size
    transparent = 0

    for y in range(height):
        for x in range(width):
            red, green, blue, alpha = pixels[x, y]
            alpha_value = soft_alpha(
                distance((red, green, blue), key),
                args.transparent_threshold,
                args.opaque_threshold,
            )
            alpha_value = round(alpha_value * (alpha / 255))
            if alpha_value == 0:
                transparent += 1
                pixels[x, y] = (0, 0, 0, 0)
            else:
                pixels[x, y] = (red, green, blue, alpha_value)

    output.parent.mkdir(parents=True, exist_ok=True)
    image.save(output)
    print(f"Wrote {output}")
    print(f"Key color: #{key[0]:02x}{key[1]:02x}{key[2]:02x}")
    print(f"Transparent pixels: {transparent}/{width * height}")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Remove a solid chroma-key background.")
    parser.add_argument("--input", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--key-color", default="#00ff00")
    parser.add_argument("--auto-key", choices=["none", "border"], default="none")
    parser.add_argument("--soft-matte", action="store_true")
    parser.add_argument("--transparent-threshold", type=float, default=12)
    parser.add_argument("--opaque-threshold", type=float, default=96)
    parser.add_argument("--despill", action="store_true")
    return parser


if __name__ == "__main__":
    remove_key(build_parser().parse_args())
