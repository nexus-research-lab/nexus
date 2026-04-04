# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：diary_entry_parser.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""日记条目解析器。"""

from __future__ import annotations

import hashlib
import re
from collections import OrderedDict
from datetime import datetime

from agent.service.memory.diary_entry import DiaryEntry


class DiaryEntryParser:
    """负责解析 markdown 日记文件。"""

    _HEADING_RE = re.compile(
        r"^### (?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}) - \[(?P<kind>[A-Z]+)\] (?P<body>.+)$"
    )
    _FIELD_RE = re.compile(r"^\*\s+\*\*(?P<key>[^*]+)\*\*: ?(?P<value>.*)$")

    def parse(self, content: str, path: str | None = None) -> list[DiaryEntry]:
        """解析一整个日记文件。"""
        entries: list[DiaryEntry] = []
        current_lines: list[str] = []

        for line in content.splitlines():
            if self._HEADING_RE.match(line):
                if current_lines:
                    entries.append(self._parse_entry(current_lines, path))
                current_lines = [line]
                continue
            if current_lines:
                current_lines.append(line)

        if current_lines:
            entries.append(self._parse_entry(current_lines, path))
        return entries

    def _parse_entry(self, lines: list[str], path: str | None) -> DiaryEntry:
        """解析单条日记。"""
        heading_match = self._HEADING_RE.match(lines[0])
        if not heading_match:
            raise ValueError("日记标题格式不正确")

        created_at = datetime.strptime(heading_match.group("timestamp"), "%Y-%m-%d %H:%M")
        kind = heading_match.group("kind")
        category, title = self._split_heading_body(kind, heading_match.group("body"))

        fields = OrderedDict()
        for line in lines[1:]:
            field_match = self._FIELD_RE.match(line)
            if not field_match:
                continue
            fields[field_match.group("key").strip()] = field_match.group("value").strip()

        entry_id = fields.pop("ID", "") or self._build_legacy_entry_id(
            kind=kind,
            created_at=created_at,
            title=title,
        )
        return DiaryEntry(
            entry_id=entry_id,
            created_at=created_at,
            kind=kind,
            title=title,
            category=category,
            fields=fields,
            path=path,
        )

    @staticmethod
    def _split_heading_body(kind: str, body: str) -> tuple[str | None, str]:
        """拆分标题体。"""
        if kind == "LRN" and ": " in body:
            category, title = body.split(": ", 1)
            return category.strip(), title.strip()
        return None, body.strip()

    @staticmethod
    def _build_legacy_entry_id(kind: str, created_at: datetime, title: str) -> str:
        """为旧条目生成回填 ID。"""
        digest = hashlib.md5(title.encode("utf-8")).hexdigest()[:8]
        return f"{kind}-{created_at.strftime('%Y%m%d-%H%M')}-{digest}"
