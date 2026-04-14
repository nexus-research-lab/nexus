# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_path_resolver.py
# @Date   ：2026/04/14 20:47
# @Author ：leemysw
# 2026/04/14 20:47   Create
# =====================================================

"""Workspace 路径解析器。"""

from pathlib import Path


class WorkspacePathResolver:
    """负责 workspace 相对路径解析与边界校验。"""

    def __init__(self, workspace_path: Path):
        self._workspace_path = workspace_path.resolve()

    def resolve(self, relative_path: str) -> Path:
        """解析并校验相对路径，禁止逃逸出 workspace。"""
        normalized = (relative_path or "").strip().lstrip("/").replace("\\", "/")
        if not normalized:
            raise ValueError("文件路径不能为空")
        if normalized == ".agents" or normalized.startswith(".agents/"):
            raise ValueError("不能直接操作内部技能目录")

        target_path = (self._workspace_path / normalized).resolve()
        if not target_path.is_relative_to(self._workspace_path):
            raise ValueError("文件路径超出 workspace 范围")
        return target_path
