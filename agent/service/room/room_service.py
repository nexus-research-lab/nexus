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

import hashlib
import json
from typing import Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_agent_persistence import (
    AgentAggregate,
    AgentRecord,
    CreateAgentAggregate,
    ProfileRecord,
    RuntimeRecord,
)
from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    ConversationRecord,
    MemberRecord,
    RoomAggregate,
    RoomRecord,
    SessionRecord,
)
from agent.service.agent.agent_name_policy import AgentNamePolicy
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.agent.agent_manager import agent_manager
from agent.service.repository.agent_repository_service import (
    agent_persistence_service,
)
from agent.service.repository.repository_service import persistence_service
from agent.storage.sqlite.conversation_sql_repository import ConversationSqlRepository
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.storage.sqlite.session_sql_repository import SessionSqlRepository
from agent.utils.utils import random_uuid

_LOCAL_USER_ID = "local-user"


def _stable_id(prefix: str, raw_value: str) -> str:
    """基于稳定输入生成短 ID。"""
    digest = hashlib.sha1(raw_value.encode("utf-8")).hexdigest()[:20]
    return f"{prefix}_{digest}"


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

    async def update_room(
        self,
        room_id: str,
        name: Optional[str] = None,
        description: Optional[str] = None,
        title: Optional[str] = None,
    ) -> ConversationContextAggregate:
        """更新房间与主对话信息。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")

            updated_room = await room_repository.update_room(
                room_id=room_id,
                name=name,
                description=description,
            )
            if updated_room is None:
                raise LookupError("Room not found")

            conversations = await conversation_repository.list_by_room(room_id)
            main_conversation = self._pick_main_conversation(conversations)
            if main_conversation is None:
                raise ValueError("Room conversation not found")
            if title is not None:
                updated_conversation = await conversation_repository.update_title(
                    conversation_id=main_conversation.id,
                    title=title,
                )
                if updated_conversation is not None:
                    main_conversation = updated_conversation

            await session.commit()

        contexts = await persistence_service.get_room_contexts(room_id)
        if not contexts:
            raise LookupError("Room not found")
        return self._pick_context_by_conversation(contexts, main_conversation.id)

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
        if MainAgentProfile.is_main_agent(agent_id):
            raise ValueError("main agent 不能加入 room")

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
            if not conversations:
                conversations = [
                    await conversation_repository.create(
                        ConversationRecord(
                            id=random_uuid(),
                            room_id=room_id,
                            conversation_type="room_main",
                            title=room_aggregate.room.name,
                        )
                    )
                ]

            created_sessions: list[SessionRecord] = []
            for conversation in conversations:
                primary_session = await session_repository.get_primary(
                    conversation_id=conversation.id,
                    agent_id=agent_id,
                )
                if primary_session is not None:
                    created_sessions.append(primary_session)
                    continue
                created_sessions.append(
                    await session_repository.create(
                        self._build_session_record(
                            conversation_id=conversation.id,
                            agent=agent_aggregate,
                        )
                    )
                )

            await session.commit()

        main_conversation = self._pick_main_conversation(conversations)
        if main_conversation is None:
            raise ValueError("Room conversation not found")
        return ConversationContextAggregate(
            room=room_aggregate.room,
            members=[*room_aggregate.members, created_member],
            conversation=main_conversation,
            sessions=[
                session
                for session in created_sessions
                if session.conversation_id == main_conversation.id
            ],
        )

    async def remove_agent_member(
        self,
        room_id: str,
        agent_id: str,
    ) -> ConversationContextAggregate:
        """移除房间中的 Agent 成员。"""
        if MainAgentProfile.is_main_agent(agent_id):
            raise ValueError("main agent 不能作为 room 成员")

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")
            if room_aggregate.room.room_type != "room":
                raise ValueError("DM room does not support removing members")

            agent_members = [
                member
                for member in room_aggregate.members
                if member.member_type == "agent" and member.member_agent_id
            ]
            if len(agent_members) <= 1:
                raise ValueError("Room 至少保留一个 agent 成员")

            removed_member = await room_repository.remove_agent_member(
                room_id=room_id,
                agent_id=agent_id,
            )
            if removed_member is None:
                raise LookupError("Room member not found")

            conversations = await conversation_repository.list_by_room(room_id)
            main_conversation = self._pick_main_conversation(conversations)
            if main_conversation is None:
                raise ValueError("Room conversation not found")

            for conversation in conversations:
                primary_session = await session_repository.get_primary(
                    conversation_id=conversation.id,
                    agent_id=agent_id,
                )
                if primary_session is not None:
                    await session_repository.delete(primary_session.id)

            await session.commit()

        contexts = await persistence_service.get_room_contexts(room_id)
        if not contexts:
            raise LookupError("Room not found")
        context = self._pick_context_by_conversation(contexts, main_conversation.id)
        context.members = [
            member for member in context.members if member.id != removed_member.id
        ]
        context.sessions = [
            session for session in context.sessions if session.agent_id != agent_id
        ]
        return context

    async def delete_room(self, room_id: str) -> None:
        """删除房间。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            deleted = await room_repository.delete_room(room_id)
            if not deleted:
                raise LookupError("Room not found")
            await session.commit()

    def _normalize_agent_ids(self, agent_ids: list[str]) -> list[str]:
        """去重并保持输入顺序。"""
        normalized_ids: list[str] = []
        for agent_id in agent_ids:
            cleaned = agent_id.strip()
            if MainAgentProfile.is_main_agent(cleaned):
                continue
            if cleaned and cleaned not in normalized_ids:
                normalized_ids.append(cleaned)
        if not normalized_ids:
            raise ValueError("room 至少需要一个普通成员 agent，main agent 不能作为 room 成员")
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

        options = agent.options.model_dump(exclude_none=True)
        slug = AgentNamePolicy.build_workspace_dir_name(agent.name)
        create_payload = CreateAgentAggregate(
            agent=AgentRecord(
                id=agent.agent_id,
                slug=slug,
                name=agent.name,
                description="",
                definition="",
                status=agent.status,
                workspace_path=agent.workspace_path,
            ),
            profile=ProfileRecord(
                id=_stable_id("profile", agent.agent_id),
                agent_id=agent.agent_id,
                display_name=agent.name,
                headline="",
                profile_markdown="",
            ),
            runtime=RuntimeRecord(
                id=_stable_id("runtime", agent.agent_id),
                agent_id=agent.agent_id,
                model=options.get("model"),
                permission_mode=options.get("permission_mode"),
                allowed_tools_json=json.dumps(options.get("allowed_tools") or [], ensure_ascii=False),
                disallowed_tools_json=json.dumps(options.get("disallowed_tools") or [], ensure_ascii=False),
                mcp_servers_json=json.dumps(options.get("mcp_servers") or {}, ensure_ascii=False),
                max_turns=options.get("max_turns"),
                max_thinking_tokens=options.get("max_thinking_tokens"),
                skills_enabled=bool(options.get("skills_enabled", False)),
                setting_sources_json=json.dumps(options.get("setting_sources") or [], ensure_ascii=False),
                runtime_version=1,
            ),
        )
        return await agent_persistence_service.create_agent_aggregate(create_payload)

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
                member_user_id=_LOCAL_USER_ID,
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

    def _pick_context_by_conversation(
        self,
        contexts: list[ConversationContextAggregate],
        conversation_id: str,
    ) -> ConversationContextAggregate:
        """根据对话 ID 选中上下文。"""
        for context in contexts:
            if context.conversation.id == conversation_id:
                return context
        return contexts[0]


room_service = RoomService()
