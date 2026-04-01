# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_repository.py
# @Date   ：2026/4/1 18:38
# @Author ：leemysw
# 2026/4/1 18:38   Create
# =====================================================

"""Skill Marketplace 数据仓库。"""

from __future__ import annotations

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from agent.infra.database.get_db import get_db
from agent.infra.database.models.skill import AgentSkill, PoolSkill
from agent.utils.snowflake import worker


class SkillRepository:
    """负责 Skill 资源池和 Agent-Skill 关系的持久化。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def get_global_states(self) -> dict[str, bool]:
        """获取技能全局启用状态。"""
        async with self._db.session() as session:
            result = await session.execute(select(PoolSkill.name, PoolSkill.global_enabled))
            return {name: enabled for name, enabled in result.all()}

    async def get_pool_installed_states(self) -> dict[str, bool]:
        """获取技能是否已安装到资源池。"""
        async with self._db.session() as session:
            result = await session.execute(select(PoolSkill.name, PoolSkill.installed))
            return {name: installed for name, installed in result.all()}

    async def get_agent_skill_names(self, agent_id: str) -> list[str]:
        """获取 Agent 已安装的 skill 名称列表。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(AgentSkill.skill_name).where(AgentSkill.agent_id == agent_id)
            )
            return sorted({skill_name for skill_name in result.scalars().all()})

    async def get_agent_ids_by_skill_name(self, skill_name: str) -> list[str]:
        """获取安装了指定 skill 的 Agent 列表。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(AgentSkill.agent_id).where(AgentSkill.skill_name == skill_name)
            )
            return sorted({agent_id for agent_id in result.scalars().all()})

    async def add_agent_skill(self, agent_id: str, skill_name: str) -> None:
        """写入 Agent-Skill 关联。"""
        async with self._db.session() as session:
            entity = await self._get_agent_skill(session, agent_id, skill_name)
            if entity is None:
                session.add(
                    AgentSkill(
                        id=worker.get_id(),
                        agent_id=agent_id,
                        skill_name=skill_name,
                    )
                )
            await session.commit()

    async def remove_agent_skill(self, agent_id: str, skill_name: str) -> None:
        """删除单个 Agent-Skill 关联。"""
        async with self._db.session() as session:
            await session.execute(
                delete(AgentSkill).where(
                    AgentSkill.agent_id == agent_id,
                    AgentSkill.skill_name == skill_name,
                )
            )
            await session.commit()

    async def remove_skill_from_all_agents(self, skill_name: str) -> list[str]:
        """删除指定 skill 的所有 Agent 关联，并返回受影响 Agent。"""
        async with self._db.session() as session:
            result = await session.execute(
                select(AgentSkill.agent_id).where(AgentSkill.skill_name == skill_name)
            )
            agent_ids = sorted({agent_id for agent_id in result.scalars().all()})
            await session.execute(delete(AgentSkill).where(AgentSkill.skill_name == skill_name))
            await session.commit()
            return agent_ids

    async def set_pool_installed(self, skill_name: str, installed: bool) -> None:
        """设置技能资源池安装状态。"""
        async with self._db.session() as session:
            entity = await self._get_pool_skill(session, skill_name)
            if entity is None:
                session.add(
                    PoolSkill(
                        name=skill_name,
                        installed=installed,
                        global_enabled=True,
                    )
                )
            else:
                entity.installed = installed
            await session.commit()

    async def set_global_enabled(self, skill_name: str, enabled: bool) -> None:
        """设置技能全局启用状态。"""
        async with self._db.session() as session:
            entity = await self._get_pool_skill(session, skill_name)
            if entity is None:
                session.add(
                    PoolSkill(
                        name=skill_name,
                        installed=True,
                        global_enabled=enabled,
                    )
                )
            else:
                entity.global_enabled = enabled
            await session.commit()

    async def delete_pool_skill(self, skill_name: str) -> None:
        """删除技能资源池记录。"""
        async with self._db.session() as session:
            await session.execute(delete(PoolSkill).where(PoolSkill.name == skill_name))
            await session.commit()

    @staticmethod
    async def _get_pool_skill(session: AsyncSession, skill_name: str) -> PoolSkill | None:
        """读取单个资源池技能记录。"""
        return await session.get(PoolSkill, skill_name)

    @staticmethod
    async def _get_agent_skill(
        session: AsyncSession,
        agent_id: str,
        skill_name: str,
    ) -> AgentSkill | None:
        """读取单个 Agent-Skill 关联。"""
        result = await session.execute(
            select(AgentSkill).where(
                AgentSkill.agent_id == agent_id,
                AgentSkill.skill_name == skill_name,
            )
        )
        return result.scalar_one_or_none()


skill_repository = SkillRepository()
