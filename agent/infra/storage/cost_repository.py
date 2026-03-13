# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cost_repository.py
# @Date   ：2026/03/10 23:40
# @Author ：leemysw
# 2026/03/10 23:40   Create
# =====================================================

"""
成本账本仓库

[INPUT]: 依赖文件存储层、Agent Repository、Session Repository
[OUTPUT]: 对外提供 result 成本落账、汇总读取与重建能力
[POS]: db 模块的成本数据访问层，被 session_store / API 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from __future__ import annotations

import uuid
from dataclasses import asdict, is_dataclass
from datetime import datetime, timezone
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional

from agent.infra.storage.agent_repository import agent_repository
from agent.infra.storage.config_store import ConfigStore
from agent.infra.storage.jsonl_store import JsonlStore
from agent.infra.storage.storage_paths import FileStoragePaths
from agent.infra.storage.session_repository import session_repository
from agent.schema.model_cost import AgentCostSummary, CostLedgerEntry, SessionCostSummary
from agent.schema.model_message import AMessage
from agent.utils.logger import logger


class CostRepository:
    """基于 workspace 文件的成本账本仓库。"""

    def __init__(self) -> None:
        self._paths = FileStoragePaths()
        self._lock = Lock()
        self._paths.ensure_directories()

    @staticmethod
    def _default_session_summary(agent_id: str, session_key: str, session_id: str = "") -> SessionCostSummary:
        """构造空的 Session 成本汇总。"""
        return SessionCostSummary(
            agent_id=agent_id,
            session_key=session_key,
            session_id=session_id,
        )

    @staticmethod
    def _default_agent_summary(agent_id: str) -> AgentCostSummary:
        """构造空的 Agent 成本汇总。"""
        return AgentCostSummary(agent_id=agent_id)

    @staticmethod
    def _normalize_result_payload(payload: Any) -> Dict[str, Any]:
        """规范化 result payload。"""
        if payload is None:
            return {}
        if isinstance(payload, dict):
            return dict(payload)
        if is_dataclass(payload):
            return asdict(payload)
        if hasattr(payload, "model_dump"):
            return payload.model_dump(mode="json")
        if hasattr(payload, "__dict__"):
            return dict(payload.__dict__)
        return {}

    @staticmethod
    def _to_int(value: Any) -> int:
        """安全转换整数。"""
        try:
            return int(value or 0)
        except (TypeError, ValueError):
            return 0

    @staticmethod
    def _to_float(value: Any) -> float:
        """安全转换浮点数。"""
        try:
            return float(value or 0)
        except (TypeError, ValueError):
            return 0.0

    async def _resolve_workspace_path(self, agent_id: str) -> Path:
        """按 Agent ID 解析 workspace 路径。"""
        agent = await agent_repository.get_agent(agent_id)
        if agent and agent.workspace_path:
            workspace_path = Path(agent.workspace_path).expanduser()
        else:
            workspace_path = self._paths.workspace_base / agent_id
        self._paths.migrate_workspace_runtime_layout(workspace_path)
        return workspace_path

    async def _resolve_session_dir(self, session_key: str, agent_id: Optional[str] = None) -> Optional[Path]:
        """解析 session 目录。"""
        session = await session_repository.get_session(session_key)
        resolved_agent_id = agent_id or (session.agent_id if session else None) or "main"
        workspace_path = await self._resolve_workspace_path(resolved_agent_id)
        session_dir = self._paths.get_session_dir(workspace_path, session_key)
        if session_dir.exists():
            return session_dir
        return session_dir if session else None

    async def _read_cost_rows(self, session_key: str, agent_id: Optional[str] = None) -> List[Dict[str, Any]]:
        """读取会话的成本账本行。"""
        session_dir = await self._resolve_session_dir(session_key, agent_id=agent_id)
        if not session_dir:
            return []
        return JsonlStore.read(session_dir / "telemetry_cost.jsonl")

    async def _write_session_summary(
        self,
        session_key: str,
        summary: SessionCostSummary,
        agent_id: Optional[str] = None,
    ) -> None:
        """写入 Session 成本汇总。"""
        session_dir = await self._resolve_session_dir(session_key, agent_id=agent_id)
        if not session_dir:
            return
        ConfigStore.write(session_dir / "telemetry_cost_summary.json", summary.model_dump(mode="json"))

    async def _read_session_summary(self, session_key: str, agent_id: Optional[str] = None) -> Optional[SessionCostSummary]:
        """读取 Session 成本汇总。"""
        session_dir = await self._resolve_session_dir(session_key, agent_id=agent_id)
        if not session_dir:
            return None
        summary_path = session_dir / "telemetry_cost_summary.json"
        payload = ConfigStore.read(summary_path, None)
        if not payload:
            return None
        return SessionCostSummary(**payload)

    async def _write_agent_summary(self, agent_id: str, summary: AgentCostSummary) -> None:
        """写入 Agent 成本汇总。"""
        workspace_path = await self._resolve_workspace_path(agent_id)
        ConfigStore.write(
            self._paths.get_agent_cost_summary_path(workspace_path),
            summary.model_dump(mode="json"),
        )

    async def _read_agent_summary(self, agent_id: str) -> Optional[AgentCostSummary]:
        """读取 Agent 成本汇总。"""
        workspace_path = await self._resolve_workspace_path(agent_id)
        payload = ConfigStore.read(self._paths.get_agent_cost_summary_path(workspace_path), None)
        if not payload:
            return None
        return AgentCostSummary(**payload)

    async def _backfill_session_from_messages(
        self,
        session_key: str,
        agent_id: Optional[str] = None,
    ) -> SessionCostSummary:
        """从历史 messages.jsonl 回填成本账本。"""
        session = await session_repository.get_session(session_key)
        resolved_agent_id = agent_id or (session.agent_id if session else None) or "main"
        workspace_path = await self._resolve_workspace_path(resolved_agent_id)
        session_dir = await self._resolve_session_dir(session_key, agent_id=resolved_agent_id)
        if not session_dir:
            return self._default_session_summary(resolved_agent_id, session_key)

        result_entries: List[Dict[str, Any]] = []
        messages = await session_repository.get_session_messages(session_key)
        for message in messages:
            if message.message_type != "result":
                continue

            normalized_message = message.model_copy(
                update={
                    "agent_id": resolved_agent_id,
                    "session_id": message.session_id or (session.session_id if session else ""),
                }
            )
            entry = self._build_cost_entry(normalized_message)
            result_entries.append(entry.model_dump(mode="json"))

        JsonlStore.write(self._paths.get_session_cost_log_path(workspace_path, session_key), result_entries)
        summary = await self.rebuild_session_summary(session_key, agent_id=resolved_agent_id)
        await self.rebuild_agent_summary(resolved_agent_id)
        return summary

    async def _backfill_agent_from_messages(self, agent_id: str) -> AgentCostSummary:
        """从 Agent 下的历史会话消息回填成本汇总。"""
        sessions = await session_repository.get_all_sessions()
        for session in sessions:
            if session.agent_id != agent_id:
                continue
            await self._backfill_session_from_messages(session.session_key, agent_id=agent_id)
        return await self.rebuild_agent_summary(agent_id)

    def _build_cost_entry(self, message: AMessage) -> CostLedgerEntry:
        """将 result 消息转换为成本账本条目。"""
        payload = self._normalize_result_payload(message.message)
        usage = payload.get("usage") or {}

        return CostLedgerEntry(
            entry_id=uuid.uuid4().hex,
            agent_id=message.agent_id,
            session_key=message.session_key,
            session_id=message.session_id or "",
            round_id=message.round_id,
            message_id=message.message_id,
            subtype=str(payload.get("subtype") or "success"),
            input_tokens=self._to_int(usage.get("input_tokens")),
            output_tokens=self._to_int(usage.get("output_tokens")),
            cache_creation_input_tokens=self._to_int(
                usage.get("cache_creation_input_tokens") or usage.get("cache_creation_tokens")
            ),
            cache_read_input_tokens=self._to_int(usage.get("cache_read_input_tokens")),
            total_cost_usd=self._to_float(payload.get("total_cost_usd")),
            duration_ms=self._to_int(payload.get("duration_ms")),
            duration_api_ms=self._to_int(payload.get("duration_api_ms")),
            num_turns=self._to_int(payload.get("num_turns")),
            created_at=message.timestamp or datetime.now(timezone.utc),
        )

    @staticmethod
    def _build_session_summary(agent_id: str, session_key: str, rows: List[Dict[str, Any]]) -> SessionCostSummary:
        """从账本行构建 Session 汇总。"""
        if not rows:
            return SessionCostSummary(agent_id=agent_id, session_key=session_key)

        total_input_tokens = sum(int(row.get("input_tokens") or 0) for row in rows)
        total_output_tokens = sum(int(row.get("output_tokens") or 0) for row in rows)
        total_cache_creation = sum(int(row.get("cache_creation_input_tokens") or 0) for row in rows)
        total_cache_read = sum(int(row.get("cache_read_input_tokens") or 0) for row in rows)
        total_cost = sum(float(row.get("total_cost_usd") or 0) for row in rows)
        error_rounds = sum(1 for row in rows if str(row.get("subtype") or "success") != "success")
        latest_row = rows[-1]

        return SessionCostSummary(
            agent_id=agent_id,
            session_key=session_key,
            session_id=str(latest_row.get("session_id") or ""),
            total_input_tokens=total_input_tokens,
            total_output_tokens=total_output_tokens,
            total_tokens=total_input_tokens + total_output_tokens,
            total_cache_creation_input_tokens=total_cache_creation,
            total_cache_read_input_tokens=total_cache_read,
            total_cost_usd=total_cost,
            completed_rounds=len(rows),
            error_rounds=error_rounds,
            last_round_id=latest_row.get("round_id"),
            last_run_duration_ms=int(latest_row.get("duration_ms") or 0),
            last_run_cost_usd=float(latest_row.get("total_cost_usd") or 0),
            updated_at=datetime.now(timezone.utc),
        )

    async def rebuild_session_summary(self, session_key: str, agent_id: Optional[str] = None) -> SessionCostSummary:
        """重建并持久化 Session 成本汇总。"""
        session = await session_repository.get_session(session_key)
        resolved_agent_id = agent_id or (session.agent_id if session else None) or "main"
        rows = await self._read_cost_rows(session_key, agent_id=resolved_agent_id)
        summary = self._build_session_summary(resolved_agent_id, session_key, rows)
        summary.session_id = session.session_id or summary.session_id or ""
        await self._write_session_summary(session_key, summary, agent_id=resolved_agent_id)
        return summary

    async def rebuild_agent_summary(self, agent_id: str) -> AgentCostSummary:
        """按 Agent 下的所有 Session 汇总重建成本摘要。"""
        workspace_path = await self._resolve_workspace_path(agent_id)
        summary_paths = list(
            self._paths.get_runtime_dir(workspace_path).glob("sessions/*/telemetry_cost_summary.json")
        )

        session_summaries: List[SessionCostSummary] = []
        for summary_path in summary_paths:
            payload = ConfigStore.read(summary_path, None)
            if not payload:
                continue
            try:
                session_summaries.append(SessionCostSummary(**payload))
            except Exception as exc:
                logger.warning(f"⚠️ 跳过损坏的成本汇总: path={summary_path}, error={exc}")

        agent_summary = AgentCostSummary(
            agent_id=agent_id,
            total_input_tokens=sum(item.total_input_tokens for item in session_summaries),
            total_output_tokens=sum(item.total_output_tokens for item in session_summaries),
            total_tokens=sum(item.total_tokens for item in session_summaries),
            total_cache_creation_input_tokens=sum(
                item.total_cache_creation_input_tokens for item in session_summaries
            ),
            total_cache_read_input_tokens=sum(
                item.total_cache_read_input_tokens for item in session_summaries
            ),
            total_cost_usd=sum(item.total_cost_usd for item in session_summaries),
            completed_rounds=sum(item.completed_rounds for item in session_summaries),
            error_rounds=sum(item.error_rounds for item in session_summaries),
            cost_sessions=len(session_summaries),
            updated_at=datetime.now(timezone.utc),
        )
        await self._write_agent_summary(agent_id, agent_summary)
        return agent_summary

    async def record_result_message(self, message: AMessage) -> bool:
        """对 result 消息落账并刷新汇总。"""
        if message.message_type != "result":
            return False

        workspace_path = await self._resolve_workspace_path(message.agent_id)
        log_path = self._paths.get_session_cost_log_path(workspace_path, message.session_key)
        entry = self._build_cost_entry(message)

        with self._lock:
            JsonlStore.append(log_path, entry.model_dump(mode="json"))

        await self.rebuild_session_summary(message.session_key, agent_id=message.agent_id)
        await self.rebuild_agent_summary(message.agent_id)
        return True

    async def delete_round_costs(self, session_key: str, round_id: str, agent_id: Optional[str] = None) -> int:
        """删除指定轮次的成本账本并重建汇总。"""
        session = await session_repository.get_session(session_key)
        resolved_agent_id = agent_id or (session.agent_id if session else None) or "main"
        session_dir = await self._resolve_session_dir(session_key, agent_id=resolved_agent_id)
        if not session_dir:
            return 0

        log_path = session_dir / "telemetry_cost.jsonl"
        rows = JsonlStore.read(log_path)
        remaining_rows = [row for row in rows if row.get("round_id") != round_id]
        deleted_count = len(rows) - len(remaining_rows)

        with self._lock:
            JsonlStore.write(log_path, remaining_rows)

        await self.rebuild_session_summary(session_key, agent_id=resolved_agent_id)
        await self.rebuild_agent_summary(resolved_agent_id)
        return deleted_count

    async def handle_session_deleted(self, session_key: str, agent_id: str) -> None:
        """会话删除后的 Agent 汇总刷新。"""
        await self.rebuild_agent_summary(agent_id)

    async def get_session_cost_summary(self, session_key: str) -> SessionCostSummary:
        """获取 Session 成本汇总。"""
        session = await session_repository.get_session(session_key)
        agent_id = session.agent_id if session else "main"
        summary = await self._read_session_summary(session_key, agent_id=agent_id)
        rows = await self._read_cost_rows(session_key, agent_id=agent_id)
        if summary and summary.completed_rounds > 0 and summary.total_tokens == 0 and summary.total_cost_usd == 0:
            return await self._backfill_session_from_messages(session_key, agent_id=agent_id)
        if summary and (summary.completed_rounds > 0 or rows):
            return summary
        if rows:
            return await self.rebuild_session_summary(session_key, agent_id=agent_id)
        return await self._backfill_session_from_messages(session_key, agent_id=agent_id)

    async def get_agent_cost_summary(self, agent_id: str) -> AgentCostSummary:
        """获取 Agent 成本汇总。"""
        summary = await self._read_agent_summary(agent_id)
        if summary and summary.completed_rounds > 0 and summary.total_tokens == 0 and summary.total_cost_usd == 0:
            return await self._backfill_agent_from_messages(agent_id)
        if summary and (summary.completed_rounds > 0 or summary.total_tokens > 0 or summary.total_cost_usd > 0):
            return summary
        return await self._backfill_agent_from_messages(agent_id)


cost_repository = CostRepository()
