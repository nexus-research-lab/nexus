# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_session_keys.py
# @Date   ：2026/03/31 14:20
# @Author ：leemysw
# 2026/03/31 14:20   Create
# =====================================================

"""Room 会话键工具。"""

from __future__ import annotations

from typing import Optional

from agent.service.session.session_router import (
    build_room_shared_session_key as build_room_gateway_session_key,
    build_session_key,
    parse_session_key,
)


def build_room_shared_session_key(conversation_id: str) -> str:
    """构建 Room 共享消息流的 session_key。"""
    return build_room_gateway_session_key(conversation_id)


def is_room_shared_session_key(session_key: str) -> bool:
    """判断是否为 Room 共享消息流键。"""
    parsed = parse_session_key(session_key)
    return (
        parsed.get("kind") == "room"
        and bool(parsed.get("is_structured"))
        and bool(parsed.get("conversation_id"))
    )


def parse_room_conversation_id(session_key: str) -> Optional[str]:
    """从 Room 共享消息流键中提取 conversation_id。"""
    parsed = parse_session_key(session_key)
    if parsed.get("kind") != "room":
        return None
    conversation_id = str(parsed.get("conversation_id") or "").strip()
    return conversation_id or None


def build_room_agent_session_key(
    conversation_id: str,
    agent_id: str,
    room_type: str = "room",
) -> str:
    """构建 Room 成员的 SDK 会话键。"""
    chat_type = "dm" if room_type == "dm" else "group"
    return build_session_key(
        channel="ws",
        chat_type=chat_type,
        ref=conversation_id,
        agent_id=agent_id,
    )
