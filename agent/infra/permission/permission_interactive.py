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

from agent.channels.message_sender import MessageSender
from agent.config.config import settings
from agent.infra.agent.session_manager import session_manager
from agent.infra.permission.permission_strategy import PermissionStrategy
from agent.infra.runtime.permission_runtime import (
    PendingPermissionRequest,
    PermissionRequestPresenter,
    PermissionUpdateCodec,
)
from agent.schema.model_message import AEvent
from agent.utils.logger import logger


class InteractivePermissionStrategy(PermissionStrategy):
    """交互式权限策略。"""

    def __init__(self, sender: MessageSender):
        self.sender = sender
        self._permission_requests: Dict[str, PendingPermissionRequest] = {}
        self._permission_responses: Dict[str, Dict[str, Any]] = {}
        self._is_closed = False

    async def request_permission(
            self,
            session_key: str,
            tool_name: str,
            input_data: dict[str, Any],
            context: ToolPermissionContext | None = None,
    ) -> PermissionResult:
        """通过当前通道向前端请求权限确认。"""
        if self._is_closed or self._sender_is_closed():
            return PermissionResultDeny(message="Permission channel closed", interrupt=True)

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
        self._permission_requests[request_id] = pending_request

        suggestion_updates = PermissionUpdateCodec.serialize_updates(
            context.suggestions if context else None
        )

        permission_event = AEvent(
            event_type="permission_request",
            agent_id=session_key,
            session_id=session_manager.get_session_id(session_key),
            data=PermissionRequestPresenter.build_payload(
                pending_request,
                suggestion_updates,
            ),
        )

        try:
            await self.sender.send_event(permission_event)
        except Exception as exc:
            logger.warning(f"⚠️ 发送权限请求失败: tool={tool_name}, error={exc}")
            self._cleanup_request(request_id)
            return PermissionResultDeny(message="Failed to dispatch permission request", interrupt=True)

        if self._is_closed or self._sender_is_closed():
            self._cleanup_request(request_id)
            return PermissionResultDeny(message="Permission channel closed", interrupt=True)

        try:
            await asyncio.wait_for(pending_request.event.wait(), timeout=timeout_seconds)
            response = self._permission_responses.get(request_id, {})
            return self._build_permission_result(tool_name, input_data, response)
        except asyncio.TimeoutError:
            logger.warning(f"⏰ 权限请求超时: {tool_name}")
            return PermissionResultDeny(message="Permission request timeout")
        finally:
            self._cleanup_request(request_id)

    def handle_permission_response(self, message: Dict[str, Any]) -> bool:
        """处理前端权限响应。"""
        request_id = message.get("request_id")
        if not request_id or self._is_closed:
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
        pending_request = self._permission_requests.get(request_id)
        if pending_request:
            pending_request.event.set()
            return True
        return False

    def close(self) -> None:
        """关闭权限策略并唤醒所有等待请求。"""
        if self._is_closed:
            return

        self._is_closed = True
        for request_id, pending_request in list(self._permission_requests.items()):
            self._permission_responses[request_id] = {
                "decision": "deny",
                "message": "Permission channel closed",
                "interrupt": True,
            }
            pending_request.event.set()

    def _cleanup_request(self, request_id: str) -> None:
        """清理单个权限请求。"""
        self._permission_requests.pop(request_id, None)
        self._permission_responses.pop(request_id, None)

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
                    question_idx = answer.get("questionIndex", 0)
                    selected_options = answer.get("selectedOptions", [])
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
