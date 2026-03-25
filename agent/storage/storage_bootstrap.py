# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：storage_bootstrap.py
# @Date   ：2026/3/12 20:39
# @Author ：leemysw
# 2026/3/12 20:39   Create
# =====================================================

"""
文件存储初始化器。

[INPUT]: 依赖文件路径规则与 JSON 文件读写工具
[OUTPUT]: 对外提供存储初始化与默认 Agent 引导能力
[POS]: storage 的引导层，在 repository 启动时确保基础存储可用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from threading import Lock
from typing import Any, Dict, List

from agent.config.config import settings
from agent.service.agent.main_agent_profile import MainAgentProfile
from agent.service.workspace.workspace_template_initializer import WorkspaceTemplateInitializer
from agent.storage.config_store import ConfigStore
from agent.storage.storage_paths import FileStoragePaths
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
        """确保 main agent 与其工作区模板存在。"""
        workspace_path = self.paths.workspace_base / settings.DEFAULT_AGENT_ID
        record = MainAgentProfile.build_storage_record(workspace_path)
        record["created_at"] = datetime.now().isoformat()
        records = ConfigStore.read(self.paths.agents_index_path, [])
        if not isinstance(records, list):
            records = []

        main_index = next(
            (
                index
                for index, item in enumerate(records)
                if item.get("agent_id") == MainAgentProfile.AGENT_ID
            ),
            None,
        )
        if main_index is None:
            records.insert(0, record)
        else:
            existing_record = records[main_index]
            existing_record["agent_id"] = MainAgentProfile.AGENT_ID
            existing_record["name"] = MainAgentProfile.AGENT_ID
            existing_record["workspace_path"] = str(workspace_path)
            existing_record["status"] = "active"
            if not existing_record.get("created_at"):
                existing_record["created_at"] = record["created_at"]
            existing_record["options"] = MainAgentProfile.merge_options(
                existing_record.get("options"),
            )
            record = existing_record

        workspace_path.mkdir(parents=True, exist_ok=True)
        ConfigStore.write(self.paths.agents_index_path, records)
        ConfigStore.write(self.paths.get_agent_file_path(workspace_path), record)
        WorkspaceTemplateInitializer(
            MainAgentProfile.AGENT_ID,
            workspace_path,
        ).ensure_initialized(MainAgentProfile.AGENT_ID)
        logger.info(f"🧩 已初始化 main Agent 存储: {workspace_path}")

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
