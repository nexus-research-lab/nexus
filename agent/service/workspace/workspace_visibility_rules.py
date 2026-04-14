# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_visibility_rules.py
# @Date   ：2026/04/14 21:13
# @Author ：leemysw
# 2026/04/14 21:13   Create
# =====================================================

"""Workspace 可见性规则。"""

from pathlib import Path


HIDDEN_WORKSPACE_PARTS = frozenset({".agents", ".git", "__pycache__", ".claude", ".DS_Store"})


def is_hidden_workspace_path(path: str | Path) -> bool:
    """判断路径是否属于 workspace 隐藏区域。"""
    parts = Path(path).parts if not isinstance(path, Path) else path.parts
    return any(part in HIDDEN_WORKSPACE_PARTS or part.startswith(".DS_") for part in parts)
