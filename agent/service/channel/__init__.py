# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/2/25 15:45
# @Author ：leemysw
#
# 2026/2/25 15:45   Create
# =====================================================

"""
消息通道抽象层

[OUTPUT]: 对外提供 MessageSender/MessageChannel/PermissionStrategy 协议,
          ChannelManager 管理器, WebSocketChannel/DiscordChannel/TelegramChannel 实现
[POS]: agent/service/channel 的模块入口，被 handler 层和 app.py 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.service.channel.channel import MessageChannel, MessageSender, PermissionStrategy
from agent.service.channel.channel_manager import ChannelManager

__all__ = [
    "MessageSender",
    "MessageChannel",
    "PermissionStrategy",
    "ChannelManager",
]
