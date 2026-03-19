# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_service.py
# @Date   ：2026/03/19 22:10
# @Author ：leemysw
# 2026/03/19 22:10   Create
# =====================================================

"""Room 应用服务。"""

from __future__ import annotations

from typing import Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_agent_persistence import AgentAggregate
from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    ConversationRecord,
    MemberRecord,
    RoomAggregate,
    RoomRecord,
    SessionRecord,
)
from agent.service.agent.agent_manager import agent_manager
from agent.service.persistence.agent_persistence_service import (
    agent_persistence_service,
)
from agent.service.persistence.legacy_sync_bridge import (
    LOCAL_USER_ID,
    build_agent_aggregate_from_legacy,
)
from agent.service.persistence.persistence_service import persistence_service
from agent.storage.sqlite.conversation_sql_repository import ConversationSqlRepository
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.storage.sqlite.session_sql_repository import SessionSqlRepository
from agent.utils.utils import random_uuid


class RoomService:
    """负责编排 Room 相关用例。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def list_rooms(self, limit: int = 20) -> list[RoomAggregate]:
        """列出最近房间。"""
        return await persistence_service.list_rooms(limit=limit)

    async def get_room(self, room_id: str) -> RoomAggregate:
        """读取单个房间。"""
        room = await persistence_service.get_room(room_id)
        if room is None:
            raise LookupError("Room not found")
        return room

    async def get_room_contexts(self, room_id: str) -> list[ConversationContextAggregate]:
        """读取房间上下文。"""
        contexts = await persistence_service.get_room_contexts(room_id)
        if not contexts:
            raise LookupError("Room not found")
        return contexts

    async def create_room(
        self,
        agent_ids: list[str],
        name: Optional[str] = None,
        description: str = "",
        title: Optional[str] = None,
    ) -> ConversationContextAggregate:
        """创建 Room，并为每个 Agent 初始化会话。"""
        normalized_agent_ids = self._normalize_agent_ids(agent_ids)
        agent_aggregates = await self._load_agent_aggregates(normalized_agent_ids)

        room_id = random_uuid()
        conversation_id = random_uuid()
        room_type = "dm" if len(normalized_agent_ids) == 1 else "room"
        room_name = name or self._build_room_name(agent_aggregates, room_type)
        conversation_title = title or room_name

        room_record = RoomRecord(
            id=room_id,
            room_type=room_type,
            name=room_name,
            description=description,
        )
        members = self._build_members(room_id, normalized_agent_ids)
        conversation_record = ConversationRecord(
            id=conversation_id,
            room_id=room_id,
            conversation_type="dm" if room_type == "dm" else "room_main",
            title=conversation_title,
        )

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.create(room_record, members)
            created_conversation = await conversation_repository.create(conversation_record)
            sessions = []
            for aggregate in agent_aggregates:
                sessions.append(
                    await session_repository.create(
                        self._build_session_record(
                            conversation_id=conversation_id,
                            agent=aggregate,
                        )
                    )
                )
            await session.commit()

        return ConversationContextAggregate(
            room=room_aggregate.room,
            members=room_aggregate.members,
            conversation=created_conversation,
            sessions=sessions,
        )

    async def add_agent_member(
        self,
        room_id: str,
        agent_id: str,
    ) -> ConversationContextAggregate:
        """向群房间追加 Agent 成员，并为其创建运行时会话。"""
        agent_aggregate = await self._ensure_agent_aggregate(agent_id)

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")
            if room_aggregate.room.room_type != "room":
                raise ValueError("DM room does not support adding members")

            existing_agent_ids = {
                member.member_agent_id
                for member in room_aggregate.members
                if member.member_type == "agent"
            }
            if agent_id in existing_agent_ids:
                raise ValueError("Agent already exists in room")

            member_record = MemberRecord(
                id=random_uuid(),
                room_id=room_id,
                member_type="agent",
                member_agent_id=agent_id,
            )
            created_member = await room_repository.add_member(member_record)

            conversations = await conversation_repository.list_by_room(room_id)
            main_conversation = self._pick_main_conversation(conversations)
            if main_conversation is None:
                main_conversation = await conversation_repository.create(
                    ConversationRecord(
                        id=random_uuid(),
                        room_id=room_id,
                        conversation_type="room_main",
                        title=room_aggregate.room.name,
                    )
                )

            primary_session = await session_repository.get_primary(
                conversation_id=main_conversation.id,
                agent_id=agent_id,
            )
            if primary_session is None:
                primary_session = await session_repository.create(
                    self._build_session_record(
                        conversation_id=main_conversation.id,
                        agent=agent_aggregate,
                    )
                )

            await session.commit()

        return ConversationContextAggregate(
            room=room_aggregate.room,
            members=[*room_aggregate.members, created_member],
            conversation=main_conversation,
            sessions=[primary_session],
        )

    def _normalize_agent_ids(self, agent_ids: list[str]) -> list[str]:
        """去重并保持输入顺序。"""
        normalized_ids: list[str] = []
        for agent_id in agent_ids:
            cleaned = agent_id.strip()
            if cleaned and cleaned not in normalized_ids:
                normalized_ids.append(cleaned)
        if not normalized_ids:
            raise ValueError("At least one agent is required")
        return normalized_ids

    async def _load_agent_aggregates(
        self,
        agent_ids: list[str],
    ) -> list[AgentAggregate]:
        """批量读取 Agent 聚合。"""
        aggregates = []
        for agent_id in agent_ids:
            aggregates.append(await self._ensure_agent_aggregate(agent_id))
        return aggregates

    async def _ensure_agent_aggregate(self, agent_id: str) -> AgentAggregate:
        """确保 Agent 已同步到新库。"""
        agent = await agent_manager.get_agent(agent_id)
        if agent is None:
            raise LookupError(f"Agent not found: {agent_id}")

        aggregate = await agent_persistence_service.get_agent_aggregate(agent_id)
        if aggregate is not None:
            return aggregate
        return await agent_persistence_service.create_agent_aggregate(
            build_agent_aggregate_from_legacy(agent)
        )

    def _build_room_name(
        self,
        agent_aggregates: list[AgentAggregate],
        room_type: str,
    ) -> str:
        """根据成员推导默认房间名称。"""
        if room_type == "dm":
            return agent_aggregates[0].profile.display_name
        return "、".join(item.profile.display_name for item in agent_aggregates)

    def _build_members(self, room_id: str, agent_ids: list[str]) -> list[MemberRecord]:
        """构造房间成员列表。"""
        members = [
            MemberRecord(
                id=random_uuid(),
                room_id=room_id,
                member_type="user",
                member_user_id=LOCAL_USER_ID,
            )
        ]
        for agent_id in agent_ids:
            members.append(
                MemberRecord(
                    id=random_uuid(),
                    room_id=room_id,
                    member_type="agent",
                    member_agent_id=agent_id,
                )
            )
        return members

    def _build_session_record(
        self,
        conversation_id: str,
        agent: AgentAggregate,
    ) -> SessionRecord:
        """为房间里的 Agent 初始化主会话。"""
        return SessionRecord(
            id=random_uuid(),
            conversation_id=conversation_id,
            agent_id=agent.agent.id,
            runtime_id=agent.runtime.id,
            version_no=1,
            branch_key="main",
            is_primary=True,
            status="active",
        )

    def _pick_main_conversation(
        self,
        conversations: list[ConversationRecord],
    ) -> Optional[ConversationRecord]:
        """优先选择主对话，避免邀请成员时挂错上下文。"""
        for conversation in conversations:
            if conversation.conversation_type == "room_main":
                return conversation
        return conversations[0] if conversations else None


room_service = RoomService()
