# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_service.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""会话应用服务。"""

from typing import List, Optional

from agent.infra.agent.session_manager import session_manager
from agent.infra.session.session_router import build_session_key, get_default_agent_id
from agent.service.process.protocol_adapter import ProtocolAdapter
from agent.schema.model_cost import SessionCostSummary
from agent.schema.model_session import ASession
from agent.service.session.session_store import session_store


class SessionService:
    """负责 Session 相关应用编排。"""

    def __init__(self) -> None:
        self._protocol_adapter = ProtocolAdapter()

    def to_session_key(self, session_key: str, agent_id: Optional[str] = None) -> str:
        """将前端 session_key 规范化为内部 session_key。"""
        if session_key.startswith("agent:"):
            return session_key
        return build_session_key(
            channel="ws",
            chat_type="dm",
            ref=session_key,
            agent_id=agent_id or get_default_agent_id(),
        )

    async def get_sessions(self) -> List[ASession]:
        """获取所有会话列表。"""
        return await session_store.get_all_sessions()

    async def create_session(
        self,
        session_key: str,
        agent_id: Optional[str] = None,
        title: Optional[str] = "New Chat",
    ) -> ASession:
        """创建会话。"""
        internal_key = self.to_session_key(session_key, agent_id)
        existing = await session_store.get_session_info(internal_key)
        if existing:
            raise ValueError("Session already exists")

        success = await session_store.update_session(
            session_key=internal_key,
            agent_id=agent_id or get_default_agent_id(),
            title=title,
        )
        if not success:
            raise RuntimeError("Failed to create session")

        session_info = await session_store.get_session_info(internal_key)
        if not session_info:
            raise RuntimeError("Failed to retrieve created session")
        return session_info

    async def update_session(
        self,
        session_key: str,
        title: Optional[str] = None,
    ) -> ASession:
        """更新会话信息。"""
        internal_key = self.to_session_key(session_key)
        existing = await session_store.get_session_info(internal_key)
        if not existing:
            raise LookupError("Session not found")

        success = await session_store.update_session(
            session_key=internal_key,
            title=title,
        )
        if not success:
            raise RuntimeError("Failed to update session")

        updated = await session_store.get_session_info(internal_key)
        if not updated:
            raise RuntimeError("Failed to retrieve updated session")
        return updated

    async def get_session_messages(self, session_key: str) -> list[dict]:
        """获取会话历史消息。"""
        internal_key = self.to_session_key(session_key)
        messages = await session_store.get_session_messages(internal_key)
        return self._protocol_adapter.build_history_messages(messages)

    async def get_session_cost_summary(self, session_key: str) -> SessionCostSummary:
        """获取会话成本汇总。"""
        internal_key = self.to_session_key(session_key)
        session_info = await session_store.get_session_info(internal_key)
        if not session_info:
            raise LookupError("Session not found")
        return await session_store.get_session_cost_summary(internal_key)

    async def delete_session(self, session_key: str) -> None:
        """删除会话。"""
        internal_key = self.to_session_key(session_key)
        session_manager.remove_session(internal_key)

        success = await session_store.delete_session(internal_key)
        if not success:
            raise LookupError("Session not found")

    async def delete_round(self, session_key: str, round_id: str) -> int:
        """删除一轮对话。"""
        internal_key = self.to_session_key(session_key)
        existing = await session_store.get_session_info(internal_key)
        if not existing:
            raise LookupError("Session not found")

        deleted_count = await session_store.delete_round(internal_key, round_id)
        if deleted_count < 0:
            raise RuntimeError("Failed to delete round")
        return deleted_count


session_service = SessionService()
