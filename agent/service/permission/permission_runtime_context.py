# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_runtime_context.py
# @Date   ：2026/04/03 10:33
# @Author ：leemysw
# 2026/04/03 10:33   Create
# =====================================================

"""权限运行时生命周期上下文。"""

from __future__ import annotations

import asyncio
import uuid
from typing import Any, Dict

from claude_agent_sdk import (
    PermissionResult,
    PermissionResultAllow,
    PermissionResultDeny,
    ToolPermissionContext,
)

from agent.config.config import settings
from agent.schema.model_message import EventMessage
from agent.service.channels.message_sender import MessageSender
from agent.service.permission.pending_permission_request import PendingPermissionRequest
from agent.service.permission.permission_dispatch_router import PermissionDispatchRouter
from agent.service.permission.permission_error_codes import (
    PERMISSION_CHANNEL_UNAVAILABLE_MESSAGE,
    PERMISSION_REQUEST_TIMEOUT_MESSAGE,
)
from agent.service.permission.permission_request_presenter import PermissionRequestPresenter
from agent.service.permission.permission_route_context import PermissionRouteContext
from agent.service.permission.permission_update_codec import PermissionUpdateCodec
from agent.service.permission.session_control_registry import SessionControlRegistry
from agent.service.session.session_manager import session_manager
from agent.utils.logger import logger


