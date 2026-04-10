# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：heartbeat_scheduler.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation heartbeat 调度器。"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

from agent.schema.model_automation import AutomationHeartbeatConfig


@dataclass(slots=True)
class HeartbeatRuntimeState:
    """单个 agent 的心跳运行态。"""

    config: AutomationHeartbeatConfig
    next_run_at: datetime | None = None
    pending_wake: bool = False
    inflight: bool = False
    last_dispatch_at: datetime | None = None
    last_dispatch_reason: str | None = None


class HeartbeatScheduler:
    """维护 heartbeat 下一次触发时间与 wake 请求。"""

    def __init__(
        self,
        *,
        dispatcher,
        tick_seconds: float = 1.0,
    ) -> None:
        self._dispatcher = dispatcher
        self._tick_seconds = tick_seconds
        self._states: dict[str, HeartbeatRuntimeState] = {}
        self._running = False
        self._task: asyncio.Task | None = None
        self._stop_event = asyncio.Event()

    async def start(self) -> None:
        """启动后台调度循环。"""
        if self._running:
            return
        self._running = True
        self._stop_event = asyncio.Event()
        self._task = asyncio.create_task(self._run_loop())

    async def stop(self) -> None:
        """停止后台调度循环。"""
        self._running = False
        self._stop_event.set()
        if self._task is not None:
            await self._task
            self._task = None

    async def sync_agent(self, agent_id: str, config: AutomationHeartbeatConfig) -> None:
        """同步 agent 的心跳配置到运行态。"""
        now = datetime.now(timezone.utc)
        state = self._states.get(agent_id)
        if state is None:
            next_run_at = now + timedelta(seconds=config.every_seconds) if config.enabled else None
            self._states[agent_id] = HeartbeatRuntimeState(
                config=config,
                next_run_at=next_run_at,
            )
            return

        schedule_changed = (
            state.config.enabled != config.enabled
            or state.config.every_seconds != config.every_seconds
        )
        state.config = config
        if not config.enabled:
            state.next_run_at = None
            state.pending_wake = False
            return
        if schedule_changed or state.next_run_at is None:
            state.next_run_at = now + timedelta(seconds=config.every_seconds)

    async def request_wake(self, *, agent_id: str, mode: str) -> dict[str, object]:
        """记录 wake 请求，并按模式决定是否立刻执行。"""
        state = self._states.get(agent_id)
        if state is None:
            state = HeartbeatRuntimeState(
                config=AutomationHeartbeatConfig(agent_id=agent_id),
                next_run_at=None,
            )
            self._states[agent_id] = state

        if mode == "now":
            asyncio.create_task(self._dispatch(agent_id, state, reason="wake-now"))
            return {"agent_id": agent_id, "mode": mode, "scheduled": True}

        state.pending_wake = True
        return {"agent_id": agent_id, "mode": mode, "scheduled": False}

    def get_runtime_status(self, agent_id: str) -> dict[str, object]:
        """导出给 service/API 使用的运行态快照。"""
        state = self._states.get(agent_id)
        if state is None:
            return {
                "running": self._running,
                "next_run_at": None,
                "pending_wake": False,
                "last_dispatch_at": None,
                "last_dispatch_reason": None,
            }
        return {
            "running": self._running,
            "next_run_at": state.next_run_at,
            "pending_wake": state.pending_wake,
            "last_dispatch_at": state.last_dispatch_at,
            "last_dispatch_reason": state.last_dispatch_reason,
        }

    async def _run_loop(self) -> None:
        """轮询所有已知 heartbeat 状态并执行到期任务。"""
        while not self._stop_event.is_set():
            await self._run_due_once()
            try:
                await asyncio.wait_for(self._stop_event.wait(), timeout=self._tick_seconds)
            except asyncio.TimeoutError:
                continue

    async def _run_due_once(self) -> None:
        now = datetime.now(timezone.utc)
        for agent_id, state in list(self._states.items()):
            if not state.config.enabled or state.next_run_at is None:
                continue
            if state.next_run_at > now:
                continue
            await self._dispatch(agent_id, state, reason="heartbeat")

    async def _dispatch(
        self,
        agent_id: str,
        state: HeartbeatRuntimeState,
        *,
        reason: str,
    ) -> None:
        if state.inflight:
            return

        state.inflight = True
        try:
            await self._dispatcher.dispatch(agent_id=agent_id, config=state.config)
            now = datetime.now(timezone.utc)
            state.last_dispatch_at = now
            state.last_dispatch_reason = reason
            state.pending_wake = False
            if state.config.enabled:
                state.next_run_at = now + timedelta(seconds=state.config.every_seconds)
            else:
                state.next_run_at = None
        finally:
            state.inflight = False
