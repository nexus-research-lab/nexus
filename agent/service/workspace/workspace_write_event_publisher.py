# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_write_event_publisher.py
# @Date   ：2026/3/17 20:20
# @Author ：leemysw
# 2026/3/17 20:20   Create
# =====================================================

"""Workspace 写入事件发布器。"""

from typing import Optional

from agent.schema.model_workspace import WorkspaceEvent
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_event_suppressor import workspace_event_suppressor


class WorkspaceWriteEventPublisher:
    """负责发布 workspace 文件写入事件。"""

    def __init__(self, agent_id: str, snapshot_builder):
        self._agent_id = agent_id
        self._snapshot_builder = snapshot_builder

    def publish_write_start(
            self,
            relative_path: str,
            before_content: str,
            source: str,
            session_key: Optional[str] = None,
            tool_use_id: Optional[str] = None,
    ) -> None:
        """发布写入开始事件。"""
        workspace_event_bus.publish(
            WorkspaceEvent(
                type="file_write_start",
                agent_id=self._agent_id,
                path=relative_path,
                version=1,
                source=source,
                session_key=session_key,
                tool_use_id=tool_use_id,
                content_snapshot=self._snapshot_builder(before_content),
            )
        )

    def publish_write_end(
            self,
            relative_path: str,
            before_content: str,
            after_content: str,
            source: str,
            diff_stats,
            session_key: Optional[str] = None,
            tool_use_id: Optional[str] = None,
            version: int = 1,
    ) -> None:
        """发布写入完成事件。"""
        content_snapshot = self._snapshot_builder(after_content)
        workspace_event_suppressor.register_write(
            self._agent_id,
            relative_path,
            content_snapshot,
        )
        workspace_event_bus.publish(
            WorkspaceEvent(
                type="file_write_end",
                agent_id=self._agent_id,
                path=relative_path,
                version=version,
                source=source,
                session_key=session_key,
                tool_use_id=tool_use_id,
                content_snapshot=content_snapshot,
                diff_stats=diff_stats,
            )
        )
