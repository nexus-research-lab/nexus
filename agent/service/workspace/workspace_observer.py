# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_observer.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""兼容导出新的 workspace 观察器。"""

from agent.infra.workspace.monitor import (
    ActiveWriteState,
    ObservedFileSnapshot,
    WorkspaceObserver,
    workspace_observer,
)

__all__ = [
    "ActiveWriteState",
    "ObservedFileSnapshot",
    "WorkspaceObserver",
    "workspace_observer",
]
