# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：automation_delivery_route_sql_repository.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation delivery route SQL 仓储。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import select

from agent.infra.database.models.automation_delivery_route import AutomationDeliveryRoute
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.utils.utils import random_uuid


class AutomationDeliveryRouteSqlRepository(BaseSqlRepository):
    """消息投递路由的 CRUD 仓储。"""

    _UPDATABLE_FIELDS = {
        "mode",
        "channel",
        "to",
        "account_id",
        "thread_id",
        "enabled",
    }

    async def get_latest_route(self, agent_id: str) -> AutomationDeliveryRoute | None:
        """读取指定 agent 的最新路由配置。"""
        stmt = (
            select(AutomationDeliveryRoute)
            .where(AutomationDeliveryRoute.agent_id == agent_id)
            .order_by(
                AutomationDeliveryRoute.updated_at.desc(),
                AutomationDeliveryRoute.route_id.desc(),
            )
            .limit(1)
        )
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def upsert_route(
        self,
        *,
        agent_id: str,
        route_id: str | None = None,
        **fields,
    ) -> AutomationDeliveryRoute:
        """插入或更新路由配置。"""
        self._reject_unknown_fields(fields)
        defaults = {
            "mode": "none",
            "channel": None,
            "to": None,
            "account_id": None,
            "thread_id": None,
            "enabled": True,
        }
        payload = {**defaults, **fields}
        entity = None
        if route_id is not None:
            entity = await self._session.get(AutomationDeliveryRoute, route_id)
        if entity is None:
            entity = AutomationDeliveryRoute(
                route_id=route_id or random_uuid(),
                agent_id=agent_id,
                updated_at=datetime.now(),
                **payload,
            )
            self._session.add(entity)
        else:
            entity.agent_id = agent_id
            # 保持最新路由覆盖显式传入的字段，不碰未传入的旧值。
            for field_name, value in fields.items():
                setattr(entity, field_name, value)
            entity.updated_at = datetime.now()
        await self.flush()
        await self.refresh(entity)
        return entity

    def _reject_unknown_fields(self, fields: dict[str, object]) -> None:
        """阻止把未知字段挂到 ORM 实体上。"""
        unexpected = set(fields) - self._UPDATABLE_FIELDS
        if unexpected:
            raise ValueError(f"unknown delivery route fields: {sorted(unexpected)}")
