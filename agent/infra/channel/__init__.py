# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/3/12 20:29
# @Author ：leemysw
# 2026/3/12 20:29   Create
# =====================================================

"""
消息通道基础设施入口。

[OUTPUT]: 对外提供通道协议、生命周期管理器与多端通道实现
[POS]: infra 层的传输通道聚合入口，供 app/service 层装配使用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.infra.channel.channel import MessageChannel, MessageSender, PermissionStrategy
from agent.infra.channel.channel_manager import ChannelManager
from agent.infra.channel.discord_channel import AutoAllowPermissionStrategy, DiscordChannel, DiscordSender
from agent.infra.channel.telegram_channel import TelegramChannel, TelegramSender
from agent.infra.channel.websocket_channel import (
    InteractivePermissionStrategy,
    WebSocketChannel,
    WebSocketSender,
)

__all__ = [
    "AutoAllowPermissionStrategy",
    "ChannelManager",
    "DiscordChannel",
    "DiscordSender",
    "InteractivePermissionStrategy",
    "MessageChannel",
    "MessageSender",
    "PermissionStrategy",
    "TelegramChannel",
    "TelegramSender",
    "WebSocketChannel",
    "WebSocketSender",
]
