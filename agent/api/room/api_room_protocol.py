# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_room_protocol.py
# @Date   ：2026/03/26
# @Author ：OpenAI
# =====================================================

"""Protocol room 兼容 API。"""

from __future__ import annotations

from typing import Any, Literal, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.service.protocol.protocol_service import protocol_room_service

router = APIRouter(tags=["room-protocol"])


class CreateProtocolRunRequest(BaseModel):
    definition_slug: str = Field(default="werewolf_demo", description="协议定义 slug")
    title: Optional[str] = Field(default=None, description="运行标题")
    run_config: dict[str, Any] = Field(default_factory=dict, description="运行配置")


class SubmitProtocolActionRequest(BaseModel):
    request_id: str = Field(..., description="动作请求 ID")
    payload: dict[str, Any] = Field(default_factory=dict, description="动作载荷")
    actor_agent_id: Optional[str] = Field(default=None, description="执行动作的 agent")
    actor_user_id: Optional[str] = Field(default=None, description="执行动作的 user")


class ControlProtocolRunRequest(BaseModel):
    operation: Literal["pause", "resume", "inject_message", "force_transition", "override_action", "terminate_run"] = Field(..., description="控制操作")
    payload: dict[str, Any] = Field(default_factory=dict, description="控制参数")


@router.get("/rooms/{room_id}/protocol-runs")
async def list_room_protocol_runs(room_id: str):
    try:
        runs = await protocol_room_service.list_room_runs(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in runs]))


@router.post("/rooms/{room_id}/protocol-runs")
async def create_room_protocol_run(room_id: str, request: CreateProtocolRunRequest):
    try:
        detail = await protocol_room_service.create_run(room_id=room_id, definition_slug=request.definition_slug, title=request.title, run_config=request.run_config)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.get("/protocol-runs/{run_id}")
async def get_protocol_run(run_id: str, viewer_agent_id: Optional[str] = None):
    try:
        detail = await protocol_room_service.get_run_detail(run_id, viewer_agent_id=viewer_agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.get("/protocol-runs/{run_id}/channels")
async def list_protocol_run_channels(run_id: str, viewer_agent_id: Optional[str] = None):
    try:
        channels = await protocol_room_service.list_channels(run_id, viewer_agent_id=viewer_agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[channel.model_dump(mode="json") for channel in channels]))


@router.post("/protocol-runs/{run_id}/actions")
async def submit_protocol_action(run_id: str, request: SubmitProtocolActionRequest):
    try:
        detail = await protocol_room_service.submit_action(run_id=run_id, request_id=request.request_id, payload=request.payload, actor_agent_id=request.actor_agent_id, actor_user_id=request.actor_user_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))


@router.post("/protocol-runs/{run_id}/control")
async def control_protocol_run(run_id: str, request: ControlProtocolRunRequest):
    try:
        detail = await protocol_room_service.control_run(run_id=run_id, operation=request.operation, payload=request.payload)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=detail.model_dump(mode="json")))
