# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_workspace_event.py
# @Date   ：2026/3/10
# @Author ：Codex
# =====================================================

"""
Workspace 实时事件模型

[INPUT]: 无
[OUTPUT]: 对外提供 WorkspaceEvent / WorkspaceDiffStats
[POS]: schema 层的 workspace 事件协议，供后端广播与前端消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from typing import Literal, Optional

from pydantic import BaseModel, Field


class WorkspaceDiffStats(BaseModel):
    """文件变更摘要。"""

    additions: int = Field(default=0, description="新增行数")
    deletions: int = Field(default=0, description="删除行数")
    changed_lines: int = Field(default=0, description="总改动行数")


class WorkspaceEvent(BaseModel):
    """Workspace 文件实时事件。"""

    type: Literal["file_write_start", "file_write_delta", "file_write_end"] = Field(..., description="事件类型")
    agent_id: str = Field(..., description="Agent 实体 ID")
    path: str = Field(..., description="相对 workspace 的文件路径")
    version: int = Field(default=1, description="文件写入版本号")
    source: Literal["agent", "api", "system", "unknown"] = Field(default="unknown", description="变更来源")
    session_key: Optional[str] = Field(default=None, description="触发本次写入的 session_key")
    tool_use_id: Optional[str] = Field(default=None, description="触发本次写入的工具调用 ID")
    content_snapshot: Optional[str] = Field(default=None, description="当前文件完整快照")
    appended_text: Optional[str] = Field(default=None, description="本次 delta 追加的文本")
    diff_stats: Optional[WorkspaceDiffStats] = Field(default=None, description="写入完成后的 diff 摘要")
    timestamp: datetime = Field(default_factory=datetime.now, description="事件时间")
