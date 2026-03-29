# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_observer.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 轮询观察器。"""

import asyncio
from typing import Dict, Optional

from agent.schema.model_workspace import ActiveWriteState, ObservedFileSnapshot, WorkspaceEvent
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_file_manager import WorkspaceFileManager
from agent.utils.logger import logger


class WorkspaceObserver:
    """按 Agent 轮询 workspace 文件变化，并推断写入事件。"""

    def __init__(self):
        self._subscription_counts: Dict[str, int] = {}
        self._watch_tasks: Dict[str, asyncio.Task] = {}
        self._snapshots: Dict[str, Dict[str, ObservedFileSnapshot]] = {}
        self._active_writes: Dict[str, Dict[str, ActiveWriteState]] = {}
        self._poll_interval_seconds = 2.0
        self._quiet_window_seconds = 2.5
        self._max_snapshot_bytes = 128 * 1024

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

            current_content = await self._read_snapshot_content(workspace, path, current.size)
            if path not in active_writes:
                active_writes[path] = ActiveWriteState(
                    before_content=None,
                    current_content=current_content,
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
                    )
                )
            else:
                state = active_writes[path]
                state.version += 1
                state.current_content = current_content
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
                    content_snapshot=state.current_content,
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
                    diff_stats=WorkspaceFileManager.build_diff_stats(state.before_content, state.current_content)
                    if state.before_content is not None and state.current_content is not None
                    else None,
                )
            )
            active_writes.pop(path, None)

        self._snapshots[agent_id] = current_snapshot

    async def _capture_snapshot(self, workspace) -> Dict[str, ObservedFileSnapshot]:
        """抓取当前 workspace 可见文件的元数据快照。

        list_files() 是同步 I/O（os.scandir），用 run_in_executor 卸载到线程池，
        避免阻塞 asyncio 事件循环。
        """
        loop = asyncio.get_running_loop()
        file_entries = await loop.run_in_executor(None, workspace.list_files)
        snapshot: Dict[str, ObservedFileSnapshot] = {}
        for entry in file_entries:
            if entry["is_dir"]:
                continue
            path = str(entry["path"])
            snapshot[path] = ObservedFileSnapshot(
                modified_at=str(entry["modified_at"]),
                size=int(entry.get("size") or 0),
            )
        return snapshot

    async def _read_snapshot_content(self, workspace, path: str, size: int) -> Optional[str]:
        """仅读取发生变化的小文件内容，避免轮询时全量读取所有文档。

        read_relative_file 是同步 I/O，用 run_in_executor 卸载到线程池。
        """
        if size > self._max_snapshot_bytes:
            return None

        loop = asyncio.get_running_loop()
        try:
            return await loop.run_in_executor(None, workspace.read_relative_file, path)
        except (UnicodeDecodeError, ValueError, FileNotFoundError):
            return None


workspace_observer = WorkspaceObserver()
