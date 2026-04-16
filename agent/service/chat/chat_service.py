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
import uuid
from typing import Any, Dict

from agent.schema.model_message import (
    EventMessage,
    Message,
    build_error_event,
    build_round_status_event,
)
from agent.service.agent.agent_runtime import agent_runtime
from agent.service.channels.message_sender import MessageSender
from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry
from agent.service.message.chat_message_processor import ChatMessageProcessor
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.permission.strategy.permission_strategy import PermissionStrategy
from agent.service.session.session_manager import session_manager
from agent.service.session.session_router import (
    StructuredSessionKeyError,
    require_structured_session_key,
    resolve_agent_id,
)
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
        session_key = await self._resolve_session_key(message)
        if not session_key:
            return
        message["round_id"] = str(message.get("round_id") or uuid.uuid4())
        existing_session = await session_store.get_session_info(session_key)
        requested_agent_id = message.get("agent_id", "")
        real_agent_id = resolve_agent_id(
            existing_session.agent_id if existing_session else requested_agent_id
        )
        message["agent_id"] = real_agent_id

        if session_key in chat_tasks and not chat_tasks[session_key].done():
            logger.info(f"⚠️ 取消旧 chat 任务: {session_key}")
            await self._interrupt_previous_round(
                session_key=session_key,
                task=chat_tasks[session_key],
            )

        task = asyncio.create_task(self.handle_chat_message(message))
        chat_tasks[session_key] = task
        ws_chat_task_registry.register(session_key, task, message.get("round_id"))
        ws_chat_task_registry.register_agent_task(
            task_key=session_key,
            agent_id=real_agent_id,
            task=task,
            round_id=message.get("round_id"),
        )
        task.add_done_callback(
            lambda current_task: self._on_task_done(
                session_key,
                current_task,
                chat_tasks,
            )
        )

    async def handle_chat_message(self, message: Dict[str, Any]) -> None:
        """处理聊天消息并推动 Claude 对话循环。"""
        session_key = await self._resolve_session_key(message)
        if not session_key:
            return
        requested_agent_id = message.get("agent_id", "")
        content = message.get("content")
        round_id = str(message.get("round_id") or uuid.uuid4())
        message["round_id"] = round_id
        existing_session = await session_store.get_session_info(session_key)
        real_agent_id = resolve_agent_id(
            existing_session.agent_id if existing_session else requested_agent_id
        )

        try:
            client = await agent_runtime.get_or_create_client(
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
                    session_key=session_key,
                    agent_id=real_agent_id,
                )
            )
            await self._sender.send(
                build_round_status_event(
                    round_id=round_id,
                    status="error",
                    session_key=session_key,
                    agent_id=real_agent_id,
                )
            )
            return

        async with session_manager.get_lock(session_key):
            logger.info(f"📨 处理消息: key={session_key}, round_id={round_id}")
            processor = ChatMessageProcessor(
                session_key=session_key,
                query=content,
                round_id=round_id,
                agent_id=real_agent_id,
                session_id=existing_session.session_id if existing_session else None,
            )
            round_started = False
            round_finished = False

            try:
                # 中文注释：DM 权限事件也必须显式携带 round 归属。
                # 否则 AskUserQuestion 超时后，前端无法把残留 permission
                # 和 terminal round_status 关联起来，状态会一直卡在等待输入。
                self._permission_strategy.bind_session_route(
                    session_key,
                    PermissionRouteContext(
                        route_session_key=session_key,
                        agent_id=real_agent_id,
                        caused_by=round_id,
                    ),
                )
                await self._sender.send(
                    build_round_status_event(
                        round_id=round_id,
                        status="running",
                        session_key=session_key,
                        agent_id=real_agent_id,
                        session_id=processor.session_id,
                    )
                )
                round_started = True

                client = await agent_runtime.query_with_recovery(
                    session_key=session_key,
                    agent_id=real_agent_id,
                    permission_strategy=self._permission_strategy,
                    prompt=content,
                    client=client,
                    resolved_agent_id=real_agent_id,
                )

                async for response_msg in client.receive_messages():
                    processed_messages = await processor.process_messages(response_msg)
                    for a_message in processed_messages:
                        await self._sender.send(a_message)
                        if isinstance(a_message, Message) and a_message.role == "result":
                            await self._sender.send(
                                build_round_status_event(
                                    round_id=round_id,
                                    status="finished",
                                    session_key=session_key,
                                    agent_id=real_agent_id,
                                    session_id=processor.session_id,
                                    message_id=a_message.message_id,
                                    result_subtype=a_message.subtype,
                                )
                            )
                            round_finished = True
                    if processor.subtype in ["success", "error", "interrupted"]:
                        break
            except asyncio.CancelledError:
                raise
            except Exception as exc:
                logger.error(f"❌ 消息处理失败: key={session_key}, round_id={round_id}, error={exc}")
                await self._sender.send(
                    self._build_error(
                        error_type="chat_error",
                        message=str(exc),
                        session_key=session_key,
                        agent_id=real_agent_id,
                        session_id=processor.session_id,
                        details={"round_id": round_id},
                    )
                )
                if round_started and not round_finished:
                    await self._sender.send(
                        build_round_status_event(
                            round_id=round_id,
                            status="error",
                            session_key=session_key,
                            agent_id=real_agent_id,
                            session_id=processor.session_id,
                        )
                    )
                raise
            finally:
                self._permission_strategy.unbind_session_route(session_key)

            logger.info(f"✅ 消息处理完成: key={session_key}, 共 {processor.message_count} 条响应")

    async def _interrupt_previous_round(
        self,
        session_key: str,
        task: asyncio.Task,
    ) -> None:
        """在同会话开启新一轮前，先把上一轮收口为 interrupted。"""
        round_id = ws_chat_task_registry.get_running_round_id(session_key)
        if not round_id:
            round_id = await session_store.get_latest_round_id(session_key)

        client = await session_manager.get_session(session_key)
        if client is not None:
            try:
                await client.interrupt()
            except Exception as exc:
                logger.warning(f"⚠️ 中断旧会话失败: key={session_key}, error={exc}")

        self._permission_strategy.cancel_requests_for_session(
            session_key,
            message="收到新的用户消息，上一轮已停止",
        )

        if not task.done():
            try:
                await asyncio.wait_for(task, timeout=3.0)
            except asyncio.TimeoutError:
                task.cancel()
                await asyncio.gather(task, return_exceptions=True)

        if not round_id:
            return

        repaired_messages = await session_store.repair_unfinished_round(
            session_key=session_key,
            round_id=round_id,
            result_text="收到新的用户消息，上一轮已停止",
        )
        for repaired_message in repaired_messages:
            await self._sender.send(repaired_message)
        if not repaired_messages:
            return
        session_info = await session_store.get_session_info(session_key)
        await self._sender.send(
            build_round_status_event(
                round_id=round_id,
                status="interrupted",
                session_key=session_key,
                agent_id=resolve_agent_id(session_info.agent_id if session_info else None),
                session_id=session_info.session_id if session_info else None,
            )
        )

    async def _resolve_session_key(self, message: Dict[str, Any]) -> str | None:
        """解析并校验聊天消息的结构化 session_key。"""
        raw_session_key = message.get("session_key")
        if not isinstance(raw_session_key, str):
            raw_session_key = ""

        try:
            session_key = require_structured_session_key(raw_session_key)
        except StructuredSessionKeyError as exc:
            await self._sender.send(
                self._build_error(
                    error_type=(
                        "validation_error"
                        if str(exc) == "session_key is required"
                        else "invalid_session_key"
                    ),
                    message=str(exc),
                    session_key=raw_session_key or None,
                )
            )
            return None

        message["session_key"] = session_key
        return session_key

    @staticmethod
    def _on_task_done(
        session_key: str,
        task: asyncio.Task,
        chat_tasks: Dict[str, Any],
    ) -> None:
        """聊天任务完成回调。"""
        ws_chat_task_registry.unregister(session_key, task)
        ws_chat_task_registry.unregister_agent_task(session_key, task)
        chat_tasks.pop(session_key, None)
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
            session_key: str | None = None,
            agent_id: str | None = None,
            session_id: str | None = None,
            details: Dict[str, Any] | None = None,
    ) -> EventMessage:
        """构建错误响应。"""
        return build_error_event(
            error_type=error_type,
            message=message,
            session_key=session_key,
            agent_id=agent_id,
            session_id=session_id,
            details=details,
        )
