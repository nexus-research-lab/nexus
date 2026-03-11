#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_handler.py
# @Date   ：2025/12/06
# @Author ：leemysw
#
# 2025/12/06   Create
# 2026/2/25    重构：session_key 路由 + Workspace 注入
# =====================================================

"""
聊天消息处理器

[INPUT]: 依赖 channel.channel 的 MessageSender/PermissionStrategy,
         依赖 session_manager 和 session_store 管理会话,
         依赖 agent.workspace 构建 system prompt,
         依赖 ChatMessageProcessor 处理 SDK 响应
[OUTPUT]: 对外提供 ChatHandler
[POS]: handler 模块的核心处理器，负责用户消息 → Agent 调用 → 流式响应
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
from typing import Any, Dict

from claude_agent_sdk import ClaudeSDKClient, PermissionResult, ToolPermissionContext

from agent.service.agent_manager import agent_manager
from agent.service.channel.channel import MessageSender, PermissionStrategy
from agent.service.handler.base_handler import BaseHandler
from agent.service.process.chat_message_processor import ChatMessageProcessor
from agent.service.session_manager import session_manager
from agent.service.session_store import session_store
from agent.utils.logger import logger


class ChatHandler(BaseHandler):
    """聊天消息处理器"""

    def __init__(self, sender: MessageSender, permission_strategy: PermissionStrategy):
        super().__init__(sender)
        self.permission_strategy = permission_strategy

    async def handle_chat_message_with_task(
            self,
            message: Dict[str, Any],
            chat_tasks: Dict[str, Any],
    ) -> None:
        """处理聊天消息，包含任务管理逻辑"""
        session_key = message.get("session_key") or message.get("agent_id", "")
        if not session_key:
            await self.send(self.create_error_response(
                error_type="validation_error",
                message="session_key is required for chat messages",
            ))
            return

        # 确保 message 中有 session_key
        message["session_key"] = session_key

        # 取消旧任务
        if session_key in chat_tasks and not chat_tasks[session_key].done():
            logger.info(f"⚠️ 取消旧 chat 任务: {session_key}")
            chat_tasks[session_key].cancel()

        task = asyncio.create_task(self.handle_chat_message(message))
        chat_tasks[session_key] = task
        task.add_done_callback(lambda t: self._on_task_done(session_key, t))

    @staticmethod
    def _on_task_done(session_key: str, task: asyncio.Task) -> None:
        """chat 任务完成回调"""
        if task.cancelled():
            logger.info(f"🛑 任务被取消: {session_key}")
        elif task.exception():
            logger.error(f"❌ 任务异常: {session_key}, error={task.exception()}")
        else:
            logger.debug(f"✅ 任务完成: {session_key}")

    async def handle_chat_message(self, message: Dict[str, Any]) -> None:
        """处理聊天消息 — session_key 路由"""
        session_key = message.get("session_key") or message.get("agent_id", "")
        requested_agent_id = message.get("agent_id", "")  # Agent ID
        content = message.get("content")
        round_id = message.get("round_id")
        existing_session = await session_store.get_session_info(session_key)
        real_agent_id = (
            existing_session.agent_id
            if existing_session and existing_session.agent_id
            else requested_agent_id or "main"
        )

        try:
            client = await self._get_or_create_client(session_key, real_agent_id)
        except Exception as e:
            logger.error(f"❌ 获取 client 失败: {e}")
            await self.send(self.create_error_response(
                error_type="client_error",
                message=f"Failed to get or create client: {str(e)}",
                agent_id=session_key,
            ))
            return

        async with session_manager.get_lock(session_key):
            logger.info(f"📨 处理消息: key={session_key}, round_id={round_id}")

            await client.query(content)

            processor = ChatMessageProcessor(
                session_key=session_key,
                query=content,
                round_id=round_id,
                agent_id=real_agent_id,
            )

            async for response_msg in client.receive_messages():
                processed_messages = await processor.process_messages(response_msg)
                for a_message in processed_messages:
                    await self.send(a_message)
                if processor.subtype in ["success", "error"]:
                    break

            logger.info(f"✅ 消息处理完成: key={session_key}, 共 {processor.message_count} 条响应")

    async def _get_or_create_client(self, session_key: str, agent_id: str = "") -> ClaudeSDKClient:
        """懒加载：按需获取或创建 SDK client

        配置来源优先级: Agent Workspace (cwd + prompt) → Agent Options (model + tools)
        每次创建 client 重新读取 workspace 文件，修改后立即生效。
        """
        import os

        # 1. 检查内存
        client = await session_manager.get_session(session_key)
        if client:
            logger.debug(f"♻️ 复用现有 session: {session_key}")
            return client

        # 2. 查询会话存储获取 resume session_id + 真正的 agent_id
        existing_session = await session_store.get_session_info(session_key)
        session_id = existing_session.session_id if existing_session else None

        # 从 session 记录获取真正的 Agent 实体 ID（不是 WebSocket 传来的路由键）
        real_agent_id = existing_session.agent_id if existing_session else agent_id

        # 3. 从 AgentManager 构建 SDK options（cwd + prompt + model + tools）
        try:
            sdk_options = await agent_manager.build_sdk_options(real_agent_id)
            logger.info(f"📋 SDK options 从 Agent 构建: agent={real_agent_id}")
        except (ValueError, Exception):
            # Agent 不存在时使用默认配置，必须提供 cwd
            logger.warning(f"⚠️ Agent 不存在: {real_agent_id}，使用默认配置")
            sdk_options = {"cwd": os.getcwd()}

        # 4. 恢复已有会话
        if session_id:
            sdk_options["resume"] = session_id

        # 5. 创建权限回调
        async def can_use_tool(name: str, data: dict[str, Any], context: ToolPermissionContext) -> PermissionResult:
            return await self.permission_strategy.request_permission(session_key, name, data, context)

        # 6. 创建 client
        client = await session_manager.create_session(
            session_key=session_key,
            can_use_tool=can_use_tool,
            session_id=session_id,
            session_options=sdk_options,
        )

        # 7. 连接 SDK
        await client.connect()

        logger.info(f"✅ Client 就绪: key={session_key}, agent={real_agent_id}, session_id={session_id}")
        return client
