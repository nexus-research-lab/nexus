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
from datetime import datetime, timezone
from pathlib import Path
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
    MessageRecord,
    RoomRecord,
    RoundRecord,
    SessionRecord,
)
from agent.schema.model_message import Message
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


def build_room_id(session_key: str) -> str:
    """根据旧 session_key 生成房间 ID。"""
    return _stable_id("room", session_key)


def build_conversation_id(session_key: str) -> str:
    """根据旧 session_key 生成对话 ID。"""
    return _stable_id("conv", session_key)


def build_persistent_session_id(session_key: str) -> str:
    """根据旧 session_key 生成持久化会话 ID。"""
    return _stable_id("sess", session_key)


def _timestamp_ms_to_datetime(timestamp_ms: int | None) -> Optional[datetime]:
    """将毫秒时间戳转换为 UTC datetime。"""
    if not timestamp_ms:
        return None
    return datetime.fromtimestamp(timestamp_ms / 1000, tz=timezone.utc)


def build_message_record_from_legacy(
    message: Message,
    session_info: ASession,
    jsonl_path: str,
    jsonl_offset: Optional[int] = None,
) -> MessageRecord:
    """将旧消息模型映射为消息索引记录。"""
    sender_type_map = {
        "user": "user",
        "assistant": "agent",
        "system": "system",
        "result": "agent",
    }
    kind = "text"
    if message.role == "system":
        kind = "event"
    elif message.role == "result" and (message.is_error or message.subtype in {"error", "interrupted"}):
        kind = "error"
    elif isinstance(message.content, list):
        block_types = {
            getattr(block, "type", None) if not isinstance(block, dict) else block.get("type")
            for block in message.content
        }
        if "tool_use" in block_types:
            kind = "tool_call"
        elif "tool_result" in block_types:
            kind = "tool_result"

    content_preview: Optional[str] = None
    if isinstance(message.content, str):
        content_preview = message.content[:500]
    elif message.result:
        content_preview = message.result[:500]

    return MessageRecord(
        id=message.message_id,
        conversation_id=build_conversation_id(session_info.session_key),
        session_id=build_persistent_session_id(session_info.session_key),
        sender_type=sender_type_map.get(message.role, "system"),
        sender_agent_id=message.agent_id or session_info.agent_id or None,
        kind=kind,
        content_preview=content_preview,
        jsonl_path=str(Path(jsonl_path)),
        jsonl_offset=jsonl_offset,
        round_id=message.round_id,
        created_at=_timestamp_ms_to_datetime(message.timestamp),
    )


def build_round_record_from_legacy(
    message: Message,
    session_info: ASession,
) -> Optional[RoundRecord]:
    """将旧消息映射为轮次索引记录。"""
    if not message.round_id:
        return None
    status = "running"
    if message.role == "result":
        if message.subtype == "success":
            status = "success"
        elif message.subtype == "interrupted":
            status = "cancelled"
        else:
            status = "error"

    finished_at = _timestamp_ms_to_datetime(message.timestamp) if message.role == "result" else None
    return RoundRecord(
        id=_stable_id("round", f"{session_info.session_key}:{message.round_id}"),
        session_id=build_persistent_session_id(session_info.session_key),
        round_id=message.round_id,
        trigger_message_id=message.round_id,
        status=status,
        started_at=_timestamp_ms_to_datetime(message.timestamp),
        finished_at=finished_at,
    )
