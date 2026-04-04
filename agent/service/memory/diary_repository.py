# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：diary_repository.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""日记文件仓储。"""

from __future__ import annotations

from datetime import date, datetime, timedelta
from pathlib import Path
from typing import Callable

from agent.service.memory.diary_entry import DiaryEntry
from agent.service.memory.diary_entry_parser import DiaryEntryParser


class DiaryRepository:
    """负责工作区记忆文件读写。"""

    _ROOT_MEMORY_FILES = (
        "MEMORY.md",
        "SOUL.md",
        "TOOLS.md",
        "AGENTS.md",
        "RUNBOOK.md",
    )

    def __init__(self, workspace_path: Path | str) -> None:
        self._workspace_path = Path(workspace_path).expanduser().resolve()
        self._parser = DiaryEntryParser()

    def search(self, query: str, limit: int = 20) -> list[dict[str, object]]:
        """按关键词搜索记忆内容。"""
        terms = [item for item in query.lower().split() if item]
        if not terms:
            raise ValueError("query 不能为空")

        matches: list[dict[str, object]] = []
        for path in self._iter_search_files():
            for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
                lower_line = line.lower()
                if not all(term in lower_line for term in terms):
                    continue
                matches.append(
                    {
                        "path": path.relative_to(self._workspace_path).as_posix(),
                        "line": line_number,
                        "content": line.strip(),
                    }
                )
                if len(matches) >= limit:
                    return matches
        return matches

    def read_slice(self, relative_path: str, from_line: int = 1, lines: int = 50) -> dict[str, object]:
        """读取指定文件片段。"""
        target_path = self._resolve_workspace_file(relative_path)
        content_lines = target_path.read_text(encoding="utf-8").splitlines()
        start_index = max(from_line, 1) - 1
        end_index = start_index + max(lines, 1)
        snippet = content_lines[start_index:end_index]
        return {
            "path": target_path.relative_to(self._workspace_path).as_posix(),
            "from_line": start_index + 1,
            "to_line": start_index + len(snippet),
            "content": "\n".join(snippet),
        }

    def list_recent_entries(self, days: int = 7, limit: int = 50) -> list[DiaryEntry]:
        """按时间倒序返回近期条目。"""
        today = date.today()
        entries: list[DiaryEntry] = []

        for diary_path in self._iter_diary_files():
            diary_date = self._parse_diary_date(diary_path)
            if diary_date is None or diary_date < today - timedelta(days=max(days - 1, 0)):
                continue

            relative_path = diary_path.relative_to(self._workspace_path).as_posix()
            parsed_entries = self._parser.parse(
                diary_path.read_text(encoding="utf-8"),
                path=relative_path,
            )
            entries.extend(reversed(parsed_entries))
            if len(entries) >= limit:
                break
        return entries[:limit]

    def append_entry(self, entry: DiaryEntry) -> str:
        """追加单条日记。"""
        diary_path = self._workspace_path / "diary" / f"{entry.created_at.strftime('%Y-%m-%d')}.md"
        diary_path.parent.mkdir(parents=True, exist_ok=True)

        existing = diary_path.read_text(encoding="utf-8").rstrip() if diary_path.exists() else ""
        body = entry.to_markdown()
        next_content = body + "\n" if not existing else existing + "\n\n" + body + "\n"
        diary_path.write_text(next_content, encoding="utf-8")

        relative_path = diary_path.relative_to(self._workspace_path).as_posix()
        entry.path = relative_path
        return relative_path

    def update_entry(self, entry_id: str, updater: Callable[[DiaryEntry], None]) -> DiaryEntry:
        """更新指定条目。"""
        for diary_path in self._iter_diary_files():
            relative_path = diary_path.relative_to(self._workspace_path).as_posix()
            entries = self._parser.parse(diary_path.read_text(encoding="utf-8"), path=relative_path)
            for entry in entries:
                if entry.entry_id != entry_id:
                    continue
                updater(entry)
                diary_path.write_text(self._render_entries(entries), encoding="utf-8")
                return entry
        raise FileNotFoundError(f"未找到条目: {entry_id}")

    def append_to_memory_section(self, filename: str, section_title: str, bullet: str) -> str:
        """向长期文件追加一条规则。"""
        target_path = self._workspace_path / filename
        existing = target_path.read_text(encoding="utf-8") if target_path.exists() else f"# {filename}\n\n"
        marker = f"## {section_title}\n"
        normalized = existing if existing.endswith("\n") else existing + "\n"

        if marker not in normalized:
            updated = normalized + f"\n## {section_title}\n{bullet}\n"
        else:
            start = normalized.index(marker) + len(marker)
            next_section = normalized.find("\n## ", start)
            if next_section == -1:
                section_body = normalized[start:].rstrip("\n")
                prefix = normalized[:start]
                suffix = ""
            else:
                section_body = normalized[start:next_section].rstrip("\n")
                prefix = normalized[:start]
                suffix = normalized[next_section:]
            if section_body:
                section_body += "\n"
            updated = prefix + section_body + bullet + "\n" + suffix

        target_path.write_text(updated, encoding="utf-8")
        return filename

    def _iter_search_files(self):
        """按优先级遍历可搜索文件。"""
        for name in self._ROOT_MEMORY_FILES:
            path = self._workspace_path / name
            if path.exists():
                yield path

        for diary_path in self._iter_diary_files():
            yield diary_path

        memory_dir = self._workspace_path / "memory"
        if not memory_dir.exists():
            return
        for path in sorted(memory_dir.glob("*.md"), reverse=True):
            if path.is_file():
                yield path

    def _iter_diary_files(self):
        """返回所有 diary 文件。"""
        diary_dir = self._workspace_path / "diary"
        if not diary_dir.exists():
            return []
        return sorted(diary_dir.glob("*.md"), reverse=True)

    def _resolve_workspace_file(self, relative_path: str) -> Path:
        """解析工作区内路径。"""
        normalized = relative_path.strip().lstrip("/").replace("\\", "/")
        if not normalized:
            raise ValueError("path 不能为空")

        target_path = (self._workspace_path / normalized).resolve()
        if not target_path.is_relative_to(self._workspace_path):
            raise ValueError("path 超出 workspace 范围")
        if not target_path.exists():
            raise FileNotFoundError(f"文件不存在: {normalized}")
        if target_path.is_dir():
            raise ValueError("不能直接读取目录")
        return target_path

    @staticmethod
    def _parse_diary_date(path: Path) -> date | None:
        """从日记文件名提取日期。"""
        try:
            return datetime.strptime(path.stem, "%Y-%m-%d").date()
        except ValueError:
            return None

    @staticmethod
    def _render_entries(entries: list[DiaryEntry]) -> str:
        """按标准格式重写日记文件。"""
        return "\n\n".join(entry.to_markdown() for entry in entries).rstrip() + "\n"
