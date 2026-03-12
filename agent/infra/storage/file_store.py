# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：file_store.py
# @Date   ：2026/3/9 22:25
# @Author ：leemysw
# 2026/3/9 22:25   Create
# =====================================================

"""
文件存储基础设施

[INPUT]: 依赖 workspace 基础路径和本地文件系统
[OUTPUT]: 对外提供路径解析、JSON/JSONL 读写、旧 SQLite 迁移能力
[POS]: storage 模块的底层基础设施，被 Agent/Session Repository 复用
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import base64
import json
import sqlite3
import shutil
from datetime import datetime
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional

from agent.infra.runtime.workspace import get_workspace_base_path
from agent.utils.logger import logger


class FileStoragePaths:
    """统一管理文件存储路径。"""

    INTERNAL_DIR_NAME = ".agent"

    def __init__(self) -> None:
        self.home_root = Path.home() / ".nexus-core"
        self.workspace_base = get_workspace_base_path()
        self.agents_dir = self.home_root / "agents"
        self.agents_index_path = self.agents_dir / "index.json"
        self.legacy_db_path = Path.cwd() / "cache" / "data" / "nexus-core.db"

    def ensure_directories(self) -> None:
        """确保基础目录存在。"""
        self.home_root.mkdir(parents=True, exist_ok=True)
        self.agents_dir.mkdir(parents=True, exist_ok=True)
        self.workspace_base.mkdir(parents=True, exist_ok=True)

    def get_runtime_dir(self, workspace_path: str | Path) -> Path:
        """返回 workspace 内部运行目录。"""
        runtime_dir = Path(workspace_path).expanduser() / self.INTERNAL_DIR_NAME
        runtime_dir.mkdir(parents=True, exist_ok=True)
        return runtime_dir

    def migrate_workspace_runtime_layout(self, workspace_path: str | Path) -> None:
        """将旧版运行时文件迁移到 `.agent/` 目录。"""
        workspace_root = Path(workspace_path).expanduser()
        if not workspace_root.exists():
            return

        runtime_dir = self.get_runtime_dir(workspace_root)
        migrations = [
            (workspace_root / "agent.json", runtime_dir / "agent.json"),
            (workspace_root / "telemetry_cost_summary.json", runtime_dir / "telemetry_cost_summary.json"),
            (workspace_root / "sessions", runtime_dir / "sessions"),
        ]

        for source, target in migrations:
            if not source.exists():
                continue

            if target.exists():
                if source.is_dir():
                    shutil.rmtree(source, ignore_errors=True)
                else:
                    try:
                        source.unlink()
                    except FileNotFoundError:
                        pass
                continue

            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.move(str(source), str(target))

    @staticmethod
    def build_session_dir_name(session_key: str) -> str:
        """将 session_key 编码为安全目录名。"""
        encoded = base64.urlsafe_b64encode(session_key.encode("utf-8")).decode("ascii")
        return encoded.rstrip("=")

    def get_agent_file_path(self, workspace_path: str | Path) -> Path:
        """返回 Agent 快照文件路径。"""
        return self.get_runtime_dir(workspace_path) / "agent.json"

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

    def get_session_cost_summary_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话成本汇总路径。"""
        return self.get_session_dir(workspace_path, session_key) / "telemetry_cost_summary.json"

    def get_agent_cost_summary_path(self, workspace_path: str | Path) -> Path:
        """返回 Agent 成本汇总路径。"""
        return self.get_runtime_dir(workspace_path) / "telemetry_cost_summary.json"


class JsonFileStore:
    """JSON 文件读写工具。"""

    @staticmethod
    def read_json(path: Path, default: Any) -> Any:
        """读取 JSON 文件，不存在时返回默认值。"""
        if not path.exists():
            return default

        try:
            return json.loads(path.read_text(encoding="utf-8"))
        except Exception as exc:
            logger.warning(f"⚠️ 读取 JSON 失败，使用默认值: path={path}, error={exc}")
            return default

    @staticmethod
    def write_json(path: Path, payload: Any) -> None:
        """原子写入 JSON 文件。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = path.with_suffix(path.suffix + ".tmp")
        temp_path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        temp_path.replace(path)

    @staticmethod
    def append_jsonl(path: Path, payload: Dict[str, Any]) -> None:
        """向 JSONL 文件追加一行。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("a", encoding="utf-8") as file:
            file.write(json.dumps(payload, ensure_ascii=False) + "\n")

    @staticmethod
    def write_jsonl(path: Path, rows: List[Dict[str, Any]]) -> None:
        """覆写 JSONL 文件。"""
        path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = path.with_suffix(path.suffix + ".tmp")
        content = "\n".join(json.dumps(row, ensure_ascii=False) for row in rows)
        if content:
            content += "\n"
        temp_path.write_text(content, encoding="utf-8")
        temp_path.replace(path)

    @staticmethod
    def read_jsonl(path: Path) -> List[Dict[str, Any]]:
        """读取 JSONL 文件。"""
        if not path.exists():
            return []

        rows: List[Dict[str, Any]] = []
        for raw_line in path.read_text(encoding="utf-8").splitlines():
            line = raw_line.strip()
            if not line:
                continue
            try:
                parsed = json.loads(line)
            except Exception as exc:
                logger.warning(f"⚠️ 跳过损坏的 JSONL 行: path={path}, error={exc}")
                continue
            if isinstance(parsed, dict):
                rows.append(parsed)
        return rows


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
        JsonFileStore.write_json(self.paths.agents_index_path, [record])
        JsonFileStore.write_json(self.paths.get_agent_file_path(workspace_path), record)
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
