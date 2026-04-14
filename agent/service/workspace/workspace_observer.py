# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_observer.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# 2026/3/30         重构：使用 watchfiles 替代轮询
# =====================================================

"""Workspace 文件观察器（基于 watchfiles / OS 原生事件）。"""

import asyncio
import os
from typing import Dict, Optional, Set

from watchfiles import Change, awatch

from agent.schema.model_workspace import ActiveWriteState, WorkspaceEvent
from agent.service.workspace.workspace_diff_builder import WorkspaceDiffBuilder
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_event_suppressor import workspace_event_suppressor
from agent.service.workspace.workspace_visibility_rules import is_hidden_workspace_path
from agent.utils.logger import logger


class WorkspaceObserver:
    """基于 watchfiles（Rust notify）监听 workspace 文件变化，推断写入事件。

    使用操作系统原生事件（macOS FSEvents / Linux inotify），
    CPU 占用几乎为零，替代原有的 2 秒轮询方案。
    """

    def __init__(self):
        self._subscription_counts: Dict[str, int] = {}
        self._watch_tasks: Dict[str, asyncio.Task] = {}
        self._active_writes: Dict[str, Dict[str, ActiveWriteState]] = {}
        self._last_known_contents: Dict[str, Dict[str, Optional[str]]] = {}
        # 文件写入静默窗口：最后一次变化后等待此时间才认为写入结束
        self._quiet_window_seconds = 3.0
        # 读取文件内容的最大字节数限制
        self._max_snapshot_bytes = 128 * 1024
        # 空闲超时保护：长时间无文件变化自动停止（防止订阅泄漏）
        self._max_idle_seconds = 3600.0
        # 定期检查写入结束的间隔
        self._settle_check_interval = 2.0

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
            self._active_writes.pop(agent_id, None)
            logger.debug(f"🛑 停止观察 workspace: agent={agent_id}")
            return

        self._subscription_counts[agent_id] = count - 1

    async def _watch_agent(self, agent_id: str) -> None:
        """使用 watchfiles 监听 Agent workspace 的文件变化。"""
        from agent.service.agent.agent_manager import agent_manager

        try:
            workspace = await agent_manager.get_agent_workspace(agent_id)
            watch_path = str(workspace.path.resolve())
            self._active_writes.setdefault(agent_id, {})
            self._last_known_contents.setdefault(agent_id, {})

            logger.info(f"👀 watchfiles 开始监听: agent={agent_id}, path={watch_path}")

            # 启动定期检查写入结束的后台任务
            settle_task = asyncio.get_running_loop().create_task(
                self._settle_checker(agent_id, workspace)
            )

            try:
                last_event_time = asyncio.get_running_loop().time()

                async for changes in awatch(
                    watch_path,
                    # 去抖间隔：300ms 内的连续变化合并为一批
                    debounce=300,
                    # 使用 Rust 层过滤，减少 Python 回调次数
                    rust_timeout=5000,
                    yield_on_timeout=True,
                ):
                    # 检查订阅是否仍然有效
                    if self._subscription_counts.get(agent_id, 0) <= 0:
                        break

                    now = asyncio.get_running_loop().time()

                    if not changes:
                        # yield_on_timeout 触发的空变化集，用于空闲超时检测
                        if now - last_event_time > self._max_idle_seconds:
                            logger.warning(
                                f"⏰ workspace 观察空闲超时，自动停止: agent={agent_id}"
                            )
                            break
                        continue

                    last_event_time = now
                    await self._handle_changes(agent_id, workspace, watch_path, changes)

            finally:
                settle_task.cancel()
                try:
                    await settle_task
                except asyncio.CancelledError:
                    pass

        except asyncio.CancelledError:
            raise
        except Exception as exc:
            logger.warning(f"⚠️ workspace 观察失败: agent={agent_id}, error={exc}")
        finally:
            # 清理订阅状态，防止残留
            self._subscription_counts.pop(agent_id, None)
            self._watch_tasks.pop(agent_id, None)
            self._active_writes.pop(agent_id, None)
            self._last_known_contents.pop(agent_id, None)

    async def _handle_changes(
        self,
        agent_id: str,
        workspace,
        watch_path: str,
        changes: Set[tuple],
    ) -> None:
        """处理一批文件变化事件。"""
        active_writes = self._active_writes.setdefault(agent_id, {})
        last_known_contents = self._last_known_contents.setdefault(agent_id, {})
        now = asyncio.get_running_loop().time()

        for change_type, abs_path in changes:
            # 过滤隐藏目录
            if is_hidden_workspace_path(abs_path):
                continue

            # 计算相对路径
            try:
                rel_path = os.path.relpath(abs_path, watch_path)
            except ValueError:
                continue

            # 跳过目录变化
            if os.path.isdir(abs_path):
                continue

            # 文件删除：发布删除事件，并清理残留的 active write
            if change_type == Change.deleted:
                state = active_writes.pop(rel_path, None)
                last_known_contents.pop(rel_path, None)
                workspace_event_bus.publish(
                    WorkspaceEvent(
                        type="file_deleted",
                        agent_id=agent_id,
                        path=rel_path,
                        version=(state.version + 1) if state else 1,
                        source="agent",
                    )
                )
                continue

            # 只处理文件创建和修改事件
            if change_type not in (Change.added, Change.modified):
                continue

            # 获取文件大小
            try:
                file_size = os.path.getsize(abs_path)
            except OSError:
                continue

            # 读取文件内容（小文件）
            content = await self._read_file_content(workspace, rel_path, file_size)
            if workspace_event_suppressor.should_suppress_write(agent_id, rel_path, content):
                last_known_contents[rel_path] = content
                active_writes.pop(rel_path, None)
                continue

            if rel_path not in active_writes:
                # 新的写入开始
                active_writes[rel_path] = ActiveWriteState(
                    before_content=last_known_contents.get(rel_path),
                    current_content=content,
                    last_modified_at=str(now),
                    last_change_at=now,
                    version=1,
                )
                workspace_event_bus.publish(
                    WorkspaceEvent(
                        type="file_write_start",
                        agent_id=agent_id,
                        path=rel_path,
                        version=1,
                        source="agent",
                    )
                )
            else:
                # 更新已有的写入状态
                state = active_writes[rel_path]
                state.version += 1
                state.current_content = content
                state.last_modified_at = str(now)
                state.last_change_at = now

            # 发布 delta 事件
            state = active_writes[rel_path]
            workspace_event_bus.publish(
                WorkspaceEvent(
                    type="file_write_delta",
                    agent_id=agent_id,
                    path=rel_path,
                    version=state.version,
                    source="agent",
                    content_snapshot=state.current_content,
                )
            )

    async def _settle_checker(self, agent_id: str, workspace) -> None:
        """定期检查 active_writes 中是否有已静默的文件，发布 write_end 事件。"""
        try:
            while self._subscription_counts.get(agent_id, 0) > 0:
                await asyncio.sleep(self._settle_check_interval)
                active_writes = self._active_writes.get(agent_id)
                last_known_contents = self._last_known_contents.setdefault(agent_id, {})
                if not active_writes:
                    continue

                now = asyncio.get_running_loop().time()
                settled_paths = []

                for path, state in active_writes.items():
                    if now - state.last_change_at < self._quiet_window_seconds:
                        continue
                    settled_paths.append(path)

                for path in settled_paths:
                    state = active_writes.pop(path, None)
                    if state is None:
                        continue

                    # 在线程池中执行 diff 计算
                    diff_stats = None
                    if state.before_content is not None and state.current_content is not None:
                        diff_stats = await self._compute_diff_stats(
                            state.before_content, state.current_content
                        )

                    workspace_event_bus.publish(
                        WorkspaceEvent(
                            type="file_write_end",
                            agent_id=agent_id,
                            path=path,
                            version=state.version,
                            source="agent",
                            content_snapshot=state.current_content,
                            diff_stats=diff_stats,
                        )
                    )
                    last_known_contents[path] = state.current_content
        except asyncio.CancelledError:
            raise

    async def _read_file_content(self, workspace, path: str, size: int) -> Optional[str]:
        """读取小文件内容，大文件跳过。在线程池中执行 I/O。"""
        if size > self._max_snapshot_bytes:
            return None

        loop = asyncio.get_running_loop()
        try:
            return await loop.run_in_executor(None, workspace.read_relative_file, path)
        except (UnicodeDecodeError, ValueError, FileNotFoundError):
            return None

    @staticmethod
    async def _compute_diff_stats(before_content: str, after_content: str):
        """在线程池中执行 diff 计算，避免阻塞事件循环。"""
        loop = asyncio.get_running_loop()
        return await loop.run_in_executor(None, WorkspaceDiffBuilder.build, before_content, after_content)


workspace_observer = WorkspaceObserver()
