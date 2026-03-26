# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room_runtime_service.py
# @Date   ：2026/03/26
# @Author ：OpenAI
# =====================================================

"""统一 room runtime 服务。"""

from __future__ import annotations

from typing import Any, Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_agent import AgentOptions
from agent.schema.model_chat_persistence import MemberRecord
from agent.schema.model_room_runtime import RoomMemberSpec, RoomRuntimeView
from agent.service.agent.agent_service import agent_service
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.protocol.protocol_service import protocol_room_service
from agent.service.room.room_service import room_service
from agent.service.room_runtime.open_room_runtime_service import open_room_runtime_service
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository


class RoomRuntimeService:
    """协调 open / protocol 两类 room 运行时。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    async def create_room(
        self,
        *,
        mode: str,
        member_specs: list[RoomMemberSpec],
        name: Optional[str],
        title: Optional[str],
        description: str,
        ruleset_slug: Optional[str],
        goal: str,
    ) -> RoomRuntimeView:
        """创建统一 room。"""
        resolved = await self._resolve_member_specs(member_specs)
        context = await room_service.create_room(
            agent_ids=[item["agent_id"] for item in resolved],
            name=name,
            description=description,
            title=title,
        )
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(context.room.id)
            if room is None:
                raise LookupError("Room not found after create")
            updated_room = room.room.model_copy(update={
                "mode": mode,
                "runtime_status": "created",
                "orchestrator_agent_id": MainAgentProfile.AGENT_ID,
                "ruleset_slug": ruleset_slug if mode == "protocol" else None,
                "goal": goal,
                "runtime_state": open_room_runtime_service.build_initial_state(goal),
                "capabilities": self._build_capabilities(mode),
            })
            await repository.update_room(updated_room)
            for member in room.members:
                if member.member_type == "user":
                    await repository.update_member(member.model_copy(update={"member_status": "listening"}))
                    continue
                resolved_member = next((item for item in resolved if item["agent_id"] == member.member_agent_id), None)
                await repository.update_member(member.model_copy(update={
                    "member_source": resolved_member["source"] if resolved_member else "existing",
                    "member_role": resolved_member["role_hint"] if resolved_member else None,
                    "member_status": "listening",
                    "member_visibility_scope": ["room"],
                    "workspace_binding": resolved_member["workspace_binding"] if resolved_member else True,
                }))
            await session.commit()
        return await self.get_room_view(context.room.id)

    async def add_member(
        self,
        room_id: str,
        *,
        member_spec: RoomMemberSpec,
    ) -> RoomRuntimeView:
        """向 room 添加成员。"""
        resolved = (await self._resolve_member_specs([member_spec]))[0]
        context = await room_service.add_agent_member(room_id=room_id, agent_id=resolved["agent_id"])
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(context.room.id)
            if room is None:
                raise LookupError("Room not found")
            created_member = next((item for item in room.members if item.member_agent_id == resolved["agent_id"]), None)
            if created_member is not None:
                await repository.update_member(created_member.model_copy(update={
                    "member_source": resolved["source"],
                    "member_role": resolved["role_hint"],
                    "member_status": "listening",
                    "member_visibility_scope": ["room"],
                    "workspace_binding": resolved["workspace_binding"],
                }))
            await session.commit()
        await open_room_runtime_service.ensure_workspace_subscriptions(room_id)
        return await self.get_room_view(room_id)

    async def get_room_view(self, room_id: str, viewer_member_id: Optional[str] = None) -> RoomRuntimeView:
        """读取 room 视图。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
        if room is None:
            raise LookupError("Room not found")
        if room.room.mode == "open":
            return await open_room_runtime_service.get_view(room_id, viewer_member_id=viewer_member_id)

        runs = await protocol_room_service.list_room_runs(room_id)
        active_run = room.room.active_run_id or (runs[0].run.id if runs else None)
        detail = await protocol_room_service.get_run_detail(active_run, viewer_agent_id=self._viewer_agent_id(room.members, viewer_member_id)) if active_run else None
        view = RoomRuntimeView(room=room.room, members=room.members, protocol_runs=runs, protocol_detail=detail, viewer_member_id=viewer_member_id)
        if detail:
            for snapshot in detail.snapshots:
                view.events.append(open_room_runtime_service._build_event(room=room.room, event_type=snapshot.event_type, actor_member_id=None, title=snapshot.headline or snapshot.event_type, body=snapshot.body or "", member_ids=[member.id for member in room.members], visibility=snapshot.visibility, metadata=snapshot.metadata))
        return view

    async def start_room(self, room_id: str) -> RoomRuntimeView:
        """启动 room。"""
        room = await room_service.get_room(room_id)
        if room.room.mode == "open":
            return await open_room_runtime_service.start(room_id)
        detail = await protocol_room_service.create_run(room_id=room_id, definition_slug=room.room.ruleset_slug or "werewolf_demo")
        await self._update_protocol_room_runtime(room_id, detail.run.id, "running")
        return await self.get_room_view(room_id)

    async def post_message(self, room_id: str, *, scope: str, content: str, sender_member_id: Optional[str], target_member_ids: Optional[list[str]] = None, metadata: Optional[dict[str, Any]] = None) -> RoomRuntimeView:
        """向 room 投递消息。"""
        room = await room_service.get_room(room_id)
        if room.room.mode == "open":
            return await open_room_runtime_service.append_message(room_id, scope=scope, content=content, sender_member_id=sender_member_id, target_member_ids=target_member_ids, metadata=metadata)
        detail = await self.get_room_view(room_id)
        if not detail.protocol_detail:
            raise ValueError("Protocol room has no active run")
        channel = next((item for item in detail.protocol_detail.channels if item.channel.slug == "public-main"), detail.protocol_detail.channels[0])
        await protocol_room_service.control_run(detail.protocol_detail.run.id, "inject_message", {"channel_id": channel.channel.id, "content": content, "headline": "Room broadcast", "message_kind": scope})
        return await self.get_room_view(room_id)

    async def post_action(self, room_id: str, *, action_type: str, actor_member_id: Optional[str], target_member_id: Optional[str], payload: Optional[dict[str, Any]] = None) -> RoomRuntimeView:
        """提交 room 动作。"""
        room = await room_service.get_room(room_id)
        if room.room.mode == "open":
            return await open_room_runtime_service.submit_action(room_id, action_type=action_type, actor_member_id=actor_member_id, target_member_id=target_member_id, payload=payload)
        raise ValueError("Protocol room actions should use protocol action APIs in this phase")

    async def list_events(self, room_id: str) -> list[dict[str, Any]]:
        """读取 room 事件流。"""
        return (await self.get_room_view(room_id)).model_dump(mode="json")["events"]

    async def list_artifacts(self, room_id: str) -> list[dict[str, Any]]:
        """读取 room 产物。"""
        return (await self.get_room_view(room_id)).model_dump(mode="json")["artifacts"]

    async def tick_room(self, room_id: str) -> RoomRuntimeView:
        """推进 room 一步。"""
        return await self.run_phase(room_id)

    async def run_phase(self, room_id: str) -> RoomRuntimeView:
        """推进当前 phase。"""
        room = await room_service.get_room(room_id)
        if room.room.mode == "open":
            view = await open_room_runtime_service.get_view(room_id)
            if view.room.runtime_status == "created":
                return await open_room_runtime_service.start(room_id)
            if view.room.runtime_state.get("phase") == "review":
                async with self._db.session() as session:
                    repository = RoomSqlRepository(session)
                    current = await repository.get(room_id)
                    if current is None:
                        raise LookupError("Room not found")
                    state = dict(current.room.runtime_state)
                    state["phase"] = "finished"
                    await repository.update_room(current.room.model_copy(update={"runtime_status": "finished", "runtime_state": state}))
                    await session.commit()
                return await open_room_runtime_service.get_view(room_id)
            return view
        detail = await self.get_room_view(room_id)
        if not detail.protocol_detail:
            return await self.start_room(room_id)
        await protocol_room_service.control_run(detail.protocol_detail.run.id, "force_transition", {})
        return await self.get_room_view(room_id)

    async def run_until_finished(self, room_id: str) -> RoomRuntimeView:
        """尽量将 room 推进到结束。"""
        view = await self.get_room_view(room_id)
        guard = 0
        while view.room.runtime_status not in {"finished", "error"} and guard < 12:
            view = await self.run_phase(room_id)
            guard += 1
        return view

    async def _resolve_member_specs(self, member_specs: list[RoomMemberSpec]) -> list[dict[str, Any]]:
        resolved: list[dict[str, Any]] = []
        for spec in member_specs:
            if spec.existing_agent_id:
                resolved.append({"agent_id": spec.existing_agent_id, "source": spec.source, "role_hint": spec.role_hint, "workspace_binding": spec.workspace_binding})
                continue
            create_spec = spec.create_spec or {}
            created = await agent_service.create_agent(
                name=str(create_spec.get("name") or f"room-member-{len(resolved) + 1}"),
                options=AgentOptions(model=create_spec.get("model"), permission_mode="default", skills_enabled=True, setting_sources=["user", "project", "local"]),
            )
            resolved.append({"agent_id": created.agent_id, "source": "ephemeral", "role_hint": spec.role_hint or create_spec.get("role_hint"), "workspace_binding": spec.workspace_binding})
        return resolved

    async def _update_protocol_room_runtime(self, room_id: str, run_id: str, runtime_status: str) -> None:
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None:
                return
            await repository.update_room(room.room.model_copy(update={"active_run_id": run_id, "runtime_status": runtime_status}))
            await session.commit()

    def _viewer_agent_id(self, members: list[MemberRecord], viewer_member_id: Optional[str]) -> Optional[str]:
        if not viewer_member_id:
            return None
        member = next((item for item in members if item.id == viewer_member_id), None)
        return member.member_agent_id if member else None

    def _build_capabilities(self, mode: str) -> dict[str, Any]:
        return {"invite": True, "create_agent": True, "private_channels": mode == "protocol", "action_resolution": mode == "protocol", "workspace_handoff": mode == "open"}


room_runtime_service = RoomRuntimeService()
