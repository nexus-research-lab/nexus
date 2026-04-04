# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：diary_entry_factory.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""日记条目工厂。"""

from __future__ import annotations

from collections import OrderedDict
from datetime import datetime

from agent.service.memory.diary_entry import DiaryEntry
from agent.utils.snowflake import worker


class DiaryEntryFactory:
    """负责生成标准化日记条目。"""

    _DEFAULT_FIELDS = {
        "LRN": (
            ("优先级", "medium"),
            ("领域", "general"),
            ("详情", ""),
            ("行动", ""),
            ("来源", "conversation"),
            ("次数", "1"),
            ("标签", ""),
            ("关联", ""),
            ("状态", "pending"),
        ),
        "ERR": (
            ("优先级", "high"),
            ("领域", "general"),
            ("错误", ""),
            ("上下文", ""),
            ("修复", ""),
            ("可复现", "unknown"),
            ("次数", "1"),
            ("标签", ""),
            ("关联", ""),
            ("状态", "pending"),
        ),
        "FEAT": (
            ("优先级", "medium"),
            ("领域", "general"),
            ("需求", ""),
            ("用户背景", ""),
            ("复杂度", "medium"),
            ("实现", ""),
            ("频率", "first_time"),
            ("状态", "pending"),
        ),
        "REF": (
            ("做了什么", ""),
            ("结果", "success"),
            ("反思", ""),
            ("经验", ""),
            ("状态", "pending"),
        ),
    }

    _CONFIRMATION_CATEGORIES = {"correction", "knowledge_gap", "best_practice"}

    def create(
        self,
        kind: str,
        title: str,
        category: str | None = None,
        fields: list[tuple[str, str]] | None = None,
        related_entries: list[DiaryEntry] | None = None,
        now: datetime | None = None,
    ) -> DiaryEntry:
        """创建新条目。"""
        created_at = now or datetime.now()
        normalized_kind = kind.upper().strip()
        if normalized_kind not in self._DEFAULT_FIELDS:
            raise ValueError(f"不支持的日记类型: {kind}")

        merged_fields = OrderedDict(self._DEFAULT_FIELDS[normalized_kind])
        for key, value in fields or []:
            clean_key = key.strip()
            if clean_key:
                merged_fields[clean_key] = value.strip()

        entry = DiaryEntry(
            entry_id=self._build_entry_id(normalized_kind, created_at),
            created_at=created_at,
            kind=normalized_kind,
            title=title.strip(),
            category=(category or "").strip() or None,
            fields=merged_fields,
        )
        self._apply_related_context(entry, related_entries or [])
        return entry

    @staticmethod
    def infer_auto_promotion_target(entry: DiaryEntry) -> str | None:
        """推断是否需要立即提升。"""
        if entry.kind == "LRN" and entry.category == "preference":
            return "soul"
        return None

    def _apply_related_context(self, entry: DiaryEntry, related_entries: list[DiaryEntry]) -> None:
        """根据相似历史补齐次数、关联和状态。"""
        if not related_entries:
            return

        entry.set_related_ids([item.entry_id for item in related_entries[:5]])
        if "次数" in entry.fields:
            entry.set_count(max(item.count for item in related_entries) + 1)

        # 中文注释：纠正类学习达到 3 次后，不直接替用户做长期固化，
        # 而是把状态推到 needs_confirmation，提示后续显式确认。
        if (
            entry.kind == "LRN"
            and entry.category in self._CONFIRMATION_CATEGORIES
            and entry.count >= 3
            and entry.status == "pending"
        ):
            entry.set_status("needs_confirmation")

    @staticmethod
    def _build_entry_id(kind: str, created_at: datetime) -> str:
        """生成稳定条目 ID。"""
        return f"{kind}-{created_at.strftime('%Y%m%d-%H%M%S')}-{worker.get_id()}"
