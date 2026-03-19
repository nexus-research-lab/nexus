# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：query_service.py
# @Date   ：2026/3/19 00:26
# @Author ：leemysw
# 2026/3/19 00:26   Create
# =====================================================

"""新持久化层查询服务。"""

from __future__ import annotations

from typing import Optional

from agent.schema.model_agent_persistence import AgentAggregate
from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    ConversationRecord,
    MessageRecord,
    RoomAggregate,
    RoundRecord,
    SessionRecord,
)
from agent.service.persistence.agent_persistence_service import agent_persistence_service
from agent.service.persistence.conversation_persistence_service import (
    conversation_persistence_service,
)
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.storage.sqlite.message_sql_repository import MessageSqlRepository
from agent.infra.database.get_db import get_db


class PersistenceQueryService:
    """面向新数据库的只读查询服务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def get_agent(self, agent_id: str) -> Optional[AgentAggregate]:
        """读取单个 Agent 聚合。"""
        return await agent_persistence_service.get_agent_aggregate(agent_id)

    async def list_active_agents(self) -> list[AgentAggregate]:
        """列出活跃 Agent 聚合。"""
        return await agent_persistence_service.list_active_agents()

    async def list_recent_rooms(self, limit: int = 20) -> list[RoomAggregate]:
        """列出最近房间。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            return await repository.list_recent(limit=limit)

    async def get_room(self, room_id: str) -> Optional[RoomAggregate]:
        """读取单个房间聚合。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            return await repository.get(room_id)

    async def get_room_contexts(
        self,
        room_id: str,
    ) -> list[ConversationContextAggregate]:
        """读取房间下的全部对话上下文。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room_aggregate = await repository.get(room_id)
        if room_aggregate is None:
            return []

        conversations = await conversation_persistence_service.get_room_conversations(room_id)
        contexts: list[ConversationContextAggregate] = []
        for conversation in conversations:
            sessions = await conversation_persistence_service.get_conversation_sessions(
                conversation.id,
            )
            contexts.append(
                ConversationContextAggregate(
                    room=room_aggregate.room,
                    members=room_aggregate.members,
                    conversation=conversation,
                    sessions=sessions,
                )
            )
        return contexts

    async def get_conversation_sessions(
        self,
        conversation_id: str,
    ) -> list[SessionRecord]:
        """读取对话下的全部运行时会话。"""
        return await conversation_persistence_service.get_conversation_sessions(
            conversation_id,
        )

    async def get_session_messages(
        self,
        session_id: str,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """读取会话下的消息索引。"""
        async with self._db.session() as session:
            repository = MessageSqlRepository(session)
            return await repository.list_by_session(session_id, limit=limit)

    async def get_session_rounds(self, session_id: str) -> list[RoundRecord]:
        """读取会话下的轮次索引。"""
        async with self._db.session() as session:
            repository = MessageSqlRepository(session)
            return await repository.list_rounds(session_id)


persistence_query_service = PersistenceQueryService()
