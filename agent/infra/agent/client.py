# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：client.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""Claude SDK Client 运行时封装。"""

import os
from typing import Any

from claude_agent_sdk import ClaudeSDKClient, PermissionResult, ToolPermissionContext

from agent.infra.agent.session_manager import session_manager
from agent.infra.permission.permission_strategy import PermissionStrategy
from agent.service.agent.agent_manager import agent_manager
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class AgentClientRuntime:
    """负责按 session_key 获取或初始化 Claude SDK client。"""

    async def get_or_create_client(
        self,
        session_key: str,
        agent_id: str,
        permission_strategy: PermissionStrategy,
    ) -> ClaudeSDKClient:
        """按需获取或创建 SDK client。"""
        client = await session_manager.get_session(session_key)
        if client:
            logger.debug(f"♻️ 复用现有 session: {session_key}")
            return client

        existing_session = await session_store.get_session_info(session_key)
        session_id = existing_session.session_id if existing_session else None
        real_agent_id = existing_session.agent_id if existing_session else agent_id

        try:
            sdk_options = await agent_manager.build_sdk_options(real_agent_id)
            logger.info(f"📋 SDK options 从 Agent 构建: agent={real_agent_id}")
        except Exception:
            logger.warning(f"⚠️ Agent 不存在: {real_agent_id}，使用默认配置")
            sdk_options = {"cwd": os.getcwd()}

        if session_id:
            sdk_options["resume"] = session_id

        async def can_use_tool(
            name: str,
            data: dict[str, Any],
            context: ToolPermissionContext,
        ) -> PermissionResult:
            return await permission_strategy.request_permission(session_key, name, data, context)

        client = await session_manager.create_session(
            session_key=session_key,
            can_use_tool=can_use_tool,
            session_id=session_id,
            session_options=sdk_options,
        )
        await client.connect()

        logger.info(f"✅ Client 就绪: key={session_key}, agent={real_agent_id}, session_id={session_id}")
        return client


agent_client_runtime = AgentClientRuntime()
