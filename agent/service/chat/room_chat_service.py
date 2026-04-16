#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Room 聊天服务 — 处理 Room 内的 @mention 消息分发。

与 ChatService（DM 单 Agent）不同，RoomChatService 负责：
1. 解析消息中的 @Agent 提及
2. 为每个 Agent 预分配占位槽位，通过 chat_ack 立即告知前端
3. 所有 Agent 并发执行（asyncio.gather），互不阻塞
4. 所有消息存储在共享的 room session_key 下
"""

import asyncio
import uuid
from typing import Any, Dict, List, Tuple

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.session_sql_repository import SessionSqlRepository
from agent.schema.model_chat_persistence import SessionRecord
from agent.schema.model_message import (
    EventMessage,
    Message,
    StreamMessage,
    build_error_event,
    build_round_status_event,
    build_transport_event,
    current_timestamp_ms,
)
from agent.service.agent.agent_manager import AgentManager
from agent.service.agent.agent_runtime import agent_runtime
from agent.service.channels.message_sender import MessageSender
from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry
from agent.service.message.chat_message_processor import ChatMessageProcessor
from agent.service.permission.strategy.permission_strategy import PermissionStrategy
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.room.room_agent_runtime_factory import (
    room_agent_runtime_factory,
)
from agent.service.room.room_conversation_orchestrator import (
    room_conversation_orchestrator,
)
from agent.service.room.room_interrupt_service import room_interrupt_service
from agent.service.room.room_message_store import room_message_store
from agent.service.room.room_round_store import room_round_store
from agent.service.room.room_session_keys import (
    build_room_agent_session_key,
    build_room_shared_session_key,
)
from agent.service.session.session_manager import session_manager
from agent.utils.logger import logger
from agent.utils.mention_parser import resolve_mention_agent_ids
from agent.utils.utils import random_uuid

def build_room_session_key(conversation_id: str) -> str:
    """构建 Room 共享 session_key。"""
    return build_room_shared_session_key(conversation_id)


class RoomChatService:
    """Room 内的多 Agent 对话编排。"""

    def __init__(
        self,
        sender: MessageSender,
        permission_strategy: PermissionStrategy,
    ) -> None:
        self._sender = sender
        self._permission_strategy = permission_strategy
        self._db = get_db("async_sqlite")

    async def handle_room_message_with_task(
        self,
        message: Dict[str, Any],
        chat_tasks: Dict[str, Any],
    ) -> None:
        """处理 Room 消息入口 — 管理异步任务。"""
        conversation_id = message.get("conversation_id", "")
        room_session_key = message.get("session_key", "")

        if not room_session_key:
            room_session_key = build_room_session_key(conversation_id)
            message["session_key"] = room_session_key
        message["round_id"] = str(message.get("round_id") or uuid.uuid4())

        # 取消该 Room 会话下的旧整体任务
        task_key = f"room:{room_session_key}"
        if task_key in chat_tasks and not chat_tasks[task_key].done():
            logger.info(f"⚠️ 取消旧 room chat 任务: {task_key}")
            await self._interrupt_previous_room_round(
                room_session_key=room_session_key,
                room_id=message.get("room_id", ""),
                conversation_id=conversation_id,
                task=chat_tasks[task_key],
            )

        task = asyncio.create_task(self._handle_room_message(message, chat_tasks))
        ws_chat_task_registry.register(room_session_key, task, message.get("round_id"))
        chat_tasks[task_key] = task
        task.add_done_callback(
            lambda t: self._on_task_done(task_key, t, chat_tasks)
        )

    async def _handle_room_message(self, message: Dict[str, Any], chat_tasks: Dict[str, Any]) -> None:
        """处理 Room 消息核心逻辑。"""
        room_id = message.get("room_id", "")
        conversation_id = message.get("conversation_id", "")
        content = message.get("content", "")
        round_id = message.get("round_id", str(uuid.uuid4()))
        req_id = message.get("req_id", round_id)
        room_session_key = message["session_key"]

        if not conversation_id:
            await self._send_error(
                session_key=room_session_key,
                error_message="conversation_id is required",
                room_id=room_id or None,
                conversation_id=conversation_id or None,
                caused_by=round_id,
            )
            await self._broadcast_room_event(room_id, build_round_status_event(
                round_id=round_id,
                status="error",
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id or None,
            ))
            return

        # 1. 构建 Room 成员的 name→id 映射
        try:
            agent_name_to_id, room_type = await self._build_room_member_map(room_id, conversation_id)
        except ValueError as exc:
            await self._send_error(
                session_key=room_session_key,
                error_message=str(exc),
                room_id=room_id or None,
                conversation_id=conversation_id,
                caused_by=round_id,
            )
            await self._broadcast_room_event(room_id, build_round_status_event(
                round_id=round_id,
                status="error",
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
            ))
            return
        if not agent_name_to_id:
            await self._send_error(
                session_key=room_session_key,
                error_message="Room has no agent members",
                room_id=room_id or None,
                conversation_id=conversation_id,
                caused_by=round_id,
            )
            await self._broadcast_room_event(room_id, build_round_status_event(
                round_id=round_id,
                status="error",
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
            ))
            return

        # 2. 解析 @mention — 确定要回复的 Agent
        target_agent_ids = resolve_mention_agent_ids(content, agent_name_to_id)
        agent_name_by_id = {
            agent_id: agent_name
            for agent_name, agent_id in agent_name_to_id.items()
        }

        # 没有 @任何人 → 只保存用户消息，不触发回复
        if not target_agent_ids:
            await self._save_user_message_only(room_session_key, content, round_id)
            user_msg = Message(
                message_id=round_id,
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
                agent_id="",
                round_id=round_id,
                role="user",
                content=content,
            )
            await self._broadcast_room_message(room_id, user_msg)
            hint_msg = Message(
                message_id=str(uuid.uuid4()),
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
                agent_id="",
                round_id=round_id,
                role="result",
                # 中文注释：Room 的提示结果也必须遵守统一 result subtype 协议。
                # 这里语义上是一次正常完成的提示，不能再塞一个未定义的 info。
                subtype="success",
                result="请使用 @AgentName 指定要对话的成员",
            )
            await room_message_store.save_message(hint_msg)
            await self._broadcast_room_message(room_id, hint_msg)
            await self._broadcast_room_event(room_id, build_round_status_event(
                round_id=round_id,
                status="finished",
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
                message_id=hint_msg.message_id,
                result_subtype=hint_msg.subtype,
            ))
            return

        # 3. 保存用户消息
        user_msg = Message(
            message_id=round_id,
            session_key=room_session_key,
            room_id=room_id or None,
            conversation_id=conversation_id,
            agent_id="",
            round_id=round_id,
            role="user",
            content=content,
        )
        await room_message_store.save_message(user_msg)
        await self._broadcast_room_message(room_id, user_msg)
        trigger_timestamp_ms = user_msg.timestamp

        # 4. 预分配每个 Agent 的 message_id，构建 pending 列表
        #    task_key 格式: room:{session_key}:{msg_id} (精确到单条消息)
        pending: List[Dict[str, str]] = []
        agent_dispatch_params: List[Tuple[str, SessionRecord, str, str]] = []
        room_sessions = await self._list_room_sessions(conversation_id)

        multi_agent = len(target_agent_ids) > 1
        for agent_id in target_agent_ids:
            room_session = room_sessions.get(agent_id)
            # 自动补齐缺失的 Room 主会话（成员加入后未生成 session、数据迁移等场景）
            if room_session is None:
                room_session = await self._auto_provision_session(
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                )
            if room_session is None:
                await self._send_error(
                    session_key=room_session_key,
                    error_message=f"Room session 不存在且无法自动创建: conversation={conversation_id}, agent={agent_id}",
                    room_id=room_id or None,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    caused_by=round_id,
                )
                continue
            agent_round_id = f"{round_id}:{agent_id}" if multi_agent else round_id
            msg_id = str(uuid.uuid4())
            await room_round_store.start_round(
                session_id=room_session.id,
                round_id=agent_round_id,
                trigger_message_id=round_id,
                started_at_ms=trigger_timestamp_ms,
            )
            await room_message_store.create_pending_message(
                message_id=msg_id,
                session_key=room_session_key,
                agent_id=agent_id,
                round_id=agent_round_id,
                room_session_id=room_session.id,
            )
            pending.append({
                "agent_id": agent_id,
                "msg_id": msg_id,
                "round_id": agent_round_id,
                "status": "pending",
                "timestamp": trigger_timestamp_ms,
            })
            agent_dispatch_params.append((agent_id, room_session, agent_round_id, msg_id))

        if not agent_dispatch_params:
            await self._broadcast_room_event(room_id, build_round_status_event(
                round_id=round_id,
                status="error",
                session_key=room_session_key,
                room_id=room_id or None,
                conversation_id=conversation_id,
            ))
            return

        await self._broadcast_room_event(room_id, build_round_status_event(
            round_id=round_id,
            status="running",
            session_key=room_session_key,
            room_id=room_id or None,
            conversation_id=conversation_id,
        ))

        # 5. 立即下发 chat_ack，前端据此渲染独立占位槽位
        await self._broadcast_room_event(room_id, EventMessage(
            event_type="chat_ack",
            session_key=room_session_key,
            room_id=room_id or None,
            conversation_id=conversation_id,
            caused_by=round_id,
            data={
                "req_id": req_id,
                "round_id": round_id,
                "pending": pending,
            },
        ))

        # 6. 注册 per-msg_id task，并发执行所有 Agent
        async def _run_agent(
            agent_id: str,
            room_session: SessionRecord,
            agent_round_id: str,
            msg_id: str,
        ) -> None:
            agent_task_key = f"room:{room_session_key}:{msg_id}"
            task = asyncio.current_task()
            if task:
                chat_tasks[agent_task_key] = task
                ws_chat_task_registry.register_agent_task(
                    task_key=agent_task_key,
                    agent_id=agent_id,
                    task=task,
                    round_id=agent_round_id,
                )
            try:
                await self._dispatch_to_agent(
                    room_id=room_id,
                    room_session_key=room_session_key,
                    conversation_id=conversation_id,
                    room_session=room_session,
                    agent_id=agent_id,
                    content=content,
                    agent_name_by_id=agent_name_by_id,
                    round_id=agent_round_id,
                    trigger_timestamp_ms=trigger_timestamp_ms,
                    msg_id=msg_id,
                    room_type=room_type,
                )
            except asyncio.CancelledError:
                logger.info(f"🛑 Room agent 任务被取消: {agent_task_key}")
                await room_message_store.mark_message_status(msg_id, "cancelled")
                await room_round_store.finish_round(
                    session_id=room_session.id,
                    round_id=agent_round_id,
                    status="cancelled",
                    finished_at_ms=current_timestamp_ms(),
                )
                await self._broadcast_room_event(room_id, EventMessage(
                    event_type="stream_cancelled",
                    session_key=room_session_key,
                    room_id=room_id or None,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    message_id=msg_id,
                    caused_by=agent_round_id,
                    data={"msg_id": msg_id, "agent_id": agent_id},
                ))
                raise
            except Exception as exc:
                logger.error(f"❌ Room agent 执行失败: agent={agent_id}, error={exc}")
                await self._handle_agent_dispatch_failure(
                    room_id=room_id,
                    room_session_key=room_session_key,
                    conversation_id=conversation_id,
                    room_session=room_session,
                    agent_id=agent_id,
                    round_id=agent_round_id,
                    msg_id=msg_id,
                    error_message=str(exc),
                    room_type=room_type,
                )
            finally:
                ws_chat_task_registry.unregister_agent_task(agent_task_key, task)
                chat_tasks.pop(agent_task_key, None)

        agent_coroutines = [
            _run_agent(agent_id, room_session, agent_round_id, msg_id)
            for agent_id, room_session, agent_round_id, msg_id in agent_dispatch_params
        ]
        results = await asyncio.gather(*agent_coroutines, return_exceptions=True)

        # 记录异常（已取消的不算错误）
        for i, result in enumerate(results):
            if isinstance(result, Exception) and not isinstance(result, asyncio.CancelledError):
                agent_id = agent_dispatch_params[i][0]
                logger.error(f"❌ Room agent 异常: agent={agent_id}, error={result}")

        await self._broadcast_room_event(room_id, build_round_status_event(
            round_id=round_id,
            status="finished",
            session_key=room_session_key,
            room_id=room_id or None,
            conversation_id=conversation_id,
        ))

    async def _dispatch_to_agent(
        self,
        room_id: str,
        room_session_key: str,
        conversation_id: str,
        room_session: SessionRecord,
        agent_id: str,
        content: str,
        agent_name_by_id: Dict[str, str],
        round_id: str,
        trigger_timestamp_ms: int,
        msg_id: str,
        room_type: str,
    ) -> None:
        """向单个 Agent 发起对话并流式返回结果。"""
        sdk_session_key = build_room_agent_session_key(
            conversation_id=conversation_id,
            agent_id=agent_id,
            room_type=room_type,
        )
        async with session_manager.get_lock(sdk_session_key):
            # Room 使用“共享快照 + 私有 workspace”双轨上下文。
            # 这里强制每轮创建 fresh client，禁止沿用旧 SDK 对话历史，
            # 否则共享消息会再次从 SDK 内部历史泄漏成第二真相源。
            client = await agent_runtime.get_or_create_client(
                session_key=sdk_session_key,
                agent_id=agent_id,
                permission_strategy=self._permission_strategy,
                resolved_agent_id=agent_id,
                force_fresh=True,
            )
            logger.info(
                f"📨 Room dispatch: agent={agent_id}, "
                f"round={round_id}, msg_id={msg_id}, conversation={conversation_id}"
            )
            dispatch_query = await room_conversation_orchestrator.build_dispatch_query(
                conversation_id=conversation_id,
                latest_user_message=content,
                trigger_timestamp_ms=trigger_timestamp_ms,
                agent_name_by_id=agent_name_by_id,
                target_agent_id=agent_id,
            )
            self._permission_strategy.bind_session_route(
                sdk_session_key,
                PermissionRouteContext(
                    route_session_key=room_session_key,
                    room_id=room_id or None,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    message_id=msg_id,
                    caused_by=round_id,
                ),
            )

            try:
                # 通知前端：该 Agent 开始生成
                await room_message_store.mark_message_status(msg_id, "streaming")
                await self._broadcast_room_event(room_id, EventMessage(
                    event_type="stream_start",
                    session_key=room_session_key,
                    room_id=room_id or None,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    message_id=msg_id,
                    caused_by=round_id,
                    data={"msg_id": msg_id, "agent_id": agent_id, "round_id": round_id},
                ))

                client = await agent_runtime.query_with_recovery(
                    session_key=sdk_session_key,
                    agent_id=agent_id,
                    permission_strategy=self._permission_strategy,
                    prompt=dispatch_query,
                    client=client,
                    resolved_agent_id=agent_id,
                    force_fresh=True,
                )

                processor = ChatMessageProcessor(
                    session_key=room_session_key,
                    query=content,
                    round_id=round_id,
                    agent_id=agent_id,
                    session_id=room_session.sdk_session_id,
                    persist_message=self._build_room_persist_callback(
                        room_session_id=room_session.id,
                        cost_session_key=sdk_session_key,
                    ),
                    register_session=self._build_room_register_callback(
                        room_session_id=room_session.id,
                        sdk_session_key=sdk_session_key,
                    ),
                )
                processor._is_user_message_saved = True

                async for response_msg in client.receive_messages():
                    processed = await processor.process_messages(response_msg)
                    for msg in processed:
                        await self._broadcast_room_message(
                            room_id,
                            self._with_room_route(
                                message=msg,
                                room_id=room_id or None,
                                conversation_id=conversation_id,
                            )
                        )
                    if processor.subtype in ("success", "error", "interrupted"):
                        break

                # 通知前端：该 Agent 生成结束
                await self._broadcast_room_event(room_id, EventMessage(
                    event_type="stream_end",
                    session_key=room_session_key,
                    room_id=room_id or None,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    message_id=msg_id,
                    caused_by=round_id,
                    data={"msg_id": msg_id, "agent_id": agent_id, "round_id": round_id},
                ))

                logger.info(
                    f"✅ Room agent done: agent={agent_id}, "
                    f"messages={processor.message_count}"
                )
            finally:
                self._permission_strategy.unbind_session_route(sdk_session_key)

    async def _handle_agent_dispatch_failure(
        self,
        room_id: str,
        room_session_key: str,
        conversation_id: str,
        room_session: SessionRecord,
        agent_id: str,
        round_id: str,
        msg_id: str,
        error_message: str,
        room_type: str,
    ) -> None:
        """为单个 Agent 写入错误终态，避免前端和 SQL 悬空。"""
        await room_message_store.mark_message_status(msg_id, "error")
        result_message = Message(
            message_id=random_uuid(),
            session_key=room_session_key,
            room_id=room_id or None,
            conversation_id=conversation_id,
            agent_id=agent_id,
            round_id=round_id,
            session_id=room_session.sdk_session_id,
            parent_id=msg_id,
            role="result",
            subtype="error",
            duration_ms=0,
            duration_api_ms=0,
            num_turns=0,
            total_cost_usd=0,
            result=error_message,
            is_error=True,
        )
        await room_message_store.save_message(
            message=result_message,
            room_session_id=room_session.id,
            cost_session_key=build_room_agent_session_key(
                conversation_id=conversation_id,
                agent_id=agent_id,
                room_type=room_type,
            ),
        )
        await self._broadcast_room_message(room_id, result_message)
        await self._send_error(
            session_key=room_session_key,
            error_message=error_message,
            room_id=room_id or None,
            conversation_id=conversation_id,
            agent_id=agent_id,
            message_id=msg_id,
            caused_by=round_id,
            details={"round_id": round_id},
        )

    async def _build_room_member_map(
        self,
        room_id: str,
        conversation_id: str,
    ) -> Tuple[Dict[str, str], str]:
        """构建 Room 成员 name → agent_id 映射，并返回 room_type。"""
        from agent.infra.database.get_db import get_db
        from agent.infra.database.repositories.room_sql_repository import (
            RoomSqlRepository,
        )

        if room_id:
            try:
                db = get_db("async_sqlite")
                async with db.session() as session:
                    repo = RoomSqlRepository(session)
                    room_agg = await repo.get(room_id)
                    if room_agg:
                        agent_ids = [
                            m.member_agent_id
                            for m in room_agg.members
                            if m.member_type == "agent" and m.member_agent_id
                        ]
                        agents = await AgentManager.get_all_agents()
                        return (
                            {
                                a.name: a.agent_id
                                for a in agents
                                if a.agent_id in agent_ids
                            },
                            room_agg.room.room_type,
                        )
            except Exception as exc:
                logger.warning(f"⚠️ 查询 Room 成员失败: {exc}")

        raise ValueError(f"无法获取 Room 成员列表: room_id={room_id!r}")

    async def _save_user_message_only(
        self,
        session_key: str,
        content: str,
        round_id: str,
    ) -> None:
        """仅保存用户消息（无 Agent 回复时）。"""
        user_msg = Message(
            message_id=round_id,
            session_key=session_key,
            agent_id="",
            round_id=round_id,
            role="user",
            content=content,
        )
        await room_message_store.save_message(user_msg)

    async def _send_error(
        self,
        session_key: str,
        error_message: str,
        room_id: str | None = None,
        conversation_id: str | None = None,
        agent_id: str = "",
        message_id: str | None = None,
        caused_by: str | None = None,
        details: Dict[str, Any] | None = None,
    ) -> None:
        """发送错误事件给前端。"""
        await self._broadcast_room_event(
            room_id,
            build_error_event(
                error_type="room_error",
                message=error_message,
                session_key=session_key,
                room_id=room_id,
                conversation_id=conversation_id,
                agent_id=agent_id,
                message_id=message_id,
                caused_by=caused_by,
                details=details,
            )
        )

    async def _broadcast_room_event(self, room_id: str | None, event: EventMessage) -> None:
        """向订阅了当前 Room 的所有连接广播事件。"""
        from agent.service.channels.ws.ws_connection_registry import ws_connection_registry

        if not room_id:
            await self._sender.send_event_message(event)
            return
        await ws_connection_registry.broadcast_to_room_subscribers(
            room_id,
            event,
            fallback_sender=self._sender,
        )

    async def _broadcast_room_message(
        self,
        room_id: str | None,
        message: Message | StreamMessage,
    ) -> None:
        """向订阅了当前 Room 的所有连接广播消息。"""
        await self._broadcast_room_event(room_id, build_transport_event(message))

    @staticmethod
    def _with_room_route(
        message: Message | StreamMessage,
        room_id: str | None,
        conversation_id: str,
    ) -> Message | StreamMessage:
        """为 Room 消息补齐顶层路由字段。"""
        return message.model_copy(
            update={
                "room_id": room_id,
                "conversation_id": conversation_id,
            }
        )

    @staticmethod
    def _on_task_done(
        task_key: str,
        task: asyncio.Task,
        chat_tasks: Dict[str, Any],
    ) -> None:
        """任务完成回调。"""
        room_session_key = task_key.removeprefix("room:")
        ws_chat_task_registry.unregister(room_session_key, task)
        chat_tasks.pop(task_key, None)
        if task.cancelled():
            logger.info(f"🛑 Room 任务被取消: {task_key}")
        elif task.exception():
            logger.error(
                f"❌ Room 任务异常: {task_key}, error={task.exception()}"
            )
        else:
            logger.debug(f"✅ Room 任务完成: {task_key}")

    async def _interrupt_previous_room_round(
        self,
        room_session_key: str,
        room_id: str,
        conversation_id: str,
        task: asyncio.Task,
    ) -> None:
        """把旧的 Room 主轮次收口为 interrupted。"""
        running_round_id = ws_chat_task_registry.get_running_round_id(room_session_key)
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)

        if not running_round_id:
            return

        repair_result = await room_interrupt_service.repair_cancelled_slots(
            session_key=room_session_key,
            round_id=running_round_id,
        )
        repaired_room_id = repair_result.get("room_id") or room_id or None
        repaired_conversation_id = repair_result.get("conversation_id") or conversation_id
        for slot in repair_result.get("slots") or []:
            await self._broadcast_room_event(repaired_room_id, EventMessage(
                event_type="stream_cancelled",
                session_key=room_session_key,
                room_id=repaired_room_id,
                conversation_id=repaired_conversation_id,
                agent_id=slot.get("agent_id"),
                message_id=slot.get("msg_id"),
                caused_by=slot.get("round_id") or running_round_id,
                data={
                    "msg_id": slot.get("msg_id"),
                    "agent_id": slot.get("agent_id"),
                    "round_id": slot.get("round_id") or running_round_id,
                },
            ))

        await self._broadcast_room_event(repaired_room_id, build_round_status_event(
            round_id=running_round_id,
            status="interrupted",
            session_key=room_session_key,
            room_id=repaired_room_id,
            conversation_id=repaired_conversation_id,
        ))

    async def _get_room_session(
        self,
        conversation_id: str,
        agent_id: str,
    ):
        """读取 Room 成员的主会话。"""
        async with self._db.session() as session:
            repository = SessionSqlRepository(session)
            return await repository.get_primary(
                conversation_id=conversation_id,
                agent_id=agent_id,
            )

    async def _list_room_sessions(self, conversation_id: str) -> Dict[str, SessionRecord]:
        """批量读取当前对话下的主会话，减少重复查询。"""
        async with self._db.session() as session:
            repository = SessionSqlRepository(session)
            sessions = await repository.list_by_conversation(conversation_id)
        return {
            session_record.agent_id: session_record
            for session_record in sessions
            if session_record.is_primary
        }

    async def _auto_provision_session(
        self,
        conversation_id: str,
        agent_id: str,
    ) -> SessionRecord | None:
        """自动为缺失主会话的 Room 成员补齐 session 记录。"""
        try:
            agent_aggregate = await room_agent_runtime_factory.ensure_agent_aggregate(agent_id)
            session_record = room_agent_runtime_factory.build_session_record(
                conversation_id=conversation_id,
                agent=agent_aggregate,
            )
            async with self._db.session() as session:
                repository = SessionSqlRepository(session)
                created = await repository.create(session_record)
                await session.commit()
            logger.info(
                f"🔧 自动补齐 Room session: conversation={conversation_id}, agent={agent_id}"
            )
            return created
        except Exception as exc:
            logger.warning(
                f"⚠️ 自动创建 Room session 失败: conversation={conversation_id}, "
                f"agent={agent_id}, error={exc}"
            )
            return None

    @staticmethod
    def _build_room_persist_callback(
        room_session_id: str,
        cost_session_key: str,
    ):
        """构建 Room 消息落库回调。"""

        async def _persist(message: Message) -> None:
            await room_message_store.save_message(
                message=message,
                room_session_id=room_session_id,
                cost_session_key=cost_session_key,
            )

        return _persist

    @staticmethod
    def _build_room_register_callback(
        room_session_id: str,
        sdk_session_key: str,
    ):
        """构建 Room SDK session 注册回调。"""

        async def _register(_: str, sdk_session_id: str) -> None:
            await room_message_store.register_sdk_session(
                room_session_id=room_session_id,
                sdk_session_key=sdk_session_key,
                sdk_session_id=sdk_session_id,
            )

        return _register
