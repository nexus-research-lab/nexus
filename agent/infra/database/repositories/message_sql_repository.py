# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Message SQL 仓储。"""

from __future__ import annotations

from datetime import datetime
from typing import Optional

from sqlalchemy import or_, select

from agent.infra.database.models.message import Message
from agent.infra.database.models.round import Round
from agent.schema.model_chat_persistence import MessageRecord, RoundRecord
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository


class MessageSqlRepository(BaseSqlRepository):
    """消息与轮次 SQL 仓储。"""

    async def upsert_message(self, message: MessageRecord) -> MessageRecord:
        """创建或更新消息索引。"""
        entity = await self._session.get(Message, message.id)
        if entity is None:
            entity = Message(**message.model_dump(exclude={"created_at", "updated_at"}))
            self._session.add(entity)
        else:
            payload = message.model_dump(exclude={"created_at", "updated_at"})
            for field_name, value in payload.items():
                setattr(entity, field_name, value)
        await self.flush()
        await self.refresh(entity)
        return MessageRecord.model_validate(entity)

    async def list_by_conversation(
        self,
        conversation_id: str,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """列出对话消息索引。"""
        stmt = (
            select(Message)
            .where(Message.conversation_id == conversation_id)
            .order_by(Message.created_at.asc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [MessageRecord.model_validate(entity) for entity in result.scalars().all()]

    async def list_by_session(
        self,
        session_id: str,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """列出会话消息索引。"""
        stmt = (
            select(Message)
            .where(Message.session_id == session_id)
            .order_by(Message.created_at.asc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [MessageRecord.model_validate(entity) for entity in result.scalars().all()]

    async def get_message(self, message_id: str) -> Optional[MessageRecord]:
        """按消息 ID 获取索引。"""
        entity = await self._session.get(Message, message_id)
        if entity is None:
            return None
        return MessageRecord.model_validate(entity)

    async def list_completed_before(
        self,
        conversation_id: str,
        before: datetime,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """查询指定时间前已完成的消息索引。"""
        stmt = (
            select(Message)
            .where(
                Message.conversation_id == conversation_id,
                Message.status == "completed",
                Message.created_at < before,
            )
            .order_by(Message.created_at.asc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [MessageRecord.model_validate(entity) for entity in result.scalars().all()]

    async def list_inflight_by_conversation_round(
        self,
        conversation_id: str,
        root_round_id: str,
        limit: int = 200,
    ) -> list[MessageRecord]:
        """查询指定用户轮次下仍未结束的消息索引。"""
        stmt = (
            select(Message)
            .where(
                Message.conversation_id == conversation_id,
                Message.status.in_(("pending", "streaming")),
                or_(
                    Message.round_id == root_round_id,
                    Message.round_id.like(f"{root_round_id}:%"),
                ),
            )
            .order_by(Message.created_at.asc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [MessageRecord.model_validate(entity) for entity in result.scalars().all()]

    async def update_message_status(
        self,
        message_id: str,
        status: str,
        updated_at: Optional[datetime] = None,
    ) -> Optional[MessageRecord]:
        """仅更新消息状态，避免覆盖现有索引字段。"""
        entity = await self._session.get(Message, message_id)
        if entity is None:
            return None
        entity.status = status
        if updated_at is not None:
            entity.updated_at = updated_at
        await self.flush()
        await self.refresh(entity)
        return MessageRecord.model_validate(entity)

    async def upsert_round(self, round_record: RoundRecord) -> RoundRecord:
        """创建或更新轮次索引。"""
        stmt = select(Round).where(Round.round_id == round_record.round_id)
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            entity = Round(**round_record.model_dump(exclude={"created_at", "updated_at"}))
            self._session.add(entity)
        else:
            payload = round_record.model_dump(exclude={"created_at", "updated_at"})
            for field_name, value in payload.items():
                if field_name == "started_at" and getattr(entity, field_name, None):
                    continue
                if value is not None:
                    setattr(entity, field_name, value)
        await self.flush()
        await self.refresh(entity)
        return RoundRecord.model_validate(entity)

    async def get_round(self, round_id: str) -> Optional[RoundRecord]:
        """按外部 round_id 获取轮次。"""
        stmt = select(Round).where(Round.round_id == round_id)
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return RoundRecord.model_validate(entity)

    async def list_rounds(self, session_id: str) -> list[RoundRecord]:
        """列出会话轮次。"""
        stmt = (
            select(Round)
            .where(Round.session_id == session_id)
            .order_by(Round.started_at.asc())
        )
        result = await self._session.execute(stmt)
        return [RoundRecord.model_validate(entity) for entity in result.scalars().all()]
