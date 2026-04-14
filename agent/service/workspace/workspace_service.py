# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_service.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 应用服务。"""

from datetime import datetime
from typing import Tuple

from fastapi import UploadFile

from agent.schema.model_agent import UploadWorkspaceFileResponse
from agent.service.agent.agent_manager import agent_manager
from agent.service.session.session_manager import session_manager


# 支持的文件类型扩展名
TEXT_EXTENSIONS = {
    "txt", "md", "markdown", "json", "jsonl", "yaml", "yml", "toml", "xml",
    "csv", "ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "java", "go", "rs",
    "rb", "php", "sh", "bash", "zsh", "sql", "html", "css", "scss", "less",
    "log", "ini", "conf", "env", "dockerfile", "makefile", "cmake", "gradle",
    "proto", "graphql", "svg", "rst", "adoc"
}

# 文件大小限制（字节）
MAX_FILE_SIZE = 20 * 1024 * 1024  # 20MB


class WorkspaceService:
    """负责 workspace 读写相关用例。"""

    @staticmethod
    def _normalize_uploaded_name(filename: str | None) -> str:
        """规整上传文件名，避免客户端注入路径片段。"""
        raw_name = (filename or "uploaded_file").replace("\\", "/").split("/")[-1].strip()
        return raw_name or "uploaded_file"

    @staticmethod
    def _split_filename(filename: str) -> tuple[str, str]:
        """拆分主名与扩展名。"""
        if "." not in filename:
            return filename, ""
        return filename.rsplit(".", 1)[0], filename.rsplit(".", 1)[1]

    @classmethod
    def _build_unique_upload_path(cls, workspace, relative_path: str) -> str:
        """为上传文件生成唯一保存路径，避免覆盖已有文件。"""
        normalized_path = relative_path.strip().strip("/")
        if not workspace.entry_exists(normalized_path):
            return normalized_path

        parent_dir, _, file_name = normalized_path.rpartition("/")
        name_without_ext, ext = cls._split_filename(file_name)
        timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        unique_name = f"{name_without_ext}-{timestamp}.{ext}" if ext else f"{name_without_ext}-{timestamp}"
        return f"{parent_dir}/{unique_name}" if parent_dir else unique_name

    @staticmethod
    async def get_workspace_files(agent_id: str) -> list[dict]:
        """获取 workspace 文件列表。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        return workspace.list_files()

    @staticmethod
    async def get_workspace_file(agent_id: str, path: str) -> str:
        """读取 workspace 文件。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        return workspace.read_relative_file(path)

    @staticmethod
    async def update_workspace_file(agent_id: str, path: str, content: str) -> str:
        """更新 workspace 文件并刷新活跃会话。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        saved_path = workspace.write_relative_file(path, content, source="api")
        await session_manager.refresh_agent_sessions(agent_id)
        return saved_path

    @staticmethod
    async def create_workspace_entry(
            agent_id: str,
            path: str,
            entry_type: str,
            content: str = "",
    ) -> str:
        """创建 workspace 条目。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        created_path = workspace.create_entry(path, entry_type, content)
        await session_manager.refresh_agent_sessions(agent_id)
        return created_path

    @staticmethod
    async def rename_workspace_entry(agent_id: str, path: str, new_path: str) -> Tuple[str, str]:
        """重命名 workspace 条目。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        renamed_paths = workspace.rename_entry(path, new_path)
        await session_manager.refresh_agent_sessions(agent_id)
        return renamed_paths

    @staticmethod
    async def delete_workspace_entry(agent_id: str, path: str) -> str:
        """删除 workspace 条目。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        deleted_path = workspace.delete_entry(path)
        await session_manager.refresh_agent_sessions(agent_id)
        return deleted_path

    @classmethod
    async def upload_file(
            cls,
            agent_id: str,
            file: UploadFile,
            path: str | None = None,
    ) -> UploadWorkspaceFileResponse:
        """上传文件到 workspace。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        original_name = cls._normalize_uploaded_name(file.filename)
        _, ext = cls._split_filename(original_name)
        ext = ext.lower()

        try:
            content = await file.read()
            file_size = len(content)
            if file_size > MAX_FILE_SIZE:
                raise ValueError(f"文件大小超过限制 ({MAX_FILE_SIZE / 1024 / 1024:.0f}MB)")

            if path:
                target_path = f"{path}{original_name}" if path.endswith("/") else path
            else:
                target_path = original_name

            saved_relative_path = cls._build_unique_upload_path(workspace, target_path)

            if ext in TEXT_EXTENSIONS:
                try:
                    decoded_content = content.decode("utf-8")
                    saved_path = workspace.write_relative_file(saved_relative_path, decoded_content, source="api")
                except UnicodeDecodeError:
                    saved_path = workspace.write_binary_file(saved_relative_path, content)
            else:
                saved_path = workspace.write_binary_file(saved_relative_path, content)

            await session_manager.refresh_agent_sessions(agent_id)

            return UploadWorkspaceFileResponse(
                path=saved_path,
                name=saved_path.rsplit("/", 1)[-1],
                size=file_size,
            )
        finally:
            await file.close()

    @staticmethod
    async def get_file_for_download(agent_id: str, path: str) -> Tuple[str, str]:
        """获取 workspace 文件用于下载。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        file_path = workspace.get_existing_file_path(path)
        return str(file_path), path.rsplit("/", 1)[-1]


workspace_service = WorkspaceService()
