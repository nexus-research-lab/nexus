# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：websocket_channel.py
# @Date   ：2026/2/25 15:45
# @Author ：leemysw
#
# 2026/2/25 15:45   Create
# =====================================================

"""
WebSocket 通道实现

[INPUT]: 依赖 fastapi.WebSocket，依赖 channel.py 的 MessageSender/PermissionStrategy,
         依赖 claude_agent_sdk 的权限相关类型
[OUTPUT]: 对外提供 WebSocketSender/InteractivePermissionStrategy/WebSocketChannel
[POS]: channel 模块的 WebSocket 实现，封装现有 WebSocket 行为（纯重构、零行为变更）
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import uuid
from typing import Any, Dict, Optional, Union

from claude_agent_sdk import (
    PermissionResult,
    PermissionResultAllow,
    PermissionResultDeny,
    ToolPermissionContext,
)
from fastapi import WebSocket

from agent.config.config import settings
from agent.service.channel.channel import MessageChannel, MessageSender, PermissionStrategy
from agent.service.channel.permission_runtime import (
    PendingPermissionRequest,
    PermissionRequestPresenter,
    PermissionUpdateCodec,
)
from agent.service.process.protocol_adapter import ProtocolAdapter
from agent.service.schema.model_message import AError, AEvent, AMessage
from agent.service.schema.model_workspace_event import WorkspaceEvent
from agent.service.workspace_observer import workspace_observer
from agent.service.session_manager import session_manager
from agent.service.workspace_event_bus import workspace_event_bus
from agent.utils.logger import logger


# =====================================================
# WebSocketSender — WebSocket 版消息发送器
#
# 将 AMessage/AEvent/AError 序列化为 JSON 并通过
# WebSocket 推送到前端。保持原 BaseHandler.send() 的行为。
# =====================================================

class WebSocketSender(MessageSender):
    """WebSocket 消息发送器"""

    def __init__(self, websocket: WebSocket):
        self.websocket = websocket
        self.protocol_adapter = ProtocolAdapter()
        self._workspace_subscriptions: Dict[str, str] = {}
        self._is_closed = False

    async def _safe_send_json(self, payload: Dict[str, Any]) -> None:
        """安全发送，连接关闭后不再继续推送。"""
        if self._is_closed:
            return

        try:
            await self.websocket.send_json(payload)
        except RuntimeError as exc:
            self._is_closed = True
            self.unsubscribe_all_workspace()
            logger.warning(f"⚠️ WebSocket 已关闭，停止发送 workspace 事件: {exc}")
        except Exception:
            self._is_closed = True
            self.unsubscribe_all_workspace()
            raise

    async def send_message(self, message: AMessage) -> None:
        event = self.protocol_adapter.build_ws_event(message)
        if event is None:
            return

        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)
        logger.debug(f"💬发送消息: {payload}")

    async def send_event(self, event: AEvent) -> None:
        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

    async def send_error(self, error: AError) -> None:
        payload = error.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

    async def send_workspace_event(self, event: WorkspaceEvent) -> None:
        payload = self.protocol_adapter.build_workspace_event(event).model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

    def subscribe_workspace(self, agent_id: str) -> None:
        """订阅指定 Agent 的 workspace 事件。"""
        if not agent_id or agent_id in self._workspace_subscriptions:
            return

        token_holder: Dict[str, str] = {}

        async def _listener(event: WorkspaceEvent) -> None:
            try:
                await self.send_workspace_event(event)
            except Exception as exc:
                logger.warning(f"⚠️ workspace 事件推送失败: {exc}")
                token = token_holder.get("token")
                if token:
                    workspace_event_bus.unsubscribe(token)

        token = workspace_event_bus.subscribe(agent_id, _listener)
        token_holder["token"] = token
        self._workspace_subscriptions[agent_id] = token
        workspace_observer.subscribe(agent_id)

    def unsubscribe_workspace(self, agent_id: str) -> None:
        """取消订阅指定 Agent 的 workspace 事件。"""
        token = self._workspace_subscriptions.pop(agent_id, None)
        if not token:
            return
        workspace_event_bus.unsubscribe(token)
        workspace_observer.unsubscribe(agent_id)

    def unsubscribe_all_workspace(self) -> None:
        """取消当前连接的所有 workspace 事件订阅。"""
        self._is_closed = True
        for agent_id in list(self._workspace_subscriptions.keys()):
            self.unsubscribe_workspace(agent_id)

    @property
    def is_closed(self) -> bool:
        """返回当前连接是否已关闭。"""
        return self._is_closed


# =====================================================
# InteractivePermissionStrategy — 交互式权限策略
#
# 从原 PermissionHandler 提取。通过 WebSocket 发送权限请求，
# 阻塞等待用户在前端 UI 中点击允许/拒绝。
# =====================================================

class InteractivePermissionStrategy(PermissionStrategy):
    """交互式权限策略 — WebSocket 通道专用"""

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
        """通过 WebSocket 请求用户权限确认"""
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

        logger.info(
            "🔐 请求工具权限: "
            f"session={session_key}, tool={tool_name}, request_id={request_id}"
        )

        # 发送权限请求到前端
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

        # 等待前端响应
        try:
            await asyncio.wait_for(pending_request.event.wait(), timeout=timeout_seconds)
            response = self._permission_responses.get(request_id, {})
            return self._build_permission_result(tool_name, input_data, response)

        except asyncio.TimeoutError:
            logger.warning(f"⏰ 权限请求超时: {tool_name}")
            return PermissionResultDeny(message="Permission request timeout")
        finally:
            self._cleanup_request(request_id)

    def handle_permission_response(self, message: Dict[str, Any]) -> None:
        """处理前端的权限响应回调

        Args:
            message: 前端权限响应消息
        """
        request_id = message.get("request_id")
        if not request_id:
            logger.warning("⚠️ permission_response消息缺少request_id")
            return
        if self._is_closed:
            logger.warning(f"⚠️ 权限通道已关闭，忽略响应: request_id={request_id}")
            return

        response_data = {
            "decision": message.get("decision", "deny"),
            "message": message.get("message", ""),
            "interrupt": bool(message.get("interrupt", False)),
        }

        # AskUserQuestion 的用户答案
        user_answers = message.get("user_answers")
        if user_answers:
            response_data["user_answers"] = user_answers
            logger.debug(f"📝 收到 AskUserQuestion 用户答案: {user_answers}")

        updated_permissions = message.get("updated_permissions")
        if isinstance(updated_permissions, list) and updated_permissions:
            response_data["updated_permissions"] = updated_permissions

        self._permission_responses[request_id] = response_data

        pending_request = self._permission_requests.get(request_id)
        if pending_request:
            pending_request.event.set()
            logger.debug(f"📨 收到权限响应: request_id={request_id}, decision={message.get('decision')}")
        else:
            logger.warning(f"⚠️ 未找到对应的权限请求: request_id={request_id}")

    def close(self) -> None:
        """关闭权限策略并唤醒所有等待中的请求。"""
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

    def _build_permission_result(
        self,
        tool_name: str,
        input_data: dict[str, Any],
        response: Dict[str, Any],
    ) -> PermissionResult:
        """根据前端响应构建 SDK 权限结果。"""
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

        logger.info(f"❌ 权限拒绝: {tool_name}")
        return PermissionResultDeny(
            message=response.get("message", "User denied permission"),
            interrupt=bool(response.get("interrupt", False)),
        )

    def _cleanup_request(self, request_id: str) -> None:
        """清理权限请求上下文。"""
        self._permission_requests.pop(request_id, None)
        self._permission_responses.pop(request_id, None)

    def _sender_is_closed(self) -> bool:
        """判断底层发送器是否已关闭。"""
        return bool(getattr(self.sender, "is_closed", False))


# =====================================================
# WebSocketChannel — WebSocket 通道（无操作占位）
#
# WebSocket 的生命周期由 FastAPI 管理（每连接创建销毁），
# 无需 ChannelManager 管理。channel_type 用于标识。
# =====================================================

class WebSocketChannel(MessageChannel):
    """WebSocket 通道 — 生命周期由 FastAPI 管理"""

    @property
    def channel_type(self) -> str:
        return "websocket"

    async def start(self) -> None:
        logger.info("📡 WebSocket 通道就绪（由 FastAPI 管理连接）")

    async def stop(self) -> None:
        logger.info("📡 WebSocket 通道关闭")
