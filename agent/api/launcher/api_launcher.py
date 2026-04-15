#!/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_launcher.py
# @Date   ：2026/3/30 00:00
# @Author ：leemysw
# 2026/3/30 00:00   Create
# =====================================================

"""Launcher API。"""

from __future__ import annotations

import asyncio

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from agent.infra.server.common import resp
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.launcher.launcher_service import launcher_service, LauncherAction
from agent.service.room.room_service import room_service

router = APIRouter(tags=["launcher"])


class LauncherQueryRequest(BaseModel):
    """Launcher 查询请求。"""

    query: str = Field(..., description="查询字符串")


class LauncherQueryResponse(BaseModel):
    """Launcher 查询响应。"""

    action_type: str = Field(..., description="动作类型：open_agent_dm, open_room")
    target_id: str = Field(..., description="目标 ID（room_id 或 agent_id）")
    initial_message: str | None = Field(default=None, description="初始消息内容")


class LauncherSuggestion(BaseModel):
    """Launcher 推荐项。"""

    type: str = Field(..., description="类型：agent 或 room")
    id: str = Field(..., description="Agent ID 或 Room ID")
    name: str = Field(..., description="名称")
    avatar: str | None = Field(default=None, description="头像")
    last_activity: str | None = Field(default=None, description="最后活动时间")


class LauncherSuggestionsResponse(BaseModel):
    """Launcher 推荐列表响应。"""

    agents: list[LauncherSuggestion] = Field(default_factory=list, description="推荐 Agent 列表")
    rooms: list[LauncherSuggestion] = Field(default_factory=list, description="推荐 Room 列表")


@router.post("/launcher/query")
async def handle_launcher_query(request: LauncherQueryRequest):
    """处理 Launcher 查询请求。

    解析用户的 @agent-name 或 #room-name 语法，
    返回对应的导航动作。
    """
    try:
        action = await launcher_service.handle_launcher_query(
            query=request.query,
        )

        return resp.ok(
            resp.Resp(
                data=LauncherQueryResponse(
                    action_type=action.action_type,
                    target_id=action.target_id,
                    initial_message=action.initial_message,
                ).model_dump(mode="json"),
            )
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc


@router.get("/launcher/suggestions")
async def get_launcher_suggestions():
    """获取 Launcher 推荐列表。

    返回用户最近使用的 Agents 和 Rooms，
    用于在 Launcher 页面显示快捷入口。
    """
    try:
        agents, rooms = await asyncio.gather(
            launcher_service.get_suggested_agents(),
            launcher_service.get_suggested_rooms(),
        )

        agent_suggestions = [
            LauncherSuggestion(
                type="agent",
                id=agent.agent.id,
                name=agent.profile.display_name or agent.agent.name,
            )
            for agent in MainAgentProfile.filter_regular_agents(
                agents,
                lambda item: item.agent.id,
            )
        ]

        room_suggestions = [
            LauncherSuggestion(
                type="room",
                id=room.room.id,
                name=room.room.name or "未命名 Room",
                last_activity=room.room.created_at.isoformat() if room.room.created_at else None,
            )
            for room in rooms
        ]

        return resp.ok(
            resp.Resp(
                data=LauncherSuggestionsResponse(
                    agents=agent_suggestions,
                    rooms=room_suggestions,
                ).model_dump(mode="json"),
            )
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
