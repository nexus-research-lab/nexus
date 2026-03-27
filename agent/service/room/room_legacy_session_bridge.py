# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_legacy_session_bridge.py
# @Date   ：2026/03/26 02:17
# @Author ：leemysw
# 2026/03/26 02:17   Create
# =====================================================

"""Room 旧会话桥接服务。"""

from __future__ import annotations

from typing import Iterable

from agent.schema.model_chat_persistence import (
    ConversationContextAggregate,
    SessionRecord,
)
from agent.service.session.session_router import build_session_key
from agent.service.session.session_store import session_store


class RoomLegacySessionBridge:
    """负责把 room 持久化会话同步成旧聊天链路可消费的 session。"""

    async def ensure_context(self, context: ConversationContextAggregate) -> None:
        """确保一个上下文下的旧会话已完整创建。"""
        await self.ensure_sessions(
            room_type=context.room.room_type,
            room_id=context.room.id,
            conversation_id=context.conversation.id,
            title=context.conversation.title or context.room.name or "未命名对话",
            sessions=context.sessions,
        )

    async def ensure_sessions(
        self,
        room_type: str,
        room_id: str,
        conversation_id: str,
        title: str,
        sessions: Iterable[SessionRecord],
    ) -> None:
        """为一组 room session 同步旧会话。"""
        for session_record in sessions:
            await self.ensure_session(
                room_type=room_type,
                room_id=room_id,
                conversation_id=conversation_id,
                title=title,
                session_record=session_record,
            )

    async def ensure_session(
        self,
        room_type: str,
        room_id: str,
        conversation_id: str,
        title: str,
        session_record: SessionRecord,
    ) -> None:
        """为单个 room session 建立旧会话入口。"""
        session_key = self.build_session_key(
            room_type=room_type,
            conversation_id=conversation_id,
            agent_id=session_record.agent_id,
        )
        options = {
            "room_id": room_id,
            "conversation_id": conversation_id,
            "room_session_id": session_record.id,
        }
        existing = await session_store.get_session_info(session_key)
        if existing is None:
            await session_store.create_session_by_key(
                session_key=session_key,
                channel_type="ws",
                chat_type=self._to_chat_type(room_type),
                title=title,
                options=options,
            )
        elif existing.room_session_id == session_record.id and existing.session_id == session_record.id:
            # 清理上一版错误写入的 SQL session_id，避免运行时误当成 SDK resume id。
            await session_store.update_session(
                session_key=session_key,
                session_id="",
            )
        await session_store.update_session(
            session_key=session_key,
            agent_id=session_record.agent_id,
            title=title,
            options=options,
        )

    async def delete_sessions(
        self,
        room_type: str,
        sessions: Iterable[SessionRecord],
    ) -> None:
        """删除一组旧会话。"""
        for session_record in sessions:
            session_key = self.build_session_key(
                room_type=room_type,
                conversation_id=session_record.conversation_id,
                agent_id=session_record.agent_id,
            )
            await session_store.delete_session(session_key)

    def build_session_key(
        self,
        room_type: str,
        conversation_id: str,
        agent_id: str,
    ) -> str:
        """按 room 对话生成稳定的旧会话键。"""
        return build_session_key(
            channel="ws",
            chat_type=self._to_chat_type(room_type),
            ref=conversation_id,
            agent_id=agent_id,
        )

    def _to_chat_type(self, room_type: str) -> str:
        """把 room_type 映射成旧链路的 chat_type。"""
        return "dm" if room_type == "dm" else "group"


room_legacy_session_bridge = RoomLegacySessionBridge()
