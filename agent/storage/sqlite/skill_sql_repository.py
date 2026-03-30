# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_sql_repository.py
# @Date   ：2026/3/31
# @Author ：leemysw
# 2026/3/31   Create
# =====================================================

"""Skill SQL 仓储 —— 技能池和 Agent-Skill 关联的 CRUD。"""

from __future__ import annotations

from typing import Optional

from sqlalchemy import delete, select

from agent.infra.database.models.skill import AgentSkill, PoolSkill
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class SkillSqlRepository(BaseSqlRepository):
    """技能池与 Agent-Skill 关联 SQL 仓储。"""

    # ── PoolSkill ─────────────────────────────────────

    async def get_pool_skill(self, name: str) -> Optional[PoolSkill]:
        stmt = select(PoolSkill).where(PoolSkill.name == name)
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def list_pool_skills(self) -> list[PoolSkill]:
        stmt = select(PoolSkill).order_by(PoolSkill.name)
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def upsert_pool_skill(
        self,
        name: str,
        installed: Optional[bool] = None,
        global_enabled: Optional[bool] = None,
    ) -> PoolSkill:
        """插入或更新技能池条目。"""
        existing = await self.get_pool_skill(name)
        if existing:
            if installed is not None:
                existing.installed = installed
            if global_enabled is not None:
                existing.global_enabled = global_enabled
            await self.flush()
            return existing
        row = PoolSkill(
            name=name,
            installed=installed if installed is not None else False,
            global_enabled=global_enabled if global_enabled is not None else True,
        )
        self._session.add(row)
        await self.flush()
        return row

    async def delete_pool_skill(self, name: str) -> None:
        stmt = delete(PoolSkill).where(PoolSkill.name == name)
        await self._session.execute(stmt)
        await self.flush()

    # ── AgentSkill ────────────────────────────────────

    async def list_agent_skills(self, agent_id: str) -> list[AgentSkill]:
        stmt = (
            select(AgentSkill)
            .where(AgentSkill.agent_id == agent_id)
            .order_by(AgentSkill.skill_name)
        )
        result = await self._session.execute(stmt)
        return list(result.scalars().all())

    async def get_agent_skill(
        self, agent_id: str, skill_name: str
    ) -> Optional[AgentSkill]:
        stmt = select(AgentSkill).where(
            AgentSkill.agent_id == agent_id,
            AgentSkill.skill_name == skill_name,
        )
        result = await self._session.execute(stmt)
        return result.scalar_one_or_none()

    async def add_agent_skill(
        self, row_id: str, agent_id: str, skill_name: str
    ) -> AgentSkill:
        row = AgentSkill(id=row_id, agent_id=agent_id, skill_name=skill_name)
        self._session.add(row)
        await self.flush()
        return row

    async def remove_agent_skill(
        self, agent_id: str, skill_name: str
    ) -> None:
        stmt = delete(AgentSkill).where(
            AgentSkill.agent_id == agent_id,
            AgentSkill.skill_name == skill_name,
        )
        await self._session.execute(stmt)
        await self.flush()

    async def remove_all_agent_skills_by_name(self, skill_name: str) -> list[str]:
        """删除所有 Agent 关联某个 skill 的记录，返回受影响的 agent_id 列表。"""
        stmt = select(AgentSkill.agent_id).where(
            AgentSkill.skill_name == skill_name
        )
        result = await self._session.execute(stmt)
        agent_ids = list(result.scalars().all())
        if agent_ids:
            del_stmt = delete(AgentSkill).where(
                AgentSkill.skill_name == skill_name
            )
            await self._session.execute(del_stmt)
            await self.flush()
        return agent_ids
