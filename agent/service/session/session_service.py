# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""会话应用服务。"""

from datetime import datetime, timezone

from sqlalchemy import func, select

from agent.infra.database.get_db import get_db
from agent.infra.database.models.conversation import Conversation as ConversationEntity
from agent.infra.database.models.message import Message as MessageEntity
from agent.infra.database.models.room import Room as RoomEntity
from agent.infra.database.models.session import Session as SessionEntity
from agent.service.room.room_message_store import room_message_store
from agent.service.room.room_session_keys import (
    build_room_agent_session_key,
    is_room_shared_session_key,
)
from agent.service.session.cost_repository import cost_repository
from typing import List, Optional

from agent.service.session.session_manager import session_manager
from agent.service.session.session_router import (
    get_default_agent_id,
    require_structured_session_key,
)
from agent.schema.model_cost import SessionCostSummary
from agent.schema.model_message import Message
from agent.schema.model_session import ASession
from agent.service.session.session_context_resolver import session_context_resolver
from agent.service.session.session_store import session_store


class SessionService:
    """负责 Session 相关应用编排。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    @staticmethod
    def _ensure_utc_datetime(value: datetime) -> datetime:
        """把时间统一转换为 UTC，避免 naive/aware 混用导致排序失败。"""
        if value.tzinfo is None or value.tzinfo.utcoffset(value) is None:
            return value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc)

    def _normalize_session_datetimes(self, session_info: ASession) -> ASession:
        """规范化会话时间字段，保证 API 输出稳定。"""
        return session_info.model_copy(
            update={
                "created_at": self._ensure_utc_datetime(session_info.created_at),
                "last_activity": self._ensure_utc_datetime(session_info.last_activity),
            }
        )

    def to_session_key(self, session_key: str) -> str:
        """要求 Session API 只接受结构化 session_key。"""
        return require_structured_session_key(session_key)

    async def get_sessions(self) -> List[ASession]:
        """获取所有会话列表。"""
        sessions = [
            self._normalize_session_datetimes(session)
            for session in await session_store.get_all_sessions()
            if not session.room_session_id
            and not is_room_shared_session_key(session.session_key)
        ]
        room_sessions = [
            self._normalize_session_datetimes(session)
            for session in await self._list_room_sessions()
        ]
        all_sessions = [*sessions, *room_sessions]
        all_sessions.sort(key=lambda item: item.last_activity.timestamp(), reverse=True)
        return await session_context_resolver.enrich_sessions(all_sessions)

    async def create_session(
        self,
        session_key: str,
        agent_id: Optional[str] = None,
        title: Optional[str] = "New Chat",
    ) -> ASession:
        """创建会话。"""
        internal_key = self.to_session_key(session_key)
        existing = await session_store.get_session_info(internal_key)
        if existing:
            raise ValueError("Session already exists")

        success = await session_store.update_session(
            session_key=internal_key,
            agent_id=agent_id or get_default_agent_id(),
            title=title,
        )
        if not success:
            raise RuntimeError("Failed to create session")

        session_info = await session_store.get_session_info(internal_key)
        if not session_info:
            raise RuntimeError("Failed to retrieve created session")
        return await session_context_resolver.enrich_session(session_info)

    async def update_session(
        self,
        session_key: str,
        title: Optional[str] = None,
    ) -> ASession:
        """更新会话信息。"""
        internal_key = self.to_session_key(session_key)
        existing = await session_store.get_session_info(internal_key)
        if not existing:
            raise LookupError("Session not found")

        success = await session_store.update_session(
            session_key=internal_key,
            title=title,
        )
        if not success:
            raise RuntimeError("Failed to update session")

        updated = await session_store.get_session_info(internal_key)
        if not updated:
            raise RuntimeError("Failed to retrieve updated session")
        return await session_context_resolver.enrich_session(updated)

    async def get_session_messages(self, session_key: str) -> list[Message]:
        """获取会话历史消息。"""
        internal_key = self.to_session_key(session_key)
        if is_room_shared_session_key(internal_key):
            return await room_message_store.get_messages(internal_key)
        return await session_store.get_session_messages(internal_key)

    async def get_session_cost_summary(self, session_key: str) -> SessionCostSummary:
        """获取会话成本汇总。"""
        internal_key = self.to_session_key(session_key)
        session_info = await session_store.get_session_info(internal_key)
        if session_info:
            return await session_store.get_session_cost_summary(internal_key)
        return await cost_repository.get_session_cost_summary(internal_key)

    async def delete_session(self, session_key: str) -> None:
        """删除会话。"""
        internal_key = self.to_session_key(session_key)
        session_manager.remove_session(internal_key)

        success = await session_store.delete_session(internal_key)
        if not success:
            raise LookupError("Session not found")

    async def _list_room_sessions(self) -> list[ASession]:
        """从 SQL 组装 Room 会话视图。"""
        message_count_subquery = (
            select(
                MessageEntity.conversation_id.label("conversation_id"),
                func.count(MessageEntity.id).label("message_count"),
            )
            .group_by(MessageEntity.conversation_id)
            .subquery()
        )
        stmt = (
            select(
                SessionEntity,
                ConversationEntity,
                RoomEntity,
                func.coalesce(message_count_subquery.c.message_count, 0),
            )
            .join(
                ConversationEntity,
                SessionEntity.conversation_id == ConversationEntity.id,
            )
            .join(RoomEntity, ConversationEntity.room_id == RoomEntity.id)
            .outerjoin(
                message_count_subquery,
                message_count_subquery.c.conversation_id == ConversationEntity.id,
            )
            .where(SessionEntity.is_primary.is_(True))
            .order_by(SessionEntity.last_activity_at.desc())
        )
        async with self._db.session() as session:
            result = await session.execute(stmt)
            room_sessions: list[ASession] = []
            for sql_session, conversation, room, message_count in result.all():
                created_at = self._ensure_utc_datetime(sql_session.created_at)
                last_activity_at = self._ensure_utc_datetime(
                    sql_session.last_activity_at
                )
                room_sessions.append(
                    ASession(
                        session_key=build_room_agent_session_key(
                            conversation_id=conversation.id,
                            agent_id=sql_session.agent_id,
                            room_type=room.room_type,
                        ),
                        agent_id=sql_session.agent_id,
                        session_id=sql_session.sdk_session_id,
                        room_session_id=sql_session.id,
                        room_id=room.id,
                        conversation_id=conversation.id,
                        channel_type="ws",
                        chat_type="dm" if room.room_type == "dm" else "group",
                        status=sql_session.status,
                        created_at=created_at,
                        last_activity=last_activity_at,
                        title=conversation.title or room.name or "New Chat",
                        message_count=int(message_count or 0),
                        options={},
                    )
                )
            return room_sessions


session_service = SessionService()
