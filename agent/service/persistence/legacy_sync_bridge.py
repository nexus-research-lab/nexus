# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：legacy_sync_bridge.py
# @Date   ：2026/3/19 00:20
# @Author ：leemysw
# 2026/3/19 00:20   Create
# =====================================================

"""旧模型到新持久化模型的同步桥接。"""

from __future__ import annotations

import hashlib
import json
from typing import Optional

from agent.schema.model_agent import AAgent
from agent.schema.model_agent_persistence import (
    AgentRecord,
    CreateAgentAggregate,
    ProfileRecord,
    RuntimeRecord,
)
from agent.schema.model_chat_persistence import (
    ConversationRecord,
    MemberRecord,
    RoomRecord,
    SessionRecord,
)
from agent.schema.model_session import ASession
from agent.service.agent.agent_name_policy import AgentNamePolicy

LOCAL_USER_ID = "local-user"


def _stable_id(prefix: str, raw_value: str) -> str:
    """基于稳定输入生成短 ID。"""
    digest = hashlib.sha1(raw_value.encode("utf-8")).hexdigest()[:20]
    return f"{prefix}_{digest}"


def build_agent_aggregate_from_legacy(agent: AAgent) -> CreateAgentAggregate:
    """将旧 Agent 模型映射为新聚合模型。"""
    options = agent.options.model_dump(exclude_none=True)
    slug = AgentNamePolicy.build_workspace_dir_name(agent.name)
    return CreateAgentAggregate(
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
            disallowed_tools_json=json.dumps(
                options.get("disallowed_tools") or [],
                ensure_ascii=False,
            ),
            mcp_servers_json=json.dumps(options.get("mcp_servers") or {}, ensure_ascii=False),
            max_turns=options.get("max_turns"),
            max_thinking_tokens=options.get("max_thinking_tokens"),
            skills_enabled=bool(options.get("skills_enabled", False)),
            setting_sources_json=json.dumps(
                options.get("setting_sources") or [],
                ensure_ascii=False,
            ),
            runtime_version=1,
        ),
    )


def build_dm_context_from_legacy(
    session_info: ASession,
    runtime_id: str,
    user_id: str = LOCAL_USER_ID,
) -> tuple[RoomRecord, list[MemberRecord], ConversationRecord, SessionRecord]:
    """将旧 Session 模型映射为 1v1 Room 上下文。"""
    room_id = _stable_id("room", session_info.session_key)
    conversation_id = _stable_id("conv", session_info.session_key)
    persistent_session_id = _stable_id("sess", session_info.session_key)

    room = RoomRecord(
        id=room_id,
        room_type="dm",
        name=session_info.title,
        description="",
    )
    members = [
        MemberRecord(
            id=_stable_id("member", f"{room_id}:user:{user_id}"),
            room_id=room_id,
            member_type="user",
            member_user_id=user_id,
        ),
        MemberRecord(
            id=_stable_id("member", f"{room_id}:agent:{session_info.agent_id}"),
            room_id=room_id,
            member_type="agent",
            member_agent_id=session_info.agent_id,
        ),
    ]
    conversation = ConversationRecord(
        id=conversation_id,
        room_id=room_id,
        conversation_type="dm",
        title=session_info.title,
    )
    session_record = SessionRecord(
        id=persistent_session_id,
        conversation_id=conversation_id,
        agent_id=session_info.agent_id,
        runtime_id=runtime_id,
        version_no=1,
        branch_key="main",
        is_primary=True,
        sdk_session_id=session_info.session_id,
        status=session_info.status,
        last_activity_at=session_info.last_activity,
    )
    return room, members, conversation, session_record


def extract_runtime_id(agent_aggregate: CreateAgentAggregate) -> str:
    """返回聚合中的 runtime ID。"""
    return agent_aggregate.runtime.id


def extract_existing_runtime_id(runtime_id: Optional[str], agent_id: str) -> str:
    """兜底返回 runtime ID。"""
    return runtime_id or _stable_id("runtime", agent_id)
