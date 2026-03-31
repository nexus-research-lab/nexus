# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：storage_paths.py
# @Date   ：2026/3/12 20:39
# @Author ：leemysw
# 2026/3/12 20:39   Create
# =====================================================

"""
文件存储路径规则。

[INPUT]: 依赖 workspace 基础路径和本地文件系统
[OUTPUT]: 对外提供文件存储相关的路径解析能力
[POS]: storage 的路径规则层，被 repository/bootstrap 复用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import base64
from pathlib import Path

from agent.service.workspace.workspace_paths import get_workspace_base_path


class FileStoragePaths:
    """统一管理文件存储路径。"""

    INTERNAL_DIR_NAME = ".agents"

    def __init__(self) -> None:
        self.home_root = Path.home() / ".nexus"
        self.workspace_base = get_workspace_base_path()

    def ensure_directories(self) -> None:
        """确保基础目录存在。"""
        self.home_root.mkdir(parents=True, exist_ok=True)
        self.workspace_base.mkdir(parents=True, exist_ok=True)

    def get_runtime_dir(self, workspace_path: str | Path) -> Path:
        """返回 workspace 内部运行目录。"""
        runtime_dir = Path(workspace_path).expanduser() / self.INTERNAL_DIR_NAME
        runtime_dir.mkdir(parents=True, exist_ok=True)
        return runtime_dir

    @staticmethod
    def build_session_dir_name(session_key: str) -> str:
        """将 session_key 编码为安全目录名。"""
        encoded = base64.urlsafe_b64encode(session_key.encode("utf-8")).decode("ascii")
        return encoded.rstrip("=")

    def get_session_dir(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话目录。"""
        return self.get_runtime_dir(workspace_path) / "sessions" / self.build_session_dir_name(session_key)

    def get_session_meta_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话元数据路径。"""
        return self.get_session_dir(workspace_path, session_key) / "meta.json"

    def get_session_message_log_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话消息日志路径。"""
        return self.get_session_dir(workspace_path, session_key) / "messages.jsonl"

    def get_session_cost_log_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话成本账本路径。"""
        return self.get_session_dir(workspace_path, session_key) / "telemetry_cost.jsonl"

    def get_agent_cost_summary_path(self, workspace_path: str | Path) -> Path:
        """返回 Agent 成本汇总路径。"""
        return self.get_runtime_dir(workspace_path) / "telemetry_cost_summary.json"
