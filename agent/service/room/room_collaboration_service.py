#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_collaboration_service.py
# @Date   ：2026/3/30 00:00
# @Author ：leemysw
# 2026/3/30 00:00   Create
# =====================================================

"""Room 协作服务 — 处理多 Agent 间的消息通信。"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING

from agent.infra.database.get_db import get_db
from agent.infra.database.repositories.conversation_sql_repository import ConversationSqlRepository
from agent.infra.database.repositories.message_sql_repository import MessageSqlRepository
from agent.infra.database.repositories.room_sql_repository import RoomSqlRepository
from agent.infra.database.repositories.session_sql_repository import SessionSqlRepository
from agent.schema.model_chat_persistence import MessageRecord
from agent.schema.model_message import EventMessage, Message, current_timestamp_ms
from agent.service.room.room_session_keys import build_room_agent_session_key
from agent.service.session.session_repository import session_repository as file_session_repository
from agent.service.session.session_store import session_store
from agent.utils.logger import logger
from agent.utils.utils import random_uuid

if TYPE_CHECKING:
    from agent.service.channels.ws.websocket_sender import WebSocketSender


class RoomCollaborationService:
    """处理 Room 内的多 Agent 协作逻辑。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")
        self._websocket_sender: WebSocketSender | None = None

    def set_websocket_sender(self, sender: WebSocketSender) -> None:
        """设置 WebSocket 发送器用于推送协作消息。"""
        self._websocket_sender = sender

    @staticmethod
    def _to_created_at(timestamp_ms: int) -> datetime:
        """把毫秒时间戳转换为数据库使用的 naive 时间。"""
        return datetime.fromtimestamp(timestamp_ms / 1000)

    async def handle_agent_message(
        self,
        room_id: str,
        conversation_id: str,
        sender_agent_id: str,
        content: str,
    ) -> None:
        """处理 Agent 在 Room 内发送的消息。

        流程：
        1. 验证发送者是否是 Room 成员
        2. 保存消息到各 Agent 会话（作为系统消息）
        3. 通过 WebSocket 广播给所有在线的 Agent 会话

        Args:
            room_id: Room ID
            conversation_id: Conversation ID
            sender_agent_id: 发送消息的 Agent ID
            content: 消息内容
        """
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            message_repository = MessageSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            # 1. 验证 Room 存在
            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                logger.warning(f"Room not found: {room_id}")
                return

            conversation = await conversation_repository.get(conversation_id)
            if conversation is None or conversation.room_id != room_id:
                logger.warning(
                    f"Conversation {conversation_id} not found in room {room_id}"
                )
                return

            # 2. 验证发送者是否是 Room 成员
            agent_members = [
                member
                for member in room_aggregate.members
                if member.member_type == "agent" and member.member_agent_id
            ]

            if not any(
                member.member_agent_id == sender_agent_id
                for member in agent_members
            ):
                logger.warning(
                    f"Agent {sender_agent_id} is not a member of room {room_id}"
                )
                return

            # 3. 为每个 Agent 会话保存消息（写入真实 session 日志 + SQL 索引）
            room_sessions = await session_repository.list_by_conversation(conversation_id)
            primary_session_by_agent = {
                item.agent_id: item
                for item in room_sessions
                if item.is_primary
            }
            timestamp_ms = current_timestamp_ms()
            created_at = self._to_created_at(timestamp_ms)
            for member in agent_members:
                agent_id = member.member_agent_id
                if not agent_id:
                    continue
                room_session = primary_session_by_agent.get(agent_id)
                if room_session is None:
                    logger.warning(
                        "Room collaboration skipped missing session: conversation=%s, agent=%s",
                        conversation_id,
                        agent_id,
                    )
                    continue
                session_key = build_room_agent_session_key(
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    room_type=room_aggregate.room.room_type,
                )
                await session_store.update_session(
                    session_key=session_key,
                    agent_id=agent_id,
                    title=conversation.title or room_aggregate.room.name or "New Chat",
                    options={"room_session_id": room_session.id},
                )
                message_id = random_uuid()
                message_text = content if agent_id == sender_agent_id else f"@{sender_agent_id}: {content}"
                message = Message(
                    message_id=message_id,
                    session_key=session_key,
                    room_id=room_id,
                    conversation_id=conversation_id,
                    agent_id=agent_id,
                    round_id=f"collab:{message_id}",
                    role="assistant" if agent_id == sender_agent_id else "system",
                    timestamp=timestamp_ms,
                    content=message_text,
                    metadata={
                        "room_collaboration": True,
                        "sender_agent_id": sender_agent_id,
                    },
                )
                saved = await session_store.save_message(message)
                if not saved:
                    logger.warning(
                        "Room collaboration failed to persist session log: conversation=%s, agent=%s",
                        conversation_id,
                        agent_id,
                    )
                    continue
                log_path = file_session_repository._find_message_log_path(session_key)
                if not log_path:
                    logger.warning(
                        "Room collaboration missing session log path: conversation=%s, agent=%s",
                        conversation_id,
                        agent_id,
                    )
                    continue
                await message_repository.upsert_message(
                    MessageRecord(
                        id=message_id,
                        conversation_id=conversation_id,
                        session_id=room_session.id,
                        sender_type="agent" if agent_id == sender_agent_id else "system",
                        sender_agent_id=sender_agent_id,
                        kind="text" if agent_id == sender_agent_id else "event",
                        status="completed",
                        content_preview=message_text[:200],
                        jsonl_path=str(log_path),
                        jsonl_offset=None,
                        round_id=message.round_id,
                        created_at=created_at,
                        updated_at=created_at,
                    )
                )
                await session_repository.touch(
                    room_session.id,
                    last_activity_at=created_at,
                )

            await session.commit()

        # 4. 广播消息到所有在线的 Agent 会话（事务提交后执行，避免嵌套 DB session）
        await self.broadcast_to_room_sessions(
            room_id=room_id,
            conversation_id=conversation_id,
            message={
                "type": "agent_message",
                "room_id": room_id,
                "conversation_id": conversation_id,
                "sender_agent_id": sender_agent_id,
                "content": content,
                "timestamp": 0,
            },
        )
        logger.info(
            f"✅ Agent message broadcasted: room={room_id}, "
            f"sender={sender_agent_id}, content={content[:50]}"
        )

    async def broadcast_to_room_sessions(
        self,
        room_id: str,
        conversation_id: str,
        message: dict,
    ) -> None:
        """向 Room 内所有 Agent 会话广播消息。

        Args:
            room_id: Room ID
            conversation_id: Conversation ID
            message: 要广播的消息内容
        """
        if not self._websocket_sender:
            logger.warning("WebSocket sender not set, cannot broadcast message")
            return

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            # 获取 Room 内所有 Agent 会话
            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                return

            conversation = await conversation_repository.get(conversation_id)
            if conversation is None or conversation.room_id != room_id:
                logger.warning(
                    f"Conversation {conversation_id} not found in room {room_id}"
                )
                return

            # 获取所有 Agent 会话
            sessions = await session_repository.list_by_conversation(conversation_id)

            # 为每个 Agent 会话构建 session_key
            for session_record in sessions:
                session_key = build_room_agent_session_key(
                    conversation_id=conversation_id,
                    agent_id=session_record.agent_id,
                    room_type=room_aggregate.room.room_type,
                )

                # 构建协作消息事件
                collaboration_event = {
                    "message_type": str(message.get("type") or "room_broadcast"),
                    "room_id": room_id,
                    "conversation_id": conversation_id,
                    "sender_agent_id": message.get("sender_agent_id"),
                    "content": message.get("content"),
                }

                await self._websocket_sender.send_event_message(
                    EventMessage(
                        event_type="room_collaboration",
                        session_key=session_key,
                        room_id=room_id,
                        conversation_id=conversation_id,
                        agent_id=session_record.agent_id,
                        data=collaboration_event,
                        timestamp=current_timestamp_ms(),
                    )
                )

    async def get_room_session_keys(
        self,
        room_id: str,
        conversation_id: str,
    ) -> list[str]:
        """获取 Room 内指定对话的所有 session_key。

        用于前端切换 Agent 时获取可用的会话。

        Returns:
            Session key 列表
        """
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                return []

            conversation = await conversation_repository.get(conversation_id)
            if conversation is None or conversation.room_id != room_id:
                logger.warning(
                    f"Conversation {conversation_id} not found in room {room_id}"
                )
                return []

            sessions = await session_repository.list_by_conversation(conversation_id)
            return [
                build_room_agent_session_key(
                    conversation_id=conversation_id,
                    agent_id=session_record.agent_id,
                    room_type=room_aggregate.room.room_type,
                )
                for session_record in sessions
            ]


room_collaboration_service = RoomCollaborationService()
