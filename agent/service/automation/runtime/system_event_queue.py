# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：system_event_queue.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation system event 队列服务。"""

from __future__ import annotations

from datetime import datetime

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.automation_system_event_sql_repository import (
    AutomationSystemEventSqlRepository,
)


class SystemEventQueue:
    """薄封装的系统事件持久化队列。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def enqueue(self, **fields):
        """追加一个新的系统事件。"""
        async with self._db.session() as session:
            repo = AutomationSystemEventSqlRepository(session)
            row = await repo.create_event(**fields)
            await session.commit()
            return row

    async def list_pending_events(self):
        """按创建顺序列出待处理事件。"""
        async with self._db.session() as session:
            repo = AutomationSystemEventSqlRepository(session)
            return await repo.list_pending_events()

    async def mark_processing(self, event_id: str):
        """把事件标记为处理中。"""
        async with self._db.session() as session:
            repo = AutomationSystemEventSqlRepository(session)
            row = await repo.mark_processing(event_id)
            await session.commit()
            return row

    async def mark_processed(
        self,
        event_id: str,
        processed_at: datetime | None = None,
    ):
        """把事件标记为已处理。"""
        async with self._db.session() as session:
            repo = AutomationSystemEventSqlRepository(session)
            row = await repo.mark_processed(event_id, processed_at=processed_at)
            await session.commit()
            return row

    async def mark_failed(
        self,
        event_id: str,
        processed_at: datetime | None = None,
    ):
        """把事件标记为失败。"""
        async with self._db.session() as session:
            repo = AutomationSystemEventSqlRepository(session)
            row = await repo.mark_failed(event_id, processed_at=processed_at)
            await session.commit()
            return row
