# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：websocket_sender.py
# @Date   ：2026/3/13 18:26
# @Author ：leemysw
# 2026/3/13 18:26   Create
# =====================================================

"""WebSocket 消息发送器。"""

from typing import Any, Dict

from fastapi import WebSocket

from agent.service.channels.message_sender import MessageSender
from agent.service.protocol.protocol_event_bus import protocol_event_bus
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_observer import workspace_observer
from agent.schema.model_message import EventMessage, Message, StreamMessage
from agent.schema.model_workspace import WorkspaceEvent
from agent.utils.logger import logger


class WebSocketSender(MessageSender):
    """WebSocket 消息发送器。"""

    def __init__(self, websocket: WebSocket):
        self.websocket = websocket
        self._workspace_subscriptions: Dict[str, str] = {}
        self._protocol_run_subscriptions: Dict[str, str] = {}
        self._is_closed = False

    async def _safe_send_json(self, payload: Dict[str, Any]) -> None:
        """安全发送，连接关闭后不再继续推送。"""
        if self._is_closed:
            return

        try:
            await self.websocket.send_json(payload)
        except RuntimeError as exc:
            self._is_closed = True
            self.unsubscribe_all_workspace()
            logger.warning(f"⚠️ WebSocket 已关闭，停止发送 workspace 事件: {exc}")
        except Exception:
            self._is_closed = True
            self.unsubscribe_all_workspace()
            raise

    async def send_message(self, message: Message) -> None:
        """发送完整消息。"""
        event = EventMessage(
            event_type="message",
            session_key=message.session_key,
            agent_id=message.agent_id,
            session_id=message.session_id,
            data=message.model_dump(mode="json", exclude_none=True),
        )
        await self._safe_send_json(event.model_dump(mode="json", exclude_none=True))

    async def send_stream_message(self, message: StreamMessage) -> None:
        """发送流式消息。"""
        event = EventMessage(
            event_type="stream",
            session_key=message.session_key,
            agent_id=message.agent_id,
            session_id=message.session_id,
            data=message.model_dump(mode="json", exclude_none=True),
        )
        await self._safe_send_json(event.model_dump(mode="json", exclude_none=True))

    async def send_event_message(self, event: EventMessage) -> None:
        """发送事件消息。"""
        await self._safe_send_json(event.model_dump(mode="json", exclude_none=True))

    async def send_workspace_event(self, event: WorkspaceEvent) -> None:
        """发送 workspace 事件。"""
        message = EventMessage(
            event_type="workspace_event",
            session_key=event.session_key,
            agent_id=event.agent_id,
            data=event.model_dump(mode="json", exclude_none=True),
        )
        await self._safe_send_json(message.model_dump(mode="json", exclude_none=True))

    def subscribe_protocol_run(self, run_id: str) -> None:
        """订阅指定协议运行的实时事件。"""
        if not run_id or run_id in self._protocol_run_subscriptions:
            return

        token_holder: Dict[str, str] = {}

        async def _listener(event: EventMessage) -> None:
            try:
                await self.send_event_message(event)
            except Exception as exc:
                logger.warning(f"⚠️ protocol 事件推送失败: {exc}")
                token = token_holder.get("token")
                if token:
                    protocol_event_bus.unsubscribe(token)

        token = protocol_event_bus.subscribe(run_id, _listener)
        token_holder["token"] = token
        self._protocol_run_subscriptions[run_id] = token

    def unsubscribe_protocol_run(self, run_id: str) -> None:
        """取消订阅指定协议运行。"""
        token = self._protocol_run_subscriptions.pop(run_id, None)
        if not token:
            return
        protocol_event_bus.unsubscribe(token)

    def subscribe_workspace(self, agent_id: str) -> None:
        """订阅指定 Agent 的 workspace 事件。"""
        if not agent_id or agent_id in self._workspace_subscriptions:
            return

        token_holder: Dict[str, str] = {}

        async def _listener(event: WorkspaceEvent) -> None:
            try:
                await self.send_workspace_event(event)
            except Exception as exc:
                logger.warning(f"⚠️ workspace 事件推送失败: {exc}")
                token = token_holder.get("token")
                if token:
                    workspace_event_bus.unsubscribe(token)

        token = workspace_event_bus.subscribe(agent_id, _listener)
        token_holder["token"] = token
        self._workspace_subscriptions[agent_id] = token
        workspace_observer.subscribe(agent_id)

    def unsubscribe_workspace(self, agent_id: str) -> None:
        """取消订阅指定 Agent 的 workspace 事件。"""
        token = self._workspace_subscriptions.pop(agent_id, None)
        if not token:
            return
        workspace_event_bus.unsubscribe(token)
        workspace_observer.unsubscribe(agent_id)

    def unsubscribe_all_workspace(self) -> None:
        """取消当前连接的所有 workspace 事件订阅。"""
        self._is_closed = True
        for agent_id in list(self._workspace_subscriptions.keys()):
            self.unsubscribe_workspace(agent_id)
        for run_id in list(self._protocol_run_subscriptions.keys()):
            self.unsubscribe_protocol_run(run_id)

    @property
    def is_closed(self) -> bool:
        """返回当前连接是否已关闭。"""
        return self._is_closed