class PermissionRuntimeContext:
    """管理权限请求、session 绑定与控制权。"""

    def __init__(self) -> None:
        self._permission_requests: Dict[str, PendingPermissionRequest] = {}
        self._permission_responses: Dict[str, Dict[str, Any]] = {}
        self._session_routes: Dict[str, PermissionRouteContext] = {}
        self._session_control_registry = SessionControlRegistry()
        self._dispatch_router = PermissionDispatchRouter()

    async def request_permission(
        self,
        session_key: str,
        tool_name: str,
        input_data: dict[str, Any],
        context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """为某个运行时 session 发起权限确认。"""
        timeout_seconds = max(float(settings.PERMISSION_REQUEST_TIMEOUT_SECONDS), 1.0)
        reconnect_grace_seconds = min(timeout_seconds, 3.0)
        route_context = self._session_routes.get(session_key)
        dispatch_session_key = route_context.route_session_key if route_context else session_key
        pending_request = PendingPermissionRequest(
            request_id=str(uuid.uuid4()),
            session_key=session_key,
            dispatch_session_key=dispatch_session_key,
            tool_name=tool_name,
            input_data=input_data,
            suggestion_updates=PermissionUpdateCodec.serialize_updates(
                context.suggestions if context else None,
            ),
            event=asyncio.Event(),
            expires_at=PermissionRequestPresenter.build_expiry(timeout_seconds),
            route_context=route_context,
        )
        self._permission_requests[pending_request.request_id] = pending_request
        dispatched = await self._await_initial_dispatch(
            pending_request=pending_request,
            reconnect_grace_seconds=reconnect_grace_seconds,
        )

        try:
            if not dispatched:
                logger.warning(
                    "⚠️ 权限请求未能投递到控制端: session=%s tool=%s",
                    pending_request.session_key,
                    pending_request.tool_name,
                )
                return self._build_denied_result(
                    tool_name,
                    PERMISSION_CHANNEL_UNAVAILABLE_MESSAGE,
                )

            await asyncio.wait_for(pending_request.event.wait(), timeout=timeout_seconds)
            response = self._permission_responses.get(pending_request.request_id, {})
            return self._build_permission_result(tool_name, input_data, response)
        except asyncio.TimeoutError:
            logger.warning("⏰ 权限请求超时: %s", tool_name)
            return self._build_denied_result(tool_name, PERMISSION_REQUEST_TIMEOUT_MESSAGE)
        finally:
            self._cleanup_request(pending_request.request_id)

    def handle_permission_response(self, message: Dict[str, Any]) -> bool:
        """处理来自前端的权限确认结果。"""
        request_id = message.get("request_id")
        if not request_id:
            return False

        pending_request = self._permission_requests.get(request_id)
        if pending_request is None:
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

        self._permission_responses[request_id] = response_data
        pending_request.event.set()
        return True

    def bind_session_route(self, session_key: str, route_context: PermissionRouteContext) -> None:
        """记录运行时 session 到前端路由会话的映射。"""
        self._session_routes[session_key] = route_context

    def unbind_session_route(self, session_key: str) -> None:
        """移除运行时 session 的路由映射。"""
        self._session_routes.pop(session_key, None)

    def resolve_dispatch_session_key(self, session_key: str) -> str:
        """把运行时 session_key 解析成前端路由 session_key。"""
        route_context = self._session_routes.get(session_key)
        return route_context.route_session_key if route_context else session_key

    def has_pending_request_for_session(self, session_key: str) -> bool:
        """判断指定 session 是否仍有等待确认的权限请求。"""
        return any(
            pending_request.session_key == session_key
            for pending_request in self._permission_requests.values()
        )

    def cancel_requests_for_session(
        self,
        session_key: str,
        message: str = "Permission request cancelled",
    ) -> int:
        """主动取消指定 session 下仍在等待的权限请求。"""
        cancelled = 0
        for request_id, pending_request in list(self._permission_requests.items()):
            if pending_request.session_key != session_key:
                continue
            self._permission_responses[request_id] = {
                "decision": "deny",
                "message": message,
                "interrupt": True,
            }
            pending_request.event.set()
            cancelled += 1
        return cancelled

    def bind_session_sender(
        self,
        session_key: str,
        sender: MessageSender,
        client_id: str,
        request_control: bool,
    ) -> dict[str, Any]:
        """把 sender 绑定到某个前端 session。"""
        snapshot = self._session_control_registry.bind_session(
            session_key=session_key,
            sender=sender,
            client_id=client_id,
            request_control=request_control,
        )
        asyncio.create_task(self._replay_pending_requests(session_key))
        return snapshot

    def unbind_session_sender(
        self,
        session_key: str,
        sender: MessageSender,
    ) -> dict[str, Any]:
        """移除某个 sender 对单个前端 session 的绑定。"""
        snapshot = self._session_control_registry.unbind_session(session_key, sender)
        asyncio.create_task(self._replay_pending_requests(session_key))
        return snapshot

    def unregister_sender(self, sender: MessageSender) -> tuple[str, ...]:
        """移除某个 sender 持有的全部前端 session。"""
        changed_session_keys = self._session_control_registry.unregister_sender(sender)
        for session_key in changed_session_keys:
            asyncio.create_task(self._replay_pending_requests(session_key))
        return changed_session_keys

    def has_bound_sender(self, session_key: str) -> bool:
        """判断指定 session 是否存在任意绑定连接。"""
        return self._session_control_registry.has_bindings(session_key)

    def is_sender_bound(self, session_key: str, sender: MessageSender) -> bool:
        """判断 sender 是否已绑定到该 session。"""
        return self._session_control_registry.is_bound(session_key, sender)

    def is_session_controller(self, session_key: str, sender: MessageSender) -> bool:
        """判断 sender 是否是该 session 当前控制端。"""
        return self._session_control_registry.is_session_controller(session_key, sender)

    def resolve_controller_sender(self, session_key: str) -> MessageSender | None:
        """解析当前 session 的控制端。"""
        return self._session_control_registry.resolve_controller_sender(session_key)

    def resolve_session_senders(self, session_key: str) -> tuple[MessageSender, ...]:
        """解析当前 session 的全部绑定连接。"""
        return self._session_control_registry.resolve_session_senders(session_key)

    def get_session_control_snapshot(self, session_key: str) -> dict[str, Any]:
        """返回当前 session 的控制权快照。"""
        return self._session_control_registry.get_session_snapshot(session_key)

    async def broadcast_session_status(
        self,
        session_key: str,
        running_round_ids: list[str],
    ) -> None:
        """向当前 session 的全部绑定连接广播运行态与控制权快照。"""
        session_senders = self.resolve_session_senders(session_key)
        if not session_senders:
            return

        event = self._build_session_status_event(
            session_key=session_key,
            running_round_ids=running_round_ids,
        )
        await asyncio.gather(
            *(sender.send_event_message(event) for sender in session_senders),
            return_exceptions=True,
        )

    async def _replay_pending_requests(self, dispatch_session_key: str) -> None:
        """把仍在等待中的权限请求重新投递到当前控制端。"""
        for pending_request in list(self._permission_requests.values()):
            if pending_request.dispatch_session_key != dispatch_session_key:
                continue
            if pending_request.event.is_set():
                continue
            await self._dispatch_request_if_possible(pending_request)

    async def _dispatch_request_if_possible(self, pending_request: PendingPermissionRequest) -> bool:
        """在当前存在控制端时推送权限请求。"""
        try:
            dispatched = await self._dispatch_router.dispatch(
                pending_request=pending_request,
                build_event=self._build_permission_event,
                resolve_sender=self.resolve_controller_sender,
            )
            if not dispatched:
                logger.info(
                    "⏳ 权限请求等待控制端恢复: session=%s tool=%s dispatch=%s room=%s",
                    pending_request.session_key,
                    pending_request.tool_name,
                    pending_request.dispatch_session_key,
                    pending_request.route_context.room_id if pending_request.route_context else None,
                )
                return False

            logger.info(
                "📨 权限请求已投递: request=%s session=%s tool=%s dispatch=%s target=%s",
                pending_request.request_id,
                pending_request.session_key,
                pending_request.tool_name,
                pending_request.dispatch_session_key,
                pending_request.dispatched_target_key,
            )
            return True
        except Exception as exc:
            sender = self.resolve_controller_sender(pending_request.dispatch_session_key)
            logger.warning(
                "⚠️ 发送权限请求失败，等待后续重连重投: request=%s session=%s tool=%s error=%s",
                pending_request.request_id,
                pending_request.session_key,
                pending_request.tool_name,
                exc,
            )
            if sender is not None:
                self.unregister_sender(sender)
            pending_request.dispatched_target_key = None
            return False

    async def _await_initial_dispatch(
        self,
        pending_request: PendingPermissionRequest,
        reconnect_grace_seconds: float,
    ) -> bool:
        """短暂等待控制端恢复，避免把用户确认超时浪费在无连接状态。"""
        loop = asyncio.get_running_loop()
        deadline = loop.time() + reconnect_grace_seconds
        while True:
            if await self._dispatch_request_if_possible(pending_request):
                return True
            remaining = deadline - loop.time()
            if remaining <= 0:
                return False
            await asyncio.sleep(min(0.2, remaining))

    def _build_permission_event(self, pending_request: PendingPermissionRequest) -> EventMessage:
        """组装发给前端的权限确认事件。"""
        route_context = pending_request.route_context
        return EventMessage(
            event_type="permission_request",
            session_key=pending_request.dispatch_session_key,
            room_id=route_context.room_id if route_context else None,
            conversation_id=route_context.conversation_id if route_context else None,
            agent_id=route_context.agent_id if route_context else None,
            message_id=route_context.message_id if route_context else None,
            session_id=session_manager.get_session_id(pending_request.session_key),
            caused_by=route_context.caused_by if route_context else None,
            data=PermissionRequestPresenter.build_payload(
                pending_request,
                pending_request.suggestion_updates,
            ),
        )

    def _build_session_status_event(
        self,
        session_key: str,
        running_round_ids: list[str],
    ) -> EventMessage:
        """构造 session_status 事件。"""
        snapshot = self.get_session_control_snapshot(session_key)
        return EventMessage(
            event_type="session_status",
            session_key=session_key,
            data={
                "is_generating": len(running_round_ids) > 0,
                "running_round_ids": running_round_ids,
                "controller_client_id": snapshot.get("controller_client_id"),
                "observer_count": snapshot.get("observer_count", 0),
                "bound_client_count": snapshot.get("bound_client_count", 0),
            },
        )

    def _cleanup_request(self, request_id: str) -> None:
        """清理单个权限请求。"""
        self._permission_requests.pop(request_id, None)
        self._permission_responses.pop(request_id, None)

    @staticmethod
    def _build_denied_result(tool_name: str, message: str) -> PermissionResult:
        """构建系统侧拒绝结果。"""
        return PermissionResultDeny(message=message, interrupt=(tool_name == "AskUserQuestion"))

    def _build_permission_result(
        self,
        tool_name: str,
        input_data: dict[str, Any],
        response: Dict[str, Any],
    ) -> PermissionResult:
        """将前端权限响应转换为 Claude SDK 结果。"""
        decision = response.get("decision", "deny")
        if decision == "allow":
            logger.info("✅ 权限允许: %s", tool_name)
            updated_input = input_data.copy()
            if tool_name == "AskUserQuestion" and "user_answers" in response:
                updated_input["answers"] = self._build_question_answers(
                    input_data,
                    response["user_answers"],
                )
            updated_permissions = PermissionUpdateCodec.deserialize_updates(
                response.get("updated_permissions"),
            )
            return PermissionResultAllow(
                updated_input=updated_input,
                updated_permissions=updated_permissions or None,
            )

        logger.info("🚫 用户拒绝工具权限: tool=%s", tool_name)
        return PermissionResultDeny(
            message=response.get("message", "User denied permission"),
            interrupt=bool(response.get("interrupt", False)),
        )

    @staticmethod
    def _build_question_answers(
        input_data: dict[str, Any],
        user_answers: list[dict[str, Any]],
    ) -> dict[str, str]:
        """把 AskUserQuestion 的回答整理回 SDK 期望结构。"""
        questions = input_data.get("questions", [])
        answers: dict[str, str] = {}
        for answer in user_answers:
            question_idx = answer.get("question_index", answer.get("questionIndex", 0))
            selected_options = answer.get(
                "selected_options",
                answer.get("selectedOptions", []),
            )
            if 0 <= question_idx < len(questions):
                question_text = questions[question_idx].get("question", "")
                answers[question_text] = ", ".join(selected_options)
        logger.info("📝 AskUserQuestion 用户回答: %s", answers)
        return answers


permission_runtime_context = PermissionRuntimeContext()
