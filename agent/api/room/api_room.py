# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_room.py
# @Date   ：2026/03/19 22:10
# @Author ：leemysw
# =====================================================

"""Room API。"""

from __future__ import annotations

from typing import Any, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.schema.model_room_runtime import RoomMemberSpec
from agent.service.room.room_service import room_service
from agent.service.room_runtime.room_runtime_service import room_runtime_service

router = APIRouter(tags=["room"])


class RoomMemberSpecRequest(BaseModel):
    """Room 成员请求。"""

    existing_agent_id: Optional[str] = Field(default=None, description="已有 agent")
    create_spec: Optional[dict[str, Any]] = Field(default=None, description="临时成员创建参数")
    role_hint: Optional[str] = Field(default=None, description="角色提示")
    source: str = Field(default="existing", description="成员来源")
    workspace_binding: bool = Field(default=True, description="是否绑定 workspace")
    agent_id: Optional[str] = Field(default=None, description="旧版 agent_id")

    def to_member_spec(self) -> RoomMemberSpec:
        return RoomMemberSpec(
            existing_agent_id=self.existing_agent_id or self.agent_id,
            create_spec=self.create_spec,
            role_hint=self.role_hint,
            source=self.source,
            workspace_binding=self.workspace_binding,
        )


class CreateRoomRequest(BaseModel):
    """创建 Room 请求。"""

    mode: str = Field(default="open", description="room 模式")
    agent_ids: list[str] = Field(default_factory=list, description="旧版 agent 列表")
    member_specs: list[RoomMemberSpecRequest] = Field(default_factory=list, description="成员规格")
    name: Optional[str] = Field(default=None, description="房间名称")
    description: str = Field(default="", description="房间描述")
    title: Optional[str] = Field(default=None, description="主对话标题")
    ruleset_slug: Optional[str] = Field(default=None, description="规则集标识")
    goal: str = Field(default="", description="房间目标")


class RoomMessageRequest(BaseModel):
    """Room 消息请求。"""

    scope: str = Field(default="broadcast", description="消息范围")
    content: str = Field(..., description="消息正文")
    sender_member_id: Optional[str] = Field(default=None, description="发送成员")
    target_member_ids: list[str] = Field(default_factory=list, description="目标成员")
    metadata: dict[str, Any] = Field(default_factory=dict, description="附加信息")


class RoomActionRequest(BaseModel):
    """Room 动作请求。"""

    action_type: str = Field(..., description="动作类型")
    actor_member_id: Optional[str] = Field(default=None, description="发起成员")
    target_member_id: Optional[str] = Field(default=None, description="目标成员")
    payload: dict[str, Any] = Field(default_factory=dict, description="动作负载")


def _translate_errors(exc: Exception) -> HTTPException:
    if isinstance(exc, LookupError):
        return HTTPException(status_code=404, detail=str(exc))
    return HTTPException(status_code=400, detail=str(exc))


def _normalize_member_specs(request: CreateRoomRequest) -> list[RoomMemberSpec]:
    """统一兼容旧版 agent_ids 与新版 member_specs。"""
    if request.member_specs:
        return [item.to_member_spec() for item in request.member_specs]
    return [RoomMemberSpec(existing_agent_id=agent_id) for agent_id in request.agent_ids]


@router.get("/rooms")
async def list_rooms(limit: int = 20):
    rooms = await room_service.list_rooms(limit=limit)
    return resp.ok(resp.Resp(data=[room.model_dump(mode="json") for room in rooms]))


@router.post("/rooms")
async def create_room(request: CreateRoomRequest):
    try:
        view = await room_runtime_service.create_room(
            mode=request.mode,
            member_specs=_normalize_member_specs(request),
            name=request.name,
            title=request.title,
            description=request.description,
            ruleset_slug=request.ruleset_slug,
            goal=request.goal,
        )
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.get("/rooms/{room_id}")
async def get_room(room_id: str):
    try:
        room = await room_service.get_room(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=room.model_dump(mode="json")))


@router.get("/rooms/{room_id}/contexts")
async def get_room_contexts(room_id: str):
    try:
        contexts = await room_service.get_room_contexts(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in contexts]))


@router.get("/rooms/{room_id}/view")
async def get_room_view(room_id: str):
    try:
        view = await room_runtime_service.get_room_view(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.get("/rooms/{room_id}/members/{member_id}/view")
async def get_room_member_view(room_id: str, member_id: str):
    try:
        view = await room_runtime_service.get_room_view(room_id, viewer_member_id=member_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/start")
async def start_room(room_id: str):
    try:
        view = await room_runtime_service.start_room(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/tick")
async def tick_room(room_id: str):
    try:
        view = await room_runtime_service.tick_room(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/run-phase")
async def run_room_phase(room_id: str):
    try:
        view = await room_runtime_service.run_phase(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/run-until-finished")
async def run_room_until_finished(room_id: str):
    try:
        view = await room_runtime_service.run_until_finished(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/messages")
async def post_room_message(room_id: str, request: RoomMessageRequest):
    try:
        view = await room_runtime_service.post_message(
            room_id,
            scope=request.scope,
            content=request.content,
            sender_member_id=request.sender_member_id,
            target_member_ids=request.target_member_ids,
            metadata=request.metadata,
        )
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/actions")
async def post_room_action(room_id: str, request: RoomActionRequest):
    try:
        view = await room_runtime_service.post_action(
            room_id,
            action_type=request.action_type,
            actor_member_id=request.actor_member_id,
            target_member_id=request.target_member_id,
            payload=request.payload,
        )
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.post("/rooms/{room_id}/members")
async def add_room_member(room_id: str, request: RoomMemberSpecRequest):
    try:
        view = await room_runtime_service.add_member(room_id, member_spec=request.to_member_spec())
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=view.model_dump(mode="json")))


@router.get("/rooms/{room_id}/events")
async def list_room_events(room_id: str):
    try:
        events = await room_runtime_service.list_events(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=events))


@router.get("/rooms/{room_id}/artifacts")
async def list_room_artifacts(room_id: str):
    try:
        artifacts = await room_runtime_service.list_artifacts(room_id)
    except Exception as exc:
        raise _translate_errors(exc) from exc
    return resp.ok(resp.Resp(data=artifacts))
