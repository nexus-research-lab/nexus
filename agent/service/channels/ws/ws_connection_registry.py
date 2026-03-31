#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""全局 WebSocket 连接注册表。

允许 REST 路由向所有已连接的前端广播事件，
例如 Room 删除、成员变更等非 chat 触发的服务端事件。
"""

from __future__ import annotations

import asyncio
from typing import Dict, Set
from weakref import WeakSet

from agent.schema.model_message import EventMessage
from agent.utils.logger import logger


class WsConnectionRegistry:
    """轻量全局 WS 连接注册表，支持 room 级别订阅。"""

    def __init__(self) -> None:
        # WeakSet 避免持有 sender 导致连接无法 GC
        self._senders: "WeakSet" = WeakSet()
        # room_id -> set of sender ids subscribed; use object id as key
        self._room_subscriptions: Dict[str, Set[int]] = {}
        # sender_id -> sender object (for broadcasting to room subscribers)
        self._sender_by_id: Dict[int, "object"] = {}

    def register(self, sender: "object") -> None:
        """注册一个 WebSocketSender。"""
        self._senders.add(sender)
        self._sender_by_id[id(sender)] = sender

    def unregister(self, sender: "object") -> None:
        """注销一个 WebSocketSender（断开时调用）。"""
        self._senders.discard(sender)
        sender_id = id(sender)
        self._sender_by_id.pop(sender_id, None)
        # Clean up room subscriptions for this sender
        for room_id in list(self._room_subscriptions.keys()):
            self._room_subscriptions[room_id].discard(sender_id)
            if not self._room_subscriptions[room_id]:
                del self._room_subscriptions[room_id]

    def subscribe_room(self, sender: "object", room_id: str) -> None:
        """订阅指定 room 的事件。"""
        sender_id = id(sender)
        if room_id not in self._room_subscriptions:
            self._room_subscriptions[room_id] = set()
        self._room_subscriptions[room_id].add(sender_id)
        logger.debug(f"WS subscribe_room: room_id={room_id}, sender_id={sender_id}")

    def unsubscribe_room(self, sender: "object", room_id: str) -> None:
        """取消订阅指定 room 的事件。"""
        sender_id = id(sender)
        if room_id in self._room_subscriptions:
            self._room_subscriptions[room_id].discard(sender_id)
            if not self._room_subscriptions[room_id]:
                del self._room_subscriptions[room_id]

    async def broadcast(self, event: EventMessage) -> None:
        """向所有活跃连接广播一条事件。"""
        payload = event.model_dump(mode="json", exclude_none=True)
        tasks = []
        for sender in list(self._senders):
            tasks.append(self._safe_send(sender, payload))
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

    async def broadcast_to_room_subscribers(self, room_id: str, event: EventMessage) -> None:
        """只向订阅了指定 room 的连接广播事件。"""
        subscriber_ids = self._room_subscriptions.get(room_id, set()).copy()
        if not subscriber_ids:
            return
        payload = event.model_dump(mode="json", exclude_none=True)
        tasks = []
        for sender_id in subscriber_ids:
            sender = self._sender_by_id.get(sender_id)
            if sender is not None:
                tasks.append(self._safe_send(sender, payload))
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

    @staticmethod
    async def _safe_send(sender: "object", payload: Dict) -> None:
        try:
            await sender._safe_send_json(payload)  # type: ignore[attr-defined]
        except Exception as exc:
            logger.debug(f"broadcast: send failed: {exc}")


ws_connection_registry = WsConnectionRegistry()
