# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：wake_service.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation wake 请求簿记服务。"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Literal
from uuid import uuid4

WakeMode = Literal["now", "next-heartbeat"]


@dataclass(slots=True)
class WakeRequest:
    """面向后续调度层的轻量 wake 请求快照。"""

    request_id: str
    agent_id: str
    session_key: str
    wake_mode: WakeMode
    metadata: dict[str, object] = field(default_factory=dict)
    created_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))


class WakeService:
    """记录 automation session 的 wake 请求。"""

    def __init__(self) -> None:
        self._now_requests: dict[str, WakeRequest] = {}
        self._heartbeat_requests: dict[str, WakeRequest] = {}

    def request(
        self,
        *,
        agent_id: str,
        session_key: str,
        wake_mode: WakeMode,
        metadata: dict[str, object] | None = None,
    ) -> WakeRequest:
        """记录一个 wake 请求，并按 session_key 去重。"""
        request = WakeRequest(
            request_id=str(uuid4()),
            agent_id=agent_id,
            session_key=session_key,
            wake_mode=wake_mode,
            metadata=dict(metadata or {}),
        )

        # 中文注释：同一个 session 只保留最后一次意图，
        # 这样后续 heartbeat/cron 消费端只需要处理最新请求。
        if wake_mode == "now":
            self._now_requests[session_key] = request
        else:
            self._heartbeat_requests[session_key] = request
        return request

    def request_now(
        self,
        *,
        agent_id: str,
        session_key: str,
        metadata: dict[str, object] | None = None,
    ) -> WakeRequest:
        """记录一个立即唤醒请求。"""
        return self.request(
            agent_id=agent_id,
            session_key=session_key,
            wake_mode="now",
            metadata=metadata,
        )

    def request_next_heartbeat(
        self,
        *,
        agent_id: str,
        session_key: str,
        metadata: dict[str, object] | None = None,
    ) -> WakeRequest:
        """记录一个下次心跳消费的请求。"""
        return self.request(
            agent_id=agent_id,
            session_key=session_key,
            wake_mode="next-heartbeat",
            metadata=metadata,
        )

    def drain_now(self, agent_id: str | None = None) -> list[WakeRequest]:
        """弹出立即执行的 wake 请求。"""
        matched = self._filter_requests(self._now_requests, agent_id=agent_id)
        for item in matched:
            self._now_requests.pop(item.session_key, None)
        return matched

    def list_next_heartbeat(self, agent_id: str | None = None) -> list[WakeRequest]:
        """查看等待下一次心跳消费的请求。"""
        return self._filter_requests(self._heartbeat_requests, agent_id=agent_id)

    def clear(self, session_key: str) -> None:
        """清理某个 session 的所有 wake 请求。"""
        self._now_requests.pop(session_key, None)
        self._heartbeat_requests.pop(session_key, None)

    @staticmethod
    def _filter_requests(
        requests: dict[str, WakeRequest],
        *,
        agent_id: str | None,
    ) -> list[WakeRequest]:
        items = list(requests.values())
        if agent_id is None:
            return items
        return [item for item in items if item.agent_id == agent_id]
