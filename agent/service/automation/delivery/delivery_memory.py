# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：delivery_memory.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation last-route memory。"""

from __future__ import annotations

from contextlib import asynccontextmanager
from typing import Any, Callable

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.automation_delivery_route_sql_repository import (
    AutomationDeliveryRouteSqlRepository,
)
from agent.service.automation.delivery.delivery_target import (
    DeliveryTarget,
    resolve_delivery_target,
)


class DeliveryMemory:
    """基于 Task 2 仓储的最后投递路由记忆。"""

    def __init__(self, session_factory: Callable[[], Any] | None = None) -> None:
        self._session_factory = session_factory

    async def get_last_route(self, agent_id: str) -> DeliveryTarget | None:
        """读取最后一次可复用的投递目标。"""
        async with self._session_scope() as session:
            repo = AutomationDeliveryRouteSqlRepository(session)
            row = await repo.get_latest_route(agent_id)
            if row is None or not row.enabled:
                return None
            if row.mode != "explicit":
                return None
            if not row.channel or not row.to:
                return None
            return resolve_delivery_target(
                {
                    "mode": "explicit",
                    "channel": row.channel,
                    "to": row.to,
                    "account_id": row.account_id,
                    "thread_id": row.thread_id,
                }
            )

    async def remember_route(
        self,
        *,
        agent_id: str,
        channel: str,
        to: str,
        account_id: str | None = None,
        thread_id: str | None = None,
    ) -> DeliveryTarget:
        """刷新最后一次成功送达的目标。"""
        target = resolve_delivery_target(
            {
                "mode": "explicit",
                "channel": channel,
                "to": to,
                "account_id": account_id,
                "thread_id": thread_id,
            }
        )
        async with self._session_scope() as session:
            repo = AutomationDeliveryRouteSqlRepository(session)
            latest = await repo.get_latest_route(agent_id)
            row = await repo.upsert_route(
                route_id=latest.route_id if latest is not None else None,
                agent_id=agent_id,
                mode="explicit",
                channel=target.channel,
                to=target.to,
                account_id=target.account_id,
                thread_id=target.thread_id,
                enabled=True,
            )
            await session.commit()
            return resolve_delivery_target(
                {
                    "mode": "explicit",
                    "channel": row.channel,
                    "to": row.to,
                    "account_id": row.account_id,
                    "thread_id": row.thread_id,
                }
            )

    @asynccontextmanager
    async def _session_scope(self):
        """统一兼容测试注入与默认数据库会话。"""
        session_factory = self._session_factory
        if session_factory is None:
            session_factory = get_db("async_sqlite").session
        async with session_factory() as session:
            yield session
