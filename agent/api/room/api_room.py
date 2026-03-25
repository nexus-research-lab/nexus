# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_room.py
# @Date   ：2026/03/19 22:10
# @Author ：leemysw
# 2026/03/19 22:10   Create
# =====================================================

"""Room API。"""

from __future__ import annotations

from typing import Any, Literal, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.service.room.room_service import room_service
from agent.service.protocol.protocol_service import protocol_room_service

router = APIRouter(tags=["room"])


class CreateRoomRequest(BaseModel):
    """创建 Room 请求。"""

    agent_ids: list[str] = Field(..., description="参与房间的 Agent 列表")
    name: Optional[str] = Field(default=None, description="房间名称")
    description: str = Field(default="", description="房间描述")
    title: Optional[str] = Field(default=None, description="主对话标题")


class AddRoomMemberRequest(BaseModel):
    """追加 Room 成员请求。"""

    agent_id: str = Field(..., description="要邀请的 Agent")


class CreateProtocolRunRequest(BaseModel):
    """创建协议运行请求。"""

    definition_slug: str = Field(default="werewolf_demo", description="协议定义 slug")
    title: Optional[str] = Field(default=None, description="运行标题")
    run_config: dict[str, Any] = Field(default_factory=dict, description="运行配置")


class SubmitProtocolActionRequest(BaseModel):
    """提交协议动作请求。"""

    request_id: str = Field(..., description="动作请求 ID")
    payload: dict[str, Any] = Field(default_factory=dict, description="动作载荷")
    actor_agent_id: Optional[str] = Field(default=None, description="执行动作的 agent")
    actor_user_id: Optional[str] = Field(default=None, description="执行动作的 user")


class ControlProtocolRunRequest(BaseModel):
    """协议运行控制请求。"""

    operation: Literal[
        "pause",
        "resume",
        "inject_message",
        "force_transition",
        "override_action",
        "terminate_run",
    ] = Field(..., description="控制操作")
    payload: dict[str, Any] = Field(default_factory=dict, description="控制参数")


@router.get("/rooms")
async def list_rooms(limit: int = 20):
    """读取最近房间列表。"""
    rooms = await room_service.list_rooms(limit=limit)
    return resp.ok(resp.Resp(data=[room.model_dump(mode="json") for room in rooms]))


@router.post("/rooms")
async def create_room(request: CreateRoomRequest):
    """创建一个新的房间上下文。"""
    try:
        context = await room_service.create_room(
            agent_ids=request.agent_ids,
            name=request.name,
            description=request.description,
            title=request.title,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.get("/rooms/{room_id}")
async def get_room(room_id: str):
    """读取单个房间。"""
    try:
        room = await room_service.get_room(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=room.model_dump(mode="json")))


@router.get("/rooms/{room_id}/contexts")
async def get_room_contexts(room_id: str):
    """读取房间下的全部上下文。"""
    try:
        contexts = await room_service.get_room_contexts(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in contexts]))


@router.post("/rooms/{room_id}/members")
async def add_room_member(room_id: str, request: AddRoomMemberRequest):
    """向房间追加 Agent 成员。"""
    try:
        context = await room_service.add_agent_member(room_id=room_id, agent_id=request.agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.get("/rooms/{room_id}/protocol-runs")
async def list_room_protocol_runs(room_id: str):
    """读取 room 下的协议运行列表。"""
    try:
        runs = await protocol_room_service.list_room_runs(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in runs]))


@router.post("/rooms/{room_id}/protocol-runs")
async def create_room_protocol_run(room_id: str, request: CreateProtocolRunRequest):
    """创建一个新的协议运行。"""
    try:
        detail = await protocol_room_service.create_run(
            room_id=room_id,
            definition_slug=request.definition_slug,
            title=request.title,
            run_config=request.run_config,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.get("/protocol-runs/{run_id}")
async def get_protocol_run(run_id: str, viewer_agent_id: Optional[str] = None):
    """读取协议运行详情。"""
    try:
        detail = await protocol_room_service.get_run_detail(
            run_id,
            viewer_agent_id=viewer_agent_id,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.get("/protocol-runs/{run_id}/channels")
async def list_protocol_run_channels(run_id: str, viewer_agent_id: Optional[str] = None):
    """读取协议运行频道列表。"""
    try:
        channels = await protocol_room_service.list_channels(
            run_id,
            viewer_agent_id=viewer_agent_id,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[channel.model_dump(mode="json") for channel in channels]))


@router.post("/protocol-runs/{run_id}/actions")
async def submit_protocol_action(run_id: str, request: SubmitProtocolActionRequest):
    """提交协议动作。"""
    try:
        detail = await protocol_room_service.submit_action(
            run_id=run_id,
            request_id=request.request_id,
            payload=request.payload,
            actor_agent_id=request.actor_agent_id,
            actor_user_id=request.actor_user_id,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.post("/protocol-runs/{run_id}/control")
async def control_protocol_run(run_id: str, request: ControlProtocolRunRequest):
    """执行协议运行控制。"""
    try:
        detail = await protocol_room_service.control_run(
            run_id=run_id,
            operation=request.operation,
            payload=request.payload,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))
