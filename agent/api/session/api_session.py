# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_session.py
# @Date   ：2026/2/5 15:09
# @Author ：leemysw
# 2026/2/5 15:09   Create
# =====================================================

"""
Session API

[INPUT]: 依赖 session_store, session_manager, protocol_adapter
[OUTPUT]: 对外提供 /sessions CRUD 端点
[POS]: api 层的 Session 管理端点
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from agent.service.process.protocol_adapter import ProtocolAdapter
from agent.service.schema.model_session import ASession
from agent.service.session.session_router import build_session_key, get_default_agent_id
from agent.service.session_store import session_store
from agent.service.session_manager import session_manager
from agent.shared.server.common import resp

router = APIRouter(tags=["session"])
protocol_adapter = ProtocolAdapter()


# =====================================================
# 桥接层 — 前端 session_key ↔ 内部 session_key
# =====================================================

def _to_session_key(session_key: str, agent_id: Optional[str] = None) -> str:
    """前端 session_key → 内部 session_key（幂等）。"""
    if session_key.startswith("agent:"):
        return session_key
    return build_session_key(
        channel="ws",
        chat_type="dm",
        ref=session_key,
        agent_id=agent_id or get_default_agent_id(),
    )


# =====================================================
# 请求模型
# =====================================================

class CreateSessionRequest(BaseModel):
    """创建会话请求"""
    session_key: str
    agent_id: Optional[str] = None
    title: Optional[str] = "New Chat"


class UpdateSessionRequest(BaseModel):
    """更新会话请求"""
    title: Optional[str] = None


# =====================================================
# API 端点
# =====================================================

@router.get("/sessions", response_model=List[ASession])
async def get_sessions():
    """获取所有会话列表"""
    sessions = await session_store.get_all_sessions()
    data = [s.model_dump() for s in sessions]
    return resp.ok(resp.Resp(data=data))


@router.post("/sessions")
async def create_session(request: CreateSessionRequest):
    """创建新会话"""
    session_key = _to_session_key(request.session_key, request.agent_id)

    existing = await session_store.get_session_info(session_key)
    if existing:
        raise HTTPException(status_code=409, detail="Session already exists")

    success = await session_store.update_session(
        session_key=session_key,
        agent_id=request.agent_id or get_default_agent_id(),
        title=request.title,
    )
    if not success:
        raise HTTPException(status_code=500, detail="Failed to create session")

    session_info = await session_store.get_session_info(session_key)
    if not session_info:
        raise HTTPException(status_code=500, detail="Failed to retrieve created session")

    return resp.ok(resp.Resp(data=session_info.model_dump()))


@router.patch("/sessions/{session_key}")
async def update_session(session_key: str, request: UpdateSessionRequest):
    """更新会话信息"""
    internal_key = _to_session_key(session_key)

    existing = await session_store.get_session_info(internal_key)
    if not existing:
        raise HTTPException(status_code=404, detail="Session not found")

    success = await session_store.update_session(
        session_key=internal_key,
        title=request.title,
    )
    if not success:
        raise HTTPException(status_code=500, detail="Failed to update session")

    session_info = await session_store.get_session_info(internal_key)
    return resp.ok(resp.Resp(data=session_info.model_dump()))


@router.get("/sessions/{session_key}/messages")
async def get_session_messages(session_key: str):
    """获取指定会话的所有消息"""
    internal_key = _to_session_key(session_key)
    messages = await session_store.get_session_messages(internal_key)
    data = protocol_adapter.build_history_messages(messages)
    return resp.ok(resp.Resp(data=data))


@router.delete("/sessions/{session_key}")
async def delete_session(session_key: str):
    """删除会话"""
    internal_key = _to_session_key(session_key)
    session_manager.remove_session(internal_key)

    success = await session_store.delete_session(internal_key)
    if not success:
        raise HTTPException(status_code=404, detail="Session not found")
    return resp.ok(resp.Resp(data={"success": True}))


@router.delete("/sessions/{session_key}/rounds/{round_id}")
async def delete_round(session_key: str, round_id: str):
    """删除一轮对话"""
    internal_key = _to_session_key(session_key)

    existing = await session_store.get_session_info(internal_key)
    if not existing:
        raise HTTPException(status_code=404, detail="Session not found")

    deleted_count = await session_store.delete_round(internal_key, round_id)
    if deleted_count < 0:
        raise HTTPException(status_code=500, detail="Failed to delete round")

    return resp.ok(resp.Resp(data={"success": True, "deleted_count": deleted_count}))
