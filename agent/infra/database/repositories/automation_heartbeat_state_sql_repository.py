# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_heartbeat_state_sql_repository.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation heartbeat state SQL 仓储。"""

from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.dialects.sqlite import insert as sqlite_insert

from agent.infra.database.models.automation_heartbeat_state import AutomationHeartbeatState
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.utils.utils import random_uuid


class AutomationHeartbeatStateSqlRepository(BaseSqlRepository):
    """Agent 心跳状态的 CRUD 仓储。"""

    _UPDATABLE_FIELDS = {
        "enabled",
        "every_seconds",
        "target_mode",
        "ack_max_chars",
        "last_heartbeat_at",
        "last_ack_at",
        "created_at",
        "updated_at",
    }

    async def get_state(self, agent_id: str) -> AutomationHeartbeatState | None:
        """按 agent_id 读取心跳状态。"""
        stmt = select(AutomationHeartbeatState).where(
            AutomationHeartbeatState.agent_id == agent_id
        )
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def upsert_state(self, agent_id: str, **fields) -> AutomationHeartbeatState:
        """插入或更新心跳状态。"""
        self._reject_unknown_fields(fields)
        defaults = {
            "enabled": False,
            "every_seconds": 1800,
            "target_mode": "none",
            "ack_max_chars": 300,
            "last_heartbeat_at": None,
            "last_ack_at": None,
        }
        create_payload = {**defaults, **fields}
        insert_stmt = sqlite_insert(AutomationHeartbeatState).values(
            state_id=random_uuid(),
            agent_id=agent_id,
            **create_payload,
        )
        if fields:
            stmt = insert_stmt.on_conflict_do_update(
                index_elements=[AutomationHeartbeatState.agent_id],
                set_={field_name: insert_stmt.excluded[field_name] for field_name in fields},
            )
        else:
            stmt = insert_stmt.on_conflict_do_nothing(
                index_elements=[AutomationHeartbeatState.agent_id]
            )
        await self._session.execute(stmt)
        await self.flush()
        entity = await self.get_state(agent_id)
        if entity is None:
            raise RuntimeError("heartbeat state upsert did not persist a row")
        return entity

    def _reject_unknown_fields(self, fields: dict[str, object]) -> None:
        """阻止把未知字段挂到 ORM 实体上。"""
        unexpected = set(fields) - self._UPDATABLE_FIELDS
        if unexpected:
            raise ValueError(f"unknown heartbeat state fields: {sorted(unexpected)}")
