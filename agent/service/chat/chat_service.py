# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""对话编排服务。"""

import asyncio
from typing import Any, Dict

from agent.infra.agent.client import agent_client_runtime
from agent.infra.agent.message_formatter import ChatMessageProcessor
from agent.channels.message_sender import MessageSender
from agent.infra.permission.permission_strategy import PermissionStrategy
from agent.infra.agent.session_manager import session_manager
from agent.schema.model_message import AError
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class ChatService:
    """负责单次对话链路编排。"""

    def __init__(self, sender: MessageSender, permission_strategy: PermissionStrategy):
        self._sender = sender
        self._permission_strategy = permission_strategy

    async def handle_chat_message_with_task(
        self,
        message: Dict[str, Any],
        chat_tasks: Dict[str, Any],
    ) -> None:
        """处理聊天消息并维护任务生命周期。"""
        session_key = message.get("session_key") or message.get("agent_id", "")
        if not session_key:
            await self._sender.send(
                self._build_error(
                    error_type="validation_error",
                    message="session_key is required for chat messages",
                )
            )
            return

        message["session_key"] = session_key

        if session_key in chat_tasks and not chat_tasks[session_key].done():
            logger.info(f"⚠️ 取消旧 chat 任务: {session_key}")
            chat_tasks[session_key].cancel()

        task = asyncio.create_task(self.handle_chat_message(message))
        chat_tasks[session_key] = task
        task.add_done_callback(lambda current_task: self._on_task_done(session_key, current_task))

    async def handle_chat_message(self, message: Dict[str, Any]) -> None:
        """处理聊天消息并推动 Claude 对话循环。"""
        session_key = message.get("session_key") or message.get("agent_id", "")
        requested_agent_id = message.get("agent_id", "")
        content = message.get("content")
        round_id = message.get("round_id")
        existing_session = await session_store.get_session_info(session_key)
        real_agent_id = (
            existing_session.agent_id
            if existing_session and existing_session.agent_id
            else requested_agent_id or "main"
        )

        try:
            client = await agent_client_runtime.get_or_create_client(
                session_key=session_key,
                agent_id=real_agent_id,
                permission_strategy=self._permission_strategy,
            )
        except Exception as exc:
            logger.error(f"❌ 获取 client 失败: {exc}")
            await self._sender.send(
                self._build_error(
                    error_type="client_error",
                    message=f"Failed to get or create client: {str(exc)}",
                    agent_id=session_key,
                )
            )
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
                    await self._sender.send(a_message)
                if processor.subtype in ["success", "error"]:
                    break

            logger.info(f"✅ 消息处理完成: key={session_key}, 共 {processor.message_count} 条响应")

    @staticmethod
    def _on_task_done(session_key: str, task: asyncio.Task) -> None:
        """聊天任务完成回调。"""
        if task.cancelled():
            logger.info(f"🛑 任务被取消: {session_key}")
        elif task.exception():
            logger.error(f"❌ 任务异常: {session_key}, error={task.exception()}")
        else:
            logger.debug(f"✅ 任务完成: {session_key}")

    @staticmethod
    def _build_error(
        error_type: str,
        message: str,
        agent_id: str | None = None,
        session_id: str | None = None,
        details: Dict[str, Any] | None = None,
    ) -> AError:
        """构建错误响应。"""
        return AError(
            error_type=error_type,
            message=message,
            agent_id=agent_id,
            session_id=session_id,
            details=details,
        )
