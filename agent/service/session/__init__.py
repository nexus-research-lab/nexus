# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：__init__.py
# @Date   ：2026/2/25 23:10
# @Author ：leemysw
#
# 2026/2/25 23:10   Create
# =====================================================

"""
会话管理模块

[OUTPUT]: 对外提供 session_router 路由功能
[POS]: agent/service/session 的模块入口
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from agent.service.session.session_router import build_session_key, parse_session_key, resolve_session

__all__ = ["build_session_key", "parse_session_key", "resolve_session"]
