# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：round_snapshot_normalizer.py
# @Date   ：2026/04/15 16:18
# @Author ：leemysw
# 2026/04/15 16:18   Create
# =====================================================

"""轮次快照归一化器。"""

from __future__ import annotations

from typing import Any


class RoundSnapshotNormalizer:
    """基于 durable round 状态修正历史消息快照。"""

    _TERMINAL_ASSISTANT_STATUS = {"done", "cancelled", "error"}

    def build_round_status(self, message_rows: list[dict[str, Any]]) -> dict[str, str]:
        """从消息快照构建 round 状态表。"""
        status_map: dict[str, str] = {}
        for row in message_rows:
            round_id = str(row.get("round_id") or "").strip()
            if not round_id:
                continue
            status_map.setdefault(round_id, "running")
            if row.get("role") == "result":
                status_map[round_id] = self.normalize_round_status_value(
                    row.get("subtype") or "success"
                )
        return status_map

    def normalize_assistant_rows(
        self,
        message_rows: list[dict[str, Any]],
        round_status_map: dict[str, str],
    ) -> list[dict[str, Any]]:
        """按 round 终态把 assistant 历史快照统一修正到非 live 语义。"""
        normalized_rows: list[dict[str, Any]] = []
        for row in message_rows:
            if row.get("role") != "assistant":
                normalized_rows.append(dict(row))
                continue

            round_id = str(row.get("round_id") or "").strip()
            assistant_status = self.resolve_assistant_status(
                round_status_map.get(round_id)
            )
            if not assistant_status:
                normalized_rows.append(dict(row))
                continue

            normalized_row = dict(row)
            normalized_row["is_complete"] = True

            current_stream_status = str(
                normalized_row.get("stream_status") or ""
            ).strip()
            if current_stream_status not in self._TERMINAL_ASSISTANT_STATUS:
                normalized_row["stream_status"] = assistant_status
            normalized_rows.append(normalized_row)

        return normalized_rows

    @staticmethod
    def normalize_round_status_value(status: Any) -> str:
        """把历史 round 状态统一压成稳定枚举。"""
        normalized_status = str(status or "").strip().lower()
        if not normalized_status or normalized_status == "running":
            return "running"
        if normalized_status in {"interrupted", "cancelled"}:
            return "interrupted"
        if normalized_status == "finished":
            return "success"
        if normalized_status.startswith("error") or normalized_status == "error":
            return "error"
        return "success"

    def resolve_assistant_status(self, round_status: str | None) -> str | None:
        """把 round 终态映射成 assistant 历史消息的终态。"""
        normalized_round_status = self.normalize_round_status_value(round_status)
        if normalized_round_status == "running":
            return None
        if normalized_round_status == "interrupted":
            return "cancelled"
        if normalized_round_status == "error":
            return "error"
        return "done"
