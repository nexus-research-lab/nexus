# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_event_bus.py
# @Date   ：2026/3/10
# @Author ：Codex
# =====================================================

"""
Workspace 事件总线

[INPUT]: 依赖 WorkspaceEvent
[OUTPUT]: 对外提供 workspace_event_bus 单例
[POS]: service 层广播 workspace 文件实时事件，向 WebSocket 连接分发
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
import uuid
from collections import defaultdict
from typing import Awaitable, Callable, DefaultDict, Dict, Optional

from agent.service.schema.model_workspace_event import WorkspaceEvent
from agent.utils.logger import logger

WorkspaceEventListener = Callable[[WorkspaceEvent], Awaitable[None]]


class WorkspaceEventBus:
    """按 agent_id 广播 WorkspaceEvent。"""

    def __init__(self):
        self._listeners: DefaultDict[str, Dict[str, WorkspaceEventListener]] = defaultdict(dict)
        self._subscriptions: Dict[str, str] = {}
        self._queue: Optional[asyncio.Queue[WorkspaceEvent]] = None
        self._dispatcher_task: Optional[asyncio.Task] = None

    def subscribe(self, agent_id: str, listener: WorkspaceEventListener) -> str:
        """订阅某个 Agent 的 workspace 事件。"""
        token = str(uuid.uuid4())
        self._listeners[agent_id][token] = listener
        self._subscriptions[token] = agent_id
        logger.debug(f"📡 订阅 workspace 事件: agent={agent_id}, token={token}")
        return token

    def unsubscribe(self, token: str) -> None:
        """取消订阅。"""
        agent_id = self._subscriptions.pop(token, None)
        if not agent_id:
            return

        listeners = self._listeners.get(agent_id)
        if not listeners:
            return

        listeners.pop(token, None)
        if not listeners:
            self._listeners.pop(agent_id, None)
        logger.debug(f"🧹 取消订阅 workspace 事件: agent={agent_id}, token={token}")

    def publish(self, event: WorkspaceEvent) -> None:
        """发布事件，异步串行分发，保证顺序稳定。"""
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            logger.warning("⚠️ 当前无运行中的事件循环，忽略 workspace 事件")
            return

        if self._queue is None:
            self._queue = asyncio.Queue()

        if self._dispatcher_task is None or self._dispatcher_task.done():
            self._dispatcher_task = loop.create_task(self._dispatch_loop())

        self._queue.put_nowait(event)

    async def _dispatch_loop(self) -> None:
        """消费队列并分发给订阅者。"""
        if self._queue is None:
            return

        while True:
            event = await self._queue.get()
            try:
                await self._dispatch(event)
            except Exception as exc:
                logger.warning(f"⚠️ 分发 workspace 事件失败: {exc}")
            finally:
                self._queue.task_done()

            if self._queue.empty():
                break

    async def _dispatch(self, event: WorkspaceEvent) -> None:
        listeners = list(self._listeners.get(event.agent_id, {}).values())
        if not listeners:
            return

        await asyncio.gather(*(listener(event) for listener in listeners), return_exceptions=True)


workspace_event_bus = WorkspaceEventBus()
