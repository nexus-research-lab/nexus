# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：monitor.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""Workspace 事件分发与轮询观察。"""

import asyncio
import uuid
from collections import defaultdict
from dataclasses import dataclass
from typing import Awaitable, Callable, DefaultDict, Dict, Optional

from agent.schema.model_workspace_event import WorkspaceEvent
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
        """发布事件，异步串行分发。"""
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
        """将事件分发给当前 agent 的订阅者。"""
        listeners = list(self._listeners.get(event.agent_id, {}).values())
        if listeners:
            await asyncio.gather(*(listener(event) for listener in listeners), return_exceptions=True)


@dataclass
class ObservedFileSnapshot:
    """文件快照。"""

    modified_at: str
    size: int
    content: str


@dataclass
class ActiveWriteState:
    """正在写入中的文件状态。"""

    before_content: str
    current_content: str
    last_modified_at: str
    last_change_at: float
    version: int


class WorkspaceObserver:
    """按 Agent 轮询 workspace 文件变化，并推断写入事件。"""

    def __init__(self):
        self._subscription_counts: Dict[str, int] = {}
        self._watch_tasks: Dict[str, asyncio.Task] = {}
        self._snapshots: Dict[str, Dict[str, ObservedFileSnapshot]] = {}
        self._active_writes: Dict[str, Dict[str, ActiveWriteState]] = {}
        self._poll_interval_seconds = 0.35
        self._quiet_window_seconds = 0.8

    def subscribe(self, agent_id: str) -> None:
        """增加某个 Agent 的观察订阅。"""
        count = self._subscription_counts.get(agent_id, 0) + 1
        self._subscription_counts[agent_id] = count
        if count > 1:
            return

        loop = asyncio.get_running_loop()
        self._watch_tasks[agent_id] = loop.create_task(self._watch_agent(agent_id))
        logger.debug(f"👀 开始观察 workspace: agent={agent_id}")

    def unsubscribe(self, agent_id: str) -> None:
        """减少某个 Agent 的观察订阅。"""
        count = self._subscription_counts.get(agent_id, 0)
        if count <= 1:
            self._subscription_counts.pop(agent_id, None)
            task = self._watch_tasks.pop(agent_id, None)
            if task:
                task.cancel()
            self._snapshots.pop(agent_id, None)
            self._active_writes.pop(agent_id, None)
            logger.debug(f"🛑 停止观察 workspace: agent={agent_id}")
            return

        self._subscription_counts[agent_id] = count - 1

    async def _watch_agent(self, agent_id: str) -> None:
        """轮询某个 Agent 的 workspace。"""
        from agent.service.agent.agent_manager import agent_manager

        try:
            workspace = await agent_manager.get_agent_workspace(agent_id)
            self._snapshots[agent_id] = await self._capture_snapshot(workspace)
            self._active_writes.setdefault(agent_id, {})

            while self._subscription_counts.get(agent_id, 0) > 0:
                await asyncio.sleep(self._poll_interval_seconds)
                await self._poll_workspace(agent_id, workspace)
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            logger.warning(f"⚠️ workspace 观察失败: agent={agent_id}, error={exc}")

    async def _poll_workspace(self, agent_id: str, workspace) -> None:
        """对比新旧快照，推断文件写入中的开始、增量和结束。"""
        previous_snapshot = self._snapshots.get(agent_id, {})
        current_snapshot = await self._capture_snapshot(workspace)
        active_writes = self._active_writes.setdefault(agent_id, {})
        now = asyncio.get_running_loop().time()

        for path, current in current_snapshot.items():
            previous = previous_snapshot.get(path)
            if previous and previous.modified_at == current.modified_at and previous.size == current.size:
                continue

            if path not in active_writes:
                active_writes[path] = ActiveWriteState(
                    before_content=previous.content if previous else "",
                    current_content=current.content,
                    last_modified_at=current.modified_at,
                    last_change_at=now,
                    version=1,
                )
                workspace_event_bus.publish(
                    WorkspaceEvent(
                        type="file_write_start",
                        agent_id=agent_id,
                        path=path,
                        version=1,
                        source="agent",
                        content_snapshot=previous.content if previous else "",
                    )
                )
            else:
                state = active_writes[path]
                state.version += 1
                state.current_content = current.content
                state.last_modified_at = current.modified_at
                state.last_change_at = now

            state = active_writes[path]
            workspace_event_bus.publish(
                WorkspaceEvent(
                    type="file_write_delta",
                    agent_id=agent_id,
                    path=path,
                    version=state.version,
                    source="agent",
                    content_snapshot=current.content,
                )
            )

        for path, state in list(active_writes.items()):
            current = current_snapshot.get(path)
            if not current:
                continue

            if now - state.last_change_at < self._quiet_window_seconds:
                continue

            workspace_event_bus.publish(
                WorkspaceEvent(
                    type="file_write_end",
                    agent_id=agent_id,
                    path=path,
                    version=state.version,
                    source="agent",
                    content_snapshot=state.current_content,
                    diff_stats=workspace._build_diff_stats(state.before_content, state.current_content),
                )
            )
            active_writes.pop(path, None)

        self._snapshots[agent_id] = current_snapshot

    async def _capture_snapshot(self, workspace) -> Dict[str, ObservedFileSnapshot]:
        """抓取当前 workspace 可见文本文件快照。"""
        snapshot: Dict[str, ObservedFileSnapshot] = {}
        for entry in workspace.list_files():
            if entry["is_dir"]:
                continue

            path = str(entry["path"])
            try:
                content = workspace.read_relative_file(path)
            except (UnicodeDecodeError, ValueError, FileNotFoundError):
                continue

            snapshot[path] = ObservedFileSnapshot(
                modified_at=str(entry["modified_at"]),
                size=int(entry.get("size") or 0),
                content=content,
            )

        return snapshot


workspace_event_bus = WorkspaceEventBus()
workspace_observer = WorkspaceObserver()
