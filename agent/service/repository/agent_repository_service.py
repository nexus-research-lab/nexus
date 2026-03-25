# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_repository_service.py
# @Date   ：2026/3/19 00:12
# @Author ：leemysw
# 2026/3/19 00:12   Create
# =====================================================

"""Agent 聚合持久化服务。"""

from __future__ import annotations

from typing import Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_agent_persistence import AgentAggregate, CreateAgentAggregate
from agent.storage.sqlite.agent_sql_repository import AgentSqlRepository


class AgentPersistenceService:
    """负责编排 Agent 聚合数据库事务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def create_agent_aggregate(
        self,
        payload: CreateAgentAggregate,
    ) -> AgentAggregate:
        """创建 Agent + Profile + Runtime 聚合。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregate = await repository.create(payload)
            await session.commit()
            return aggregate

    async def get_agent_aggregate(self, agent_id: str) -> Optional[AgentAggregate]:
        """读取 Agent 聚合。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            return await repository.get(agent_id)

    async def list_active_agents(self) -> list[AgentAggregate]:
        """列出所有活跃 Agent 聚合。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            return await repository.list_active()

    async def update_agent_profile(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Agent Profile 字段。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregate = await repository.update_profile_fields(agent_id, **fields)
            if aggregate is None:
                return None
            await session.commit()
            return aggregate

    async def update_agent_entity(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Agent 主表字段。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregate = await repository.update_agent_fields(agent_id, **fields)
            if aggregate is None:
                return None
            await session.commit()
            return aggregate

    async def update_agent_runtime(
        self,
        agent_id: str,
        **fields: object,
    ) -> Optional[AgentAggregate]:
        """更新 Agent Runtime 字段。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregate = await repository.update_runtime_fields(agent_id, **fields)
            if aggregate is None:
                return None
            await session.commit()
            return aggregate


agent_persistence_service = AgentPersistenceService()
