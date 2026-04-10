# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_dispatch_router.py
# @Date   ：2026/04/08 16:28
# @Author ：leemysw
# 2026/04/08 16:28   Create
# =====================================================

"""权限请求派发路由。"""

from __future__ import annotations

from typing import Callable

from agent.schema.model_message import EventMessage
from agent.service.channels.message_sender import MessageSender
from agent.service.permission.pending_permission_request import PendingPermissionRequest


class PermissionDispatchRouter:
    """把权限请求投递给当前 session 控制端。"""

    async def dispatch(
        self,
        pending_request: PendingPermissionRequest,
        build_event: Callable[[PendingPermissionRequest], EventMessage],
        resolve_sender: Callable[[str], MessageSender | None],
    ) -> bool:
        """把权限请求投递到当前控制端。"""
        return await self._dispatch_to_controller(
            pending_request=pending_request,
            build_event=build_event,
            resolve_sender=resolve_sender,
        )

    async def _dispatch_to_controller(
        self,
        pending_request: PendingPermissionRequest,
        build_event: Callable[[PendingPermissionRequest], EventMessage],
        resolve_sender: Callable[[str], MessageSender | None],
    ) -> bool:
        """DM / Room 权限请求统一直发控制端。"""
        sender = resolve_sender(pending_request.dispatch_session_key)
        if sender is None:
            return False
        target_key = f"sender:{id(sender)}"
        if pending_request.dispatched_target_key == target_key:
            return True
        await sender.send(build_event(pending_request))
        pending_request.dispatched_target_key = target_key
        return True
