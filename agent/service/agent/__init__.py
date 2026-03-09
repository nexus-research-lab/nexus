# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/2/25 23:15
# @Author ：leemysw
#
# 2026/2/25 23:15   Create
# =====================================================

"""
智能体模块

[OUTPUT]: 对外提供 AgentWorkspace, get_workspace_base_path
[POS]: agent/service/agent 的模块入口
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.service.agent.workspace import AgentWorkspace, get_workspace_base_path

__all__ = ["AgentWorkspace", "get_workspace_base_path"]
