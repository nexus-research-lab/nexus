# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：tool_guard.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""工具调用权限拦截与权限策略实现。"""

import asyncio
import uuid
from typing import Any, Dict

from claude_agent_sdk import (
    PermissionResult,
    PermissionResultAllow,
    PermissionResultDeny,
    ToolPermissionContext,
)

from agent.service.channels.message_sender import MessageSender
from agent.config.config import settings
from agent.service.session.session_manager import session_manager
from agent.service.permission.strategy.permission_strategy import PermissionStrategy
from agent.schema.model_message import EventMessage
from agent.service.permission.pending_permission_request import PendingPermissionRequest
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.permission.permission_request_presenter import PermissionRequestPresenter
from agent.service.permission.permission_update_codec import PermissionUpdateCodec
from agent.utils.logger import logger


class InteractivePermissionStrategy(PermissionStrategy):
    """交互式权限策略。"""

    _permission_requests: Dict[str, PendingPermissionRequest] = {}
    _permission_responses: Dict[str, Dict[str, Any]] = {}
    _session_routes: Dict[str, PermissionRouteContext] = {}

    def __init__(self, sender: MessageSender):
        self.sender = sender
        self._is_closed = False

    def has_pending_request_for_session(self, session_key: str) -> bool:
        """判断指定 session 是否仍有等待中的权限请求。"""
        return any(
            pending_request.session_key == session_key
            for pending_request in self.__class__._permission_requests.values()
        )

    async def request_permission(
            self,
            session_key: str,
            tool_name: str,
            input_data: dict[str, Any],
            context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """通过当前通道向前端请求权限确认。"""
        if self._is_closed or self._sender_is_closed():
            logger.warning(
                "⚠️ 权限通道不可用，按普通拒绝处理: session=%s tool=%s",
                session_key,
                tool_name,
            )
            return PermissionResultDeny(message="Permission channel closed", interrupt=False)

        request_id = str(uuid.uuid4())
        timeout_seconds = max(float(settings.PERMISSION_REQUEST_TIMEOUT_SECONDS), 1.0)
        pending_request = PendingPermissionRequest(
            request_id=request_id,
            session_key=session_key,
            tool_name=tool_name,
            input_data=input_data,
            event=asyncio.Event(),
            expires_at=PermissionRequestPresenter.build_expiry(timeout_seconds),
        )
        self.__class__._permission_requests[request_id] = pending_request

        suggestion_updates = PermissionUpdateCodec.serialize_updates(
            context.suggestions if context else None
        )
        route_context = self.__class__._session_routes.get(session_key)

        permission_event = EventMessage(
            event_type="permission_request",
            session_key=(
                route_context.route_session_key
                if route_context else session_key
            ),
            room_id=route_context.room_id if route_context else None,
            conversation_id=route_context.conversation_id if route_context else None,
            agent_id=route_context.agent_id if route_context else None,
            message_id=route_context.message_id if route_context else None,
            session_id=session_manager.get_session_id(session_key),
            caused_by=route_context.caused_by if route_context else None,
            data=PermissionRequestPresenter.build_payload(
                pending_request,
                suggestion_updates,
            ),
        )

        try:
            await self.sender.send(permission_event)
        except Exception as exc:
            logger.warning(f"⚠️ 发送权限请求失败: tool={tool_name}, error={exc}")
            self._cleanup_request(request_id)
            return PermissionResultDeny(
                message="Failed to dispatch permission request",
                interrupt=False,
            )

        if self._is_closed or self._sender_is_closed():
            self._cleanup_request(request_id)
            logger.warning(
                "⚠️ 权限请求已发出但通道随后关闭，按普通拒绝处理: session=%s tool=%s",
                session_key,
                tool_name,
            )
            return PermissionResultDeny(message="Permission channel closed", interrupt=False)

        try:
            await asyncio.wait_for(pending_request.event.wait(), timeout=timeout_seconds)
            response = self.__class__._permission_responses.get(request_id, {})
            return self._build_permission_result(tool_name, input_data, response)
        except asyncio.TimeoutError:
            logger.warning(f"⏰ 权限请求超时: {tool_name}")
            return PermissionResultDeny(message="Permission request timeout")
        finally:
            self._cleanup_request(request_id)

    def handle_permission_response(self, message: Dict[str, Any]) -> bool:
        """处理前端权限响应。"""
        request_id = message.get("request_id")
        if not request_id:
            return False

        response_data = {
            "decision": message.get("decision", "deny"),
            "message": message.get("message", ""),
            "interrupt": bool(message.get("interrupt", False)),
        }

        user_answers = message.get("user_answers")
        if user_answers:
            response_data["user_answers"] = user_answers

        updated_permissions = message.get("updated_permissions")
        if isinstance(updated_permissions, list) and updated_permissions:
            response_data["updated_permissions"] = updated_permissions

        self.__class__._permission_responses[request_id] = response_data
        pending_request = self.__class__._permission_requests.get(request_id)
        if pending_request:
            pending_request.event.set()
            return True
        return False

    def close(self) -> None:
        """关闭当前连接的权限策略。

        这里不再直接拒绝所有等待中的请求。
        AskUserQuestion/权限确认在前端已经展示后，用户可能在 WebSocket 重连后才提交回答，
        因此请求需要继续保留到超时或真正收到响应。
        """
        if self._is_closed:
            return

        self._is_closed = True

    def bind_session_route(
        self,
        session_key: str,
        route_context: PermissionRouteContext,
    ) -> None:
        """记录运行时 session 到前端路由会话的映射。"""
        self.__class__._session_routes[session_key] = route_context

    def unbind_session_route(self, session_key: str) -> None:
        """清理运行时 session 的前端路由映射。"""
        self.__class__._session_routes.pop(session_key, None)

    def cancel_requests_for_session(
        self,
        session_key: str,
        message: str = "Permission request cancelled",
    ) -> int:
        """主动取消指定 session 下仍在等待的权限请求。"""
        cancelled = 0
        for request_id, pending_request in list(self.__class__._permission_requests.items()):
            if pending_request.session_key != session_key:
                continue
            self.__class__._permission_responses[request_id] = {
                "decision": "deny",
                "message": message,
                "interrupt": True,
            }
            pending_request.event.set()
            cancelled += 1
        return cancelled

    def _cleanup_request(self, request_id: str) -> None:
        """清理单个权限请求。"""
        self.__class__._permission_requests.pop(request_id, None)
        self.__class__._permission_responses.pop(request_id, None)

    def _build_permission_result(
            self,
            tool_name: str,
            input_data: dict[str, Any],
            response: Dict[str, Any],
    ) -> PermissionResult:
        """将前端响应转换为 Claude SDK 需要的权限结果。"""
        decision = response.get("decision", "deny")
        if decision == "allow":
            logger.info(f"✅ 权限允许: {tool_name}")

            updated_input = input_data.copy()
            if tool_name == "AskUserQuestion" and "user_answers" in response:
                user_answers = response["user_answers"]
                questions = input_data.get("questions", [])
                answers = {}
                for answer in user_answers:
                    question_idx = answer.get("question_index", answer.get("questionIndex", 0))
                    selected_options = answer.get(
                        "selected_options",
                        answer.get("selectedOptions", []),
                    )
                    if 0 <= question_idx < len(questions):
                        question_text = questions[question_idx].get("question", "")
                        answers[question_text] = ", ".join(selected_options)
                updated_input["answers"] = answers
                logger.info(f"📝 AskUserQuestion 用户回答: {answers}")

            updated_permissions = PermissionUpdateCodec.deserialize_updates(
                response.get("updated_permissions")
            )
            return PermissionResultAllow(
                updated_input=updated_input,
                updated_permissions=updated_permissions or None,
            )

        logger.info(f"🚫 用户拒绝工具权限: tool={tool_name}")
        return PermissionResultDeny(
            message=response.get("message", "User denied permission"),
            interrupt=bool(response.get("interrupt", False)),
        )

    def _sender_is_closed(self) -> bool:
        """判断发送通道是否已关闭。"""
        return bool(getattr(self.sender, "is_closed", False))
