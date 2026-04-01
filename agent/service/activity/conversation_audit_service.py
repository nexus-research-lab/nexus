# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：conversation_audit_service.py
# @Date   ：2026/04/01 22:49
# @Author ：leemysw
# 2026/04/01 22:49   Create
# =====================================================

"""对话与权限审计服务。"""

from __future__ import annotations

from datetime import datetime
from typing import Any

from agent.infra.database.get_db import get_db
from agent.infra.database.models.activity_event import ActivityEventType
from agent.infra.database.repositories.conversation_sql_repository import (
    ConversationSqlRepository,
)
from agent.infra.database.repositories.session_sql_repository import SessionSqlRepository
from agent.service.activity.activity_event_service import activity_event_service
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.session.session_router import parse_session_key


class ConversationAuditService:
    """负责记录运行时对话与权限审计事件。"""

    def __init__(self) -> None:
        self._db = get_db("async_sqlite")

    @staticmethod
    def _resolve_round_event_type(status: str) -> str:
        """根据轮次终态映射 Activity 事件类型。"""
        if status == "success":
            return ActivityEventType.ROOM_ROUND_COMPLETED
        if status == "cancelled":
            return ActivityEventType.ROOM_ROUND_CANCELLED
        return ActivityEventType.ROOM_ROUND_FAILED

    @staticmethod
    def _build_round_summary(agent_id: str, status: str) -> str:
        """构建轮次终态摘要。"""
        if status == "success":
            return f"Agent {agent_id} 完成了一轮协作"
        if status == "cancelled":
            return f"Agent {agent_id} 的协作轮次已中断"
        return f"Agent {agent_id} 的协作轮次失败"

    @staticmethod
    def _resolve_permission_route(
        session_key: str,
        route_context: PermissionRouteContext | None,
    ) -> dict[str, str | None]:
        """解析权限事件的路由元数据。"""
        parsed = parse_session_key(session_key)
        return {
            "runtime_session_key": session_key,
            "route_session_key": (
                route_context.route_session_key if route_context else session_key
            ),
            "room_id": route_context.room_id if route_context else None,
            "conversation_id": route_context.conversation_id if route_context else None,
            "agent_id": (
                route_context.agent_id if route_context and route_context.agent_id
                else parsed.get("agent_id")
            ),
            "message_id": route_context.message_id if route_context else None,
            "caused_by": route_context.caused_by if route_context else None,
        }

    @staticmethod
    def _resolve_permission_target(route_metadata: dict[str, str | None]) -> tuple[str, str | None]:
        """解析权限事件的目标实体。"""
        if route_metadata["room_id"]:
            return "room", route_metadata["room_id"]
        return "agent", route_metadata["agent_id"]

    async def record_room_round_terminal(
        self,
        session_id: str,
        round_id: str,
        status: str,
        finished_at_ms: int,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        """记录 Room 轮次终态。"""
        async with self._db.session() as session:
            session_repository = SessionSqlRepository(session)
            conversation_repository = ConversationSqlRepository(session)
            session_record = await session_repository.get(session_id)
            if session_record is None:
                return
            conversation = await conversation_repository.get(
                session_record.conversation_id
            )

        room_id = conversation.room_id if conversation else None
        agent_id = session_record.agent_id
        audit_metadata = {
            "room_id": room_id,
            "conversation_id": session_record.conversation_id,
            "session_id": session_id,
            "agent_id": agent_id,
            "round_id": round_id,
            "status": status,
            "finished_at": datetime.fromtimestamp(finished_at_ms / 1000).isoformat(),
        }
        if metadata:
            audit_metadata.update(metadata)

        await activity_event_service.create_event(
            event_type=self._resolve_round_event_type(status),
            actor_type="agent",
            actor_id=agent_id,
            target_type="room",
            target_id=room_id,
            summary=self._build_round_summary(agent_id, status),
            metadata=audit_metadata,
        )

    async def record_permission_request(
        self,
        session_key: str,
        tool_name: str,
        route_context: PermissionRouteContext | None,
        request_id: str,
        tool_summary: str,
        expires_at: datetime,
    ) -> None:
        """记录权限请求审计事件。"""
        route_metadata = self._resolve_permission_route(session_key, route_context)
        target_type, target_id = self._resolve_permission_target(route_metadata)
        agent_id = route_metadata["agent_id"]
        await activity_event_service.create_event(
            event_type=ActivityEventType.PERMISSION_REQUESTED,
            actor_type="agent" if agent_id else "system",
            actor_id=agent_id,
            target_type=target_type,
            target_id=target_id,
            summary=f"工具 {tool_name} 正在等待权限确认",
            metadata={
                **route_metadata,
                "request_id": request_id,
                "tool_name": tool_name,
                "tool_summary": tool_summary,
                "status": "pending",
                "expires_at": expires_at.isoformat(),
            },
        )

    async def record_permission_resolution(
        self,
        session_key: str,
        tool_name: str,
        decision: str,
        route_context: PermissionRouteContext | None,
        request_id: str,
        interrupt: bool = False,
        message: str | None = None,
    ) -> None:
        """记录权限决策审计事件。"""
        route_metadata = self._resolve_permission_route(session_key, route_context)
        target_type, target_id = self._resolve_permission_target(route_metadata)
        decision_text = {
            "allow": "已允许",
            "deny": "已拒绝",
            "timeout": "已超时",
            "cancelled": "已取消",
        }.get(decision, "已处理")
        await activity_event_service.create_event(
            event_type=ActivityEventType.PERMISSION_RESOLVED,
            actor_type="user",
            actor_id="local-user",
            target_type=target_type,
            target_id=target_id,
            summary=f"{decision_text}工具 {tool_name} 的权限请求",
            metadata={
                **route_metadata,
                "request_id": request_id,
                "tool_name": tool_name,
                "decision": decision,
                "interrupt": interrupt,
                "message": message,
            },
        )


conversation_audit_service = ConversationAuditService()
