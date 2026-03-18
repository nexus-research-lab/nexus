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

[INPUT]: 依赖 session_service 与统一消息协议
[OUTPUT]: 对外提供 /sessions CRUD 端点
[POS]: api 层的 Session 管理端点
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from typing import List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from agent.schema.model_cost import SessionCostSummary
from agent.schema.model_session import ASession
from agent.service.session.session_service import session_service
from agent.infra.server.common import resp

router = APIRouter(tags=["session"])


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
    sessions = await session_service.get_sessions()
    data = [s.model_dump() for s in sessions]
    return resp.ok(resp.Resp(data=data))


@router.post("/sessions")
async def create_session(request: CreateSessionRequest):
    """创建新会话"""
    try:
        session_info = await session_service.create_session(
            session_key=request.session_key,
            agent_id=request.agent_id,
            title=request.title,
        )
    except ValueError as exc:
        raise HTTPException(status_code=409, detail="Session already exists")
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    return resp.ok(resp.Resp(data=session_info.model_dump()))


@router.patch("/sessions/{session_key}")
async def update_session(session_key: str, request: UpdateSessionRequest):
    """更新会话信息"""
    try:
        session_info = await session_service.update_session(session_key=session_key, title=request.title)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=session_info.model_dump()))


@router.get("/sessions/{session_key}/messages")
async def get_session_messages(session_key: str):
    """获取指定会话的所有消息"""
    data = await session_service.get_session_messages(session_key)
    return resp.ok(resp.Resp(data=[item.model_dump(mode="json", exclude_none=True) for item in data]))


@router.get("/sessions/{session_key}/cost/summary", response_model=SessionCostSummary)
async def get_session_cost_summary(session_key: str):
    """获取指定会话的成本汇总。"""
    try:
        summary = await session_service.get_session_cost_summary(session_key)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data=summary.model_dump(mode="json")))


@router.delete("/sessions/{session_key}")
async def delete_session(session_key: str):
    """删除会话"""
    try:
        await session_service.delete_session(session_key)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return resp.ok(resp.Resp(data={"success": True}))


@router.delete("/sessions/{session_key}/rounds/{round_id}")
async def delete_round(session_key: str, round_id: str):
    """删除一轮对话"""
    try:
        deleted_count = await session_service.delete_round(session_key, round_id)
    except LookupError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    return resp.ok(resp.Resp(data={"success": True, "deleted_count": deleted_count}))
