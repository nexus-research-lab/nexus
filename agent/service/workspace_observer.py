# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_observer.py
# @Date   ：2026/3/10
# @Author ：Codex
# =====================================================

"""
Workspace 轮询观察器

[INPUT]: 依赖 agent_manager 获取 workspace，依赖 workspace_event_bus 广播事件
[OUTPUT]: 对外提供 workspace_observer 单例
[POS]: service 层用于捕捉 SDK/模型直接写入 workspace 文件系统的变化
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import asyncio
from dataclasses import dataclass
from typing import Dict, Optional

from agent.service.agent_manager import agent_manager
from agent.service.agent.workspace import AgentWorkspace
from agent.service.schema.model_workspace_event import WorkspaceEvent
from agent.service.workspace_event_bus import workspace_event_bus
from agent.utils.logger import logger


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

    async def _poll_workspace(self, agent_id: str, workspace: AgentWorkspace) -> None:
        """对比新旧快照，推断文件写入中的开始/增量/结束。"""
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
                workspace_event_bus.publish(WorkspaceEvent(
                    type="file_write_start",
                    agent_id=agent_id,
                    path=path,
                    version=1,
                    source="agent",
                    content_snapshot=previous.content if previous else "",
                ))
            else:
                state = active_writes[path]
                state.version += 1
                state.current_content = current.content
                state.last_modified_at = current.modified_at
                state.last_change_at = now

            state = active_writes[path]
            workspace_event_bus.publish(WorkspaceEvent(
                type="file_write_delta",
                agent_id=agent_id,
                path=path,
                version=state.version,
                source="agent",
                content_snapshot=current.content,
            ))

        for path, state in list(active_writes.items()):
            current = current_snapshot.get(path)
            if not current:
                continue

            unchanged_for = now - state.last_change_at
            if unchanged_for < self._quiet_window_seconds:
                continue

            workspace_event_bus.publish(WorkspaceEvent(
                type="file_write_end",
                agent_id=agent_id,
                path=path,
                version=state.version,
                source="agent",
                content_snapshot=state.current_content,
                diff_stats=workspace._build_diff_stats(state.before_content, state.current_content),
            ))
            active_writes.pop(path, None)

        self._snapshots[agent_id] = current_snapshot

    async def _capture_snapshot(self, workspace: AgentWorkspace) -> Dict[str, ObservedFileSnapshot]:
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


workspace_observer = WorkspaceObserver()
