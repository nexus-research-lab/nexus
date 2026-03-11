# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：websocket_handler.py
# @Date   ：2025/11/28 15:27
# @Author ：leemysw
#
# 2025/11/28 15:27   Create
# 2026/2/25          重构：使用 WebSocketSender + InteractivePermissionStrategy
# =====================================================

"""
WebSocket 连接处理器

[INPUT]: 依赖 fastapi.WebSocket，
         依赖 channel.websocket_channel 的 WebSocketSender/InteractivePermissionStrategy,
         依赖 handler 层的 ChatHandler/PermissionHandler/InterruptHandler/PingHandler/ErrorHandler
[OUTPUT]: 对外提供 WebSocketHandler
[POS]: service 层的 WebSocket 入口，每个连接创建一个实例
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
from typing import Any, Dict, Optional

from fastapi import WebSocket, WebSocketDisconnect

from agent.service.channel.websocket_channel import InteractivePermissionStrategy, WebSocketSender
from agent.service.handler import ChatHandler, ErrorHandler, InterruptHandler, PermissionHandler, PingHandler
from agent.service.session.session_router import build_session_key
from agent.service.session_manager import session_manager
from agent.utils.logger import logger


class WebSocketHandler:
    """WebSocket消息处理器"""

    def __init__(self):
        self.websocket: Optional[WebSocket] = None
        self.chat_tasks: Dict[str, asyncio.Task] = {}
        self.sender: Optional[WebSocketSender] = None
        self.permission_strategy: Optional[InteractivePermissionStrategy] = None

        # Handler 实例（连接时初始化）
        self.permission_handler: Optional[PermissionHandler] = None
        self.chat_handler: Optional[ChatHandler] = None
        self.interrupt_handler: Optional[InterruptHandler] = None
        self.ping_handler: Optional[PingHandler] = None
        self.error_handler: Optional[ErrorHandler] = None

    def init_handlers(self, websocket: WebSocket) -> None:
        """初始化各处理器 — 使用 WebSocketSender 和 InteractivePermissionStrategy"""
        sender = WebSocketSender(websocket)
        self.sender = sender
        permission_strategy = InteractivePermissionStrategy(sender)
        self.permission_strategy = permission_strategy

        self.permission_handler = PermissionHandler(sender, permission_strategy)
        self.chat_handler = ChatHandler(sender, permission_strategy)
        self.interrupt_handler = InterruptHandler(sender)
        self.ping_handler = PingHandler(sender)
        self.error_handler = ErrorHandler(sender)

    async def handle_websocket_connection(self, websocket: WebSocket) -> None:
        """处理 WebSocket 连接的主要逻辑"""
        self.websocket = websocket
        await self.websocket.accept()
        self.init_handlers(websocket)

        try:
            while True:
                message = await self.websocket.receive_json()
                logger.debug(f"💌收到消息: {message}")
                msg_type = message.get("type")
                await self.on_message(message, msg_type)
        except WebSocketDisconnect as wde:
            raise wde
        except Exception as e:
            await self.error_handler.handle_websocket_error(e)
        finally:
            await self.on_close()

    async def on_message(self, message: Dict[str, Any], msg_type: str) -> None:
        """根据消息类型路由到对应处理器"""
        # 将前端 agent_id 转换为 session_key
        if "agent_id" in message and "session_key" not in message:
            message["session_key"] = build_session_key(
                channel="ws",
                chat_type="dm",
                ref=message["agent_id"],
                agent_id=message["agent_id"],
            )

        if msg_type == "chat":
            await self.chat_handler.handle_chat_message_with_task(message, self.chat_tasks)
        elif msg_type == "interrupt":
            await self.interrupt_handler.handle_interrupt(message, self.chat_tasks)
        elif msg_type == "permission_response":
            await self.permission_handler.handle_permission_response(message)
        elif msg_type == "subscribe_workspace":
            agent_id = message.get("agent_id", "")
            if self.sender and agent_id:
                self.sender.subscribe_workspace(agent_id)
        elif msg_type == "unsubscribe_workspace":
            agent_id = message.get("agent_id", "")
            if self.sender and agent_id:
                self.sender.unsubscribe_workspace(agent_id)
        elif msg_type == "ping":
            await self.ping_handler.handle_ping(message)
        else:
            await self.error_handler.handle_unknown_message_type(message)

    async def on_close(self) -> None:
        """清理 WebSocket 连接资源"""
        logger.info("🧹 WebSocket连接清理")
        if self.permission_strategy:
            self.permission_strategy.close()

        for session_key, task in self.chat_tasks.items():
            if not task.done():
                logger.info(f"🛑 清理: 取消 chat 任务 {session_key}")
                task.cancel()

            try:
                client = await session_manager.get_session(session_key)
                if client:
                    await client.interrupt()
                    logger.info(f"⏸️ 清理: 中断 SDK 生成 {session_key}")
            except Exception as e:
                logger.warning(f"⚠️ 中断 SDK 失败 {session_key}: {e}")

        if self.chat_tasks:
            await asyncio.gather(*self.chat_tasks.values(), return_exceptions=True)

        self.chat_tasks.clear()
        if self.permission_strategy:
            self.permission_strategy = None
        if self.sender:
            self.sender.unsubscribe_all_workspace()
            self.sender = None
        self.websocket = None
