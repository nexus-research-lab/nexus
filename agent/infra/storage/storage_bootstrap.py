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
[POS]: infra/storage 的引导层，在 repository 启动时确保基础存储可用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from threading import Lock
from typing import Any, Dict, List

from agent.infra.storage.config_store import ConfigStore
from agent.infra.storage.storage_paths import FileStoragePaths
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

            if not self.paths.agents_index_path.exists():
                self._bootstrap_default_agent()

            self.__class__._initialized = True

    def _bootstrap_default_agent(self) -> None:
        """在全新环境下创建默认 Agent。"""
        workspace_path = self.paths.workspace_base / "main"
        record = {
            "agent_id": "main",
            "name": "main",
            "workspace_path": str(workspace_path),
            "options": {},
            "created_at": datetime.now().isoformat(),
            "status": "active",
        }
        workspace_path.mkdir(parents=True, exist_ok=True)
        ConfigStore.write(self.paths.agents_index_path, [record])
        ConfigStore.write(self.paths.get_agent_file_path(workspace_path), record)
        logger.info(f"🧩 已初始化默认 Agent 存储: {workspace_path}")

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
