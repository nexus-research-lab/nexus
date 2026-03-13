# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：handler.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""WebSocket 连接处理器。"""

import asyncio
from typing import Any, Dict, Optional

from fastapi import WebSocket, WebSocketDisconnect

from agent.channels.ws.manager import WebSocketConnectionManager
from agent.utils.logger import logger


class WebSocketHandler:
    """负责单个 WebSocket 连接的消息收发。"""

    def __init__(self) -> None:
        self.websocket: Optional[WebSocket] = None
        self.chat_tasks: Dict[str, asyncio.Task] = {}
        self._connection_manager: Optional[WebSocketConnectionManager] = None

    async def handle_websocket_connection(self, websocket: WebSocket) -> None:
        """处理 WebSocket 连接的主要逻辑。"""
        self.websocket = websocket
        await self.websocket.accept()
        self._connection_manager = WebSocketConnectionManager(websocket)
        dispatcher = self._connection_manager.build_dispatcher(self.chat_tasks)

        try:
            while True:
                message = await self.websocket.receive_json()
                logger.debug(f"💌收到消息: {message}")
                await dispatcher.dispatch(message)
        except WebSocketDisconnect as exc:
            raise exc
        finally:
            await self.on_close()

    async def on_close(self) -> None:
        """清理 WebSocket 连接资源。"""
        if self._connection_manager:
            await self._connection_manager.cleanup(self.chat_tasks)
            self._connection_manager = None
        self.websocket = None
