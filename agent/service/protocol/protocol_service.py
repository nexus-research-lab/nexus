# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol_service.py
# @Date   ：2026/3/25 21:10
# @Author ：OpenAI
# =====================================================

"""Protocol Room 应用服务。"""

from __future__ import annotations

from copy import deepcopy
from datetime import datetime
from typing import Any, Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_chat_persistence import MemberRecord
from agent.schema.model_protocol import (
    ActionRequestRecord,
    ActionSubmissionRecord,
    ChannelAggregate,
    ChannelMemberRecord,
    ChannelRecord,
    ProtocolRunDetail,
    ProtocolRunListItem,
    ProtocolRunRecord,
    RunStateSnapshotRecord,
)
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.protocol.protocol_definition import (
    ActionRequestBlueprint,
    PhasePlan,
    ProtocolDefinition,
    SnapshotBlueprint,
    protocol_definition_registry,
)
from agent.service.protocol.protocol_event_bus import protocol_event_bus
from agent.storage.sqlite.protocol_sql_repository import ProtocolSqlRepository
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.utils.utils import random_uuid

_LOCAL_USER_ID = "local-user"


class ProtocolRoomService:
    """Protocol Room 应用服务。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def list_room_runs(
        self,
        room_id: str,
    ) -> list[ProtocolRunListItem]:
        """读取 room 下的协议运行列表。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            protocol_repository = ProtocolSqlRepository(session)

            room = await room_repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")

            runs = await protocol_repository.list_runs_by_room(room_id)
            items: list[ProtocolRunListItem] = []
            for run in runs:
                definition = await protocol_repository.get_definition(run.protocol_definition_id)
                if definition is None:
                    continue
                items.append(ProtocolRunListItem(run=run, definition=definition))
            return items

    async def create_run(
        self,
        room_id: str,
        definition_slug: str = "werewolf_demo",
        title: Optional[str] = None,
        run_config: Optional[dict[str, Any]] = None,
    ) -> ProtocolRunDetail:
        """创建协议运行。"""
        definition = self._resolve_definition(definition_slug)

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            protocol_repository = ProtocolSqlRepository(session)

            room = await room_repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")

            member_agent_ids = self._room_agent_ids(room.members)
            if not member_agent_ids:
                raise ValueError("Protocol room 至少需要一个 agent 成员")

            persisted_definition = await protocol_repository.upsert_definition(definition.to_record())
            initial_state = definition.build_initial_state(member_agent_ids)

            run = await protocol_repository.create_run(
                ProtocolRunRecord(
                    id=random_uuid(),
                    room_id=room_id,
                    protocol_definition_id=persisted_definition.id,
                    title=title or f"{room.room.name or room.room.id} · {persisted_definition.name}",
                    status="running",
                    current_phase="setup",
                    phase_index=0,
                    current_turn_key=None,
                    coordinator_agent_id=MainAgentProfile.AGENT_ID,
                    run_config={
                        "definition_slug": definition_slug,
                        **(run_config or {}),
                    },
                    state=initial_state,
                )
            )

            for blueprint in definition.build_channels(room.members, run.state):
                channel_id = random_uuid()
                await protocol_repository.create_channel_aggregate(
                    ChannelRecord(
                        id=channel_id,
                        room_id=room_id,
                        protocol_run_id=run.id,
                        slug=blueprint.slug,
                        name=blueprint.name,
                        channel_type=blueprint.channel_type,
                        visibility=blueprint.visibility,
                        topic=blueprint.topic,
                        position=blueprint.position,
                        metadata=blueprint.metadata,
                    ),
                    self._build_channel_members(channel_id, blueprint),
                )

            channels = await protocol_repository.list_channels_by_run(run.id)
            channel_map = {aggregate.channel.slug: aggregate for aggregate in channels}

            await self._append_snapshot(
                protocol_repository,
                run,
                SnapshotBlueprint(
                    event_type="run_started",
                    phase_name="setup",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Protocol run started",
                    body=f"{persisted_definition.name} has been attached to this room.",
                    metadata={"message_kind": "phase_event"},
                ),
                channel_map,
            )
            await self._append_snapshot(
                protocol_repository,
                run,
                SnapshotBlueprint(
                    event_type="phase_started",
                    phase_name="setup",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Setup",
                    body="The coordinator is assigning initial protocol state and channels.",
                    metadata={"message_kind": "phase_event"},
                ),
                channel_map,
            )
            await self._append_snapshot(
                protocol_repository,
                run,
                SnapshotBlueprint(
                    event_type="phase_resolved",
                    phase_name="setup",
                    channel_slug="system-broadcast",
                    visibility="system",
                    headline="Setup complete",
                    body="The protocol is moving into the first active phase.",
                    metadata={"message_kind": "phase_event"},
                    ),
                channel_map,
            )

            initial_active_phase = (
                definition.phases[1]
                if len(definition.phases) > 1 and definition.phases[0] == "setup"
                else definition.phases[0]
            )
            run = run.model_copy(
                update={
                    "current_phase": initial_active_phase,
                    "phase_index": definition.phases.index(initial_active_phase),
                    "current_turn_key": None,
                }
            )
            run = await protocol_repository.update_run(run)
            run = await self._enter_phase(
                protocol_repository,
                definition,
                room.members,
                run,
                channel_map,
                initial_active_phase,
            )
            await session.commit()

        detail = await self.get_run_detail(run.id)
        self._publish_run_event(
            detail.run,
            reason="run_created",
            headline="Protocol run created",
        )
        return detail

    async def get_run_detail(
        self,
        run_id: str,
        viewer_agent_id: Optional[str] = None,
    ) -> ProtocolRunDetail:
        """读取协议运行详情。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            protocol_repository = ProtocolSqlRepository(session)

            run = await protocol_repository.get_run(run_id)
            if run is None:
                raise LookupError("Protocol run not found")

            definition = await protocol_repository.get_definition(run.protocol_definition_id)
            if definition is None:
                raise LookupError("Protocol definition not found")

            room = await room_repository.get(run.room_id)
            if room is None:
                raise LookupError("Room not found")

            channels = await protocol_repository.list_channels_by_run(run.id)
            requests = await protocol_repository.list_action_requests(run.id)
            submissions = await protocol_repository.list_action_submissions(run.id)
            snapshots = await protocol_repository.list_snapshots(run.id)

        visible_snapshots = [
            self._project_snapshot_for_viewer(snapshot, channels, viewer_agent_id)
            for snapshot in snapshots
        ]
        annotated_channels = [
            self._annotate_channel_for_viewer(channel, viewer_agent_id)
            for channel in channels
        ]

        return ProtocolRunDetail(
            room=room.room,
            members=room.members,
            definition=definition,
            run=run,
            channels=annotated_channels,
            action_requests=requests,
            action_submissions=submissions,
            snapshots=visible_snapshots,
            viewer_agent_id=viewer_agent_id,
        )

    async def list_channels(
        self,
        run_id: str,
        viewer_agent_id: Optional[str] = None,
    ) -> list[ChannelAggregate]:
        """读取协议运行下的频道列表。"""
        detail = await self.get_run_detail(run_id, viewer_agent_id=viewer_agent_id)
        return detail.channels

    async def submit_action(
        self,
        run_id: str,
        request_id: str,
        payload: dict[str, Any],
        actor_agent_id: Optional[str] = None,
        actor_user_id: Optional[str] = None,
    ) -> ProtocolRunDetail:
        """提交动作。"""
        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            protocol_repository = ProtocolSqlRepository(session)

            run = await protocol_repository.get_run(run_id)
            if run is None:
                raise LookupError("Protocol run not found")
            if run.status != "running":
                raise ValueError("Protocol run is not active")

            request = await protocol_repository.get_action_request(request_id)
            if request is None or request.protocol_run_id != run_id:
                raise LookupError("Action request not found")
            if request.status != "pending":
                raise ValueError("Action request is not pending")

            room = await room_repository.get(run.room_id)
            if room is None:
                raise LookupError("Room not found")

            definition = self._resolve_definition_by_run(run)
            channels = await protocol_repository.list_channels_by_run(run.id)
            channel_map = {aggregate.channel.slug: aggregate for aggregate in channels}

            await self._apply_submission(
                protocol_repository=protocol_repository,
                definition=definition,
                room_members=room.members,
                run=run,
                channel_map=channel_map,
                request=request,
                payload=payload,
                actor_agent_id=actor_agent_id,
                actor_user_id=actor_user_id,
                submission_status="submitted",
            )
            await session.commit()

        detail = await self.get_run_detail(run_id)
        self._publish_run_event(
            detail.run,
            reason="action_submitted",
            headline="Protocol action submitted",
        )
        return detail

    async def control_run(
        self,
        run_id: str,
        operation: str,
        payload: Optional[dict[str, Any]] = None,
    ) -> ProtocolRunDetail:
        """执行房间控制操作。"""
        payload = payload or {}

        async with self._db.session() as session:
            room_repository = RoomSqlRepository(session)
            protocol_repository = ProtocolSqlRepository(session)

            run = await protocol_repository.get_run(run_id)
            if run is None:
                raise LookupError("Protocol run not found")
            room = await room_repository.get(run.room_id)
            if room is None:
                raise LookupError("Room not found")

            definition = self._resolve_definition_by_run(run)
            channels = await protocol_repository.list_channels_by_run(run.id)
            channel_map = {aggregate.channel.slug: aggregate for aggregate in channels}

            if operation == "pause":
                if run.status == "running":
                    run = run.model_copy(update={"status": "paused"})
                    run = await protocol_repository.update_run(run)
                    await self._append_snapshot(
                        protocol_repository,
                        run,
                        SnapshotBlueprint(
                            event_type="run_paused",
                            phase_name=run.current_phase,
                            channel_slug="system-broadcast",
                            visibility="system",
                            headline="Run paused",
                            body="The coordinator paused protocol progression.",
                            metadata={"message_kind": "phase_event"},
                        ),
                        channel_map,
                    )

            elif operation == "resume":
                if run.status == "paused":
                    run = run.model_copy(update={"status": "running"})
                    run = await protocol_repository.update_run(run)
                    await self._append_snapshot(
                        protocol_repository,
                        run,
                        SnapshotBlueprint(
                            event_type="run_resumed",
                            phase_name=run.current_phase,
                            channel_slug="system-broadcast",
                            visibility="system",
                            headline="Run resumed",
                            body="The coordinator resumed protocol progression.",
                            metadata={"message_kind": "phase_event"},
                        ),
                        channel_map,
                    )

            elif operation == "inject_message":
                channel_id = str(payload.get("channel_id") or "").strip()
                if not channel_id:
                    raise ValueError("channel_id is required")
                channel = next(
                    (aggregate for aggregate in channels if aggregate.channel.id == channel_id),
                    None,
                )
                if channel is None:
                    raise LookupError("Channel not found")

                actor_agent_id = str(payload.get("actor_agent_id") or "").strip() or None
                audience_agent_ids = [
                    member.member_agent_id
                    for member in channel.members
                    if member.member_type == "agent" and member.member_agent_id
                ]
                await self._append_snapshot(
                    protocol_repository,
                    run,
                    SnapshotBlueprint(
                        event_type="channel_message",
                        phase_name=run.current_phase,
                        channel_slug=channel.channel.slug,
                        actor_agent_id=actor_agent_id,
                        visibility=channel.channel.visibility,
                        audience_agent_ids=audience_agent_ids,
                        headline=payload.get("headline") or "Moderator injection",
                        body=str(payload.get("content") or "").strip(),
                        metadata={"message_kind": payload.get("message_kind") or "message"},
                    ),
                    channel_map,
                )

            elif operation == "force_transition":
                next_phase = str(payload.get("phase_name") or "").strip()
                if not next_phase:
                    next_phase = self._resolve_next_phase(definition, run.current_phase)
                if not next_phase:
                    raise ValueError("No next phase available")

                requests = await protocol_repository.list_action_requests(run.id)
                for request in requests:
                    if request.phase_name == run.current_phase and request.status == "pending":
                        request = request.model_copy(
                            update={"status": "cancelled", "resolved_at": datetime.now()}
                        )
                        await protocol_repository.update_action_request(request)

                await self._append_snapshot(
                    protocol_repository,
                    run,
                    SnapshotBlueprint(
                        event_type="phase_resolved",
                        phase_name=run.current_phase,
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Forced transition",
                        body=f"The coordinator manually advanced the protocol into {next_phase}.",
                        metadata={"message_kind": "phase_event", "forced": True},
                    ),
                    channel_map,
                )
                run = await self._enter_phase(
                    protocol_repository,
                    definition,
                    room.members,
                    run,
                    channel_map,
                    next_phase,
                )

            elif operation == "override_action":
                request_id = str(payload.get("request_id") or "").strip()
                if not request_id:
                    raise ValueError("request_id is required")
                request = await protocol_repository.get_action_request(request_id)
                if request is None or request.protocol_run_id != run_id:
                    raise LookupError("Action request not found")
                await self._apply_submission(
                    protocol_repository=protocol_repository,
                    definition=definition,
                    room_members=room.members,
                    run=run,
                    channel_map=channel_map,
                    request=request,
                    payload=payload.get("action_payload") or {},
                    actor_agent_id=payload.get("actor_agent_id"),
                    actor_user_id=_LOCAL_USER_ID,
                    submission_status="overridden",
                )

            elif operation == "terminate_run":
                run = run.model_copy(
                    update={"status": "terminated", "completed_at": datetime.now(), "current_turn_key": None}
                )
                run = await protocol_repository.update_run(run)
                await self._append_snapshot(
                    protocol_repository,
                    run,
                    SnapshotBlueprint(
                        event_type="run_completed",
                        phase_name=run.current_phase,
                        channel_slug="system-broadcast",
                        visibility="system",
                        headline="Run terminated",
                        body="The coordinator terminated the protocol before its natural completion.",
                        metadata={"message_kind": "phase_event", "terminated": True},
                    ),
                    channel_map,
                )

            else:
                raise ValueError(f"Unsupported control operation: {operation}")

            await session.commit()

        detail = await self.get_run_detail(run_id)
        self._publish_run_event(
            detail.run,
            reason=operation,
            headline="Protocol control applied",
        )
        return detail

    async def _apply_submission(
        self,
        *,
        protocol_repository: ProtocolSqlRepository,
        definition: ProtocolDefinition,
        room_members: list[MemberRecord],
        run: ProtocolRunRecord,
        channel_map: dict[str, ChannelAggregate],
        request: ActionRequestRecord,
        payload: dict[str, Any],
        actor_agent_id: Optional[str],
        actor_user_id: Optional[str],
        submission_status: str,
    ) -> ProtocolRunRecord:
        actor_agent_id = str(actor_agent_id or "").strip() or None
        actor_user_id = str(actor_user_id or "").strip() or None
        if actor_agent_id is None and actor_user_id is None:
            if len(request.allowed_actor_agent_ids) == 1:
                actor_agent_id = request.allowed_actor_agent_ids[0]
            else:
                raise ValueError("actor_agent_id is required for this action")
        if actor_agent_id and request.allowed_actor_agent_ids and actor_agent_id not in request.allowed_actor_agent_ids:
            raise ValueError("actor_agent_id is not allowed for this request")

        submission = await protocol_repository.create_action_submission(
            ActionSubmissionRecord(
                id=random_uuid(),
                request_id=request.id,
                protocol_run_id=run.id,
                channel_id=request.channel_id,
                actor_type="user" if actor_user_id else "agent",
                actor_agent_id=actor_agent_id,
                actor_user_id=actor_user_id,
                action_type=request.action_type,
                payload=payload,
                status=submission_status,
                metadata={"phase_name": request.phase_name},
            )
        )
        request = request.model_copy(
            update={"status": "resolved", "resolved_at": datetime.now()}
        )
        await protocol_repository.update_action_request(request)

        channel = self._channel_by_id(channel_map, request.channel_id)
        if channel is not None and request.action_type != "speak":
            audience_agent_ids = request.audience_agent_ids or self._channel_agent_ids(channel)
            await self._append_snapshot(
                protocol_repository,
                run,
                SnapshotBlueprint(
                    event_type="action_submitted",
                    phase_name=run.current_phase,
                    channel_slug=channel.channel.slug,
                    actor_agent_id=actor_agent_id,
                    visibility=channel.channel.visibility,
                    audience_agent_ids=audience_agent_ids,
                    headline=f"{request.action_type} submitted",
                    body=f"{actor_agent_id or actor_user_id} submitted an action payload.",
                    metadata={"message_kind": "action_submission", "request_id": request.id},
                ),
                channel_map,
            )

        requests = await protocol_repository.list_action_requests(run.id)
        pending_requests = [
            item for item in requests if item.phase_name == run.current_phase and item.status == "pending"
        ]
        if pending_requests:
            return run

        submissions = await protocol_repository.list_action_submissions(run.id)
        phase_requests = [item for item in requests if item.phase_name == run.current_phase]
        phase_submissions = [
            item for item in submissions if any(item.request_id == request_item.id for request_item in phase_requests)
        ]
        plan = definition.reduce_phase(
            run=run,
            members=room_members,
            channels=channel_map,
            requests=phase_requests,
            submissions=phase_submissions,
        )
        return await self._apply_phase_plan(
            protocol_repository,
            definition,
            room_members,
            run,
            channel_map,
            plan,
        )

    async def _enter_phase(
        self,
        protocol_repository: ProtocolSqlRepository,
        definition: ProtocolDefinition,
        room_members: list[MemberRecord],
        run: ProtocolRunRecord,
        channel_map: dict[str, ChannelAggregate],
        phase_name: str,
    ) -> ProtocolRunRecord:
        run = run.model_copy(
            update={
                "current_phase": phase_name,
                "phase_index": definition.phases.index(phase_name),
                "current_turn_key": None,
            }
        )
        run = await protocol_repository.update_run(run)
        plan = definition.on_phase_enter(run, room_members, channel_map)
        return await self._apply_phase_plan(
            protocol_repository,
            definition,
            room_members,
            run,
            channel_map,
            plan,
        )

    async def _apply_phase_plan(
        self,
        protocol_repository: ProtocolSqlRepository,
        definition: ProtocolDefinition,
        room_members: list[MemberRecord],
        run: ProtocolRunRecord,
        channel_map: dict[str, ChannelAggregate],
        plan: PhasePlan,
    ) -> ProtocolRunRecord:
        if plan.state_patch:
            run = run.model_copy(update={"state": self._merge_state(run.state, plan.state_patch)})
        if plan.current_turn_key is not None or plan.current_turn_key is None:
            run = run.model_copy(update={"current_turn_key": plan.current_turn_key})
        if plan.status_override:
            run = run.model_copy(
                update={
                    "status": plan.status_override,
                    "completed_at": datetime.now() if plan.status_override in {"completed", "terminated"} else run.completed_at,
                }
            )
        run = await protocol_repository.update_run(run)

        for snapshot in plan.snapshots:
            await self._append_snapshot(protocol_repository, run, snapshot, channel_map)
        for request_blueprint in plan.action_requests:
            await self._append_request(protocol_repository, run, request_blueprint, channel_map)

        if plan.next_phase and not plan.action_requests and run.status == "running":
            return await self._enter_phase(
                protocol_repository,
                definition,
                room_members,
                run,
                channel_map,
                plan.next_phase,
            )
        return run

    async def _append_request(
        self,
        protocol_repository: ProtocolSqlRepository,
        run: ProtocolRunRecord,
        blueprint: ActionRequestBlueprint,
        channel_map: dict[str, ChannelAggregate],
    ) -> None:
        channel = channel_map.get(blueprint.channel_slug or "")
        channel_id = channel.channel.id if channel else None
        audience_agent_ids = (
            blueprint.audience_agent_ids
            or (self._channel_agent_ids(channel) if channel else [])
        )
        request = await protocol_repository.create_action_request(
            ActionRequestRecord(
                id=random_uuid(),
                protocol_run_id=run.id,
                channel_id=channel_id,
                phase_name=blueprint.phase_name,
                turn_key=blueprint.turn_key,
                action_type=blueprint.action_type,
                status="pending",
                requested_by_agent_id=blueprint.requested_by_agent_id,
                allowed_actor_agent_ids=blueprint.allowed_actor_agent_ids,
                audience_agent_ids=audience_agent_ids,
                input_schema=blueprint.input_schema,
                target_scope=blueprint.target_scope,
                prompt_text=blueprint.prompt_text,
                metadata=blueprint.metadata,
            )
        )
        if channel is not None:
            await self._append_snapshot(
                protocol_repository,
                run,
                SnapshotBlueprint(
                    event_type="action_requested",
                    phase_name=blueprint.phase_name,
                    channel_slug=channel.channel.slug,
                    visibility=channel.channel.visibility,
                    audience_agent_ids=audience_agent_ids,
                    headline=f"{blueprint.action_type} requested",
                    body=blueprint.prompt_text,
                    metadata={
                        "message_kind": "action_prompt",
                        "request_id": request.id,
                        "action_type": blueprint.action_type,
                    },
                ),
                channel_map,
            )

    async def _append_snapshot(
        self,
        protocol_repository: ProtocolSqlRepository,
        run: ProtocolRunRecord,
        blueprint: SnapshotBlueprint,
        channel_map: dict[str, ChannelAggregate],
    ) -> None:
        channel = channel_map.get(blueprint.channel_slug or "")
        channel_id = channel.channel.id if channel else None
        visibility = blueprint.visibility
        if channel is not None and not blueprint.audience_agent_ids:
            audience_agent_ids = self._channel_agent_ids(channel)
        else:
            audience_agent_ids = list(blueprint.audience_agent_ids)

        latest_seq = await protocol_repository.get_latest_snapshot_seq(run.id)
        await protocol_repository.create_snapshot(
            RunStateSnapshotRecord(
                id=random_uuid(),
                protocol_run_id=run.id,
                event_seq=latest_seq + 1,
                phase_name=blueprint.phase_name,
                event_type=blueprint.event_type,
                channel_id=channel_id,
                actor_agent_id=blueprint.actor_agent_id,
                visibility=visibility,
                audience_agent_ids=audience_agent_ids,
                headline=blueprint.headline,
                body=blueprint.body,
                state=blueprint.state,
                metadata=blueprint.metadata,
            )
        )

    def _project_snapshot_for_viewer(
        self,
        snapshot: RunStateSnapshotRecord,
        channels: list[ChannelAggregate],
        viewer_agent_id: Optional[str],
    ) -> RunStateSnapshotRecord:
        channel = next(
            (item for item in channels if item.channel.id == snapshot.channel_id),
            None,
        )
        visible = self._is_snapshot_visible(snapshot, channel, viewer_agent_id)
        if visible:
            return snapshot
        return snapshot.model_copy(
            update={
                "headline": snapshot.headline or "Restricted coordination event",
                "body": None,
                "metadata": {
                    **snapshot.metadata,
                    "redacted": True,
                    "channel_name": channel.channel.name if channel else None,
                },
            }
        )

    def _annotate_channel_for_viewer(
        self,
        channel: ChannelAggregate,
        viewer_agent_id: Optional[str],
    ) -> ChannelAggregate:
        is_visible = self._is_channel_visible(channel, viewer_agent_id)
        channel_copy = channel.channel.model_copy(
            update={
                "metadata": {
                    **channel.channel.metadata,
                    "is_visible": is_visible,
                    "member_agent_ids": self._channel_agent_ids(channel),
                }
            }
        )
        return ChannelAggregate(channel=channel_copy, members=channel.members)

    def _is_snapshot_visible(
        self,
        snapshot: RunStateSnapshotRecord,
        channel: Optional[ChannelAggregate],
        viewer_agent_id: Optional[str],
    ) -> bool:
        if snapshot.visibility in {"public", "system"}:
            return True
        if viewer_agent_id and viewer_agent_id in snapshot.audience_agent_ids:
            return True
        if viewer_agent_id and channel and self._is_channel_visible(channel, viewer_agent_id):
            return True
        return False

    def _is_channel_visible(
        self,
        channel: ChannelAggregate,
        viewer_agent_id: Optional[str],
    ) -> bool:
        if channel.channel.visibility in {"public", "system"}:
            return True
        if not viewer_agent_id:
            return False
        return viewer_agent_id in self._channel_agent_ids(channel)

    def _channel_agent_ids(self, channel: Optional[ChannelAggregate]) -> list[str]:
        if channel is None:
            return []
        return [
            member.member_agent_id
            for member in channel.members
            if member.member_type == "agent" and member.member_agent_id
        ]

    def _channel_by_id(
        self,
        channel_map: dict[str, ChannelAggregate],
        channel_id: Optional[str],
    ) -> Optional[ChannelAggregate]:
        if not channel_id:
            return None
        for aggregate in channel_map.values():
            if aggregate.channel.id == channel_id:
                return aggregate
        return None

    def _resolve_definition(self, definition_slug: str) -> ProtocolDefinition:
        definition = protocol_definition_registry.get(definition_slug)
        if definition is None:
            raise ValueError(f"Unsupported protocol definition: {definition_slug}")
        return definition

    def _resolve_definition_by_run(self, run: ProtocolRunRecord) -> ProtocolDefinition:
        definition_slug = str(run.run_config.get("definition_slug") or "werewolf_demo")
        return self._resolve_definition(definition_slug)

    def _build_channel_members(
        self,
        channel_id: str,
        blueprint: Any,
    ) -> list[ChannelMemberRecord]:
        members: list[ChannelMemberRecord] = []
        if blueprint.include_user:
            members.append(
                ChannelMemberRecord(
                    id=random_uuid(),
                    channel_id=channel_id,
                    member_type="user",
                    member_user_id=_LOCAL_USER_ID,
                    role_label="observer",
                )
            )
        for agent_id in blueprint.member_agent_ids:
            members.append(
                ChannelMemberRecord(
                    id=random_uuid(),
                    channel_id=channel_id,
                    member_type="agent",
                    member_agent_id=agent_id,
                    role_label=str(blueprint.metadata.get("role") or ""),
                )
            )
        return members

    def _room_agent_ids(self, members: list[MemberRecord]) -> list[str]:
        return [
            member.member_agent_id
            for member in members
            if member.member_type == "agent" and member.member_agent_id
        ]

    def _merge_state(self, current: dict[str, Any], patch: dict[str, Any]) -> dict[str, Any]:
        merged = deepcopy(current)
        for key, value in patch.items():
            if isinstance(value, dict) and isinstance(merged.get(key), dict):
                merged[key] = self._merge_state(merged[key], value)
            else:
                merged[key] = value
        return merged

    def _resolve_next_phase(
        self,
        definition: ProtocolDefinition,
        current_phase: str,
    ) -> Optional[str]:
        try:
            index = definition.phases.index(current_phase)
        except ValueError:
            return None
        if index + 1 >= len(definition.phases):
            return None
        return definition.phases[index + 1]

    def _publish_run_event(
        self,
        run: ProtocolRunRecord,
        *,
        reason: str,
        headline: str,
    ) -> None:
        protocol_event_bus.publish(
            run.id,
            EventMessage(
                event_type="room_state",
                room_id=run.room_id,
                protocol_run_id=run.id,
                visibility="system",
                message_kind="room_state",
                data={
                    "run_id": run.id,
                    "room_id": run.room_id,
                    "status": run.status,
                    "current_phase": run.current_phase,
                    "headline": headline,
                    "reason": reason,
                    "updated_at": run.updated_at.isoformat() if run.updated_at else None,
                },
            ),
        )


protocol_room_service = ProtocolRoomService()
