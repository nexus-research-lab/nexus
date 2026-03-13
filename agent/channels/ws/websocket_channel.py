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

[INPUT]: 依赖 fastapi.WebSocket，依赖 message_sender.py / message_channel.py 抽象协议,
         依赖 workspace 事件总线
[OUTPUT]: 对外提供 WebSocketChannel
[POS]: channel 模块的 WebSocket 实现，封装现有 WebSocket 行为（纯重构、零行为变更）
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.channels.message_channel import MessageChannel
from agent.utils.logger import logger


# =====================================================
# WebSocketChannel — WebSocket 通道（无操作占位）
#
# WebSocket 的生命周期由 FastAPI 管理（每连接创建销毁），
# 无需 ChannelRegister 管理。channel_type 用于标识。
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
