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
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import Optional

from agent.config.config import settings
from agent.utils.logger import logger

# =====================================================
# 常量
# =====================================================

def get_default_agent_id() -> str:
    """返回默认 Agent ID。"""
    return getattr(settings, "DEFAULT_AGENT_ID", "main") or "main"


# =====================================================
# Session Key 构建
#
# 格式: agent:<agentId>:<channel>:<chatType>:<ref>[:topic:<threadId>]
# 确定性路由 — 无需查库即可定位会话
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
        'agent:main:ws:dm:abc-123'
        >>> build_session_key("dg", "group", "123:456", thread_id="789")
        'agent:main:dg:group:123:456:topic:789'
    """
    resolved_agent_id = agent_id or get_default_agent_id()
    key = f"agent:{resolved_agent_id}:{channel}:{chat_type}:{ref}"
    if thread_id:
        key += f":topic:{thread_id}"
    return key


def parse_session_key(session_key: str) -> dict:
    """解析 Session Key 为结构化字段

    Returns:
        dict: {agent_id, channel, chat_type, ref, thread_id?}
    """
    parts = session_key.split(":")
    # agent:<agentId>:<channel>:<chatType>:<ref...>[:topic:<threadId>]
    result = {
        "agent_id": parts[1] if len(parts) > 1 else get_default_agent_id(),
        "channel": parts[2] if len(parts) > 2 else "",
        "chat_type": parts[3] if len(parts) > 3 else "dm",
    }

    # 寻找 :topic: 边界
    topic_idx = None
    for i, part in enumerate(parts):
        if part == "topic" and i >= 4:
            topic_idx = i
            break

    if topic_idx:
        result["ref"] = ":".join(parts[4:topic_idx])
        result["thread_id"] = ":".join(parts[topic_idx + 1:])
    else:
        result["ref"] = ":".join(parts[4:])
        result["thread_id"] = None

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
