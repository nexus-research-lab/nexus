# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：delivery_router.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation delivery router。"""

from __future__ import annotations

from typing import Mapping

from agent.service.automation.delivery.delivery_memory import DeliveryMemory
from agent.service.automation.delivery.delivery_sender import (
    AutomationOutboundSender,
    ChannelAutomationOutboundSender,
    WebsocketAutomationOutboundSender,
)
from agent.service.automation.delivery.delivery_target import (
    DeliveryTarget,
    resolve_delivery_target,
)
from agent.service.channels.channel_register import ChannelRegister
from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender


class DeliveryRouter:
    """根据目标模式把自动化文本路由到具体通道。"""

    def __init__(
        self,
        *,
        memory: DeliveryMemory,
        senders: Mapping[str, AutomationOutboundSender] | None = None,
        channel_register: ChannelRegister | None = None,
        websocket_sender: WsSessionRoutingSender | None = None,
    ) -> None:
        self._memory = memory
        self._senders = dict(senders or {})
        self._channel_register = channel_register
        self._websocket_sender = websocket_sender

    async def send_text(
        self,
        *,
        agent_id: str,
        text: str,
        target: DeliveryTarget | Mapping[str, object] | None,
    ) -> DeliveryTarget:
        """解析目标并完成一次文本投递。"""
        resolved_target = resolve_delivery_target(target)
        if resolved_target.mode == "none":
            return resolved_target

        if resolved_target.mode == "last":
            final_target = await self._resolve_last_target(agent_id)
        else:
            final_target = resolved_target

        sender = self._resolve_sender(final_target.channel)
        await sender.send_text(final_target, text)
        return final_target

    async def _resolve_last_target(self, agent_id: str) -> DeliveryTarget:
        """从 last-route memory 恢复最终投递目标。"""
        target = await self._memory.get_last_route(agent_id)
        if target is None:
            raise LookupError(f"last delivery target is not available for agent: {agent_id}")
        return target

    def _resolve_sender(self, channel: str | None) -> AutomationOutboundSender:
        """按通道懒加载 sender，缺失时提供明确错误。"""
        if not channel:
            raise ValueError("delivery target requires channel")

        sender = self._senders.get(channel)
        if sender is not None:
            return sender

        # 中文注释：WebSocket 不走 ChannelRegister 生命周期，所以单独从路由 sender 构建。
        if channel == "websocket" and self._websocket_sender is not None:
            sender = WebsocketAutomationOutboundSender(self._websocket_sender)
        elif self._channel_register is not None:
            sender = ChannelAutomationOutboundSender(
                self._channel_register.get_required(channel)
            )
        else:
            raise LookupError(f"delivery sender is not configured for channel: {channel}")

        self._senders[channel] = sender
        return sender
