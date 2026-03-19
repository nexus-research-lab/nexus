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

from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.service.room.room_service import room_service

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
