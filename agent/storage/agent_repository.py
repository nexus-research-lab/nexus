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
from agent.schema.model_agent import AAgent
from agent.schema.model_agent_persistence import AgentAggregate
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.workspace_template_initializer import (
    WorkspaceTemplateInitializer,
)
from agent.storage.agent_sql_mapper import AgentSqlMapper
from agent.storage.sqlite.agent_sql_repository import AgentSqlRepository
from agent.storage.storage_bootstrap import FileStorageBootstrap
from agent.storage.storage_paths import FileStoragePaths
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
    ) -> Optional[str]:
        """创建 Agent，返回 agent_id。"""
        await self._ensure_ready()
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            if await repository.get(agent_id):
                logger.warning(f"⚠️ Agent 已存在，跳过创建: {agent_id}")
                return None

            payload = AgentSqlMapper.build_create_payload(
                agent_id=agent_id,
                name=name,
                workspace_path=workspace_path,
                options=options,
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
        return AgentSqlMapper.to_model(aggregate)

    async def get_all_agents(self) -> List[AAgent]:
        """获取所有活跃 Agent。"""
        await self._ensure_ready()
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            aggregates = await repository.list_active()
        return [AgentSqlMapper.to_model(item) for item in aggregates]

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
    ) -> bool:
        """更新 Agent 基础信息与运行参数。"""
        await self._ensure_ready()
        aggregate = await self._get_aggregate(agent_id)
        if aggregate is None or aggregate.agent.status != "active":
            return False

        merged_options = AgentSqlMapper.merge_options(aggregate, options)
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            updated = await repository.update_agent_fields(
                agent_id,
                name=name,
            )
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
                model=merged_options.get("model"),
                permission_mode=merged_options.get("permission_mode"),
                allowed_tools_json=AgentSqlMapper.to_json(
                    merged_options.get("allowed_tools") or []
                ),
                disallowed_tools_json=AgentSqlMapper.to_json(
                    merged_options.get("disallowed_tools") or []
                ),
                mcp_servers_json=AgentSqlMapper.to_json(
                    merged_options.get("mcp_servers") or {}
                ),
                max_turns=merged_options.get("max_turns"),
                max_thinking_tokens=merged_options.get("max_thinking_tokens"),
                skills_enabled=bool(merged_options.get("skills_enabled", False)),
                setting_sources_json=AgentSqlMapper.to_json(
                    merged_options.get("setting_sources") or []
                ),
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
            self._initialized = True

    async def _ensure_main_agent(self) -> None:
        """确保 main agent 已存在于 SQLite。"""
        aggregate = await self._get_aggregate(MainAgentProfile.AGENT_ID)
        workspace_path = self._paths.workspace_base / MainAgentProfile.AGENT_ID
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
                            MainAgentProfile.build_default_options()
                        ),
                    )
                    await session.commit()
            return

        payload = AgentSqlMapper.build_create_payload(
            agent_id=MainAgentProfile.AGENT_ID,
            name=MainAgentProfile.AGENT_ID,
            workspace_path=str(workspace_path),
            options=MainAgentProfile.build_default_options(),
            status="active",
        )
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            await repository.create(payload)
            await session.commit()
        logger.info(f"🧩 已初始化 main Agent 数据: {workspace_path}")

    async def _get_aggregate(self, agent_id: str) -> Optional[AgentAggregate]:
        """读取 Agent 聚合。"""
        async with self._db.session() as session:
            repository = AgentSqlRepository(session)
            return await repository.get(agent_id)


agent_repository = AgentRepository()
