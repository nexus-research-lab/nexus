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
from agent.service.room.room_conversation_service import room_conversation_service
from agent.service.room.room_service import room_service

router = APIRouter(tags=["room"])


class CreateRoomRequest(BaseModel):
    """创建 Room 请求。"""

    agent_ids: list[str] = Field(..., description="参与房间的 Agent 列表")
    name: Optional[str] = Field(default=None, description="房间名称")
    description: str = Field(default="", description="房间描述")
    title: Optional[str] = Field(default=None, description="主对话标题")
    avatar: Optional[str] = Field(default=None, description="房间头像图标")


class AddRoomMemberRequest(BaseModel):
    """追加 Room 成员请求。"""

    agent_id: str = Field(..., description="要邀请的 Agent")


class UpdateRoomRequest(BaseModel):
    """更新 Room 请求。"""

    name: Optional[str] = Field(default=None, description="房间名称")
    description: Optional[str] = Field(default=None, description="房间描述")
    title: Optional[str] = Field(default=None, description="主对话标题")
    avatar: Optional[str] = Field(default=None, description="房间头像图标")


class CreateRoomConversationRequest(BaseModel):
    """创建 Room 对话请求。"""

    title: Optional[str] = Field(default=None, description="对话标题")


class UpdateRoomConversationRequest(BaseModel):
    """更新 Room 对话请求。"""

    title: Optional[str] = Field(default=None, description="对话标题")


@router.get("/rooms/dm/{agent_id}")
async def get_or_create_dm_room(agent_id: str):
    """获取或创建与指定 Agent 的 DM room 上下文。"""
    try:
        context = await room_service.get_or_create_dm_room(agent_id=agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


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
            avatar=request.avatar,
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


@router.patch("/rooms/{room_id}")
async def update_room(room_id: str, request: UpdateRoomRequest):
    """更新房间信息。"""
    try:
        context = await room_service.update_room(
            room_id=room_id,
            name=request.name,
            description=request.description,
            title=request.title,
            avatar=request.avatar,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.get("/rooms/{room_id}/contexts")
async def get_room_contexts(room_id: str):
    """读取房间下的全部上下文。"""
    try:
        contexts = await room_service.get_room_contexts(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in contexts]))


@router.post("/rooms/{room_id}/conversations")
async def create_room_conversation(room_id: str, request: CreateRoomConversationRequest):
    """在 room 内创建一条新对话。"""
    try:
        context = await room_conversation_service.create_room_conversation(
            room_id=room_id,
            title=request.title,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.patch("/rooms/{room_id}/conversations/{conversation_id}")
async def update_room_conversation(
    room_id: str,
    conversation_id: str,
    request: UpdateRoomConversationRequest,
):
    """更新 room 对话标题。"""
    try:
        context = await room_conversation_service.update_room_conversation(
            room_id=room_id,
            conversation_id=conversation_id,
            title=request.title,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.delete("/rooms/{room_id}/conversations/{conversation_id}")
async def delete_room_conversation(room_id: str, conversation_id: str):
    """删除 room 内普通对话。"""
    try:
        context = await room_conversation_service.delete_room_conversation(
            room_id=room_id,
            conversation_id=conversation_id,
        )
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


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


@router.delete("/rooms/{room_id}/members/{agent_id}")
async def remove_room_member(room_id: str, agent_id: str):
    """移除房间中的 Agent 成员。"""
    try:
        context = await room_service.remove_agent_member(room_id=room_id, agent_id=agent_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=context.model_dump(mode="json")))


@router.delete("/rooms/{room_id}")
async def delete_room(room_id: str):
    """删除房间。"""
    try:
        await room_service.delete_room(room_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data={"success": True}))
