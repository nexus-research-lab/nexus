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
from datetime import datetime
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional

from agent.service.agent.workspace import get_workspace_base_path
from agent.utils.logger import logger


class FileStoragePaths:
    """统一管理文件存储路径。"""

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

    @staticmethod
    def build_session_dir_name(session_key: str) -> str:
        """将 session_key 编码为安全目录名。"""
        encoded = base64.urlsafe_b64encode(session_key.encode("utf-8")).decode("ascii")
        return encoded.rstrip("=")

    def get_agent_file_path(self, workspace_path: str | Path) -> Path:
        """返回 Agent 快照文件路径。"""
        return Path(workspace_path).expanduser() / "agent.json"

    def get_session_dir(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话目录。"""
        return Path(workspace_path).expanduser() / "sessions" / self.build_session_dir_name(session_key)

    def get_session_meta_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话元数据路径。"""
        return self.get_session_dir(workspace_path, session_key) / "meta.json"

    def get_session_message_log_path(self, workspace_path: str | Path, session_key: str) -> Path:
        """返回会话消息日志路径。"""
        return self.get_session_dir(workspace_path, session_key) / "messages.jsonl"


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
                if self.paths.legacy_db_path.exists():
                    self._migrate_legacy_database()
                else:
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

    def _migrate_legacy_database(self) -> None:
        """将旧 SQLite 数据迁移到文件存储。"""
        logger.info(f"🔄 检测到旧数据库，开始迁移: {self.paths.legacy_db_path}")
        connection: Optional[sqlite3.Connection] = None

        try:
            connection = sqlite3.connect(str(self.paths.legacy_db_path))
            connection.row_factory = sqlite3.Row
            table_names = self._read_table_names(connection)

            agent_records = self._load_legacy_agents(connection, table_names)
            if not agent_records:
                agent_records = self._derive_agents_from_legacy_data(connection, table_names)
            if not agent_records:
                self._bootstrap_default_agent()
                return

            JsonFileStore.write_json(self.paths.agents_index_path, agent_records)
            for agent_record in agent_records:
                workspace_path = Path(agent_record["workspace_path"]).expanduser()
                workspace_path.mkdir(parents=True, exist_ok=True)
                JsonFileStore.write_json(self.paths.get_agent_file_path(workspace_path), agent_record)

            session_workspace_map = self._migrate_legacy_sessions(connection, table_names, agent_records)
            self._migrate_legacy_messages(connection, table_names, session_workspace_map)
            logger.info("✅ 旧数据库迁移完成，已切换到文件存储")
        except Exception as exc:
            logger.error(f"❌ 迁移旧数据库失败，回退到默认 Agent: {exc}", exc_info=True)
            self._bootstrap_default_agent()
        finally:
            if connection is not None:
                try:
                    connection.close()
                except Exception:
                    pass

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

    @staticmethod
    def normalize_message_payload(payload: Any) -> Dict[str, Any]:
        """将旧数据库里的消息内容转换为字典。"""
        if isinstance(payload, dict):
            return dict(payload)
        if payload is None:
            return {}
        if isinstance(payload, str):
            try:
                parsed = json.loads(payload)
                if isinstance(parsed, dict):
                    return parsed
            except Exception:
                return {"content": payload}
        return {}

    def _read_table_names(self, connection: sqlite3.Connection) -> set[str]:
        """读取旧数据库中的表名。"""
        cursor = connection.execute("SELECT name FROM sqlite_master WHERE type='table'")
        return {row["name"] for row in cursor.fetchall()}

    def _load_legacy_agents(
        self,
        connection: sqlite3.Connection,
        table_names: set[str],
    ) -> List[Dict[str, Any]]:
        """从旧 agents 表加载 Agent。"""
        if "agents" not in table_names:
            return []

        cursor = connection.execute(
            """
            SELECT agent_id, name, workspace_path, options, created_at, status
            FROM agents
            ORDER BY created_at DESC
            """
        )

        records: List[Dict[str, Any]] = []
        for row in cursor.fetchall():
            workspace_path = row["workspace_path"] or str(self.paths.workspace_base / (row["name"] or row["agent_id"]))
            options = self.normalize_message_payload(row["options"])
            record = {
                "agent_id": row["agent_id"],
                "name": row["name"] or row["agent_id"],
                "workspace_path": workspace_path,
                "options": options,
                "created_at": row["created_at"] or datetime.now().isoformat(),
                "status": row["status"] or "active",
            }
            records.append(record)
        return records

    def _derive_agents_from_legacy_data(
        self,
        connection: sqlite3.Connection,
        table_names: set[str],
    ) -> List[Dict[str, Any]]:
        """当旧库没有 agents 表时，从会话和消息中推导 Agent。"""
        agent_ids: List[str] = []

        if "sessions" in table_names:
            cursor = connection.execute("SELECT DISTINCT agent_id FROM sessions")
            agent_ids.extend([row["agent_id"] for row in cursor.fetchall() if row["agent_id"]])

        if "messages" in table_names:
            cursor = connection.execute("SELECT DISTINCT agent_id FROM messages")
            agent_ids.extend([row["agent_id"] for row in cursor.fetchall() if row["agent_id"]])

        deduplicated_ids: List[str] = []
        for agent_id in agent_ids or ["main"]:
            if agent_id not in deduplicated_ids:
                deduplicated_ids.append(agent_id)

        records: List[Dict[str, Any]] = []
        for agent_id in deduplicated_ids:
            workspace_path = str(self.paths.workspace_base / agent_id)
            records.append(
                {
                    "agent_id": agent_id,
                    "name": agent_id,
                    "workspace_path": workspace_path,
                    "options": {},
                    "created_at": datetime.now().isoformat(),
                    "status": "active",
                }
            )
        return records

    def _migrate_legacy_sessions(
        self,
        connection: sqlite3.Connection,
        table_names: set[str],
        agent_records: List[Dict[str, Any]],
    ) -> Dict[str, str]:
        """迁移会话元数据，并返回 session_key 到 workspace_path 的映射。"""
        if "sessions" not in table_names:
            return {}

        agent_workspace_map = {
            record["agent_id"]: record["workspace_path"]
            for record in agent_records
        }
        session_workspace_map: Dict[str, str] = {}

        cursor = connection.execute(
            """
            SELECT session_key, agent_id, session_id, channel_type, chat_type, status,
                   created_at, last_activity, title, options
            FROM sessions
            ORDER BY last_activity DESC
            """
        )

        for row in cursor.fetchall():
            agent_id = row["agent_id"] or "main"
            workspace_path = agent_workspace_map.get(agent_id, str(self.paths.workspace_base / agent_id))
            session_key = row["session_key"]
            session_workspace_map[session_key] = workspace_path

            meta = {
                "session_key": session_key,
                "agent_id": agent_id,
                "session_id": row["session_id"],
                "channel_type": row["channel_type"] or "websocket",
                "chat_type": row["chat_type"] or "dm",
                "status": row["status"] or "active",
                "created_at": row["created_at"] or datetime.now().isoformat(),
                "last_activity": row["last_activity"] or row["created_at"] or datetime.now().isoformat(),
                "title": row["title"] or "New Chat",
                "message_count": 0,
                "options": self.normalize_message_payload(row["options"]),
                "latest_round_id": None,
                "round_status": {},
            }
            JsonFileStore.write_json(self.paths.get_session_meta_path(workspace_path, session_key), meta)
        return session_workspace_map

    def _migrate_legacy_messages(
        self,
        connection: sqlite3.Connection,
        table_names: set[str],
        session_workspace_map: Dict[str, str],
    ) -> None:
        """迁移消息日志。"""
        if "messages" not in table_names:
            return

        grouped_rows: Dict[str, List[Dict[str, Any]]] = {}
        cursor = connection.execute(
            """
            SELECT message_id, parent_id, session_key, agent_id, round_id, session_id,
                   message_type, block_type, message, timestamp
            FROM messages
            ORDER BY timestamp ASC
            """
        )
        for row in cursor.fetchall():
            session_key = row["session_key"]
            workspace_path = session_workspace_map.get(session_key)
            if not workspace_path:
                workspace_path = str(self.paths.workspace_base / (row["agent_id"] or "main"))
                session_workspace_map[session_key] = workspace_path

            record = {
                "message_id": row["message_id"],
                "parent_id": row["parent_id"],
                "session_key": session_key,
                "agent_id": row["agent_id"] or "main",
                "round_id": row["round_id"],
                "session_id": row["session_id"],
                "message_type": row["message_type"],
                "block_type": row["block_type"],
                "message": self.normalize_message_payload(row["message"]),
                "timestamp": row["timestamp"] or datetime.now().isoformat(),
            }
            grouped_rows.setdefault(session_key, []).append(record)

        for session_key, rows in grouped_rows.items():
            workspace_path = session_workspace_map[session_key]
            log_path = self.paths.get_session_message_log_path(workspace_path, session_key)
            compacted_rows = self.compact_messages(rows)
            for record in compacted_rows:
                JsonFileStore.append_jsonl(log_path, record)

            meta_path = self.paths.get_session_meta_path(workspace_path, session_key)
            meta = JsonFileStore.read_json(meta_path, {})
            meta["message_count"] = len(compacted_rows)
            if compacted_rows:
                latest_message = compacted_rows[-1]
                meta["latest_round_id"] = latest_message.get("round_id")
                meta["last_activity"] = latest_message.get("timestamp") or meta.get("last_activity")
            JsonFileStore.write_json(meta_path, meta)
