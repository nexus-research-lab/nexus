# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill_repository.py
# @Date   ：2026/3/31
# @Author ：leemysw
# 2026/3/31   Create
# =====================================================

"""Skill 仓库 —— 技能池状态与 Agent-Skill 关联的数据库操作封装。"""

from __future__ import annotations

from typing import Optional

from agent.infra.database.get_db import get_db
from agent.storage.sqlite.skill_sql_repository import SkillSqlRepository
from agent.utils.snowflake import worker


class SkillRepository:
    """技能池和 Agent-Skill 关联的高层仓库。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    # ── 技能池状态 ────────────────────────────────────

    async def get_pool_installed_states(self) -> dict[str, bool]:
        """返回 {skill_name: installed} 映射。"""
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            rows = await repo.list_pool_skills()
        return {row.name: row.installed for row in rows}

    async def get_global_states(self) -> dict[str, bool]:
        """返回 {skill_name: global_enabled} 映射。"""
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            rows = await repo.list_pool_skills()
        return {row.name: row.global_enabled for row in rows}

    async def set_pool_installed(self, skill_name: str, installed: bool) -> None:
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            await repo.upsert_pool_skill(skill_name, installed=installed)
            await session.commit()

    async def set_global_enabled(self, skill_name: str, enabled: bool) -> None:
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            await repo.upsert_pool_skill(skill_name, global_enabled=enabled)
            await session.commit()

    async def delete_pool_skill(self, skill_name: str) -> None:
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            await repo.delete_pool_skill(skill_name)
            await session.commit()

    # ── Agent-Skill 关联 ──────────────────────────────

    async def get_agent_skill_names(self, agent_id: str) -> list[str]:
        """获取某个 Agent 已安装的 skill 名称列表。"""
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            rows = await repo.list_agent_skills(agent_id)
        return [row.skill_name for row in rows]

    async def add_agent_skill(self, agent_id: str, skill_name: str) -> None:
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            existing = await repo.get_agent_skill(agent_id, skill_name)
            if existing:
                return
            row_id = str(worker.get_id())
            await repo.add_agent_skill(row_id, agent_id, skill_name)
            await session.commit()

    async def remove_agent_skill(self, agent_id: str, skill_name: str) -> None:
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            await repo.remove_agent_skill(agent_id, skill_name)
            await session.commit()

    async def remove_skill_from_all_agents(self, skill_name: str) -> list[str]:
        """从所有 Agent 中移除某个 skill，返回受影响的 agent_id。"""
        async with self._db.session() as session:
            repo = SkillSqlRepository(session)
            agent_ids = await repo.remove_all_agent_skills_by_name(skill_name)
            await session.commit()
        return agent_ids


# 全局单例
skill_repository = SkillRepository()
