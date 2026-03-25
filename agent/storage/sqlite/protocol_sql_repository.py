# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol_sql_repository.py
# @Date   ：2026/3/25 21:10
# @Author ：OpenAI
# =====================================================

"""Protocol Room SQL 仓储。"""

from __future__ import annotations

from typing import Optional

from sqlalchemy import func, select
from sqlalchemy.orm import selectinload

from agent.infra.database.models.protocol import (
    ActionRequest,
    ActionSubmission,
    Channel,
    ChannelMember,
    ProtocolDefinition,
    ProtocolRun,
    RunStateSnapshot,
)
from agent.schema.model_protocol import (
    ActionRequestRecord,
    ActionSubmissionRecord,
    ChannelAggregate,
    ChannelMemberRecord,
    ChannelRecord,
    ProtocolDefinitionRecord,
    ProtocolRunRecord,
    RunStateSnapshotRecord,
)
from agent.storage.sqlite.base_sql_repository import BaseSqlRepository


class ProtocolSqlRepository(BaseSqlRepository):
    """Protocol Room 持久化仓储。"""

    @staticmethod
    def _to_orm_payload(payload: dict) -> dict:
        """将 schema 字段映射为 ORM 安全属性。"""
        mapped = dict(payload)
        if "metadata" in mapped:
            mapped["metadata_json"] = mapped.pop("metadata")
        return mapped

    async def upsert_definition(
        self,
        definition: ProtocolDefinitionRecord,
    ) -> ProtocolDefinitionRecord:
        """创建或更新协议定义。"""
        stmt = select(ProtocolDefinition).where(
            ProtocolDefinition.slug == definition.slug,
            ProtocolDefinition.version == definition.version,
        )
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        payload = definition.model_dump(exclude={"created_at", "updated_at"})

        if entity is None:
            entity = ProtocolDefinition(**payload)
            self._session.add(entity)
        else:
            for field_name, value in payload.items():
                setattr(entity, field_name, value)

        await self.flush()
        await self.refresh(entity)
        return ProtocolDefinitionRecord.model_validate(entity)

    async def get_definition(self, definition_id: str) -> Optional[ProtocolDefinitionRecord]:
        """按 ID 读取协议定义。"""
        entity = await self._session.get(ProtocolDefinition, definition_id)
        if entity is None:
            return None
        return ProtocolDefinitionRecord.model_validate(entity)

    async def get_definition_by_slug(
        self,
        slug: str,
        version: Optional[int] = None,
    ) -> Optional[ProtocolDefinitionRecord]:
        """按 slug 读取协议定义。"""
        stmt = select(ProtocolDefinition).where(ProtocolDefinition.slug == slug)
        if version is not None:
            stmt = stmt.where(ProtocolDefinition.version == version)
        else:
            stmt = stmt.order_by(ProtocolDefinition.version.desc())
        result = await self._session.execute(stmt.limit(1))
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return ProtocolDefinitionRecord.model_validate(entity)

    async def create_run(self, run: ProtocolRunRecord) -> ProtocolRunRecord:
        """创建协议运行。"""
        entity = ProtocolRun(**self._to_orm_payload(run.model_dump(exclude={"created_at", "updated_at"})))
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return ProtocolRunRecord.model_validate(entity)

    async def update_run(self, run: ProtocolRunRecord) -> ProtocolRunRecord:
        """更新协议运行。"""
        entity = await self._session.get(ProtocolRun, run.id)
        if entity is None:
            raise LookupError("Protocol run not found")
        payload = self._to_orm_payload(run.model_dump(exclude={"created_at", "updated_at"}))
        for field_name, value in payload.items():
            setattr(entity, field_name, value)
        await self.flush()
        await self.refresh(entity)
        return ProtocolRunRecord.model_validate(entity)

    async def get_run(self, run_id: str) -> Optional[ProtocolRunRecord]:
        """读取单个协议运行。"""
        entity = await self._session.get(ProtocolRun, run_id)
        if entity is None:
            return None
        return ProtocolRunRecord.model_validate(entity)

    async def list_runs_by_room(
        self,
        room_id: str,
        limit: int = 20,
    ) -> list[ProtocolRunRecord]:
        """读取 room 下的协议运行列表。"""
        stmt = (
            select(ProtocolRun)
            .where(ProtocolRun.room_id == room_id)
            .order_by(ProtocolRun.created_at.desc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [ProtocolRunRecord.model_validate(entity) for entity in result.scalars().all()]

    async def create_channel_aggregate(
        self,
        channel: ChannelRecord,
        members: list[ChannelMemberRecord],
    ) -> ChannelAggregate:
        """创建频道及其成员。"""
        entity = Channel(**self._to_orm_payload(channel.model_dump(exclude={"created_at", "updated_at"})))
        self._session.add(entity)
        for member in members:
            self._session.add(
                ChannelMember(**member.model_dump(exclude={"created_at", "updated_at"}))
            )
        await self.flush()
        return await self.get_channel(channel.id) or ChannelAggregate(channel=channel, members=members)

    async def get_channel(self, channel_id: str) -> Optional[ChannelAggregate]:
        """按 ID 读取频道聚合。"""
        stmt = (
            select(Channel)
            .options(selectinload(Channel.members))
            .where(Channel.id == channel_id)
        )
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return ChannelAggregate(
            channel=ChannelRecord.model_validate(entity),
            members=[ChannelMemberRecord.model_validate(member) for member in entity.members],
        )

    async def get_channel_by_slug(
        self,
        run_id: str,
        slug: str,
    ) -> Optional[ChannelAggregate]:
        """按 slug 读取频道聚合。"""
        stmt = (
            select(Channel)
            .options(selectinload(Channel.members))
            .where(Channel.protocol_run_id == run_id, Channel.slug == slug)
        )
        result = await self._session.execute(stmt)
        entity = result.scalar_one_or_none()
        if entity is None:
            return None
        return ChannelAggregate(
            channel=ChannelRecord.model_validate(entity),
            members=[ChannelMemberRecord.model_validate(member) for member in entity.members],
        )

    async def list_channels_by_run(self, run_id: str) -> list[ChannelAggregate]:
        """读取 run 下的全部频道。"""
        stmt = (
            select(Channel)
            .options(selectinload(Channel.members))
            .where(Channel.protocol_run_id == run_id)
            .order_by(Channel.position.asc(), Channel.created_at.asc())
        )
        result = await self._session.execute(stmt)
        entities = result.scalars().unique().all()
        return [
            ChannelAggregate(
                channel=ChannelRecord.model_validate(entity),
                members=[ChannelMemberRecord.model_validate(member) for member in entity.members],
            )
            for entity in entities
        ]

    async def create_action_request(
        self,
        request: ActionRequestRecord,
    ) -> ActionRequestRecord:
        """创建动作请求。"""
        entity = ActionRequest(**self._to_orm_payload(request.model_dump(exclude={"created_at", "updated_at"})))
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return ActionRequestRecord.model_validate(entity)

    async def update_action_request(
        self,
        request: ActionRequestRecord,
    ) -> ActionRequestRecord:
        """更新动作请求。"""
        entity = await self._session.get(ActionRequest, request.id)
        if entity is None:
            raise LookupError("Action request not found")
        payload = self._to_orm_payload(request.model_dump(exclude={"created_at", "updated_at"}))
        for field_name, value in payload.items():
            setattr(entity, field_name, value)
        await self.flush()
        await self.refresh(entity)
        return ActionRequestRecord.model_validate(entity)

    async def get_action_request(self, request_id: str) -> Optional[ActionRequestRecord]:
        """读取动作请求。"""
        entity = await self._session.get(ActionRequest, request_id)
        if entity is None:
            return None
        return ActionRequestRecord.model_validate(entity)

    async def list_action_requests(self, run_id: str) -> list[ActionRequestRecord]:
        """列出协议运行下的动作请求。"""
        stmt = (
            select(ActionRequest)
            .where(ActionRequest.protocol_run_id == run_id)
            .order_by(ActionRequest.created_at.asc())
        )
        result = await self._session.execute(stmt)
        return [ActionRequestRecord.model_validate(entity) for entity in result.scalars().all()]

    async def create_action_submission(
        self,
        submission: ActionSubmissionRecord,
    ) -> ActionSubmissionRecord:
        """创建动作提交。"""
        entity = ActionSubmission(**self._to_orm_payload(submission.model_dump(exclude={"created_at", "updated_at"})))
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return ActionSubmissionRecord.model_validate(entity)

    async def list_action_submissions(self, run_id: str) -> list[ActionSubmissionRecord]:
        """列出协议运行下的动作提交。"""
        stmt = (
            select(ActionSubmission)
            .where(ActionSubmission.protocol_run_id == run_id)
            .order_by(ActionSubmission.created_at.asc())
        )
        result = await self._session.execute(stmt)
        return [ActionSubmissionRecord.model_validate(entity) for entity in result.scalars().all()]

    async def get_latest_snapshot_seq(self, run_id: str) -> int:
        """读取当前 run 的最大事件序号。"""
        stmt = select(func.max(RunStateSnapshot.event_seq)).where(
            RunStateSnapshot.protocol_run_id == run_id,
        )
        result = await self._session.execute(stmt)
        latest_seq = result.scalar_one_or_none()
        return int(latest_seq or 0)

    async def create_snapshot(
        self,
        snapshot: RunStateSnapshotRecord,
    ) -> RunStateSnapshotRecord:
        """创建运行态快照。"""
        entity = RunStateSnapshot(**self._to_orm_payload(snapshot.model_dump(exclude={"created_at", "updated_at"})))
        self._session.add(entity)
        await self.flush()
        await self.refresh(entity)
        return RunStateSnapshotRecord.model_validate(entity)

    async def list_snapshots(
        self,
        run_id: str,
        limit: int = 500,
    ) -> list[RunStateSnapshotRecord]:
        """列出协议运行快照。"""
        stmt = (
            select(RunStateSnapshot)
            .where(RunStateSnapshot.protocol_run_id == run_id)
            .order_by(RunStateSnapshot.event_seq.asc())
            .limit(limit)
        )
        result = await self._session.execute(stmt)
        return [RunStateSnapshotRecord.model_validate(entity) for entity in result.scalars().all()]
