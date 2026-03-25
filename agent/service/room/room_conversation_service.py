# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_conversation_service.py
# @Date   ：2026/03/25 22:40
# @Author ：leemysw
# 2026/03/25 22:40   Create
# =====================================================

"""Room Conversation 应用服务。"""

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
    RoomAggregate,
    SessionRecord,
)
from agent.service.agent.agent_manager import agent_manager
from agent.service.agent.agent_name_policy import AgentNamePolicy
from agent.service.repository.agent_repository_service import (
    agent_persistence_service,
)
from agent.service.repository.repository_service import persistence_service
from agent.storage.sqlite.conversation_sql_repository import ConversationSqlRepository
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.storage.sqlite.session_sql_repository import SessionSqlRepository
from agent.utils.utils import random_uuid


def _stable_id(prefix: str, raw_value: str) -> str:
    """基于稳定输入生成短 ID。"""
    digest = hashlib.sha1(raw_value.encode("utf-8")).hexdigest()[:20]
    return f"{prefix}_{digest}"


class RoomConversationService:
    """负责编排 Room 内部对话线程。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def create_room_conversation(
        self,
        room_id: str,
        title: Optional[str] = None,
    ) -> ConversationContextAggregate:
        """为已有 room 创建一条新对话。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")

            existing_conversations = await conversation_repository.list_by_room(room_id)
            conversation_title = self._build_conversation_title(
                room_aggregate=room_aggregate,
                existing_conversations=existing_conversations,
                title=title,
            )
            created_conversation = await conversation_repository.create(
                ConversationRecord(
                    id=random_uuid(),
                    room_id=room_id,
                    conversation_type="topic",
                    title=conversation_title,
                )
            )
            created_sessions = await self._create_room_sessions(
                conversation_id=created_conversation.id,
                room_aggregate=room_aggregate,
                session_repository=session_repository,
            )
            await session.commit()

        return ConversationContextAggregate(
            room=room_aggregate.room,
            members=room_aggregate.members,
            conversation=created_conversation,
            sessions=created_sessions,
        )

    async def update_room_conversation(
        self,
        room_id: str,
        conversation_id: str,
        title: Optional[str],
    ) -> ConversationContextAggregate:
        """更新 room 内指定对话。"""
        next_title = self._normalize_title(title)
        if next_title is None:
            raise ValueError("对话标题不能为空")

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_repository = SessionSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")

            conversation = await self._get_room_conversation(
                room_id=room_id,
                conversation_id=conversation_id,
                conversation_repository=conversation_repository,
            )
            updated_conversation = await conversation_repository.update_title(
                conversation_id=conversation.id,
                title=next_title,
            )
            if updated_conversation is None:
                raise LookupError("Conversation not found")
            sessions = await session_repository.list_by_conversation(conversation_id)
            await session.commit()

        return ConversationContextAggregate(
            room=room_aggregate.room,
            members=room_aggregate.members,
            conversation=updated_conversation,
            sessions=sessions,
        )

    async def delete_room_conversation(
        self,
        room_id: str,
        conversation_id: str,
    ) -> ConversationContextAggregate:
        """删除 room 内的普通对话，并返回下一条可用上下文。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)

            room_aggregate = await room_repository.get(room_id)
            if room_aggregate is None:
                raise LookupError("Room not found")

            conversations = await conversation_repository.list_by_room(room_id)
            if len(conversations) <= 1:
                raise ValueError("room 至少保留一个对话")

            target_conversation = next(
                (item for item in conversations if item.id == conversation_id),
                None,
            )
            if target_conversation is None:
                raise LookupError("Conversation not found")
            if target_conversation.conversation_type != "topic":
                raise ValueError("主对话不支持删除")

            deleted = await conversation_repository.delete(conversation_id)
            if not deleted:
                raise LookupError("Conversation not found")
            await session.commit()

        contexts = await persistence_service.get_room_contexts(room_id)
        if not contexts:
            raise LookupError("Room not found")
        return self._pick_fallback_context(contexts)

    async def _create_room_sessions(
        self,
        conversation_id: str,
        room_aggregate: RoomAggregate,
        session_repository: SessionSqlRepository,
    ) -> list[SessionRecord]:
        """为 room 中每个 agent 成员补齐运行时会话。"""
        created_sessions: list[SessionRecord] = []
        for member in room_aggregate.members:
            if member.member_type != "agent" or not member.member_agent_id:
                continue
            agent_aggregate = await self._ensure_agent_aggregate(member.member_agent_id)
            created_sessions.append(
                await session_repository.create(
                    self._build_session_record(
                        conversation_id=conversation_id,
                        agent=agent_aggregate,
                    )
                )
            )
        return created_sessions

    async def _get_room_conversation(
        self,
        room_id: str,
        conversation_id: str,
        conversation_repository: ConversationSqlRepository,
    ) -> ConversationRecord:
        """确保对话存在且属于指定 room。"""
        conversation = await conversation_repository.get(conversation_id)
        if conversation is None or conversation.room_id != room_id:
            raise LookupError("Conversation not found")
        return conversation

    async def _ensure_agent_aggregate(self, agent_id: str) -> AgentAggregate:
        """确保 agent 已同步到持久化层。"""
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

    def _build_session_record(
        self,
        conversation_id: str,
        agent: AgentAggregate,
    ) -> SessionRecord:
        """为对话构造默认主会话。"""
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

    def _build_conversation_title(
        self,
        room_aggregate: RoomAggregate,
        existing_conversations: list[ConversationRecord],
        title: Optional[str],
    ) -> str:
        """推导新对话标题。"""
        normalized_title = self._normalize_title(title)
        if normalized_title:
            return normalized_title

        topic_count = sum(
            1 for conversation in existing_conversations
            if conversation.conversation_type == "topic"
        )
        base_name = room_aggregate.room.name or "未命名 room"
        return f"{base_name} · 对话 {topic_count + 1}"

    def _normalize_title(self, title: Optional[str]) -> Optional[str]:
        """规范化标题输入。"""
        if title is None:
            return None
        normalized = title.strip()
        return normalized or None

    def _pick_fallback_context(
        self,
        contexts: list[ConversationContextAggregate],
    ) -> ConversationContextAggregate:
        """删除对话后优先回到主对话。"""
        for context in contexts:
            if context.conversation.conversation_type in {"room_main", "dm"}:
                return context
        return contexts[0]


room_conversation_service = RoomConversationService()
