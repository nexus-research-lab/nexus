# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/3/12 20:06
# @Author ：leemysw
# 2026/3/12 20:06   Create
# =====================================================

"""
Agent Runtime 基础设施模块。

[OUTPUT]: 对外提供 Workspace、事件总线、观察器与权限运行时组件
[POS]: infra 层的 agent runtime 聚合入口，供 service/app 层装配使用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.infra.runtime.permission_runtime import (
    PendingPermissionRequest,
    PermissionRequestPresenter,
    PermissionUpdateCodec,
)
from agent.infra.runtime.workspace import AgentWorkspace, get_workspace_base_path
from agent.infra.runtime.workspace_event_bus import WorkspaceEventBus, workspace_event_bus
from agent.infra.runtime.workspace_observer import WorkspaceObserver, workspace_observer

__all__ = [
    "AgentWorkspace",
    "PendingPermissionRequest",
    "PermissionRequestPresenter",
    "PermissionUpdateCodec",
    "WorkspaceEventBus",
    "WorkspaceObserver",
    "get_workspace_base_path",
    "workspace_event_bus",
    "workspace_observer",
]
