# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_file_manager.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 文件管理器。"""

import os
import shutil
from datetime import datetime
from pathlib import Path
from typing import Optional

from agent.schema.model_workspace import WorkspaceEvent
from agent.service.workspace.workspace_diff_builder import WorkspaceDiffBuilder
from agent.service.workspace.workspace_event_bus import workspace_event_bus
from agent.service.workspace.workspace_event_suppressor import workspace_event_suppressor
from agent.service.workspace.workspace_path_resolver import WorkspacePathResolver
from agent.service.workspace.workspace_templates import WORKSPACE_FILES
from agent.service.workspace.workspace_visibility_rules import is_hidden_workspace_path
from agent.service.workspace.workspace_write_event_publisher import WorkspaceWriteEventPublisher
from agent.utils.logger import logger


class WorkspaceFileManager:
    """负责 workspace 文件树读写与事件发布。"""

    MAX_LIVE_SNAPSHOT_BYTES = 128 * 1024

    def __init__(self, agent_id: str, workspace_path: Path):
        self._agent_id = agent_id
        self._workspace_path = workspace_path
        self._path_resolver = WorkspacePathResolver(workspace_path)
        self._event_publisher = WorkspaceWriteEventPublisher(agent_id, self._build_live_snapshot)

    @staticmethod
    def resolve_filename(name: str) -> Optional[str]:
        """解析逻辑名称到文件名。"""
        return WORKSPACE_FILES.get(name)

    def read_file(self, name: str) -> Optional[str]:
        """读取模板文件内容。"""
        filename = self.resolve_filename(name)
        if not filename:
            return None
        filepath = self._workspace_path / filename
        if not filepath.exists():
            return None
        return filepath.read_text(encoding="utf-8").strip()

    def write_file(self, name: str, content: str) -> bool:
        """写入模板文件内容。"""
        filename = self.resolve_filename(name)
        if not filename:
            logger.warning(f"⚠️ 未知的 Workspace 文件: {name}")
            return False
        filepath = self._workspace_path / filename
        filepath.write_text(content, encoding="utf-8")
        logger.info(f"📝 写入 Workspace: {filepath.name}")
        return True

    def list_files(self) -> list[dict]:
        """列出 workspace 可见文件树，并跳过运行时隐藏目录。"""
        workspace_root = self._workspace_path.resolve()
        entries: list[dict] = []

        def _walk(dir_path: Path, parts: tuple) -> None:
            try:
                with os.scandir(dir_path) as it:
                    for entry in it:
                        name = entry.name
                        if is_hidden_workspace_path(name):
                            continue
                        rel_parts = parts + (name,)
                        rel_posix = "/".join(rel_parts)
                        try:
                            st = entry.stat(follow_symlinks=False)
                        except OSError:
                            continue
                        is_dir = entry.is_dir(follow_symlinks=False)
                        entries.append(
                            {
                                "path": rel_posix,
                                "name": name,
                                "is_dir": is_dir,
                                "size": None if is_dir else st.st_size,
                                "modified_at": datetime.fromtimestamp(st.st_mtime).isoformat(),
                                "depth": len(rel_parts),
                            }
                        )
                        if is_dir:
                            _walk(Path(entry.path), rel_parts)
            except PermissionError:
                pass

        _walk(workspace_root, ())
        entries.sort(key=lambda item: (item["is_dir"] is False, item["path"]))
        return entries

    def read_relative_file(self, relative_path: str) -> str:
        """读取 workspace 内的文本文件。"""
        target_path = self._path_resolver.resolve(relative_path)
        if target_path.is_dir():
            raise ValueError("不能直接读取目录")
        if not target_path.exists():
            raise FileNotFoundError(f"文件不存在: {relative_path}")
        try:
            return target_path.read_text(encoding="utf-8")
        except UnicodeDecodeError as exc:
            raise ValueError("该文件不是 UTF-8 文本，无法直接预览") from exc

    def write_relative_file(self, relative_path: str, content: str, source: str = "unknown") -> str:
        """写入 workspace 内的文本文件。"""
        target_path = self._path_resolver.resolve(relative_path)
        relative_path_str = target_path.relative_to(self._workspace_path.resolve()).as_posix()
        try:
            before_content = target_path.read_text(encoding="utf-8") if target_path.exists() else ""
        except UnicodeDecodeError as exc:
            raise ValueError("目标文件不是 UTF-8 文本，不能按文本方式覆盖") from exc

        self._event_publisher.publish_write_start(relative_path_str, before_content, source)
        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_text(content, encoding="utf-8")
        logger.info(f"📝 写入 Workspace 文件: {target_path}")
        self._event_publisher.publish_write_end(
            relative_path_str,
            before_content,
            content,
            source,
            WorkspaceDiffBuilder.build(before_content, content),
        )
        return relative_path_str

    def write_binary_file(self, relative_path: str, content: bytes) -> str:
        """写入 workspace 内的二进制文件。"""
        target_path = self._path_resolver.resolve(relative_path)
        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_bytes(content)
        saved_path = target_path.relative_to(self._workspace_path.resolve()).as_posix()
        workspace_event_suppressor.register_write(self._agent_id, saved_path, None)
        logger.info(f"📦 写入 Workspace 二进制文件: {target_path}")
        return saved_path

    def stream_relative_file(
            self,
            relative_path: str,
            chunks: list[str],
            source: str = "agent",
            session_key: Optional[str] = None,
            tool_use_id: Optional[str] = None,
    ) -> str:
        """按 chunk 流式写入文件，并连续发布 delta 事件。"""
        target_path = self._path_resolver.resolve(relative_path)
        relative_path_str = target_path.relative_to(self._workspace_path.resolve()).as_posix()
        try:
            before_content = target_path.read_text(encoding="utf-8") if target_path.exists() else ""
        except UnicodeDecodeError as exc:
            raise ValueError("目标文件不是 UTF-8 文本，不能按文本流式写入") from exc
        accumulated = ""

        self._event_publisher.publish_write_start(
            relative_path_str,
            before_content,
            source,
            session_key=session_key,
            tool_use_id=tool_use_id,
        )

        for version, chunk in enumerate(chunks, start=1):
            accumulated += chunk
            workspace_event_bus.publish(
                WorkspaceEvent(
                    type="file_write_delta",
                    agent_id=self._agent_id,
                    path=relative_path_str,
                    version=version,
                    source=source,
                    session_key=session_key,
                    tool_use_id=tool_use_id,
                    appended_text=chunk,
                )
            )

        target_path.parent.mkdir(parents=True, exist_ok=True)
        target_path.write_text(accumulated, encoding="utf-8")
        logger.info(f"📝 流式写入 Workspace 文件: {target_path}")
        self._event_publisher.publish_write_end(
            relative_path_str,
            before_content,
            accumulated,
            source,
            WorkspaceDiffBuilder.build(before_content, accumulated),
            session_key=session_key,
            tool_use_id=tool_use_id,
            version=max(len(chunks), 1),
        )
        return relative_path_str

    def create_entry(self, relative_path: str, entry_type: str, content: str = "") -> str:
        """创建 workspace 内的文件或目录。"""
        target_path = self._path_resolver.resolve(relative_path)
        if target_path.exists():
            raise FileExistsError(f"目标已存在: {relative_path}")

        if entry_type == "directory":
            target_path.mkdir(parents=True, exist_ok=False)
        elif entry_type == "file":
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_text(content, encoding="utf-8")
        else:
            raise ValueError("仅支持创建 file 或 directory")

        logger.info(f"🆕 创建 Workspace 条目: {target_path}")
        return target_path.relative_to(self._workspace_path.resolve()).as_posix()

    def delete_entry(self, relative_path: str) -> str:
        """删除 workspace 内的文件或目录。"""
        target_path = self._path_resolver.resolve(relative_path)
        if not target_path.exists():
            raise FileNotFoundError(f"目标不存在: {relative_path}")

        if target_path.is_dir():
            shutil.rmtree(target_path)
        else:
            target_path.unlink()

        logger.info(f"🗑️ 删除 Workspace 条目: {target_path}")
        return relative_path

    def rename_entry(self, relative_path: str, new_relative_path: str) -> tuple[str, str]:
        """重命名或移动 workspace 内的文件或目录。"""
        source_path = self._path_resolver.resolve(relative_path)
        target_path = self._path_resolver.resolve(new_relative_path)

        if not source_path.exists():
            raise FileNotFoundError(f"目标不存在: {relative_path}")
        if source_path == target_path:
            raise ValueError("新旧路径不能相同")
        if target_path.exists():
            raise FileExistsError(f"目标已存在: {new_relative_path}")

        target_path.parent.mkdir(parents=True, exist_ok=True)
        source_path.rename(target_path)
        logger.info(f"✏️ 重命名 Workspace 条目: {source_path} -> {target_path}")
        return (
            source_path.relative_to(self._workspace_path.resolve()).as_posix(),
            target_path.relative_to(self._workspace_path.resolve()).as_posix(),
        )

    def save_memory_file(self, filename: str, content: str) -> None:
        """保存记忆文件到 memory 目录。"""
        memory_dir = self._workspace_path / "memory"
        memory_dir.mkdir(exist_ok=True)
        filepath = memory_dir / filename
        filepath.write_text(content, encoding="utf-8")
        logger.info(f"💾 保存记忆文件: {filepath}")

    def entry_exists(self, relative_path: str) -> bool:
        """判断 workspace 条目是否存在。"""
        return self._path_resolver.resolve(relative_path).exists()

    def get_existing_file_path(self, relative_path: str) -> Path:
        """获取可下载文件的绝对路径。"""
        target_path = self._path_resolver.resolve(relative_path)
        if target_path.is_dir():
            raise ValueError("不能直接下载目录")
        if not target_path.exists():
            raise FileNotFoundError(f"文件不存在: {relative_path}")
        return target_path

    @classmethod
    def _build_live_snapshot(cls, content: str) -> Optional[str]:
        """限制实时同步快照大小。"""
        return content if len(content.encode("utf-8")) <= cls.MAX_LIVE_SNAPSHOT_BYTES else None
