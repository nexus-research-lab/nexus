# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：session_repository
# @Date   ：2026/3/9 22:40
# @Author ：leemysw
# 2026/3/9 22:40   Create
# =====================================================

"""
会话数据仓库

[INPUT]: 依赖文件存储层、Agent Repository、schema 模型
[OUTPUT]: 对外提供 SessionRepository（会话 CRUD + 消息 CRUD）
[POS]: db 模块的数据访问层，被 session_store 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

import json
import shutil
import uuid
from dataclasses import asdict
from datetime import datetime, timezone
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional

from agent.infra.storage.agent_repository import agent_repository
from agent.infra.storage.config_store import ConfigStore
from agent.infra.storage.jsonl_store import JsonlStore
from agent.infra.storage.storage_bootstrap import FileStorageBootstrap
from agent.infra.storage.storage_paths import FileStoragePaths
from agent.schema.model_message import AMessage
from agent.schema.model_session import ASession
from agent.utils.logger import logger


class SessionRepository:
    """基于 workspace 文件系统的会话仓库。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._lock = Lock()
        self._bootstrap.ensure_ready()

    @staticmethod
    def _to_message_dict(message_obj: Any) -> Dict[str, Any]:
        """将消息对象转换为可序列化字典。"""
        if message_obj is None:
            return {}
        if isinstance(message_obj, dict):
            return dict(message_obj)
        if isinstance(message_obj, str):
            return {"content": message_obj}
        if hasattr(message_obj, "model_dump"):
            return message_obj.model_dump(mode="json")
        return asdict(message_obj)

    @staticmethod
    def _coerce_payload_dict(message_type: str, payload: Any) -> Dict[str, Any]:
        """将任意 payload 尽可能转换为字典。"""
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
                pass

            if message_type in ("assistant", "user"):
                return {"content": payload}
            if message_type == "system":
                return {"subtype": "info", "data": {"raw": payload}}
            if message_type == "result":
                return {"subtype": "error", "result": payload, "is_error": True}
            return {}

        try:
            return SessionRepository._to_message_dict(payload)
        except Exception:
            return {}

    @staticmethod
    def _normalize_message_payload(message_type: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        """规范化历史消息结构，兼容旧脏数据。"""
        normalized = dict(payload or {})

        if message_type == "user":
            tool_use_result = normalized.get("tool_use_result")
            if isinstance(tool_use_result, str):
                normalized["tool_use_result"] = {"error": tool_use_result}
            elif tool_use_result is not None and not isinstance(tool_use_result, dict):
                normalized["tool_use_result"] = {"value": tool_use_result}
            return normalized

        if message_type == "assistant":
            normalized.setdefault("model", "")
            return normalized

        if message_type == "system":
            normalized.setdefault("subtype", "info")
            normalized.setdefault("data", {})
            return normalized

        if message_type == "result":
            normalized.setdefault("subtype", "error" if normalized.get("is_error") else "success")
            normalized.setdefault("duration_ms", 0)
            normalized.setdefault("duration_api_ms", 0)
            normalized.setdefault("num_turns", 0)
            normalized.setdefault("session_id", "")
            normalized.setdefault("is_error", False)
            return normalized

        return normalized

    async def _resolve_workspace_path(self, agent_id: str) -> Path:
        """按 agent_id 解析 workspace 路径。"""
        agent = await agent_repository.get_agent(agent_id)
        if agent and agent.workspace_path:
            workspace_path = Path(agent.workspace_path).expanduser()
        else:
            workspace_path = self._paths.workspace_base / agent_id
        self._paths.migrate_workspace_runtime_layout(workspace_path)
        return workspace_path

    def _iter_known_workspace_paths(self) -> List[Path]:
        """返回当前所有已知 workspace 路径。"""
        records = ConfigStore.read(self._paths.agents_index_path, [])
        paths: List[Path] = []
        for record in records if isinstance(records, list) else []:
            workspace_path = record.get("workspace_path")
            if not workspace_path:
                continue
            path = Path(str(workspace_path)).expanduser()
            if path not in paths:
                paths.append(path)

        if self._paths.workspace_base not in paths:
            paths.append(self._paths.workspace_base)
        return paths

    def _find_session_meta_path(self, session_key: str, workspace_path: Optional[Path] = None) -> Optional[Path]:
        """定位会话 meta.json。"""
        session_dir_name = self._paths.build_session_dir_name(session_key)
        if workspace_path is not None:
            self._paths.migrate_workspace_runtime_layout(workspace_path)
            candidate = self._paths.get_session_meta_path(workspace_path, session_key)
            return candidate if candidate.exists() else None

        for root_path in self._iter_known_workspace_paths():
            self._paths.migrate_workspace_runtime_layout(root_path)
            candidate = self._paths.get_session_meta_path(root_path, session_key)
            if candidate.exists():
                return candidate

        return None

    def _find_message_log_path(self, session_key: str, workspace_path: Optional[Path] = None) -> Optional[Path]:
        """定位 messages.jsonl。"""
        meta_path = self._find_session_meta_path(session_key, workspace_path=workspace_path)
        if not meta_path:
            return None
        return meta_path.parent / "messages.jsonl"

    @staticmethod
    def _session_from_meta(meta: Dict[str, Any]) -> ASession:
        """将 meta.json 转换为 ASession。"""
        return ASession(
            session_key=meta["session_key"],
            agent_id=meta.get("agent_id") or "main",
            session_id=meta.get("session_id"),
            channel_type=meta.get("channel_type") or "websocket",
            chat_type=meta.get("chat_type") or "dm",
            status=meta.get("status") or "active",
            created_at=meta.get("created_at") or datetime.now(timezone.utc).isoformat(),
            last_activity=meta.get("last_activity") or datetime.now(timezone.utc).isoformat(),
            title=meta.get("title") or "New Chat",
            message_count=int(meta.get("message_count") or 0),
            options=meta.get("options") or {},
        )

    @staticmethod
    def _message_record_from_message(message: AMessage) -> Dict[str, Any]:
        """将 AMessage 转换为 JSONL 记录。"""
        payload = SessionRepository._normalize_message_payload(
            message.message_type,
            SessionRepository._to_message_dict(message.message),
        )
        timestamp = message.timestamp or datetime.now(timezone.utc)
        return {
            "message_id": message.message_id,
            "parent_id": message.parent_id,
            "session_key": message.session_key,
            "agent_id": message.agent_id,
            "round_id": message.round_id,
            "session_id": message.session_id,
            "message_type": message.message_type,
            "block_type": message.block_type,
            "message": payload,
            "timestamp": timestamp.isoformat() if hasattr(timestamp, "isoformat") else str(timestamp),
        }

    def _load_raw_message_rows(self, log_path: Optional[Path]) -> List[Dict[str, Any]]:
        """读取原始消息日志。"""
        if not log_path:
            return []
        return JsonlStore.read(log_path)

    def _load_compacted_message_rows(self, log_path: Optional[Path]) -> List[Dict[str, Any]]:
        """读取压缩后的消息快照。"""
        raw_rows = self._load_raw_message_rows(log_path)
        compacted = self._bootstrap.compact_messages(raw_rows)
        return compacted

    @staticmethod
    def _build_round_status(message_rows: List[Dict[str, Any]]) -> Dict[str, str]:
        """从消息快照构建轮次状态。"""
        status_map: Dict[str, str] = {}
        for row in message_rows:
            round_id = row.get("round_id")
            if not round_id:
                continue

            status_map.setdefault(round_id, "running")
            if row.get("message_type") == "result":
                payload = row.get("message") or {}
                status_map[round_id] = str(payload.get("subtype") or "success")
        return status_map

    def _refresh_meta_from_messages(self, meta: Dict[str, Any], message_rows: List[Dict[str, Any]]) -> Dict[str, Any]:
        """根据当前消息快照刷新 meta。"""
        compacted_rows = self._bootstrap.compact_messages(message_rows)
        meta["message_count"] = len(compacted_rows)
        meta["round_status"] = self._build_round_status(compacted_rows)
        meta["latest_round_id"] = compacted_rows[-1].get("round_id") if compacted_rows else None

        if compacted_rows:
            latest_timestamp = compacted_rows[-1].get("timestamp")
            if latest_timestamp:
                meta["last_activity"] = latest_timestamp
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
        """为未完成轮次补齐中断态 tool_result 和 result。"""
        rows = [dict(row) for row in message_rows]
        round_status = dict(meta.get("round_status") or self._build_round_status(rows))

        for round_id, status in round_status.items():
            if not round_id or status != "running":
                continue

            round_rows = [row for row in rows if row.get("round_id") == round_id]
            if not round_rows:
                continue

            has_result = any(row.get("message_type") == "result" for row in round_rows)
            tool_result_ids: set[str] = set()
            tool_use_rows: List[Dict[str, Any]] = []

            for row in round_rows:
                if row.get("message_type") != "assistant":
                    continue

                payload = self._coerce_payload_dict("assistant", row.get("message"))
                content = payload.get("content")
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
                payload = self._coerce_payload_dict("assistant", row.get("message"))
                content = list(payload.get("content") or [])
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
                    payload["content"] = content
                    row["message"] = payload

            if has_result:
                continue

            last_row = round_rows[-1]
            rows.append(
                {
                    "message_id": f"interrupted_result_{round_id}_{uuid.uuid4().hex[:8]}",
                    "parent_id": last_row.get("message_id"),
                    "session_key": session_key,
                    "agent_id": last_row.get("agent_id") or meta.get("agent_id") or "main",
                    "round_id": round_id,
                    "session_id": last_row.get("session_id") or meta.get("session_id") or "",
                    "message_type": "result",
                    "block_type": None,
                    "message": {
                        "subtype": "interrupted",
                        "duration_ms": 0,
                        "duration_api_ms": 0,
                        "num_turns": 0,
                        "session_id": last_row.get("session_id") or meta.get("session_id") or "",
                        "total_cost_usd": 0,
                        "usage": {
                            "input_tokens": 0,
                            "output_tokens": 0,
                            "cache_creation_input_tokens": 0,
                            "cache_read_input_tokens": 0,
                        },
                        "result": "任务已中断（页面刷新或连接断开）",
                        "is_error": True,
                    },
                    "timestamp": last_row.get("timestamp") or datetime.now().isoformat(),
                }
            )

        rows.sort(key=lambda item: str(item.get("timestamp") or ""))
        return rows

    async def create_session(
            self,
            session_key: str,
            channel_type: str = "websocket",
            chat_type: str = "dm",
            agent_id: str = "main",
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
                self._paths.migrate_workspace_runtime_layout(workspace_path)
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
                if row.get("round_id") == round_id and row.get("message_type") == "result":
                    return True
            return False
        except Exception as exc:
            logger.error(f"❌ 检查轮次 result 失败: key={session_key}, round={round_id}, error={exc}")
            return False

    async def create_message(self, message: AMessage) -> bool:
        """保存消息。"""
        try:
            meta_path = self._find_session_meta_path(message.session_key)
            log_path = self._find_message_log_path(message.session_key)
            if not meta_path or not log_path:
                logger.error(f"❌ 保存消息失败，会话不存在: {message.session_key}")
                return False

            with self._lock:
                record = self._message_record_from_message(message)
                JsonlStore.append(log_path, record)

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

    async def get_session_messages(self, session_key: str) -> List[AMessage]:
        """获取会话的所有历史消息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            meta = ConfigStore.read(meta_path, {}) if meta_path else {}
            compacted_rows = self._load_compacted_message_rows(self._find_message_log_path(session_key))
            materialized_rows = self._materialize_unfinished_rounds(session_key, meta, compacted_rows)
            message_list: List[AMessage] = []
            for row in materialized_rows:
                try:
                    normalized_payload = self._normalize_message_payload(
                        row.get("message_type") or "",
                        self._coerce_payload_dict(row.get("message_type") or "", row.get("message")),
                    )
                    message_list.append(
                        AMessage(
                            session_key=row.get("session_key") or session_key,
                            agent_id=row.get("agent_id") or "main",
                            round_id=row.get("round_id"),
                            session_id=row.get("session_id") or "",
                            message_id=row.get("message_id"),
                            message=normalized_payload,
                            message_type=row.get("message_type"),
                            block_type=row.get("block_type"),
                            parent_id=row.get("parent_id"),
                            timestamp=row.get("timestamp"),
                        )
                    )
                except Exception as exc:
                    logger.warning(
                        f"⚠️ 跳过脏消息: id={row.get('message_id')}, "
                        f"type={row.get('message_type')}, error={exc}"
                    )
            logger.info(f"📥 加载历史消息: key={session_key}, 共{len(message_list)}条")
            return message_list
        except Exception as exc:
            logger.error(f"❌ 获取历史消息失败: {exc}", exc_info=True)
            return []


session_repository = SessionRepository()
