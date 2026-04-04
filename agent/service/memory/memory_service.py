# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：memory_service.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""Agent 记忆服务。"""

from __future__ import annotations

from pathlib import Path

from agent.service.memory.diary_entry import DiaryEntry
from agent.service.memory.diary_entry_factory import DiaryEntryFactory
from agent.service.memory.diary_repository import DiaryRepository
from agent.service.memory.diary_similarity_matcher import DiarySimilarityMatcher


class MemoryService:
    """负责检索、记录、提升和流转工作区记忆。"""

    _PROMOTION_TARGETS = {
        "memory": ("MEMORY.md", "长期记忆"),
        "soul": ("SOUL.md", "行为准则"),
        "tools": ("TOOLS.md", "工具备忘"),
        "agents": ("AGENTS.md", "执行规则"),
    }

    def __init__(self, workspace_path: Path | str):
        self._workspace_path = Path(workspace_path).expanduser().resolve()
        self._factory = DiaryEntryFactory()
        self._repository = DiaryRepository(self._workspace_path)
        self._matcher = DiarySimilarityMatcher()

    def search(self, query: str, limit: int = 20) -> list[dict[str, object]]:
        """按关键词搜索记忆内容。"""
        return self._repository.search(query=query, limit=limit)

    def get(self, relative_path: str, from_line: int = 1, lines: int = 50) -> dict[str, object]:
        """读取指定文件片段。"""
        return self._repository.read_slice(
            relative_path=relative_path,
            from_line=from_line,
            lines=lines,
        )

    def review_recent_entries(self, days: int = 3, limit: int = 8) -> list[dict[str, str | int]]:
        """读取最近几天的条目摘要。"""
        entries = self._repository.list_recent_entries(days=days, limit=limit)
        return [entry.to_review_dict() for entry in entries]

    def build_review_markdown(
        self,
        days: int = 3,
        limit: int = 6,
        max_chars: int = 1200,
    ) -> str:
        """构造注入 prompt 的近期摘要。"""
        lines: list[str] = []
        total_chars = 0

        for item in self.review_recent_entries(days=days, limit=limit):
            line = (
                f"- `{item['path']}`: {item['headline'][4:]} "
                f"(状态={item['status']}, 次数={item['count']})"
            )
            total_chars += len(line)
            if total_chars > max_chars:
                break
            lines.append(line)
        return "\n".join(lines)

    def log(
        self,
        kind: str,
        title: str,
        category: str | None = None,
        fields: list[tuple[str, str]] | None = None,
        promote_target: str | None = None,
    ) -> dict[str, object]:
        """向今日日记写入条目。"""
        preview_entry = self._factory.create(
            kind=kind,
            title=title,
            category=category,
            fields=fields or [],
        )
        related_entries = self._matcher.find_related(
            target_entry=preview_entry,
            candidates=self._repository.list_recent_entries(days=90, limit=200),
        )
        entry = self._factory.create(
            kind=kind,
            title=title,
            category=category,
            fields=fields or [],
            related_entries=related_entries,
        )
        path = self._repository.append_entry(entry)

        promoted = None
        target = promote_target or self._factory.infer_auto_promotion_target(entry)
        if target:
            promoted = self.promote(
                target=target,
                content=self._build_promotion_content(entry),
                title=entry.title,
                entry_id=entry.entry_id,
            )
            entry.set_status("promoted")
            entry.set_field("提升目标", target)

        return {
            "path": path,
            "entry_id": entry.entry_id,
            "entry": entry.to_markdown(),
            "status": entry.status,
            "count": entry.count,
            "related_entries": [item.to_review_dict() for item in related_entries],
            "promoted": promoted,
        }

    def promote(
        self,
        target: str,
        content: str,
        title: str | None = None,
        entry_id: str | None = None,
    ) -> dict[str, str]:
        """把稳定规则提升到长期文件。"""
        normalized_target = target.lower().strip()
        if normalized_target not in self._PROMOTION_TARGETS:
            raise ValueError(f"不支持的提升目标: {target}")

        filename, section_title = self._PROMOTION_TARGETS[normalized_target]
        bullet = f"- {title.strip()}：{content.strip()}" if title else f"- {content.strip()}"
        path = self._repository.append_to_memory_section(
            filename=filename,
            section_title=section_title,
            bullet=bullet,
        )

        if entry_id:
            self._repository.update_entry(
                entry_id=entry_id,
                updater=lambda entry: self._mark_promoted(entry, normalized_target),
            )
        return {"path": path, "content": bullet}

    def resolve_entry(self, entry_id: str, note: str) -> dict[str, object]:
        """把条目标记为已解决。"""
        entry = self._repository.update_entry(
            entry_id=entry_id,
            updater=lambda item: self._mark_resolved(item, note),
        )
        return entry.to_review_dict()

    def set_entry_status(self, entry_id: str, status: str, note: str | None = None) -> dict[str, object]:
        """更新条目状态。"""
        entry = self._repository.update_entry(
            entry_id=entry_id,
            updater=lambda item: self._mark_status(item, status, note),
        )
        return entry.to_review_dict()

    @staticmethod
    def _build_promotion_content(entry: DiaryEntry) -> str:
        """提炼提升内容。"""
        for key in ("提升内容", "行动", "经验", "修复", "详情"):
            value = entry.fields.get(key, "").strip()
            if value:
                return value
        return entry.title

    @staticmethod
    def _mark_promoted(entry: DiaryEntry, target: str) -> None:
        """更新提升状态。"""
        entry.set_status("promoted")
        entry.set_field("提升目标", target)

    @staticmethod
    def _mark_resolved(entry: DiaryEntry, note: str) -> None:
        """更新解决状态。"""
        entry.set_status("resolved")
        entry.set_field("已解决", note.strip())

    @staticmethod
    def _mark_status(entry: DiaryEntry, status: str, note: str | None) -> None:
        """更新通用状态。"""
        entry.set_status(status.strip())
        if note:
            entry.set_field("状态说明", note.strip())
