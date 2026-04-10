# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_system_event_sql_repository.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation system event SQL 仓储。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import select

from agent.infra.database.models.automation_system_event import AutomationSystemEvent
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.utils.utils import random_uuid


class AutomationSystemEventSqlRepository(BaseSqlRepository):
    """系统事件队列的 CRUD 仓储。"""

    async def create_event(
        self,
        *,
        event_type: str,
        event_id: str | None = None,
        source_type: str | None = None,
        source_id: str | None = None,
        payload: dict[str, object] | None = None,
        status: str = "new",
        processed_at: datetime | None = None,
    ) -> AutomationSystemEvent:
        """创建系统事件。"""
        entity = AutomationSystemEvent(
            event_id=event_id or random_uuid(),
            event_type=event_type,
            source_type=source_type,
            source_id=source_id,
            payload=payload or {},
            status=status,
            processed_at=processed_at,
        )
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return entity

    async def list_pending_events(self) -> list[AutomationSystemEvent]:
        """列出尚未终态处理的事件。"""
        stmt = (
            select(AutomationSystemEvent)
            .where(AutomationSystemEvent.status == "new")
            .order_by(
                AutomationSystemEvent.created_at.asc(),
                AutomationSystemEvent.event_id.asc(),
            )
        )
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def mark_processed(
        self,
        event_id: str,
        processed_at: datetime | None = None,
    ) -> AutomationSystemEvent | None:
        """标记事件已处理。"""
        return await self._mark_status(event_id, "processed", processed_at)

    async def mark_failed(
        self,
        event_id: str,
        processed_at: datetime | None = None,
    ) -> AutomationSystemEvent | None:
        """标记事件处理失败。"""
        return await self._mark_status(event_id, "failed", processed_at)

    async def mark_processing(self, event_id: str) -> AutomationSystemEvent | None:
        """标记事件正在处理中。"""
        entity = await self._session.get(AutomationSystemEvent, event_id)
        if entity is None:
            return None
        entity.status = "processing"
        entity.processed_at = None
        await self.flush()
        await self.refresh(entity)
        return entity

    async def _mark_status(
        self,
        event_id: str,
        status: str,
        processed_at: datetime | None,
    ) -> AutomationSystemEvent | None:
        entity = await self._session.get(AutomationSystemEvent, event_id)
        if entity is None:
            return None
        entity.status = status
        entity.processed_at = processed_at or datetime.now()
        await self.flush()
        await self.refresh(entity)
        return entity
