#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：activity_sql_repository.py
# @Date   ：2026/3/30 00:00
# @Author ：leemysw
# 2026/3/30 00:00   Create
# =====================================================

"""Activity 事件 SQL 仓储。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import select

from agent.infra.database.models.activity_event import ActivityEvent
from agent.infra.database.repositories.base_sql_repository import BaseSqlRepository
from agent.schema.model_chat_persistence import ActivityEventRecord


class ActivityEventSqlRepository(BaseSqlRepository):
    """Activity 事件 SQL 仓储。"""

    @staticmethod
    def _build_record(entity: ActivityEvent) -> ActivityEventRecord:
        """将 ORM 实体转换为领域记录。"""
        return ActivityEventRecord(
            id=str(entity.id),
            event_type=entity.event_type,
            actor_type=entity.actor_type,
            actor_id=entity.actor_id,
            target_type=entity.target_type,
            target_id=entity.target_id,
            summary=entity.summary,
            metadata_json=entity.metadata_json,
            created_at=entity.created_at,
        )

    @staticmethod
    def _is_event_read(metadata_json: dict | None, user_id: str) -> bool:
        """判断事件是否已被当前用户标记为已读。"""
        if not metadata_json:
            return False

        read_by = metadata_json.get("read_by")
        if isinstance(read_by, list):
            return user_id in read_by
        if isinstance(read_by, str):
            return read_by == user_id and bool(metadata_json.get("read"))
        return bool(metadata_json.get("read"))

    async def create(
        self, event: ActivityEventRecord
    ) -> ActivityEventRecord:
        """创建 Activity 事件。"""
        if isinstance(event.created_at, str):
            created_at = datetime.fromisoformat(event.created_at)
        else:
            created_at = event.created_at or datetime.now()

        entity = ActivityEvent(
            **event.model_dump(
                exclude={"created_at"}, by_alias=False
            ),
            created_at=created_at,
        )
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return self._build_record(entity)

    async def list(
        self,
        limit: int = 50,
        offset: int = 0,
        event_type: str | None = None,
        unread_only: bool = False,
        user_id: str = "local-user",
    ) -> list[ActivityEventRecord]:
        """列出 Activity 事件。

        Args:
            limit: 返回数量限制
            offset: 偏移量
            event_type: 按事件类型筛选
            unread_only: 仅返回未读事件
            user_id: 用户 ID（用于未读标记）

        Returns:
            Activity 事件列表
        """
        stmt = select(ActivityEvent).order_by(ActivityEvent.created_at.desc())

        # 按事件类型筛选
        if event_type:
            stmt = stmt.where(ActivityEvent.event_type == event_type)

        if not unread_only:
            stmt = stmt.limit(limit).offset(offset)

        result = await self._session.execute(stmt)
        entities = result.scalars().all()

        if unread_only:
            entities = [
                entity
                for entity in entities
                if not self._is_event_read(entity.metadata_json, user_id)
            ]
            entities = entities[offset: offset + limit]

        return [self._build_record(entity) for entity in entities]

    async def get_unread_count(
        self,
        user_id: str = "local-user",
    ) -> int:
        """获取未读事件数量。"""
        result = await self._session.execute(
            select(ActivityEvent).order_by(ActivityEvent.created_at.desc())
        )
        return sum(
            1
            for entity in result.scalars().all()
            if not self._is_event_read(entity.metadata_json, user_id)
        )

    async def mark_as_read(
        self,
        event_ids: list[str],
        user_id: str = "local-user",
    ) -> int:
        """标记事件为已读。"""
        if not event_ids:
            return 0

        result = await self._session.execute(
            select(ActivityEvent).where(ActivityEvent.id.in_(event_ids))
        )
        entities = result.scalars().all()
        marked_count = 0

        for entity in entities:
            if self._is_event_read(entity.metadata_json, user_id):
                continue

            metadata_json = dict(entity.metadata_json or {})
            metadata_json.update(
                {
                    "read": True,
                    "read_by": user_id,
                    "read_at": datetime.now().isoformat(),
                }
            )
            entity.metadata_json = metadata_json
            marked_count += 1

        if marked_count:
            await self.flush()
        return marked_count
