# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Session SQL 仓储。"""

from __future__ import annotations

from datetime import datetime
from typing import Optional

from sqlalchemy import select

from agent.infra.database.models.session import Session
from agent.schema.model_chat_persistence import SessionRecord
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class SessionSqlRepository(BaseSqlRepository):
    """运行时会话 SQL 仓储。"""

    async def create(self, session: SessionRecord) -> SessionRecord:
        """创建运行时会话。"""
        entity = Session(
            **session.model_dump(exclude={"created_at", "updated_at", "last_activity_at"})
        )
        if session.last_activity_at is not None:
            entity.last_activity_at = session.last_activity_at
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return SessionRecord.model_validate(entity)

    async def get(self, session_id: str) -> Optional[SessionRecord]:
        """按 ID 获取会话。"""
        entity = await self._session.get(Session, session_id)
        if entity is None:
            return None
        return SessionRecord.model_validate(entity)

    async def list_by_conversation(self, conversation_id: str) -> list[SessionRecord]:
        """列出对话下的全部会话。"""
        stmt = (
            select(Session)
            .where(Session.conversation_id == conversation_id)
            .order_by(Session.last_activity_at.desc())
        )
        result = await self._session.execute(stmt)
        return [SessionRecord.model_validate(entity) for entity in result.scalars().all()]

    async def get_primary(
        self,
        conversation_id: str,
        agent_id: str,
    ) -> Optional[SessionRecord]:
        """获取主版本会话。"""
        stmt = (
            select(Session)
            .where(
                Session.conversation_id == conversation_id,
                Session.agent_id == agent_id,
                Session.is_primary.is_(True),
            )
        )
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return SessionRecord.model_validate(entity)

    async def touch(
        self,
        session_id: str,
        last_activity_at: Optional[datetime] = None,
    ) -> Optional[SessionRecord]:
        """刷新会话活动时间。"""
        entity = await self._session.get(Session, session_id)
        if entity is None:
            return None
        entity.last_activity_at = last_activity_at or datetime.now()
        await self.flush()
        await self.refresh(entity)
        return SessionRecord.model_validate(entity)
