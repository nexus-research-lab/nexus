# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol_event_bus.py
# @Date   ：2026/3/25 22:18
# @Author ：OpenAI
# =====================================================

"""Protocol Room 事件总线。"""

from __future__ import annotations

import asyncio
import uuid
from collections import defaultdict
from typing import Awaitable, Callable, DefaultDict, Dict, Optional

from agent.schema.model_message import EventMessage
from agent.utils.logger import logger

ProtocolEventListener = Callable[[EventMessage], Awaitable[None]]


class ProtocolEventBus:
    """按 protocol_run_id 广播协议事件。"""

    def __init__(self) -> None:
        self._listeners: DefaultDict[str, Dict[str, ProtocolEventListener]] = defaultdict(dict)
        self._subscriptions: Dict[str, str] = {}
        self._queue: Optional[asyncio.Queue[tuple[str, EventMessage]]] = None
        self._dispatcher_task: Optional[asyncio.Task] = None

    def subscribe(self, run_id: str, listener: ProtocolEventListener) -> str:
        """订阅某个协议运行的事件。"""
        token = str(uuid.uuid4())
        self._listeners[run_id][token] = listener
        self._subscriptions[token] = run_id
        logger.debug("📡 订阅 protocol 事件: run=%s token=%s", run_id, token)
        return token

    def unsubscribe(self, token: str) -> None:
        """取消订阅。"""
        run_id = self._subscriptions.pop(token, None)
        if not run_id:
            return
        listeners = self._listeners.get(run_id)
        if not listeners:
            return
        listeners.pop(token, None)
        if not listeners:
            self._listeners.pop(run_id, None)
        logger.debug("🧹 取消订阅 protocol 事件: run=%s token=%s", run_id, token)

    def publish(self, run_id: str, event: EventMessage) -> None:
        """发布协议事件。"""
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            logger.warning("⚠️ 当前无运行中的事件循环，忽略 protocol 事件")
            return

        if self._queue is None:
            self._queue = asyncio.Queue()

        if self._dispatcher_task is None or self._dispatcher_task.done():
            self._dispatcher_task = loop.create_task(self._dispatch_loop())

        self._queue.put_nowait((run_id, event))

    async def _dispatch_loop(self) -> None:
        if self._queue is None:
            return

        while True:
            run_id, event = await self._queue.get()
            try:
                await self._dispatch(run_id, event)
            except Exception as exc:
                logger.warning(f"⚠️ 分发 protocol 事件失败: {exc}")
            finally:
                self._queue.task_done()

            if self._queue.empty():
                break

    async def _dispatch(self, run_id: str, event: EventMessage) -> None:
        listeners = list(self._listeners.get(run_id, {}).values())
        if listeners:
            await asyncio.gather(*(listener(event) for listener in listeners), return_exceptions=True)


protocol_event_bus = ProtocolEventBus()
