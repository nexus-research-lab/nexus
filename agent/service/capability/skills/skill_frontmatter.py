# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_frontmatter.py
# @Date   ：2026/3/30 20:33
# @Author ：Codex
# 2026/3/30 20:33   Create
# =====================================================

"""Skill frontmatter 解析工具。"""

from __future__ import annotations

import ast
import re
from pathlib import Path


class SkillFrontmatterParser:
    """负责从 SKILL.md 中提取基础元信息。"""

    _cache: dict[str, tuple[int, dict[str, object]]] = {}

    @staticmethod
    def parse(skill_md: Path) -> dict[str, object]:
        """读取 frontmatter 与原始 markdown。"""
        stat = skill_md.stat()
        cache_key = str(skill_md.resolve())
        cached = SkillFrontmatterParser._cache.get(cache_key)
        if cached and cached[0] == stat.st_mtime_ns:
            return dict(cached[1])

        content = skill_md.read_text(encoding="utf-8")
        frontmatter = SkillFrontmatterParser._extract_frontmatter(content)
        data = SkillFrontmatterParser._parse_lines(frontmatter)
        data["readme_markdown"] = content
        data["name"] = str(data.get("name") or skill_md.parent.name)
        data["title"] = str(data.get("title") or data["name"])
        data["description"] = str(data.get("description") or "")
        data["scope"] = str(data.get("scope") or "any")
        data["tags"] = SkillFrontmatterParser._normalize_tags(data.get("tags"))
        SkillFrontmatterParser._cache[cache_key] = (stat.st_mtime_ns, dict(data))
        return data

    @staticmethod
    def _extract_frontmatter(content: str) -> str:
        match = re.match(r"^---\s*\n(.*?)\n---", content, re.DOTALL)
        if not match:
            return ""
        return match.group(1)

    @staticmethod
    def _parse_lines(frontmatter: str) -> dict[str, object]:
        data: dict[str, object] = {}
        if not frontmatter:
            return data

        pending_list_key: str | None = None
        pending_list_values: list[str] = []
        for raw_line in frontmatter.splitlines():
            line = raw_line.rstrip()
            if not line.strip():
                continue
            if pending_list_key and line.lstrip().startswith("- "):
                pending_list_values.append(line.lstrip()[2:].strip().strip("\"'"))
                continue
            if pending_list_key:
                data[pending_list_key] = pending_list_values
                pending_list_key = None
                pending_list_values = []

            if ":" not in line:
                continue
            key, _, value = line.partition(":")
            clean_key = key.strip()
            clean_value = value.strip()
            if not clean_value:
                pending_list_key = clean_key
                pending_list_values = []
                continue
            data[clean_key] = SkillFrontmatterParser._parse_value(clean_value)

        if pending_list_key:
            data[pending_list_key] = pending_list_values
        return data

    @staticmethod
    def _parse_value(value: str) -> object:
        stripped = value.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            try:
                parsed = ast.literal_eval(stripped)
                if isinstance(parsed, list):
                    return [str(item) for item in parsed]
            except (SyntaxError, ValueError):
                pass
        return stripped.strip("\"'")

    @staticmethod
    def _normalize_tags(value: object) -> list[str]:
        if isinstance(value, list):
            return [str(item).strip() for item in value if str(item).strip()]
        if isinstance(value, str) and value:
            if "," in value:
                return [item.strip().strip("[]\"'") for item in value.split(",") if item.strip()]
            return [value.strip().strip("[]\"'")]
        return []
