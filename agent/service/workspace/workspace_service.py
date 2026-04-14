# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：workspace_service.py
# @Date   ：2026/3/17 20:15
# @Author ：leemysw
# 2026/3/17 20:15   Create
# =====================================================

"""Workspace 应用服务。"""

import os
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

    @staticmethod
    async def upload_file(
        agent_id: str,
        file: UploadFile,
        path: str | None = None,
    ) -> UploadWorkspaceFileResponse:
        """上传文件到 workspace。"""
        # 获取 workspace
        workspace = await agent_manager.get_agent_workspace(agent_id)

        # 获取文件名和扩展名
        original_name = file.filename or "uploaded_file"
        ext = original_name.rsplit(".", 1)[-1].lower() if "." in original_name else ""

        # 检查文件大小
        content = await file.read()
        file_size = len(content)

        if file_size > MAX_FILE_SIZE:
            raise ValueError(f"文件大小超过限制 ({MAX_FILE_SIZE / 1024 / 1024:.0f}MB)")

        # 确定保存路径（相对 workspace 根目录）
        if path:
            # 用户指定了路径
            if path.endswith("/"):
                # 目录路径
                target_path = f"{path}{original_name}"
            else:
                # 完整路径
                target_path = path
        else:
            # 默认直接保存到 workspace 根目录
            # 如果文件名已存在，添加时间戳后缀
            existing_files = workspace.list_files()
            existing_names = {f.get("name") for f in existing_files if not f.get("is_dir")}
            if original_name in existing_names:
                from datetime import datetime
                name_without_ext = original_name.rsplit(".", 1)[0] if "." in original_name else original_name
                ext = original_name.rsplit(".", 1)[-1] if "." in original_name else ""
                timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
                original_name = f"{name_without_ext}-{timestamp}.{ext}" if ext else f"{name_without_ext}-{timestamp}"
            target_path = original_name

        # 确保目录存在
        full_path = workspace._abs_path(target_path)
        os.makedirs(os.path.dirname(full_path), exist_ok=True)

        # 写入文件
        # 文本文件使用文本模式写入，其他文件使用二进制模式
        if ext in TEXT_EXTENSIONS:
            try:
                decoded_content = content.decode("utf-8")
                saved_path = workspace.write_relative_file(target_path, decoded_content, source="api")
            except UnicodeDecodeError:
                # 解码失败，回退到二进制模式
                with open(full_path, "wb") as f:
                    f.write(content)
                saved_path = target_path
        else:
            with open(full_path, "wb") as f:
                f.write(content)
            saved_path = target_path

        # 刷新会话
        await session_manager.refresh_agent_sessions(agent_id)

        return UploadWorkspaceFileResponse(
            path=saved_path,
            name=original_name,
            size=file_size,
        )

    @staticmethod
    async def get_file_for_download(agent_id: str, path: str) -> Tuple[str, str]:
        """获取 workspace 文件用于下载。"""
        workspace = await agent_manager.get_agent_workspace(agent_id)
        file_path = workspace._abs_path(path)
        if not file_path.exists():
            raise FileNotFoundError(f"文件不存在: {path}")
        return file_path, path.rsplit("/", 1)[-1]


workspace_service = WorkspaceService()
