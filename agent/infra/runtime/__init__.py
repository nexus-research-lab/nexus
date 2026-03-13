# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/3/12 20:06
# @Author ：leemysw
# 2026/3/12 20:06   Create
# =====================================================

"""
Agent Runtime 基础设施入口。

[OUTPUT]: 对外提供权限运行时与 workspace 能力
[POS]: infra/runtime 的轻量聚合入口，避免包初始化时引入额外循环依赖
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.infra.runtime.permission_runtime import (
    PendingPermissionRequest,
    PermissionRequestPresenter,
    PermissionUpdateCodec,
)
from agent.infra.workspace.initializer import AgentWorkspace, get_workspace_base_path

__all__ = [
    "AgentWorkspace",
    "PendingPermissionRequest",
    "PermissionRequestPresenter",
    "PermissionUpdateCodec",
    "get_workspace_base_path",
]
