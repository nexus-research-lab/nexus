# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：storage_bootstrap.py
# @Date   ：2026/3/12 20:39
# @Author ：leemysw
# 2026/3/12 20:39   Create
# =====================================================

"""文件存储初始化器。"""

from threading import Lock
from typing import Any, Dict, List

from agent.config.config import settings
from agent.service.workspace.workspace_template_initializer import WorkspaceTemplateInitializer
from agent.infra.file_store.storage_paths import FileStoragePaths
from agent.utils.logger import logger


class FileStorageBootstrap:
    """文件存储初始化器。"""

    _lock = Lock()
    _initialized = False

    def __init__(self) -> None:
        self.paths = FileStoragePaths()

    def ensure_ready(self) -> None:
        """确保文件存储已初始化。"""
        with self._lock:
            if self.__class__._initialized:
                return

            self.paths.ensure_directories()
            self._ensure_main_agent()

            self.__class__._initialized = True

    def _ensure_main_agent(self) -> None:
        """确保 main agent 的工作区模板存在。"""
        workspace_path = self.paths.workspace_base / settings.DEFAULT_AGENT_ID
        workspace_path.mkdir(parents=True, exist_ok=True)
        WorkspaceTemplateInitializer(
            settings.DEFAULT_AGENT_ID,
            workspace_path,
        ).ensure_initialized(settings.DEFAULT_AGENT_ID)
        logger.info(f"🧩 已初始化 main Agent 工作区: {workspace_path}")

    @staticmethod
    def compact_messages(message_rows: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """按 message_id 压缩消息，保留最后一条快照。"""
        latest_by_id: Dict[str, Dict[str, Any]] = {}
        order: List[str] = []

        for row in message_rows:
            message_id = str(row.get("message_id", "")).strip()
            if not message_id:
                continue
            if message_id not in latest_by_id:
                order.append(message_id)
            latest_by_id[message_id] = row

        compacted = [latest_by_id[message_id] for message_id in order]
        compacted.sort(key=lambda item: str(item.get("timestamp") or ""))
        return compacted
