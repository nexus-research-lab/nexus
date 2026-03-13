# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_event_bus.py
# @Date   ：2026/3/13 14:20
# @Author ：leemysw
# 2026/3/13 14:20   Create
# =====================================================

"""兼容导出新的 workspace 事件总线。"""

from agent.infra.workspace.monitor import WorkspaceEventBus, workspace_event_bus

__all__ = ["WorkspaceEventBus", "workspace_event_bus"]
