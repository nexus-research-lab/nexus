# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：conversation_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Conversation SQL 仓储。"""

from __future__ import annotations

from typing import Optional

from sqlalchemy import select

from agent.infra.database.models.conversation import Conversation
from agent.schema.model_chat_persistence import ConversationRecord
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class ConversationSqlRepository(BaseSqlRepository):
    """对话 SQL 仓储。"""

    async def create(self, conversation: ConversationRecord) -> ConversationRecord:
        """创建对话。"""
        entity = Conversation(
            **conversation.model_dump(exclude={"created_at", "updated_at"})
        )
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return ConversationRecord.model_validate(entity)

    async def get(self, conversation_id: str) -> Optional[ConversationRecord]:
        """按 ID 获取对话。"""
        entity = await self._session.get(Conversation, conversation_id)
        if entity is None:
            return None
        return ConversationRecord.model_validate(entity)

    async def list_by_room(self, room_id: str) -> list[ConversationRecord]:
        """列出房间下的对话。"""
        stmt = (
            select(Conversation)
            .where(Conversation.room_id == room_id)
            .order_by(Conversation.created_at.desc())
        )
        result = await self._session.execute(stmt)
        return [
            ConversationRecord.model_validate(entity)
            for entity in result.scalars().all()
        ]
