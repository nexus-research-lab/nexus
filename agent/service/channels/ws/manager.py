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

from agent.service.channels.ws.dispatcher import ChannelDispatcher
from agent.service.channels.ws.handlers.error_handler import ErrorHandler
from agent.service.channels.ws.handlers.interrupt_handler import InterruptHandler
from agent.service.channels.ws.handlers.permission_handler import PermissionHandler
from agent.service.channels.ws.handlers.ping_handler import PingHandler
from agent.service.channels.ws.websocket_sender import WebSocketSender
from agent.service.channels.ws.ws_session_routing_sender import (
    WsSessionRoutingSender,
)
from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry
from agent.service.chat.chat_service import ChatService
from agent.service.chat.room_chat_service import RoomChatService
from agent.service.permission.permission_runtime_context import (
    permission_runtime_context,
)
from agent.service.permission.strategy.permission_interactive import InteractivePermissionStrategy
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
        routing_sender = WsSessionRoutingSender(self.sender)

        permission_handler = PermissionHandler(self.sender, self.permission_strategy)
        chat_service = ChatService(routing_sender, self.permission_strategy)
        room_chat_service = RoomChatService(self.sender, self.permission_strategy)
        interrupt_handler = InterruptHandler(self.sender, self.permission_strategy)
        ping_handler = PingHandler(self.sender)
        error_handler = ErrorHandler(self.sender)

        return ChannelDispatcher(
            sender=self.sender,
            chat_service=chat_service,
            room_chat_service=room_chat_service,
            interrupt_handler=interrupt_handler,
            permission_handler=permission_handler,
            ping_handler=ping_handler,
            error_handler=error_handler,
            chat_tasks=chat_tasks,
        )

    async def cleanup(self, chat_tasks: Dict[str, asyncio.Task]) -> None:
        """清理当前连接持有的运行时资源。"""
        logger.info("🧹 WebSocket连接清理")
        changed_session_keys: tuple[str, ...] = ()
        if self.sender:
            changed_session_keys = InteractivePermissionStrategy.unregister_sender(self.sender)
        if self.permission_strategy:
            self.permission_strategy.close()

        # 中文注释：WebSocket 断线不再中断后台任务。
        # 运行中的 DM/Room 任务继续执行，前端重连后由新的控制端/观察端集合继续接管实时推送。
        # chat_tasks 作为全局运行表继续保留，供重连后的 interrupt 与新消息复用。

        if self.sender:
            self.sender.unsubscribe_all_workspace()

        for session_key in changed_session_keys:
            await permission_runtime_context.broadcast_session_status(
                session_key=session_key,
                running_round_ids=ws_chat_task_registry.get_running_round_ids(session_key),
            )

        self.permission_strategy = None
        self.sender = None
