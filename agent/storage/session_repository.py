# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_repository.py
# @Date   ：2026/3/14 11:38
# @Author ：leemysw
# 2026/3/14 11:38   Create
# =====================================================

"""会话数据仓库。"""

from __future__ import annotations

import shutil
import uuid
from datetime import datetime, timezone
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional

from agent.config.config import settings
from agent.storage.agent_repository import agent_repository
from agent.storage.config_store import ConfigStore
from agent.storage.jsonl_store import JsonlStore
from agent.storage.storage_bootstrap import FileStorageBootstrap
from agent.storage.storage_paths import FileStoragePaths
from agent.schema.model_message import Message, parse_message
from agent.schema.model_session import ASession
from agent.utils.logger import logger


class SessionRepository:
    """基于 workspace 文件系统的会话仓库。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._lock = Lock()
        self._bootstrap.ensure_ready()

    async def _resolve_workspace_path(self, agent_id: str) -> Path:
        """按 agent_id 解析 workspace 路径。"""
        agent = await agent_repository.get_agent(agent_id)
        if agent and agent.workspace_path:
            workspace_path = Path(agent.workspace_path).expanduser()
        else:
            workspace_path = self._paths.workspace_base / agent_id
        return workspace_path

    def _iter_known_workspace_paths(self) -> List[Path]:
        """返回当前 workspace 根目录下的所有已知路径。"""
        paths: List[Path] = []
        workspace_base = self._paths.workspace_base.expanduser()
        if workspace_base.exists():
            for path in workspace_base.iterdir():
                if path.is_dir() and path not in paths:
                    paths.append(path)
        if workspace_base not in paths:
            paths.append(workspace_base)
        return paths

    def _find_session_meta_path(
        self,
        session_key: str,
        workspace_path: Optional[Path] = None,
    ) -> Optional[Path]:
        """定位会话 meta.json。"""
        if workspace_path is not None:
            candidate = self._paths.get_session_meta_path(workspace_path, session_key)
            return candidate if candidate.exists() else None

        for root_path in self._iter_known_workspace_paths():
            candidate = self._paths.get_session_meta_path(root_path, session_key)
            if candidate.exists():
                return candidate
        return None

    def _find_message_log_path(
        self,
        session_key: str,
        workspace_path: Optional[Path] = None,
    ) -> Optional[Path]:
        """定位消息日志路径。"""
        meta_path = self._find_session_meta_path(session_key, workspace_path=workspace_path)
        if not meta_path:
            return None
        return meta_path.parent / "messages.jsonl"

    @staticmethod
    def _session_from_meta(meta: Dict[str, Any]) -> ASession:
        """将 meta.json 转换为 ASession。"""
        now = datetime.now(timezone.utc).isoformat()
        return ASession(
            session_key=meta["session_key"],
            agent_id=meta.get("agent_id") or  settings.DEFAULT_AGENT_ID,
            session_id=meta.get("session_id"),
            room_session_id=(meta.get("options") or {}).get("room_session_id"),
            channel_type=meta.get("channel_type") or "websocket",
            chat_type=meta.get("chat_type") or "dm",
            status=meta.get("status") or "active",
            created_at=meta.get("created_at") or now,
            last_activity=meta.get("last_activity") or now,
            title=meta.get("title") or "New Chat",
            message_count=int(meta.get("message_count") or 0),
            options=meta.get("options") or {},
        )

    @staticmethod
    def _message_record_from_message(message: Message) -> Dict[str, Any]:
        """将消息转换为 JSONL 行。"""
        return message.model_dump(mode="json", exclude_none=True)

    def _load_raw_message_rows(self, log_path: Optional[Path]) -> List[Dict[str, Any]]:
        """读取原始消息日志。"""
        if not log_path:
            return []
        return JsonlStore.read(log_path)

    def _load_compacted_message_rows(self, log_path: Optional[Path]) -> List[Dict[str, Any]]:
        """读取压缩后的消息快照。"""
        raw_rows = self._load_raw_message_rows(log_path)
        return self._bootstrap.compact_messages(raw_rows)

    @staticmethod
    def _build_round_status(message_rows: List[Dict[str, Any]]) -> Dict[str, str]:
        """根据消息快照构建轮次状态。"""
        status_map: Dict[str, str] = {}
        for row in message_rows:
            round_id = row.get("round_id")
            if not round_id:
                continue
            status_map.setdefault(round_id, "running")
            if row.get("role") == "result":
                status_map[round_id] = str(row.get("subtype") or "success")
        return status_map

    def _refresh_meta_from_messages(
        self,
        meta: Dict[str, Any],
        message_rows: List[Dict[str, Any]],
    ) -> Dict[str, Any]:
        """根据当前消息快照刷新 meta。"""
        compacted_rows = self._bootstrap.compact_messages(message_rows)
        meta["message_count"] = len(compacted_rows)
        meta["round_status"] = self._build_round_status(compacted_rows)
        meta["latest_round_id"] = compacted_rows[-1].get("round_id") if compacted_rows else None

        if compacted_rows:
            latest_timestamp = compacted_rows[-1].get("timestamp")
            if latest_timestamp:
                meta["last_activity"] = datetime.fromtimestamp(
                    int(latest_timestamp) / 1000,
                    tz=timezone.utc,
                ).isoformat()
        return meta

    def _write_session_meta(self, meta_path: Path, meta: Dict[str, Any]) -> None:
        """写入会话元数据。"""
        ConfigStore.write(meta_path, meta)

    def _materialize_unfinished_rounds(
        self,
        session_key: str,
        meta: Dict[str, Any],
        message_rows: List[Dict[str, Any]],
    ) -> List[Dict[str, Any]]:
        """为未完成轮次补齐中断态工具结果和 result。"""
        rows = [dict(row) for row in message_rows]
        round_status = dict(meta.get("round_status") or self._build_round_status(rows))

        for round_id, status in round_status.items():
            if not round_id or status != "running":
                continue

            round_rows = [row for row in rows if row.get("round_id") == round_id]
            if not round_rows:
                continue

            has_result = any(row.get("role") == "result" for row in round_rows)
            tool_result_ids: set[str] = set()
            tool_use_rows: List[Dict[str, Any]] = []

            for row in round_rows:
                if row.get("role") != "assistant":
                    continue
                content = row.get("content")
                if not isinstance(content, list):
                    continue

                for block in content:
                    if not isinstance(block, dict):
                        continue
                    if block.get("type") == "tool_result" and block.get("tool_use_id"):
                        tool_result_ids.add(str(block["tool_use_id"]))
                    if block.get("type") == "tool_use" and block.get("id"):
                        tool_use_rows.append(row)

            for row in tool_use_rows:
                content = list(row.get("content") or [])
                changed = False
                for block in content:
                    if not isinstance(block, dict):
                        continue
                    if block.get("type") != "tool_use" or not block.get("id"):
                        continue

                    tool_use_id = str(block["id"])
                    if tool_use_id in tool_result_ids:
                        continue

                    content.append(
                        {
                            "type": "tool_result",
                            "tool_use_id": tool_use_id,
                            "content": "任务已中断（页面刷新或连接断开）",
                            "is_error": True,
                        }
                    )
                    tool_result_ids.add(tool_use_id)
                    changed = True

                if changed:
                    row["content"] = content

            if has_result:
                continue

            last_row = round_rows[-1]
            rows.append(
                {
                    "message_id": f"interrupted_result_{round_id}_{uuid.uuid4().hex[:8]}",
                    "parent_id": last_row.get("message_id"),
                    "session_key": session_key,
                    "agent_id": last_row.get("agent_id") or meta.get("agent_id") or  settings.DEFAULT_AGENT_ID,
                    "round_id": round_id,
                    "session_id": last_row.get("session_id") or meta.get("session_id") or "",
                    "role": "result",
                    "subtype": "interrupted",
                    "duration_ms": 0,
                    "duration_api_ms": 0,
                    "num_turns": 0,
                    "total_cost_usd": 0,
                    "usage": {
                        "input_tokens": 0,
                        "output_tokens": 0,
                        "cache_creation_input_tokens": 0,
                        "cache_read_input_tokens": 0,
                    },
                    "result": "任务已中断（页面刷新或连接断开）",
                    "is_error": True,
                    "timestamp": last_row.get("timestamp") or int(datetime.now(timezone.utc).timestamp() * 1000),
                }
            )

        def parse_timestamp(ts: Any) -> int:
            if isinstance(ts, (int, float)):
                return int(ts)
            return 0

        rows.sort(key=lambda item: parse_timestamp(item.get("timestamp")))
        return rows

    async def create_session(
        self,
        session_key: str,
        channel_type: str = "websocket",
        chat_type: str = "dm",
        agent_id: str = settings.DEFAULT_AGENT_ID,
        session_id: Optional[str] = None,
        title: Optional[str] = None,
        options: Optional[Dict[str, Any]] = None,
    ) -> bool:
        """创建新会话。"""
        try:
            workspace_path = await self._resolve_workspace_path(agent_id)
            meta_path = self._paths.get_session_meta_path(workspace_path, session_key)
            log_path = self._paths.get_session_message_log_path(workspace_path, session_key)

            with self._lock:
                if meta_path.exists():
                    logger.info(f"ℹ️ 会话已存在: key={session_key}")
                    return True

                now = datetime.now(timezone.utc).isoformat()
                meta = {
                    "session_key": session_key,
                    "agent_id": agent_id,
                    "session_id": session_id,
                    "channel_type": channel_type,
                    "chat_type": chat_type,
                    "status": "active",
                    "created_at": now,
                    "last_activity": now,
                    "title": title or "New Chat",
                    "message_count": 0,
                    "options": options or {},
                    "latest_round_id": None,
                    "round_status": {},
                }
                self._write_session_meta(meta_path, meta)
                log_path.parent.mkdir(parents=True, exist_ok=True)
                if not log_path.exists():
                    JsonlStore.write(log_path, [])

            logger.info(f"✅ 创建会话: key={session_key}")
            return True
        except Exception as exc:
            logger.error(f"❌ 创建会话失败: {exc}", exc_info=True)
            return False

    async def get_session(self, session_key: str) -> Optional[ASession]:
        """按 session_key 获取会话。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            if not meta_path:
                return None
            meta = ConfigStore.read(meta_path, {})
            if not meta:
                return None
            return self._session_from_meta(meta)
        except Exception as exc:
            logger.error(f"❌ 获取会话失败: {exc}", exc_info=True)
            return None

    async def update_session(
        self,
        session_key: str,
        session_id: Optional[str] = None,
        title: Optional[str] = None,
        options: Optional[Dict[str, Any]] = None,
        status: Optional[str] = None,
    ) -> bool:
        """更新会话信息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            if not meta_path:
                return False

            with self._lock:
                meta = ConfigStore.read(meta_path, {})
                if not meta:
                    return False
                if session_id is not None:
                    meta["session_id"] = session_id
                if title is not None:
                    meta["title"] = title
                if options is not None:
                    meta["options"] = options
                if status is not None:
                    meta["status"] = status
                meta["last_activity"] = datetime.now(timezone.utc).isoformat()
                self._write_session_meta(meta_path, meta)

            logger.info(f"🔄 更新会话: key={session_key}")
            return True
        except Exception as exc:
            logger.error(f"❌ 更新会话失败: {exc}", exc_info=True)
            return False

    async def get_all_sessions(self) -> List[ASession]:
        """获取所有会话。"""
        sessions: List[ASession] = []
        try:
            seen_paths: set[Path] = set()
            for workspace_path in self._iter_known_workspace_paths():
                runtime_dir = self._paths.get_runtime_dir(workspace_path)
                for meta_path in runtime_dir.glob("sessions/*/meta.json"):
                    if meta_path in seen_paths:
                        continue
                    seen_paths.add(meta_path)
                    meta = ConfigStore.read(meta_path, {})
                    if not meta:
                        continue
                    try:
                        sessions.append(self._session_from_meta(meta))
                    except Exception as exc:
                        logger.warning(f"⚠️ 跳过损坏的会话元数据: path={meta_path}, error={exc}")
            sessions.sort(key=lambda item: item.last_activity, reverse=True)
            logger.info(f"📋 获取会话列表: 共{len(sessions)}个")
            return sessions
        except Exception as exc:
            logger.error(f"❌ 获取会话列表失败: {exc}", exc_info=True)
            return []

    async def delete_session(self, session_key: str) -> bool:
        """删除会话及其所有消息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            if not meta_path:
                return False
            with self._lock:
                shutil.rmtree(meta_path.parent, ignore_errors=True)
            logger.info(f"🗑️ 删除会话: key={session_key}")
            return True
        except Exception as exc:
            logger.error(f"❌ 删除会话失败: {exc}", exc_info=True)
            return False

    async def delete_round(self, session_key: str, round_id: str) -> int:
        """删除一轮对话。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            log_path = self._find_message_log_path(session_key)
            if not meta_path or not log_path:
                return 0

            with self._lock:
                raw_rows = self._load_raw_message_rows(log_path)
                deleted_count = len([row for row in raw_rows if row.get("round_id") == round_id])
                remaining_rows = [row for row in raw_rows if row.get("round_id") != round_id]
                JsonlStore.write(log_path, remaining_rows)

                meta = ConfigStore.read(meta_path, {})
                refreshed_meta = self._refresh_meta_from_messages(meta, remaining_rows)
                self._write_session_meta(meta_path, refreshed_meta)

            logger.info(f"🗑️ 删除轮次: key={session_key}, round={round_id}, 共{deleted_count}条")
            return deleted_count
        except Exception as exc:
            logger.error(f"❌ 删除轮次失败: {exc}", exc_info=True)
            return -1

    async def get_latest_round_id(self, session_key: str) -> Optional[str]:
        """获取最新 round_id。"""
        try:
            compacted_rows = self._load_compacted_message_rows(self._find_message_log_path(session_key))
            if not compacted_rows:
                return None
            return compacted_rows[-1].get("round_id")
        except Exception as exc:
            logger.error(f"❌ 获取最新 round_id 失败: {exc}", exc_info=True)
            return None

    async def has_round_result(self, session_key: str, round_id: str) -> bool:
        """检查指定轮次是否已有 result 消息。"""
        try:
            compacted_rows = self._load_compacted_message_rows(self._find_message_log_path(session_key))
            for row in compacted_rows:
                if row.get("round_id") == round_id and row.get("role") == "result":
                    return True
            return False
        except Exception as exc:
            logger.error(f"❌ 检查轮次 result 失败: key={session_key}, round={round_id}, error={exc}")
            return False

    async def create_message(self, message: Message) -> bool:
        """保存消息。"""
        try:
            meta_path = self._find_session_meta_path(message.session_key)
            log_path = self._find_message_log_path(message.session_key)
            if not meta_path or not log_path:
                logger.error(f"❌ 保存消息失败，会话不存在: {message.session_key}")
                return False

            with self._lock:
                JsonlStore.append(log_path, self._message_record_from_message(message))
                raw_rows = self._load_raw_message_rows(log_path)
                meta = ConfigStore.read(meta_path, {})
                meta["agent_id"] = message.agent_id
                meta["session_id"] = message.session_id
                refreshed_meta = self._refresh_meta_from_messages(meta, raw_rows)
                self._write_session_meta(meta_path, refreshed_meta)
            return True
        except Exception as exc:
            logger.error(f"❌ 保存消息失败: {exc}", exc_info=True)
            return False

    async def get_session_messages(self, session_key: str) -> List[Message]:
        """获取会话的所有历史消息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            meta = ConfigStore.read(meta_path, {}) if meta_path else {}
            compacted_rows = self._load_compacted_message_rows(self._find_message_log_path(session_key))
            materialized_rows = self._materialize_unfinished_rounds(session_key, meta, compacted_rows)

            message_list: List[Message] = []
            for row in materialized_rows:
                try:
                    message_list.append(parse_message(row))
                except Exception as exc:
                    logger.warning(
                        f"⚠️ 跳过脏消息: id={row.get('message_id')}, role={row.get('role')}, error={exc}"
                    )
            logger.info(f"📥 加载历史消息: key={session_key}, 共{len(message_list)}条")
            return message_list
        except Exception as exc:
            logger.error(f"❌ 获取历史消息失败: {exc}", exc_info=True)
            return []


session_repository = SessionRepository()
