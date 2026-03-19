# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_sql_repository.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Room SQL 仓储。"""

from __future__ import annotations

from typing import Optional

from sqlalchemy import select
from sqlalchemy.orm import selectinload

from agent.infra.database.models.member import Member
from agent.infra.database.models.room import Room
from agent.schema.model_chat_persistence import MemberRecord, RoomAggregate, RoomRecord
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class RoomSqlRepository(BaseSqlRepository):
    """Room 聚合 SQL 仓储。"""

    async def create(
        self,
        room: RoomRecord,
        members: Optional[list[MemberRecord]] = None,
    ) -> RoomAggregate:
        """创建房间及初始成员。"""
        entity = Room(**room.model_dump(exclude={"created_at", "updated_at"}))
        self._session.add(entity)
        for member in members or []:
            self._session.add(
                Member(**member.model_dump(exclude={"joined_at"}))
            )
        await self.flush()
        return await self.get(room.id) or RoomAggregate(room=room, members=members or [])

    async def get(self, room_id: str) -> Optional[RoomAggregate]:
        """按 ID 获取房间聚合。"""
        stmt = select(Room).options(selectinload(Room.members)).where(Room.id == room_id)
        result = await self._session.execute(stmt)
        room = result.scalar_one_or_none()
        if room is None:
            return None
        members = [MemberRecord.model_validate(member) for member in room.members]
        return RoomAggregate(room=RoomRecord.model_validate(room), members=members)

    async def list_recent(self, limit: int = 50) -> list[RoomAggregate]:
        """列出最近房间。"""
        stmt = (
            select(Room)
            .options(selectinload(Room.members))
            .order_by(Room.created_at.desc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        rooms = result.scalars().unique().all()
        return [
            RoomAggregate(
                room=RoomRecord.model_validate(room),
                members=[MemberRecord.model_validate(member) for member in room.members],
            )
            for room in rooms
        ]

    async def add_member(self, member: MemberRecord) -> MemberRecord:
        """向房间追加成员。"""
        entity = Member(**member.model_dump(exclude={"joined_at"}))
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return MemberRecord.model_validate(entity)

    async def list_members(self, room_id: str) -> list[MemberRecord]:
        """列出房间成员。"""
        stmt = select(Member).where(Member.room_id == room_id).order_by(Member.joined_at.asc())
        result = await self._session.execute(stmt)
        return [MemberRecord.model_validate(member) for member in result.scalars().all()]
