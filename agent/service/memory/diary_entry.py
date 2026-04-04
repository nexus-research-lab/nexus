# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：diary_entry.py
# @Date   ：2026/04/04 13:16
# @Author ：leemysw
# 2026/04/04 13:16   Create
# =====================================================

"""日记条目对象。"""

from __future__ import annotations

from collections import OrderedDict
from datetime import datetime


class DiaryEntry:
    """表示单条日记条目。"""

    def __init__(
        self,
        entry_id: str,
        created_at: datetime,
        kind: str,
        title: str,
        category: str | None = None,
        fields: OrderedDict[str, str] | None = None,
        path: str | None = None,
    ) -> None:
        self.entry_id = entry_id
        self.created_at = created_at
        self.kind = kind
        self.title = title
        self.category = category
        self.fields = fields or OrderedDict()
        self.path = path

    @property
    def headline(self) -> str:
        """返回条目标题行。"""
        timestamp = self.created_at.strftime("%Y-%m-%d %H:%M")
        if self.kind == "LRN" and self.category:
            return f"### {timestamp} - [{self.kind}] {self.category}: {self.title}"
        return f"### {timestamp} - [{self.kind}] {self.title}"

    @property
    def status(self) -> str:
        """读取当前状态。"""
        return self.fields.get("状态", "pending")

    @property
    def count(self) -> int:
        """读取当前累计次数。"""
        raw_count = self.fields.get("次数", "1").strip()
        return int(raw_count) if raw_count.isdigit() else 1

    @property
    def related_ids(self) -> list[str]:
        """读取关联条目 ID。"""
        raw_value = self.fields.get("关联", "")
        return [item.strip() for item in raw_value.split(",") if item.strip()]

    def set_field(self, key: str, value: str) -> None:
        """写入单个字段。"""
        self.fields[key] = value.strip()

    def set_status(self, status: str) -> None:
        """更新状态字段。"""
        self.fields["状态"] = status.strip()

    def set_count(self, count: int) -> None:
        """更新次数字段。"""
        self.fields["次数"] = str(max(count, 1))

    def set_related_ids(self, related_ids: list[str]) -> None:
        """更新关联条目列表。"""
        self.fields["关联"] = ", ".join(related_ids)

    def to_markdown(self) -> str:
        """渲染为 markdown。"""
        lines = [self.headline]
        ordered_fields = OrderedDict()
        ordered_fields["ID"] = self.entry_id
        for key, value in self.fields.items():
            if key == "ID":
                continue
            ordered_fields[key] = value

        for key, value in ordered_fields.items():
            if not value:
                continue
            lines.append(f"*   **{key}**: {value}")
        return "\n".join(lines)

    def to_review_dict(self) -> dict[str, str | int]:
        """返回用于回顾展示的数据。"""
        return {
            "entry_id": self.entry_id,
            "path": self.path or "",
            "headline": self.headline,
            "kind": self.kind,
            "status": self.status,
            "count": self.count,
        }
