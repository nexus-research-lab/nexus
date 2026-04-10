# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_router.py
# @Date   ：2026/2/25 23:10
# @Author ：leemysw
#
# 2026/2/25 23:10   Create
# =====================================================

"""
会话路由器

[INPUT]: 依赖 session_store 的会话存取方法
[OUTPUT]: 对外提供 build_session_key / resolve_session
[POS]: session 模块的核心路由逻辑，被 Channel 层消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md 与 docs/session-key-spec.md
"""

from typing import Optional

from agent.config.config import settings
from agent.utils.logger import logger

# =====================================================
# 常量
# =====================================================

AGENT_SESSION_PREFIX = "agent"
ROOM_SESSION_PREFIX = "room"
ROOM_SHARED_CHAT_TYPE = "group"
TOPIC_SEGMENT = "topic"
AUTOMATION_CHANNEL = "automation"
AUTOMATION_MAIN_REF = "main"


class StructuredSessionKeyError(ValueError):
    """结构化 session_key 校验失败。"""


def get_default_agent_id() -> str:
    """返回默认 Agent ID。"""
    return settings.DEFAULT_AGENT_ID


def resolve_agent_id(agent_id: Optional[str]) -> str:
    """解析 agent_id，缺失时回退到默认 Agent。"""
    normalized_agent_id = (agent_id or "").strip()
    return normalized_agent_id or get_default_agent_id()


def is_agent_session_key(session_key: str) -> bool:
    """判断是否为 Agent 作用域的结构化会话键。"""
    return (session_key or "").startswith(f"{AGENT_SESSION_PREFIX}:")


def is_room_session_key(session_key: str) -> bool:
    """判断是否为 Room 共享作用域的结构化会话键。"""
    return (session_key or "").startswith(f"{ROOM_SESSION_PREFIX}:")


def _find_topic_index(parts: list[str]) -> Optional[int]:
    """查找保留的 topic 分段位置。"""
    for index, part in enumerate(parts):
        if part == TOPIC_SEGMENT and index >= 4:
            return index
    return None


def get_session_key_validation_error(session_key: str) -> Optional[str]:
    """返回 session_key 的协议校验错误，合法时返回 None。"""
    normalized_key = (session_key or "").strip()
    if not normalized_key:
        return "session_key is required"

    if is_agent_session_key(normalized_key):
        parts = normalized_key.split(":")
        if len(parts) < 5 or not parts[1] or not parts[2] or not parts[3]:
            return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"

        topic_index = _find_topic_index(parts)
        if topic_index is not None:
            ref = ":".join(parts[4:topic_index]).strip()
            thread_id = ":".join(parts[topic_index + 1:]).strip()
            if not ref or not thread_id:
                return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"
            return None

        ref = ":".join(parts[4:]).strip()
        if not ref:
            return "session_key must match agent:<agent_id>:<channel>:<chat_type>:<ref>[:topic:<thread_id>]"
        return None

    if is_room_session_key(normalized_key):
        parts = normalized_key.split(":")
        conversation_id = ":".join(parts[2:]).strip() if len(parts) > 2 else ""
        if (
            len(parts) < 3
            or parts[1] != ROOM_SHARED_CHAT_TYPE
            or not conversation_id
        ):
            return "session_key must match room:group:<conversation_id>"
        return None

    return "session_key must use structured gateway format"


def is_structured_session_key(session_key: str) -> bool:
    """判断是否为统一协议下的结构化会话键。"""
    return get_session_key_validation_error(session_key) is None


def require_structured_session_key(session_key: str) -> str:
    """要求入参必须是合法的结构化 session_key。"""
    error_message = get_session_key_validation_error(session_key)
    if error_message is not None:
        raise StructuredSessionKeyError(error_message)
    return session_key.strip()


# =====================================================
# Session Key 构建
#
# 协议分两族：
# 1. agent:<agentId>:<channel>:<chatType>:<ref>[:topic:<threadId>]
# 2. room:group:<conversationId>
#
# 中文注释：`room:group:*` 是历史冻结协议值，语义上表示共享会话流，
# 不是“只能群聊使用”的字面含义，后续不要直接改这个字符串形状。
# =====================================================

def build_session_key(
    channel: str,
    chat_type: str,
    ref: str,
    thread_id: Optional[str] = None,
    agent_id: Optional[str] = None,
) -> str:
    """构建结构化 Session Key

    Args:
        channel: 通道标识 (ws / dg / tg)
        chat_type: 会话类型 (dm / group)
        ref: 通道内定位标识
        thread_id: 线程/Topic ID（可选，Discord Thread / Telegram Topic）
        agent_id: 智能体 ID，未传时使用默认 Agent

    Returns:
        Session Key 字符串

    Examples:
        >>> build_session_key("ws", "dm", "abc-123")
        'agent:<default-agent-id>:ws:dm:abc-123'
        >>> build_session_key("dg", "group", "123:456", thread_id="789")
        'agent:<default-agent-id>:dg:group:123:456:topic:789'
    """
    resolved_agent_id = resolve_agent_id(agent_id)
    # 中文注释：统一在协议入口做 trim，避免调用方拼出仅空白差异的键。
    resolved_channel = channel.strip()
    resolved_chat_type = chat_type.strip()
    resolved_ref = ref.strip()
    key = (
        f"{AGENT_SESSION_PREFIX}:{resolved_agent_id}:"
        f"{resolved_channel}:{resolved_chat_type}:{resolved_ref}"
    )
    if thread_id:
        key += f":{TOPIC_SEGMENT}:{thread_id.strip()}"
    return key


