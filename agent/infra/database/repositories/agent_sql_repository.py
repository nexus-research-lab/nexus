# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Agent SQL 仓储。"""

from __future__ import annotations

from typing import Optional

from sqlalchemy import select
from sqlalchemy.orm import selectinload

from agent.infra.database.models.agent import Agent
from agent.infra.database.models.profile import Profile
from agent.infra.database.models.runtime import Runtime
from agent.schema.model_agent_persistence import (
    AgentAggregate,
    AgentRecord,
    CreateAgentAggregate,
    ProfileRecord,
    RuntimeRecord,
)
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository


class AgentSqlRepository(BaseSqlRepository):
    """Agent 聚合 SQL 仓储。"""

    async def create(self, payload: CreateAgentAggregate) -> AgentAggregate:
        """创建 Agent 聚合。"""
        agent = Agent(**payload.agent.model_dump(exclude={"created_at", "updated_at"}))
        profile = Profile(**payload.profile.model_dump(exclude={"created_at", "updated_at"}))
        runtime = Runtime(**payload.runtime.model_dump(exclude={"created_at", "updated_at"}))
        self._session.add(agent)
        self._session.add(profile)
        self._session.add(runtime)
        await self.flush()
        return await self.get(payload.agent.id) or AgentAggregate(
            agent=payload.agent,
            profile=payload.profile,
            runtime=payload.runtime,
        )

    async def get(self, agent_id: str) -> Optional[AgentAggregate]:
        """按 ID 获取 Agent 聚合。"""
        stmt = (
            select(Agent)
            .options(selectinload(Agent.profile), selectinload(Agent.runtime))
            .where(Agent.id == agent_id)
        )
        result = await self._session.execute(stmt)
        agent = result.scalar_one_or_none()
        if agent is None or agent.profile is None or agent.runtime is None:
            return None
        return self._build_aggregate(agent)

    async def list_active(self) -> list[AgentAggregate]:
        """列出所有活跃 Agent。"""
        stmt = (
            select(Agent)
            .options(selectinload(Agent.profile), selectinload(Agent.runtime))
            .where(Agent.status == "active")
            .order_by(Agent.created_at.desc())
        )
        result = await self._session.execute(stmt)
        agents = result.scalars().unique().all()
        return [
            self._build_aggregate(agent)
            for agent in agents
            if agent.profile is not None and agent.runtime is not None
        ]

    async def update_agent_fields(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Agent 主表字段。"""
        aggregate = await self.get(agent_id)
        if aggregate is None:
            return None
        entity = await self._session.get(Agent, agent_id)
        if entity is None:
            return None
        for field_name, value in fields.items():
            if value is not None and hasattr(entity, field_name):
                setattr(entity, field_name, value)
        await self.flush()
        return await self.get(agent_id)

    async def update_profile_fields(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Profile 字段。"""
        stmt = select(Profile).where(Profile.agent_id == agent_id)
        result = await self._session.execute(stmt)
        profile = result.scalar_one_or_none()
        if profile is None:
            return None
        for field_name, value in fields.items():
            if value is not None and hasattr(profile, field_name):
                setattr(profile, field_name, value)
        await self.flush()
        return await self.get(agent_id)

    async def update_runtime_fields(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Runtime 字段。"""
        stmt = select(Runtime).where(Runtime.agent_id == agent_id)
        result = await self._session.execute(stmt)
        runtime = result.scalar_one_or_none()
        if runtime is None:
            return None
        for field_name, value in fields.items():
            if hasattr(runtime, field_name):
                setattr(runtime, field_name, value)
        await self.flush()
        return await self.get(agent_id)

    def _build_aggregate(self, agent: Agent) -> AgentAggregate:
        """构造聚合返回值。"""
        return AgentAggregate(
            agent=AgentRecord.model_validate(agent),
            profile=ProfileRecord.model_validate(agent.profile),
            runtime=RuntimeRecord.model_validate(agent.runtime),
        )
