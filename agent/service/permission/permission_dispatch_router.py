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
from agent.service.channels.ws.ws_connection_registry import ws_connection_registry
from agent.service.permission.pending_permission_request import PendingPermissionRequest


class PermissionDispatchRouter:
    """根据会话类型选择权限请求的投递通道。"""

    async def dispatch(
        self,
        pending_request: PendingPermissionRequest,
        build_event: Callable[[PendingPermissionRequest], EventMessage],
        resolve_sender: Callable[[str], MessageSender | None],
    ) -> bool:
        """把权限请求投递到当前可用的前端连接。"""
        route_context = pending_request.route_context
        if route_context and route_context.room_id:
            return await self._dispatch_to_room(
                pending_request=pending_request,
                build_event=build_event,
                resolve_sender=resolve_sender,
            )
        return await self._dispatch_to_session(
            pending_request=pending_request,
            build_event=build_event,
            resolve_sender=resolve_sender,
        )

    async def _dispatch_to_room(
        self,
        pending_request: PendingPermissionRequest,
        build_event: Callable[[PendingPermissionRequest], EventMessage],
        resolve_sender: Callable[[str], MessageSender | None],
    ) -> bool:
        """Room 权限请求优先走房间订阅广播。"""
        route_context = pending_request.route_context
        if route_context is None or route_context.room_id is None:
            return False
        fallback_sender = resolve_sender(pending_request.dispatch_session_key)
        recipient_ids = ws_connection_registry.resolve_room_recipient_ids(
            room_id=route_context.room_id,
            conversation_id=route_context.conversation_id,
            fallback_sender=fallback_sender,
        )
        if not recipient_ids:
            return False
        target_key = self._build_room_target_key(
            room_id=route_context.room_id,
            conversation_id=route_context.conversation_id,
            recipient_ids=recipient_ids,
        )
        if pending_request.dispatched_target_key == target_key:
            return True
        await ws_connection_registry.broadcast_to_room_subscribers(
            room_id=route_context.room_id,
            event=build_event(pending_request),
            fallback_sender=fallback_sender,
        )
        pending_request.dispatched_target_key = target_key
        return True

    async def _dispatch_to_session(
        self,
        pending_request: PendingPermissionRequest,
        build_event: Callable[[PendingPermissionRequest], EventMessage],
        resolve_sender: Callable[[str], MessageSender | None],
    ) -> bool:
        """DM / 单会话权限请求继续走活跃 sender。"""
        sender = resolve_sender(pending_request.dispatch_session_key)
        if sender is None:
            return False
        target_key = f"sender:{id(sender)}"
        if pending_request.dispatched_target_key == target_key:
            return True
        await sender.send(build_event(pending_request))
        pending_request.dispatched_target_key = target_key
        return True

    @staticmethod
    def _build_room_target_key(
        room_id: str,
        conversation_id: str | None,
        recipient_ids: tuple[int, ...],
    ) -> str:
        """构造 room 广播目标签名，便于重连后判断是否需要重投。"""
        recipient_signature = ",".join(str(sender_id) for sender_id in recipient_ids)
        return f"room:{room_id}:{conversation_id or ''}:{recipient_signature}"
