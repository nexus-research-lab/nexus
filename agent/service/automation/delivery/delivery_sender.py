# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：delivery_sender.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation delivery outbound sender 适配层。"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Protocol

from agent.service.automation.delivery.delivery_target import DeliveryTarget
from agent.service.channels.channel_register import ChannelRegister
from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender


class SupportsChannelDelivery(Protocol):
    """支持文本投递的通道协议。"""

    async def send_delivery_text(
        self,
        *,
        to: str,
        text: str,
        account_id: str | None = None,
        thread_id: str | None = None,
    ) -> None:
        ...


class AutomationOutboundSender(ABC):
    """自动化统一 outbound 协议。"""

    @abstractmethod
    async def send_text(self, target: DeliveryTarget, text: str, lead_text: str | None = None) -> None:
        """发送文本到目标。"""
        ...


class ChannelAutomationOutboundSender(AutomationOutboundSender):
    """把通道对象适配成自动化发送器。"""

    def __init__(self, channel: SupportsChannelDelivery) -> None:
        self._channel = require_channel_delivery_support(channel)

    async def send_text(self, target: DeliveryTarget, text: str, lead_text: str | None = None) -> None:
        """转发到 IM 通道的统一文本投递入口。"""
        del lead_text
        if not target.to:
            raise ValueError("channel delivery target requires to")
        await self._channel.send_delivery_text(
            to=target.to,
            text=text,
            account_id=target.account_id,
            thread_id=target.thread_id,
        )


class WebsocketAutomationOutboundSender(AutomationOutboundSender):
    """把 session 路由发送器适配成自动化发送器。"""

    def __init__(self, sender: WsSessionRoutingSender) -> None:
        self._sender = sender

    async def send_text(self, target: DeliveryTarget, text: str, lead_text: str | None = None) -> None:
        """按 session_key 投递自动化文本。"""
        session_key = target.session_key or target.to
        if not session_key:
            raise ValueError("websocket delivery target requires session_key")
        await self._sender.send_text(session_key=session_key, text=text, lead_text=lead_text)


def require_channel_delivery_support(channel: object) -> SupportsChannelDelivery:
    """要求通道实现统一 outbound contract。"""
    send_delivery_text = getattr(channel, "send_delivery_text", None)
    if not callable(send_delivery_text):
        raise TypeError("channel must implement send_delivery_text(to=..., text=..., ...)")
    return channel


def build_delivery_senders(
    *,
    channel_register: ChannelRegister,
    websocket_sender: WsSessionRoutingSender | None = None,
) -> dict[str, AutomationOutboundSender]:
    """从运行时通道注册表构建默认 sender 映射。"""
    senders: dict[str, AutomationOutboundSender] = {}
    if websocket_sender is not None:
        senders["websocket"] = WebsocketAutomationOutboundSender(websocket_sender)

    for channel_type in ("telegram", "discord"):
        channel = channel_register.get(channel_type)
        if channel is None:
            continue
        senders[channel_type] = ChannelAutomationOutboundSender(channel)
    return senders
