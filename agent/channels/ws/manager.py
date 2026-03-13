# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：manager.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""WebSocket 连接生命周期管理器。"""

import asyncio
from typing import Any, Dict, Optional

from fastapi import WebSocket

from agent.channels.ws.dispatcher import ChannelDispatcher
from agent.channels.ws.handlers.error_handler import ErrorHandler
from agent.channels.ws.handlers.interrupt_handler import InterruptHandler
from agent.channels.ws.handlers.permission_handler import PermissionHandler
from agent.channels.ws.handlers.ping_handler import PingHandler
from agent.channels.ws.websocket_sender import WebSocketSender
from agent.infra.agent.session_manager import session_manager
from agent.infra.permission.permission_interactive import InteractivePermissionStrategy
from agent.service.chat.chat_service import ChatService
from agent.utils.logger import logger


class WebSocketConnectionManager:
    """负责装配单个 WebSocket 连接的运行时对象。"""

    def __init__(self, websocket: WebSocket) -> None:
        self._websocket = websocket
        self.sender: Optional[WebSocketSender] = None
        self.permission_strategy: Optional[InteractivePermissionStrategy] = None

    def build_dispatcher(self, chat_tasks: Dict[str, Any]) -> ChannelDispatcher:
        """初始化连接级依赖并返回消息分发器。"""
        self.sender = WebSocketSender(self._websocket)
        self.permission_strategy = InteractivePermissionStrategy(self.sender)

        permission_handler = PermissionHandler(self.sender, self.permission_strategy)
        chat_service = ChatService(self.sender, self.permission_strategy)
        interrupt_handler = InterruptHandler(self.sender)
        ping_handler = PingHandler(self.sender)
        error_handler = ErrorHandler(self.sender)

        return ChannelDispatcher(
            sender=self.sender,
            chat_service=chat_service,
            interrupt_handler=interrupt_handler,
            permission_handler=permission_handler,
            ping_handler=ping_handler,
            error_handler=error_handler,
            chat_tasks=chat_tasks,
        )

    async def cleanup(self, chat_tasks: Dict[str, asyncio.Task]) -> None:
        """清理当前连接持有的运行时资源。"""
        logger.info("🧹 WebSocket连接清理")
        if self.permission_strategy:
            self.permission_strategy.close()

        for session_key, task in chat_tasks.items():
            if not task.done():
                logger.info(f"🛑 清理: 取消 chat 任务 {session_key}")
                task.cancel()

            try:
                client = await session_manager.get_session(session_key)
                if client:
                    await client.interrupt()
                    logger.info(f"⏸️ 清理: 中断 SDK 生成 {session_key}")
            except Exception as exc:
                logger.warning(f"⚠️ 中断 SDK 失败 {session_key}: {exc}")

        if chat_tasks:
            await asyncio.gather(*chat_tasks.values(), return_exceptions=True)

        chat_tasks.clear()

        if self.sender:
            self.sender.unsubscribe_all_workspace()

        self.permission_strategy = None
        self.sender = None
