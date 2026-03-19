# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：conversation_persistence_service.py
# @Date   ：2026/3/19 00:12
# @Author ：leemysw
# 2026/3/19 00:12   Create
# =====================================================

"""Room / Conversation 持久化服务。"""

from __future__ import annotations

from typing import Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    ConversationRecord,
    MemberRecord,
    RoomRecord,
    SessionRecord,
)
from agent.storage.sqlite.conversation_sql_repository import ConversationSqlRepository
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.storage.sqlite.session_sql_repository import SessionSqlRepository


class ConversationPersistenceService:
    """负责编排房间与会话相关事务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def create_dm_context(
        self,
        room: RoomRecord,
        members: list[MemberRecord],
        conversation: ConversationRecord,
        session_record: SessionRecord,
    ) -> ConversationContextAggregate:
        """创建 1v1 房间上下文。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.create(room, members)
            created_conversation = await conversation_repository.create(conversation)
            created_session = await session_repository.create(session_record)
            await session.commit()

            return ConversationContextAggregate(
                room=room_aggregate.room,
                members=room_aggregate.members,
                conversation=created_conversation,
                sessions=[created_session],
            )

    async def get_conversation_sessions(
        self,
        conversation_id: str,
    ) -> list[SessionRecord]:
        """列出对话下的全部运行时会话。"""
        async with self._db.session() as session:
            repository = SessionSqlRepository(session)
            return await repository.list_by_conversation(conversation_id)

    async def get_room_conversations(
        self,
        room_id: str,
    ) -> list[ConversationRecord]:
        """列出房间下的全部对话。"""
        async with self._db.session() as session:
            repository = ConversationSqlRepository(session)
            return await repository.list_by_room(room_id)

    async def touch_session(
        self,
        session_id: str,
    ) -> Optional[SessionRecord]:
        """刷新指定会话的最近活动时间。"""
        async with self._db.session() as session:
            repository = SessionSqlRepository(session)
            record = await repository.touch(session_id)
            if record is None:
                return None
            await session.commit()
            return record


conversation_persistence_service = ConversationPersistenceService()
