# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：open_room_runtime_service.py
# @Date   ：2026/03/26
# @Author ：OpenAI
# =====================================================

"""Open meeting-room 运行时服务。"""

from __future__ import annotations

from datetime import datetime
from pathlib import Path
from typing import Any, Optional

from agent.infra.database.get_db import get_db
from agent.schema.model_chat_persistence import MemberRecord, RoomRecord
from agent.schema.model_room_runtime import (
    RoomArtifactRecord,
    RoomEventRecord,
    RoomRuntimeView,
    RoomTaskRecord,
)
from agent.schema.model_workspace import WorkspaceEvent
from agent.storage.sqlite.room_sql_repository import RoomSqlRepository
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_service import workspace_service
from agent.utils.utils import random_uuid


class OpenRoomRuntimeService:
    """负责开放协作 room 的运行时状态。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")
        self._workspace_tokens: dict[str, dict[str, str]] = {}

    def build_initial_state(self, goal: str = "") -> dict[str, Any]:
        """构建 open room 初始状态。"""
        return {
            "phase": "setup",
            "step": 0,
            "goal": goal,
            "events": [],
            "tasks": [],
            "artifacts": [],
        }

    async def start(self, room_id: str) -> RoomRuntimeView:
        """启动 open room。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")
            if room.room.mode != "open":
                raise ValueError("Only open rooms can use open runtime")

            state = self._normalize_state(room.room)
            state["phase"] = "active"
            state["step"] = int(state.get("step") or 0) + 1
            self._append_event(
                state,
                self._build_event(
                    room=room.room,
                    event_type="room_started",
                    title="会议室已启动",
                    body=room.room.goal or "主智能体已经将房间切换到协作状态。",
                    member_ids=self._member_ids(room.members),
                ),
            )
            updated_room = room.room.model_copy(update={"runtime_status": "running", "runtime_state": state})
            await repository.update_room(updated_room)
            await session.commit()

        await self.ensure_workspace_subscriptions(room_id)
        return await self.get_view(room_id)

    async def get_view(self, room_id: str, viewer_member_id: Optional[str] = None) -> RoomRuntimeView:
        """读取 open room 视图。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")

        state = self._normalize_state(room.room)
        events = [RoomEventRecord.model_validate(item) for item in state.get("events", [])]
        artifacts = [RoomArtifactRecord.model_validate(item) for item in state.get("artifacts", [])]
        tasks = [RoomTaskRecord.model_validate(item) for item in state.get("tasks", [])]
        if viewer_member_id:
            events = [event for event in events if self._is_event_visible(event, viewer_member_id)]
        return RoomRuntimeView(
            room=room.room,
            members=room.members,
            events=events,
            artifacts=artifacts,
            tasks=tasks,
            viewer_member_id=viewer_member_id,
        )

    async def append_message(
        self,
        room_id: str,
        *,
        scope: str,
        content: str,
        sender_member_id: Optional[str],
        target_member_ids: Optional[list[str]] = None,
        metadata: Optional[dict[str, Any]] = None,
    ) -> RoomRuntimeView:
        """向 open room 追加消息事件。"""
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")

            state = self._normalize_state(room.room)
            audience_member_ids = self._resolve_message_audience(
                room.members,
                scope=scope,
                sender_member_id=sender_member_id,
                target_member_ids=target_member_ids or [],
            )
            self._append_event(
                state,
                self._build_event(
                    room=room.room,
                    event_type="message",
                    actor_member_id=sender_member_id,
                    title="房间广播" if scope == "broadcast" else "@成员消息" if scope == "direct" else "房间消息",
                    body=content.strip(),
                    member_ids=audience_member_ids,
                    visibility=self._resolve_message_visibility(scope),
                    metadata={"scope": scope, **(metadata or {})},
                ),
            )
            await repository.update_room(room.room.model_copy(update={"runtime_state": state}))
            await session.commit()
        return await self.get_view(room_id)

    async def submit_action(
        self,
        room_id: str,
        *,
        action_type: str,
        actor_member_id: Optional[str],
        target_member_id: Optional[str],
        payload: Optional[dict[str, Any]] = None,
    ) -> RoomRuntimeView:
        """处理 open room 动作。"""
        payload = payload or {}
        task_write_job: Optional[tuple[str, str, str]] = None
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None:
                raise LookupError("Room not found")

            state = self._normalize_state(room.room)
            members_by_id = {member.id: member for member in room.members}

            if action_type == "assign_task":
                task, task_content = self._assign_task(room.room, members_by_id, actor_member_id, target_member_id, payload)
                state["tasks"].append(task.model_dump(mode="json"))
                self._append_event(
                    state,
                    self._build_event(
                        room=room.room,
                        event_type="task_assigned",
                        actor_member_id=actor_member_id,
                        title=f"任务已分配给 {task.assignee_agent_id or task.assignee_member_id}",
                        body=task.summary or task.title,
                        member_ids=self._member_ids(room.members),
                        metadata={"task_id": task.id},
                    ),
                )
                await self._update_member_status(repository, members_by_id.get(target_member_id or ""), "working")
                task_write_job = (task.assignee_agent_id or "", task.workspace_path or "room_tasks/task.md", task_content)
            elif action_type == "submit_result":
                artifact = RoomArtifactRecord(
                    id=random_uuid(),
                    owner_member_id=actor_member_id,
                    owner_agent_id=members_by_id.get(actor_member_id or "") and members_by_id[actor_member_id].member_agent_id,
                    kind=str(payload.get("kind") or "note"),
                    title=str(payload.get("title") or "提交结果"),
                    summary=str(payload.get("summary") or ""),
                    workspace_ref=payload.get("workspace_ref"),
                    source_ref=payload.get("source_ref"),
                )
                state["artifacts"].append(artifact.model_dump(mode="json"))
                self._append_event(state, self._build_event(room=room.room, event_type="artifact_created", actor_member_id=actor_member_id, title=artifact.title, body=artifact.summary, member_ids=self._member_ids(room.members), metadata={"artifact_id": artifact.id}))
                await self._update_member_status(repository, members_by_id.get(actor_member_id or ""), "done")
            elif action_type == "request_help":
                self._append_event(state, self._build_event(room=room.room, event_type="help_requested", actor_member_id=actor_member_id, title="成员请求协助", body=str(payload.get("summary") or ""), member_ids=self._member_ids(room.members)))
                await self._update_member_status(repository, members_by_id.get(actor_member_id or ""), "blocked")
            elif action_type == "mark_blocked":
                self._append_event(state, self._build_event(room=room.room, event_type="member_blocked", actor_member_id=actor_member_id, title="成员已阻塞", body=str(payload.get("summary") or ""), member_ids=self._member_ids(room.members)))
                await self._update_member_status(repository, members_by_id.get(actor_member_id or ""), "blocked")
            elif action_type == "close_task":
                for task_item in state.get("tasks", []):
                    if task_item.get("id") == payload.get("task_id"):
                        task_item["status"] = "closed"
                        task_item["updated_at"] = datetime.now().isoformat()
                self._append_event(state, self._build_event(room=room.room, event_type="task_closed", actor_member_id=actor_member_id, title="任务已关闭", body=str(payload.get("summary") or ""), member_ids=self._member_ids(room.members)))
            elif action_type == "broadcast_note":
                self._append_event(state, self._build_event(room=room.room, event_type="message", actor_member_id=actor_member_id, title="成员补充说明", body=str(payload.get("content") or ""), member_ids=self._member_ids(room.members)))
            else:
                raise ValueError(f"Unsupported open-room action: {action_type}")

            state["phase"] = self._resolve_phase(state)
            await repository.update_room(room.room.model_copy(update={"runtime_state": state, "runtime_status": "running" if state["phase"] != "finished" else "finished"}))
            await session.commit()
        if task_write_job:
            await self.ensure_workspace_subscriptions(room_id)
            await workspace_service.update_workspace_file(task_write_job[0], task_write_job[1], task_write_job[2])
        return await self.get_view(room_id)

    async def ensure_workspace_subscriptions(self, room_id: str, members: Optional[list[MemberRecord]] = None) -> None:
        """确保 room 已订阅成员 workspace 事件。"""
        if members is None:
            members = (await self.get_view(room_id)).members
        tokens = self._workspace_tokens.setdefault(room_id, {})
        for member in members:
            if not member.member_agent_id or not member.workspace_binding or member.member_agent_id in tokens:
                continue

            async def _listener(event: WorkspaceEvent, current_room_id: str = room_id) -> None:
                await self._handle_workspace_event(current_room_id, event)

            tokens[member.member_agent_id] = workspace_event_bus.subscribe(member.member_agent_id, _listener)

    def _assign_task(self, room: RoomRecord, members_by_id: dict[str, MemberRecord], actor_member_id: Optional[str], target_member_id: Optional[str], payload: dict[str, Any]) -> tuple[RoomTaskRecord, str]:
        target_member = members_by_id.get(target_member_id or "")
        if target_member is None or not target_member.member_agent_id:
            raise ValueError("assign_task requires an agent member target")
        task = RoomTaskRecord(
            id=random_uuid(),
            title=str(payload.get("title") or "未命名任务"),
            summary=str(payload.get("summary") or ""),
            assignee_member_id=target_member.id,
            assignee_agent_id=target_member.member_agent_id,
            created_by=actor_member_id,
            workspace_path=f"room_tasks/{room.id}/{random_uuid()}.md",
            metadata={"goal": room.goal},
        )
        content = "\n".join([
            f"# {task.title}",
            "",
            f"- room: {room.name or room.id}",
            f"- goal: {room.goal or '未设置'}",
            f"- summary: {task.summary or '无'}",
        ])
        return task, content

    async def _handle_workspace_event(self, room_id: str, event: WorkspaceEvent) -> None:
        if event.type not in {"file_write_start", "file_write_end"}:
            return
        async with self._db.session() as session:
            repository = RoomSqlRepository(session)
            room = await repository.get(room_id)
            if room is None or room.room.mode != "open":
                return
            member = next((item for item in room.members if item.member_agent_id == event.agent_id), None)
            if member is None:
                return
            state = self._normalize_state(room.room)
            title = "成员开始更新 workspace" if event.type == "file_write_start" else "成员完成 workspace 更新"
            body = f"{event.agent_id} · {event.path}"
            self._append_event(state, self._build_event(room=room.room, event_type="workspace_event", actor_member_id=member.id, title=title, body=body, member_ids=self._member_ids(room.members), metadata={"workspace_event_type": event.type, "path": event.path}))
            if event.type == "file_write_end":
                state["artifacts"].append(RoomArtifactRecord(id=random_uuid(), owner_member_id=member.id, owner_agent_id=event.agent_id, kind="workspace_file", title=Path(event.path).name, summary=f"{event.agent_id} 更新了 {event.path}", workspace_ref=event.path).model_dump(mode="json"))
            await repository.update_member(member.model_copy(update={"member_status": "working" if event.type == "file_write_start" else "done"}))
            await repository.update_room(room.room.model_copy(update={"runtime_state": state}))
            await session.commit()

    def _normalize_state(self, room: RoomRecord) -> dict[str, Any]:
        state = dict(room.runtime_state or {})
        state.setdefault("phase", "setup" if room.runtime_status == "created" else "active")
        state.setdefault("step", 0)
        state.setdefault("goal", room.goal)
        state.setdefault("events", [])
        state.setdefault("tasks", [])
        state.setdefault("artifacts", [])
        return state

    def _resolve_phase(self, state: dict[str, Any]) -> str:
        tasks = state.get("tasks", [])
        if tasks and all(task.get("status") == "closed" for task in tasks):
            return "review" if not state.get("artifacts") else "finished"
        return "active"

    def _member_ids(self, members: list[MemberRecord]) -> list[str]:
        return [member.id for member in members]

    def _resolve_message_audience(
        self,
        members: list[MemberRecord],
        *,
        scope: str,
        sender_member_id: Optional[str],
        target_member_ids: list[str],
    ) -> list[str]:
        if scope == "broadcast" or scope == "system":
            return self._member_ids(members)
        audience = list(target_member_ids)
        if sender_member_id and sender_member_id not in audience:
            audience.append(sender_member_id)
        return audience or self._member_ids(members)

    def _resolve_message_visibility(self, scope: str) -> str:
        if scope == "system":
            return "system"
        if scope == "broadcast":
            return "public"
        if scope == "group":
            return "group"
        return "private"

    def _append_event(self, state: dict[str, Any], event: RoomEventRecord) -> None:
        state.setdefault("events", []).append(event.model_dump(mode="json"))

    def _build_event(self, *, room: RoomRecord, event_type: str, title: str, body: str, member_ids: list[str], actor_member_id: Optional[str] = None, visibility: str = "public", metadata: Optional[dict[str, Any]] = None) -> RoomEventRecord:
        return RoomEventRecord(id=random_uuid(), room_id=room.id, run_id=room.active_run_id, event_type=event_type, actor_member_id=actor_member_id, visibility=visibility, audience_member_ids=member_ids, title=title, body=body, metadata=metadata or {})

    async def _update_member_status(self, repository: RoomSqlRepository, member: Optional[MemberRecord], status: str) -> None:
        if member is None:
            return
        await repository.update_member(member.model_copy(update={"member_status": status}))

    def _is_event_visible(self, event: RoomEventRecord, viewer_member_id: str) -> bool:
        return event.visibility in {"public", "system"} or viewer_member_id in event.audience_member_ids


open_room_runtime_service = OpenRoomRuntimeService()
