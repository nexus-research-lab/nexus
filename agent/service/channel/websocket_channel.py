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

from claude_agent_sdk import PermissionResult, PermissionResultAllow, PermissionResultDeny
from fastapi import WebSocket

from agent.service.channel.channel import MessageChannel, MessageSender, PermissionStrategy
from agent.service.process.protocol_adapter import ProtocolAdapter
from agent.service.schema.model_message import AError, AEvent, AMessage
from agent.service.session_manager import session_manager
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

    async def send_message(self, message: AMessage) -> None:
        event = self.protocol_adapter.build_ws_event(message)
        if event is None:
            return

        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self.websocket.send_json(payload)
        logger.debug(f"💬发送消息: {payload}")

    async def send_event(self, event: AEvent) -> None:
        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self.websocket.send_json(payload)

    async def send_error(self, error: AError) -> None:
        payload = error.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self.websocket.send_json(payload)


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
        self._permission_requests: Dict[str, asyncio.Event] = {}
        self._permission_responses: Dict[str, Dict[str, Any]] = {}

    async def request_permission(
        self,
        agent_id: str,
        tool_name: str,
        input_data: dict[str, Any],
    ) -> PermissionResult:
        """通过 WebSocket 请求用户权限确认"""
        request_id = str(uuid.uuid4())

        logger.info(f"🔐 请求工具权限: agent_id={agent_id}, tool={tool_name}, request_id={request_id}")

        # 创建等待事件
        event = asyncio.Event()
        self._permission_requests[request_id] = event

        # 发送权限请求到前端
        permission_event = AEvent(
            event_type="permission_request",
            agent_id=agent_id,
            session_id=session_manager.get_session_id(agent_id),
            data={
                "request_id": request_id,
                "tool_name": tool_name,
                "tool_input": input_data,
            },
        )
        await self.sender.send_event(permission_event)

        # 等待前端响应（60秒超时）
        try:
            await asyncio.wait_for(event.wait(), timeout=60.0)

            response = self._permission_responses.get(request_id, {})
            decision = response.get("decision", "deny")

            # 清理
            del self._permission_requests[request_id]
            if request_id in self._permission_responses:
                del self._permission_responses[request_id]

            if decision == "allow":
                logger.info(f"✅ 权限允许: {tool_name}")

                # AskUserQuestion 特殊处理
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

                return PermissionResultAllow(updated_input=updated_input)
            else:
                logger.info(f"❌ 权限拒绝: {tool_name}")
                return PermissionResultDeny(message=response.get("message", "User denied permission"))

        except asyncio.TimeoutError:
            logger.warning(f"⏰ 权限请求超时: {tool_name}")
            del self._permission_requests[request_id]
            return PermissionResultDeny(message="Permission request timeout")

    def handle_permission_response(self, message: Dict[str, Any]) -> None:
        """处理前端的权限响应回调

        Args:
            message: 前端权限响应消息
        """
        request_id = message.get("request_id")
        if not request_id:
            logger.warning("⚠️ permission_response消息缺少request_id")
            return

        response_data = {
            "decision": message.get("decision", "deny"),
            "message": message.get("message", ""),
        }

        # AskUserQuestion 的用户答案
        user_answers = message.get("user_answers")
        if user_answers:
            response_data["user_answers"] = user_answers
            logger.debug(f"📝 收到 AskUserQuestion 用户答案: {user_answers}")

        self._permission_responses[request_id] = response_data

        if request_id in self._permission_requests:
            self._permission_requests[request_id].set()
            logger.debug(f"📨 收到权限响应: request_id={request_id}, decision={message.get('decision')}")
        else:
            logger.warning(f"⚠️ 未找到对应的权限请求: request_id={request_id}")


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