def build_room_shared_session_key(conversation_id: str) -> str:
    """构建 Room/DM 共享消息流的 gateway session_key。"""
    resolved_conversation_id = conversation_id.strip()
    return f"{ROOM_SESSION_PREFIX}:{ROOM_SHARED_CHAT_TYPE}:{resolved_conversation_id}"


def build_automation_main_session_key(agent_id: Optional[str] = None) -> str:
    """构建 automation main session 的结构化键。"""
    return build_session_key(
        channel=AUTOMATION_CHANNEL,
        chat_type="dm",
        ref=AUTOMATION_MAIN_REF,
        agent_id=agent_id,
    )


def is_automation_session_key(session_key: str) -> bool:
    """判断是否为 automation 运行时会话。"""
    parsed = parse_session_key(session_key)
    return parsed.get("kind") == "agent" and parsed.get("channel") == AUTOMATION_CHANNEL


def is_automation_main_session_key(session_key: str) -> bool:
    """判断是否为 automation main session。"""
    parsed = parse_session_key(session_key)
    return (
        is_automation_session_key(session_key)
        and parsed.get("chat_type") == "dm"
        and parsed.get("ref") == AUTOMATION_MAIN_REF
    )


def parse_session_key(session_key: str) -> dict:
    """解析统一协议下的 Session Key 为结构化字段。

    Returns:
        dict:
            kind: agent / room / unknown
            is_structured: 是否匹配统一协议
            is_shared: 是否为共享消息流
            agent_id: Agent 作用域下的 Agent ID
            channel: Agent 作用域下的通道标识
            chat_type: 对话拓扑
            ref: Agent 作用域的通道内引用，或 Room 的 conversation_id
            thread_id: topic/thread 标识
            conversation_id: Room 共享键对应的 conversation_id
    """
    normalized_key = (session_key or "").strip()
    validation_error = get_session_key_validation_error(normalized_key)
    result = {
        "raw": normalized_key,
        "kind": "unknown",
        "is_structured": False,
        "is_shared": False,
        "agent_id": None,
        "channel": None,
        "chat_type": None,
        "ref": None,
        "thread_id": None,
        "conversation_id": None,
    }

    if is_agent_session_key(normalized_key):
        parts = normalized_key.split(":")
        result.update(
            {
                "kind": "agent",
                "is_structured": validation_error is None,
                "agent_id": resolve_agent_id(parts[1] if len(parts) > 1 else None),
                "channel": parts[2] if len(parts) > 2 and parts[2] else None,
                "chat_type": parts[3] if len(parts) > 3 and parts[3] else "dm",
            }
        )

        # 中文注释：`:topic:` 是协议保留边界，ref 可以带冒号，但不能跨过该分段。
        topic_idx = _find_topic_index(parts)
        if topic_idx is not None:
            result["ref"] = ":".join(parts[4:topic_idx]) or None
            result["thread_id"] = ":".join(parts[topic_idx + 1:]) or None
        else:
            result["ref"] = ":".join(parts[4:]) or None
        return result

    if is_room_session_key(normalized_key):
        parts = normalized_key.split(":")
        conversation_id = ":".join(parts[2:]).strip() if len(parts) > 2 else ""
        result.update(
            {
                "kind": "room",
                "is_structured": validation_error is None,
                "is_shared": validation_error is None,
                "chat_type": parts[1] if len(parts) > 1 and parts[1] else ROOM_SHARED_CHAT_TYPE,
                "ref": conversation_id or None,
                "conversation_id": conversation_id or None,
            }
        )

    return result


async def resolve_session(
    channel: str,
    chat_type: str,
    ref: str,
    thread_id: Optional[str] = None,
    agent_id: Optional[str] = None,
) -> "Session":
    """路由到现有 Session 或创建新 Session

    Args:
        channel: 通道标识
        chat_type: 会话类型
        ref: 通道内定位标识
        thread_id: 线程 ID

    Returns:
        会话模型对象
    """
    from agent.service.session.session_store import session_store

    session_key = build_session_key(channel, chat_type, ref, thread_id, agent_id=agent_id)

    # 1. 查找现有活跃 Session
    existing = await session_store.get_session_by_key(session_key)
    if existing and existing.status == "active":
        logger.debug(f"♻️ 复用会话: {session_key}")
        return existing

    # 2. 创建新 Session
    logger.info(f"✨ 创建新会话: {session_key}")
    return await session_store.create_session_by_key(
        session_key=session_key,
        channel_type=channel,
        chat_type=chat_type,
    )
