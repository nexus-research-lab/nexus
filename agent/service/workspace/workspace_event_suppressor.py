# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_event_suppressor.py
# @Date   ：2026/04/14 21:14
# @Author ：leemysw
# 2026/04/14 21:14   Create
# =====================================================

"""Workspace 事件抑制器。"""

from time import monotonic
from typing import Any


class WorkspaceEventSuppressor:
    """抑制领域层写入引发的重复文件系统事件。"""

    DEFAULT_TTL_SECONDS = 6.0

    def __init__(self):
        self._recent_writes: dict[tuple[str, str], dict[str, Any]] = {}

    def register_write(
            self,
            agent_id: str,
            path: str,
            content_snapshot: str | None,
            ttl_seconds: float | None = None,
    ) -> None:
        """登记最近一次由领域层主动触发的写入。"""
        expires_at = monotonic() + (ttl_seconds or self.DEFAULT_TTL_SECONDS)
        self._recent_writes[(agent_id, path)] = {
            "content_snapshot": content_snapshot,
            "expires_at": expires_at,
        }

    def should_suppress_write(
            self,
            agent_id: str,
            path: str,
            content_snapshot: str | None,
    ) -> bool:
        """判断文件系统写入事件是否应被抑制。"""
        now = monotonic()
        self._purge_expired(now)

        state = self._recent_writes.get((agent_id, path))
        if not state:
            return False

        expected_snapshot = state["content_snapshot"]
        if expected_snapshot is None or content_snapshot is None:
            return True
        return expected_snapshot == content_snapshot

    def _purge_expired(self, now: float) -> None:
        """清理已过期记录。"""
        expired_keys = [
            key for key, value in self._recent_writes.items()
            if value["expires_at"] <= now
        ]
        for key in expired_keys:
            self._recent_writes.pop(key, None)


workspace_event_suppressor = WorkspaceEventSuppressor()
