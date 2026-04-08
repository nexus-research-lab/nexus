#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""全局 WebSocket 连接注册表。"""

from __future__ import annotations

import asyncio
from collections import deque
from typing import Deque, Dict, Optional
from weakref import WeakSet

from agent.schema.model_message import EventMessage
from agent.utils.logger import logger


class WsConnectionRegistry:
    """轻量全局 WS 连接注册表，支持 room 级别订阅。"""

    def __init__(self) -> None:
        self._senders: "WeakSet" = WeakSet()
        self._room_subscriptions: Dict[str, Dict[int, Optional[str]]] = {}
        self._sender_by_id: Dict[int, "object"] = {}
        self._room_sequences: Dict[str, int] = {}
        self._room_replay_buffers: Dict[str, Deque[EventMessage]] = {}
        self._room_replay_buffer_size = 128

    def register(self, sender: "object") -> None:
        """注册一个 WebSocketSender。"""
        self._senders.add(sender)
        self._sender_by_id[id(sender)] = sender

    def unregister(self, sender: "object") -> None:
        """注销一个 WebSocketSender（断开时调用）。"""
        self._senders.discard(sender)
        sender_id = id(sender)
        self._sender_by_id.pop(sender_id, None)
        for room_id in list(self._room_subscriptions.keys()):
            self._room_subscriptions[room_id].pop(sender_id, None)
            if not self._room_subscriptions[room_id]:
                del self._room_subscriptions[room_id]

    async def subscribe_room(
        self,
        sender: "object",
        room_id: str,
        conversation_id: str | None = None,
        last_seen_room_seq: int | None = None,
    ) -> None:
        """订阅指定 room 的事件。"""
        sender_id = id(sender)
        if room_id not in self._room_subscriptions:
            self._room_subscriptions[room_id] = {}
        self._room_subscriptions[room_id][sender_id] = conversation_id
        logger.debug(
            "WS subscribe_room: room_id=%s, conversation_id=%s, sender_id=%s, last_seen_room_seq=%s",
            room_id, conversation_id, sender_id, last_seen_room_seq,
        )
        if last_seen_room_seq is None or last_seen_room_seq <= 0:
            return
        await self._replay_room_events(
            sender=sender,
            room_id=room_id,
            conversation_id=conversation_id,
            last_seen_room_seq=last_seen_room_seq,
        )

    def unsubscribe_room(self, sender: "object", room_id: str) -> None:
        """取消订阅指定 room 的事件。"""
        sender_id = id(sender)
        if room_id in self._room_subscriptions:
            self._room_subscriptions[room_id].pop(sender_id, None)
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

    async def broadcast_to_room_subscribers(
        self,
        room_id: str,
        event: EventMessage,
        fallback_sender: "object" | None = None,
    ) -> tuple[int, ...]:
        """只向订阅了指定 room 的连接广播事件。

        当发起方连接尚未来得及完成 room 订阅时，允许回退直发给
        fallback_sender，避免首轮实时消息因为订阅时序竞争而丢失。
        """
        subscribers = self._room_subscriptions.get(room_id, {}).copy()
        prepared_event = self._prepare_room_event(room_id, event)
        payload = prepared_event.model_dump(mode="json", exclude_none=True)
        tasks = []
        recipient_sender_ids = self.resolve_room_recipient_ids(
            room_id=room_id,
            conversation_id=prepared_event.conversation_id,
            fallback_sender=fallback_sender,
        )
        for sender_id, subscribed_conversation_id in subscribers.items():
            if not self._conversation_matches(
                subscribed_conversation_id,
                prepared_event.conversation_id,
            ):
                continue
            sender = self._sender_by_id.get(sender_id)
            if sender is not None:
                tasks.append(self._safe_send(sender, payload))

        if (
            fallback_sender is not None and
            not self._is_sender_subscribed_to_room(
                fallback_sender,
                room_id,
                prepared_event.conversation_id,
            )
        ):
            logger.debug(
                "WS room fallback send: room_id=%s, conversation_id=%s, sender_id=%s, event_type=%s",
                room_id,
                prepared_event.conversation_id,
                id(fallback_sender),
                prepared_event.event_type,
            )
            tasks.append(self._safe_send(fallback_sender, payload))

        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)
        return recipient_sender_ids

    def resolve_room_recipient_ids(
        self,
        room_id: str,
        conversation_id: str | None,
        fallback_sender: "object" | None = None,
    ) -> tuple[int, ...]:
        """解析当前 room 广播会命中的连接集合。"""
        subscribers = self._room_subscriptions.get(room_id, {}).copy()
        recipient_sender_ids: list[int] = []
        for sender_id, subscribed_conversation_id in subscribers.items():
            if not self._conversation_matches(
                subscribed_conversation_id,
                conversation_id,
            ):
                continue
            if self._sender_by_id.get(sender_id) is not None:
                recipient_sender_ids.append(sender_id)
        if (
            fallback_sender is not None and
            not self._is_sender_subscribed_to_room(
                fallback_sender,
                room_id,
                conversation_id,
            )
        ):
            recipient_sender_ids.append(id(fallback_sender))
        return tuple(sorted(set(recipient_sender_ids)))

    async def _replay_room_events(
        self,
        sender: "object",
        room_id: str,
        conversation_id: str | None,
        last_seen_room_seq: int,
    ) -> None:
        """向重连后的订阅方回放仍在缓冲区内的 durable 事件。"""
        latest_room_seq = self._room_sequences.get(room_id, 0)
        if latest_room_seq <= last_seen_room_seq:
            return
        buffer = list(self._room_replay_buffers.get(room_id, ()))
        if not buffer:
            await self._send_room_resync_required(
                sender=sender,
                room_id=room_id,
                conversation_id=conversation_id,
                last_seen_room_seq=last_seen_room_seq,
                latest_room_seq=latest_room_seq,
                buffer_start_room_seq=None,
            )
            return

        earliest_room_seq = buffer[0].room_seq
        if earliest_room_seq is None:
            return
        if last_seen_room_seq < earliest_room_seq - 1:
            await self._send_room_resync_required(
                sender=sender,
                room_id=room_id,
                conversation_id=conversation_id,
                last_seen_room_seq=last_seen_room_seq,
                latest_room_seq=latest_room_seq,
                buffer_start_room_seq=earliest_room_seq,
            )
            return

        replay_events = [
            event
            for event in buffer
            if (event.room_seq or 0) > last_seen_room_seq
            and self._conversation_matches(conversation_id, event.conversation_id)
        ]
        for replay_event in replay_events:
            await self._safe_send(sender, replay_event.model_dump(mode="json", exclude_none=True))

    async def _send_room_resync_required(
        self,
        sender: "object",
        room_id: str,
        conversation_id: str | None,
        last_seen_room_seq: int,
        latest_room_seq: int,
        buffer_start_room_seq: int | None,
    ) -> None:
        """通知前端当前 room 需要回源重拉。"""
        event = EventMessage(
            event_type="room_resync_required",
            delivery_mode="ephemeral",
            room_id=room_id,
            conversation_id=conversation_id,
            data={
                "room_id": room_id,
                "conversation_id": conversation_id,
                "last_seen_room_seq": last_seen_room_seq,
                "latest_room_seq": latest_room_seq,
                "buffer_start_room_seq": buffer_start_room_seq,
            },
        )
        await self._safe_send(sender, event.model_dump(mode="json", exclude_none=True))

    def _prepare_room_event(self, room_id: str, event: EventMessage) -> EventMessage:
        """为 room 广播事件补齐路由与顺序号。"""
        prepared_event = event
        if event.room_id != room_id:
            prepared_event = prepared_event.model_copy(update={"room_id": room_id})

        if prepared_event.delivery_mode != "durable":
            return prepared_event

        if prepared_event.room_seq is not None:
            return prepared_event

        next_room_seq = self._room_sequences.get(room_id, 0) + 1
        self._room_sequences[room_id] = next_room_seq
        prepared_event = prepared_event.model_copy(update={"room_seq": next_room_seq})

        buffer = self._room_replay_buffers.setdefault(
            room_id,
            deque(maxlen=self._room_replay_buffer_size),
        )
        buffer.append(prepared_event)
        return prepared_event

    @staticmethod
    def _conversation_matches(
        subscribed_conversation_id: str | None,
        event_conversation_id: str | None,
    ) -> bool:
        """判断订阅的 conversation 过滤条件是否匹配当前事件。"""
        if not subscribed_conversation_id:
            return True
        if event_conversation_id is None:
            return True
        return subscribed_conversation_id == event_conversation_id

    def _is_sender_subscribed_to_room(
        self,
        sender: "object",
        room_id: str,
        conversation_id: str | None,
    ) -> bool:
        """判断指定连接是否已订阅当前 room 路由。"""
        room_subscriptions = self._room_subscriptions.get(room_id, {})
        sender_id = id(sender)
        if sender_id not in room_subscriptions:
            return False
        subscribed_conversation_id = room_subscriptions.get(sender_id)
        return self._conversation_matches(
            subscribed_conversation_id,
            conversation_id,
        )

    @staticmethod
    async def _safe_send(sender: "object", payload: Dict) -> None:
        try:
            await sender._safe_send_json(payload)  # type: ignore[attr-defined]
        except Exception as exc:
            logger.debug(f"broadcast: send failed: {exc}")


ws_connection_registry = WsConnectionRegistry()
