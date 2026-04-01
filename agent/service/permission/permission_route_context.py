# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_route_context.py
# @Date   ：2026/04/01 22:20
# @Author ：leemysw
# 2026/04/01 22:20   Create
# =====================================================

"""权限事件路由上下文。"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class PermissionRouteContext:
    """描述权限请求应投递到哪个前端会话。"""

    route_session_key: str
    room_id: str | None = None
    conversation_id: str | None = None
    agent_id: str | None = None
    message_id: str | None = None
    caused_by: str | None = None
