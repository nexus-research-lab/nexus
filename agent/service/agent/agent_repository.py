# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_repository.py
# @Date   ：2026/03/25 23:59
# @Author ：leemysw
# 2026/03/25 23:59   Create
# =====================================================

"""Agent SQLite 数据仓库。"""

from __future__ import annotations

import asyncio
from pathlib import Path
from typing import Dict, List, Optional

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.agent_sql_repository import AgentSqlRepository
from agent.infra.file_store.storage_bootstrap import FileStorageBootstrap
from agent.infra.file_store.storage_paths import FileStoragePaths
from agent.schema.model_agent import AAgent
from agent.schema.model_agent_persistence import AgentAggregate
from agent.service.agent.agent_sql_mapper import AgentSqlMapper
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.workspace_template_initializer import (
    WorkspaceTemplateInitializer,
)
from agent.utils.logger import logger


class AgentRepository:
    """以 SQLite 为主真相源的 Agent 仓库。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._db = get_db("async_sqlite")
        self._init_lock = asyncio.Lock()
        self._initialized = False

    async def create_agent(
            self,
            agent_id: str,
            name: str,
            workspace_path: str,
            options: Optional[Dict] = None,
            avatar: Optional[str] = None,
            description: Optional[str] = None,
            vibe_tags: Optional[list] = None,
    ) -> Optional[str]:
        """创建 Agent，返回 agent_id。"""
        await self._ensure_ready()
        # 将身份标识字段注入 options 以便 mapper 统一处理
        merged = dict(options) if options else {}
        if avatar is not None:
            merged["avatar"] = avatar
        if description is not None:
            merged["description"] = description
        if vibe_tags is not None:
            merged["vibe_tags"] = vibe_tags

        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            if await repository.get(agent_id):
                logger.warning(f"⚠️ Agent 已存在，跳过创建: {agent_id}")
                return None

            payload = AgentSqlMapper.build_create_payload(
                agent_id=agent_id,
                name=name,
                workspace_path=workspace_path,
                options=merged,
                status="active",
            )
            await repository.create(payload)
            await session.commit()
            logger.info(f"✅ Agent 创建成功: {agent_id} ({name})")
            return agent_id

    async def get_agent(self, agent_id: str) -> Optional[AAgent]:
        """按 agent_id 获取活跃 Agent。"""
        await self._ensure_ready()
        aggregate = await self._get_aggregate(agent_id)
        if aggregate is None or aggregate.agent.status != "active":
            return None
        agent = AgentSqlMapper.to_model(aggregate)

        # 添加 skills_count
        from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer

        deployer = WorkspaceSkillDeployer(agent_id, Path(agent.workspace_path))
        agent.skills_count = len(deployer.list_deployed())

        return agent

    async def get_all_agents(self) -> List[AAgent]:
        """获取所有活跃 Agent。"""
        await self._ensure_ready()
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregates = await repository.list_active()
        agents = [AgentSqlMapper.to_model(item) for item in aggregates]

        # 为每个 Agent 添加 skills_count
        from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer

        for agent in agents:
            deployer = WorkspaceSkillDeployer(agent.agent_id, Path(agent.workspace_path))
            agent.skills_count = len(deployer.list_deployed())

        return agents

    async def exists_active_agent_name(
            self,
            name: str,
            exclude_agent_id: Optional[str] = None,
    ) -> bool:
        """检查活跃 Agent 名称是否已存在。"""
        normalized = name.lower()
        for agent in await self.get_all_agents():
            if exclude_agent_id and agent.agent_id == exclude_agent_id:
                continue
            if agent.name.lower() == normalized:
                return True
        return False

    async def update_agent(
            self,
            agent_id: str,
            name: Optional[str] = None,
            options: Optional[Dict] = None,
            avatar: Optional[str] = None,
            description: Optional[str] = None,
            vibe_tags: Optional[list] = None,
    ) -> bool:
        """更新 Agent 基础信息与运行参数。"""
        await self._ensure_ready()
        aggregate = await self._get_aggregate(agent_id)
        if aggregate is None or aggregate.agent.status != "active":
            return False

        merged_options = AgentSqlMapper.merge_options(aggregate, options)
        runtime_fields = AgentSqlMapper.build_runtime_fields(merged_options)

        # 构造 Agent 主表更新字段（含身份标识）
        agent_fields: Dict = {}
        if name is not None:
            agent_fields["name"] = name
        if avatar is not None:
            agent_fields["avatar"] = avatar
        if description is not None:
            agent_fields["description"] = description
        if vibe_tags is not None:
            agent_fields["vibe_tags"] = vibe_tags

        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            updated = await repository.update_agent_fields(agent_id, **agent_fields)
            if updated is None:
                return False

            display_name = name or aggregate.profile.display_name
            updated = await repository.update_profile_fields(
                agent_id,
                display_name=display_name,
            )
            if updated is None:
                return False

            updated = await repository.update_runtime_fields(
                agent_id,
                **runtime_fields,
            )
            if updated is None:
                return False
            await session.commit()

        logger.info(f"✅ Agent 更新成功: {agent_id}")
        return True

    async def update_agent_workspace_path(self, agent_id: str, workspace_path: str) -> bool:
        """更新 Agent 的工作空间路径。"""
        await self._ensure_ready()
        target_path = str(Path(workspace_path).expanduser())
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            updated = await repository.update_agent_fields(agent_id, workspace_path=target_path)
            if updated is None:
                return False
            await session.commit()
        logger.info(f"✅ Agent workspace_path 已更新: {agent_id} -> {target_path}")
        return True

    async def delete_agent(self, agent_id: str) -> bool:
        """归档 Agent。"""
        await self._ensure_ready()
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            updated = await repository.update_agent_fields(agent_id, status="archived")
            if updated is None:
                return False
            await session.commit()
        logger.info(f"🗑️ Agent 已归档: {agent_id}")
        return True

    async def _ensure_ready(self) -> None:
        """确保文件系统与 SQLite Agent 数据已完成初始化。"""
        if self._initialized:
            return
        async with self._init_lock:
            if self._initialized:
                return
            self._bootstrap.ensure_ready()
            await self._ensure_main_agent()
            await self._ensure_active_agent_workspaces()
            self._initialized = True

    async def _ensure_main_agent(self) -> None:
        """确保 main agent 已存在于 SQLite。"""
        aggregate = await self._get_aggregate(MainAgentProfile.AGENT_ID)
        workspace_path = self._paths.workspace_base / MainAgentProfile.AGENT_ID
        default_options = MainAgentProfile.build_default_options()
        WorkspaceTemplateInitializer(
            MainAgentProfile.AGENT_ID,
            workspace_path,
        ).ensure_initialized(MainAgentProfile.AGENT_ID)

        if aggregate is not None:
            if aggregate.agent.status != "active":
                async with self._db.session() as session:
                    repository = AgentSqlRepository(session)
                    await repository.update_agent_fields(
                        MainAgentProfile.AGENT_ID,
                        name=MainAgentProfile.AGENT_ID,
                        status="active",
                        workspace_path=str(workspace_path),
                    )
                    await repository.update_profile_fields(
                        MainAgentProfile.AGENT_ID,
                        display_name=MainAgentProfile.AGENT_ID,
                    )
                    await repository.update_runtime_fields(
                        MainAgentProfile.AGENT_ID,
                        **AgentSqlMapper.build_runtime_fields(
                            default_options
                        ),
                    )
                    await session.commit()
            elif aggregate.runtime.provider is not None:
                # 中文注释：主智能体默认跟随全局默认 Provider，
                # 这里把历史上被写死的显式 provider 清空，避免继续钉死旧值。
                async with self._db.session() as session:
                    repository = AgentSqlRepository(session)
                    await repository.update_runtime_fields(
                        MainAgentProfile.AGENT_ID,
                        **AgentSqlMapper.build_runtime_fields(default_options),
                    )
                    await session.commit()
            return

        payload = AgentSqlMapper.build_create_payload(
            agent_id=MainAgentProfile.AGENT_ID,
            name=MainAgentProfile.AGENT_ID,
            workspace_path=str(workspace_path),
            options=default_options,
            status="active",
        )
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            await repository.create(payload)
            await session.commit()
        logger.info(f"🧩 已初始化 main Agent 数据: {workspace_path}")

    async def _ensure_active_agent_workspaces(self) -> None:
        """确保所有活跃 Agent 的 workspace 模板与系统 skill 已补齐。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregates = await repository.list_active()

        for aggregate in aggregates:
            if MainAgentProfile.is_main_agent(aggregate.agent.id):
                continue
            WorkspaceTemplateInitializer(
                aggregate.agent.id,
                Path(aggregate.agent.workspace_path).expanduser(),
            ).ensure_initialized(aggregate.agent.name)

    async def _get_aggregate(self, agent_id: str) -> Optional[AgentAggregate]:
        """读取 Agent 聚合。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            return await repository.get(agent_id)


agent_repository = AgentRepository()
