# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_persistence.py
# @Date   ：2026/3/19 00:34
# @Author ：leemysw
# 2026/3/19 00:34   Create
# =====================================================

"""持久化查询与回填 API。"""

from __future__ import annotations

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.service.persistence.persistence_service import persistence_service

router = APIRouter(tags=["persistence"])


class BackfillResponse(BaseModel):
    """回填结果。"""

    agents_synced: int = Field(..., description="同步的 Agent 数量")
    sessions_synced: int = Field(..., description="同步的会话数量")
    messages_synced: int = Field(..., description="同步的消息数量")


@router.get("/persistence/agents")
async def list_persistent_agents():
    """读取新库中的活跃 Agent 聚合。"""
    agents = await persistence_service.list_agents()
    return resp.ok(resp.Resp(data=[agent.model_dump(mode="json") for agent in agents]))


@router.get("/persistence/agents/{agent_id}")
async def get_persistent_agent(agent_id: str):
    """读取新库中的单个 Agent 聚合。"""
    aggregate = await persistence_service.get_agent(agent_id)
    if aggregate is None:
        raise HTTPException(status_code=404, detail="Agent not found in persistence store")
    return resp.ok(resp.Resp(data=aggregate.model_dump(mode="json")))


@router.get("/persistence/rooms")
async def list_persistent_rooms(limit: int = 20):
    """读取新库中的最近房间。"""
    rooms = await persistence_service.list_rooms(limit=limit)
    return resp.ok(resp.Resp(data=[room.model_dump(mode="json") for room in rooms]))


@router.get("/persistence/rooms/{room_id}/contexts")
async def get_persistent_room_contexts(room_id: str):
    """读取房间下的全部对话上下文。"""
    contexts = await persistence_service.get_room_contexts(room_id)
    if not contexts:
        raise HTTPException(status_code=404, detail="Room not found in persistence store")
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in contexts]))


@router.get("/persistence/sessions/{session_id}/messages")
async def get_persistent_session_messages(session_id: str, limit: int = 200):
    """读取会话下的消息索引。"""
    messages = await persistence_service.get_session_messages(session_id=session_id, limit=limit)
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in messages]))


@router.get("/persistence/sessions/{session_id}/rounds")
async def get_persistent_session_rounds(session_id: str):
    """读取会话下的轮次索引。"""
    rounds = await persistence_service.get_session_rounds(session_id=session_id)
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json") for item in rounds]))


@router.post("/persistence/backfill", response_model=BackfillResponse)
async def backfill_persistence():
    """执行一次旧数据到新库的回填。"""
    result = await persistence_service.backfill()
    payload = BackfillResponse(**result)
    return resp.ok(resp.Resp(data=payload.model_dump(mode="json")))
