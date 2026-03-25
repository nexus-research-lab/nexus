# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：repository_service.py
# @Date   ：2026/3/19 00:34
# @Author ：leemysw
# 2026/3/19 00:34   Create
# =====================================================

"""持久化应用服务。"""

from __future__ import annotations

from typing import Optional

from agent.schema.model_agent_persistence import AgentAggregate
from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    MessageRecord,
    RoomAggregate,
    RoundRecord,
)
from agent.service.repository.query_service import persistence_query_service


class PersistenceService:
    """负责编排新持久化层查询。"""

    async def list_agents(self) -> list[AgentAggregate]:
        """列出新库中的活跃 Agent。"""
        return await persistence_query_service.list_active_agents()

    async def get_agent(self, agent_id: str) -> Optional[AgentAggregate]:
        """读取单个 Agent 聚合。"""
        return await persistence_query_service.get_agent(agent_id)

    async def list_rooms(self, limit: int = 20) -> list[RoomAggregate]:
        """列出最近房间。"""
        return await persistence_query_service.list_recent_rooms(limit=limit)

    async def get_room(self, room_id: str) -> Optional[RoomAggregate]:
        """读取单个房间。"""
        return await persistence_query_service.get_room(room_id)

    async def get_room_contexts(
        self,
        room_id: str,
    ) -> list[ConversationContextAggregate]:
        """读取房间上下文。"""
        return await persistence_query_service.get_room_contexts(room_id)

    async def get_session_messages(
        self,
        session_id: str,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """读取会话下的消息索引。"""
        return await persistence_query_service.get_session_messages(session_id, limit=limit)

    async def get_session_rounds(self, session_id: str) -> list[RoundRecord]:
        """读取会话下的轮次索引。"""
        return await persistence_query_service.get_session_rounds(session_id)



persistence_service = PersistenceService()
