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

from agent.channels.message_sender import MessageSender
from agent.infra.workspace.monitor import workspace_event_bus, workspace_observer
from agent.schema.model_message import AError, AEvent, AMessage
from agent.schema.model_workspace_event import WorkspaceEvent
from agent.service.process.protocol_adapter import ProtocolAdapter
from agent.utils.logger import logger


class WebSocketSender(MessageSender):
    """WebSocket 消息发送器。"""

    def __init__(self, websocket: WebSocket):
        self.websocket = websocket
        self.protocol_adapter = ProtocolAdapter()
        self._workspace_subscriptions: Dict[str, str] = {}
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

    async def send_message(self, message: AMessage) -> None:
        """发送会话消息。"""
        event = self.protocol_adapter.build_ws_event(message)
        if event is None:
            return

        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)
        logger.debug(f"💬发送消息: {payload}")

    async def send_event(self, event: AEvent) -> None:
        """发送事件消息。"""
        payload = event.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

    async def send_error(self, error: AError) -> None:
        """发送错误消息。"""
        payload = error.model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

    async def send_workspace_event(self, event: WorkspaceEvent) -> None:
        """发送 workspace 事件。"""
        payload = self.protocol_adapter.build_workspace_event(event).model_dump()
        payload["timestamp"] = payload["timestamp"].isoformat()
        await self._safe_send_json(payload)

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

    @property
    def is_closed(self) -> bool:
        """返回当前连接是否已关闭。"""
        return self._is_closed
