# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_workspace.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Agent workspace 聚合对象。"""

from pathlib import Path
from typing import Optional

from agent.service.workspace.workspace_file_manager import WorkspaceFileManager
from agent.service.workspace.workspace_template_initializer import WorkspaceTemplateInitializer


class AgentWorkspace:
    """聚合 workspace 初始化与文件操作。"""

    def __init__(self, agent_id: str, workspace_path: Path):
        self.agent_id = agent_id
        self.path = workspace_path
        self._initializer = WorkspaceTemplateInitializer(agent_id, workspace_path)
        self._file_manager = WorkspaceFileManager(agent_id, workspace_path)

    def ensure_exists(self) -> None:
        """确保目录存在。"""
        self._initializer.ensure_exists()

    def ensure_initialized(self, agent_name: str = "Agent") -> None:
        """确保模板初始化完成。"""
        self._initializer.ensure_initialized(agent_name)

    def read_file(self, name: str) -> Optional[str]:
        """读取模板文件。"""
        return self._file_manager.read_file(name)

    def write_file(self, name: str, content: str) -> bool:
        """写入模板文件。"""
        return self._file_manager.write_file(name, content)

    def list_files(self) -> list[dict]:
        """列出文件树。"""
        self.ensure_exists()
        return self._file_manager.list_files()

    def read_relative_file(self, relative_path: str) -> str:
        """读取相对文件。"""
        return self._file_manager.read_relative_file(relative_path)

    def write_relative_file(self, relative_path: str, content: str, source: str = "unknown") -> str:
        """写入相对文件。"""
        return self._file_manager.write_relative_file(relative_path, content, source=source)

    def write_binary_file(self, relative_path: str, content: bytes) -> str:
        """写入二进制文件。"""
        return self._file_manager.write_binary_file(relative_path, content)

    def stream_relative_file(
            self,
            relative_path: str,
            chunks: list[str],
            source: str = "agent",
            session_key: Optional[str] = None,
            tool_use_id: Optional[str] = None,
    ) -> str:
        """流式写入文件。"""
        return self._file_manager.stream_relative_file(
            relative_path=relative_path,
            chunks=chunks,
            source=source,
            session_key=session_key,
            tool_use_id=tool_use_id,
        )

    def create_entry(self, relative_path: str, entry_type: str, content: str = "") -> str:
        """创建文件或目录。"""
        return self._file_manager.create_entry(relative_path, entry_type, content)

    def delete_entry(self, relative_path: str) -> str:
        """删除文件或目录。"""
        return self._file_manager.delete_entry(relative_path)

    def rename_entry(self, relative_path: str, new_relative_path: str) -> tuple[str, str]:
        """重命名文件或目录。"""
        return self._file_manager.rename_entry(relative_path, new_relative_path)

    def entry_exists(self, relative_path: str) -> bool:
        """判断 workspace 条目是否存在。"""
        return self._file_manager.entry_exists(relative_path)

    def get_existing_file_path(self, relative_path: str) -> Path:
        """获取已存在文件的绝对路径。"""
        return self._file_manager.get_existing_file_path(relative_path)

    def save_memory_file(self, filename: str, content: str) -> None:
        """保存记忆文件。"""
        self._file_manager.save_memory_file(filename, content)
