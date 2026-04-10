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

from agent.service.agent.agent_repository import agent_repository
from agent.infra.file_store.json_store import JsonFileStore
from agent.infra.file_store.storage_bootstrap import FileStorageBootstrap
from agent.infra.file_store.storage_paths import FileStoragePaths
from agent.schema.model_message import Message, parse_message
from agent.schema.model_session import ASession
from agent.service.session.session_router import resolve_agent_id
from agent.utils.logger import logger


class SessionRepository:
    """基于 workspace 文件系统的会话仓库。"""

    def __init__(self) -> None:
        self._bootstrap = FileStorageBootstrap()
        self._paths = FileStoragePaths()
        self._lock = Lock()

    def ensure_ready(self) -> None:
        """显式初始化，由 lifespan 调用而非导入时执行。"""
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
            agent_id=resolve_agent_id(meta.get("agent_id")),
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
        return JsonFileStore.read_jsonl(log_path)

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
            latest_timestamp = self._pick_latest_reply_timestamp(compacted_rows)
            if latest_timestamp:
                meta["last_activity"] = datetime.fromtimestamp(
                    int(latest_timestamp) / 1000,
                    tz=timezone.utc,
                ).isoformat()
        return meta

    @staticmethod
    def _pick_latest_reply_timestamp(message_rows: List[Dict[str, Any]]) -> Optional[int]:
        """优先返回最后一条回复消息时间，没有回复时回退到最后一条消息时间。"""
        for row in reversed(message_rows):
            if row.get("role") not in {"assistant", "result"}:
                continue
            timestamp = row.get("timestamp")
            if isinstance(timestamp, (int, float)) and int(timestamp) > 0:
                return int(timestamp)

        last_row = message_rows[-1] if message_rows else None
        if not last_row:
            return None

        timestamp = last_row.get("timestamp")
        if isinstance(timestamp, (int, float)) and int(timestamp) > 0:
            return int(timestamp)
        return None

    def _write_session_meta(self, meta_path: Path, meta: Dict[str, Any]) -> None:
        """写入会话元数据。"""
        JsonFileStore.write_json(meta_path, meta)

    def _refresh_meta_from_log_if_needed(self, meta_path: Path) -> Dict[str, Any]:
        """按消息日志回算 meta，避免历史会话时间长期停留在旧口径。"""
        with self._lock:
            meta = JsonFileStore.read_json(meta_path, {})
            if not meta:
                return {}

            log_path = meta_path.parent / "messages.jsonl"
            raw_rows = self._load_raw_message_rows(log_path if log_path.exists() else None)
            if not raw_rows:
                return meta

            refreshed_meta = self._refresh_meta_from_messages(dict(meta), raw_rows)
            if refreshed_meta != meta:
                self._write_session_meta(meta_path, refreshed_meta)
            return refreshed_meta

    def _materialize_unfinished_rounds(
        self,
        session_key: str,
        meta: Dict[str, Any],
        message_rows: List[Dict[str, Any]],
        active_round_ids: Optional[set[str]] = None,
    ) -> List[Dict[str, Any]]:
        """为未完成轮次补齐中断态工具结果和 result。"""
        interrupted_text = "任务已中断（未收到最终结束事件）"
        rows = [dict(row) for row in message_rows]
        round_status = dict(meta.get("round_status") or self._build_round_status(rows))
        active_round_ids = active_round_ids or set()

        for round_id, status in round_status.items():
            if not round_id or status != "running":
                continue
            if round_id in active_round_ids:
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
                            "content": interrupted_text,
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
                    "agent_id": resolve_agent_id(last_row.get("agent_id") or meta.get("agent_id")),
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
                    "result": interrupted_text,
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

    @staticmethod
    def _get_running_ws_round_ids(session_key: str) -> set[str]:
        """返回当前仍在后台运行的 round_id 集合。"""
        # 中文注释：只应跳过“当前正在跑的那一轮”补偿，旧轮次如果早就停了，
        # 仍然必须补成 interrupted；否则前端会把旧工具块一直显示成处理中。
        from agent.service.channels.ws.ws_chat_task_registry import ws_chat_task_registry

        round_id = ws_chat_task_registry.get_running_round_id(session_key)
        return {round_id} if round_id else set()

    async def create_session(
        self,
        session_key: str,
        channel_type: str = "websocket",
        chat_type: str = "dm",
        agent_id: Optional[str] = None,
        session_id: Optional[str] = None,
        title: Optional[str] = None,
        options: Optional[Dict[str, Any]] = None,
    ) -> bool:
        """创建新会话。"""
        try:
            resolved_agent_id = resolve_agent_id(agent_id)
            workspace_path = await self._resolve_workspace_path(resolved_agent_id)
            meta_path = self._paths.get_session_meta_path(workspace_path, session_key)
            log_path = self._paths.get_session_message_log_path(workspace_path, session_key)

            with self._lock:
                if meta_path.exists():
                    logger.info(f"ℹ️ 会话已存在: key={session_key}")
                    return True

                now = datetime.now(timezone.utc).isoformat()
                meta = {
                    "session_key": session_key,
                    "agent_id": resolved_agent_id,
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
                    JsonFileStore.write_jsonl(log_path, [])

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
            meta = self._refresh_meta_from_log_if_needed(meta_path)
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
        clear_session_id: bool = False,
    ) -> bool:
        """更新会话信息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            if not meta_path:
                return False

            with self._lock:
                meta = JsonFileStore.read_json(meta_path, {})
                if not meta:
                    return False
                if clear_session_id:
                    meta["session_id"] = None
                elif session_id is not None:
                    meta["session_id"] = session_id
                if title is not None:
                    meta["title"] = title
                if options is not None:
                    meta["options"] = options
                if status is not None:
                    meta["status"] = status
                self._write_session_meta(meta_path, meta)

            logger.info(f"🔄 更新会话: key={session_key}")
            return True
        except Exception as exc:
            logger.error(f"❌ 更新会话失败: {exc}", exc_info=True)
            return False

    async def clear_session_id(self, session_key: str) -> bool:
        """清空持久化的 SDK session_id，避免继续恢复已损坏会话。"""
        return await self.update_session(
            session_key=session_key,
            clear_session_id=True,
        )

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
                    meta = self._refresh_meta_from_log_if_needed(meta_path)
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

    async def delete_session(
        self,
        session_key: str,
        agent_id: Optional[str] = None,
    ) -> bool:
        """删除会话及其所有消息。"""
        try:
            workspace_path = (
                await self._resolve_workspace_path(resolve_agent_id(agent_id))
                if agent_id
                else None
            )
            meta_path = self._find_session_meta_path(session_key, workspace_path=workspace_path)
            session_dir = (
                meta_path.parent
                if meta_path
                else (
                    self._paths.get_session_dir(workspace_path, session_key)
                    if workspace_path is not None
                    else None
                )
            )
            if not session_dir or not session_dir.exists():
                return False
            with self._lock:
                shutil.rmtree(session_dir, ignore_errors=True)
            logger.info(f"🗑️ 删除会话: key={session_key}")
            return True
        except Exception as exc:
            logger.error(f"❌ 删除会话失败: {exc}", exc_info=True)
            return False

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
                JsonFileStore.append_jsonl(log_path, self._message_record_from_message(message))
                raw_rows = self._load_raw_message_rows(log_path)
                meta = JsonFileStore.read_json(meta_path, {})
                meta["agent_id"] = message.agent_id
                meta["session_id"] = message.session_id
                refreshed_meta = self._refresh_meta_from_messages(meta, raw_rows)
                self._write_session_meta(meta_path, refreshed_meta)
            return True
        except Exception as exc:
            logger.error(f"❌ 保存消息失败: {exc}", exc_info=True)
            return False

    async def repair_unfinished_round(
        self,
        session_key: str,
        round_id: str,
        result_text: str,
    ) -> List[Message]:
        """把指定轮次补齐为 interrupted，返回需要同步给前端的消息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            log_path = self._find_message_log_path(session_key)
            if not meta_path or not log_path:
                return []

            repaired_messages: List[Message] = []
            now_ms = int(datetime.now(timezone.utc).timestamp() * 1000)

            with self._lock:
                raw_rows = self._load_raw_message_rows(log_path)
                if not raw_rows:
                    return []
                meta = JsonFileStore.read_json(meta_path, {})
                compacted_rows = self._bootstrap.compact_messages(raw_rows)
                round_rows = [
                    dict(row)
                    for row in compacted_rows
                    if row.get("round_id") == round_id
                ]
                if not round_rows:
                    return []
                if any(row.get("role") == "result" for row in round_rows):
                    return []

                tool_result_ids: set[str] = set()
                for row in round_rows:
                    content = row.get("content")
                    if not isinstance(content, list):
                        continue
                    for block in content:
                        if isinstance(block, dict) and block.get("type") == "tool_result" and block.get("tool_use_id"):
                            tool_result_ids.add(str(block["tool_use_id"]))

                last_row = round_rows[-1]
                last_assistant_message_id = next(
                    (
                        row.get("message_id")
                        for row in reversed(round_rows)
                        if row.get("role") == "assistant"
                    ),
                    last_row.get("message_id"),
                )

                for row in round_rows:
                    if row.get("role") != "assistant":
                        continue
                    content = row.get("content")
                    if not isinstance(content, list):
                        continue

                    updated_content = list(content)
                    changed = False
                    for block in list(updated_content):
                        if not isinstance(block, dict):
                            continue
                        if block.get("type") != "tool_use" or not block.get("id"):
                            continue
                        tool_use_id = str(block["id"])
                        if tool_use_id in tool_result_ids:
                            continue
                        updated_content.append(
                            {
                                "type": "tool_result",
                                "tool_use_id": tool_use_id,
                                "content": result_text,
                                "is_error": True,
                            }
                        )
                        tool_result_ids.add(tool_use_id)
                        changed = True

                    if not changed:
                        continue

                    updated_row = dict(row)
                    updated_row["content"] = updated_content
                    raw_rows.append(updated_row)
                    repaired_messages.append(parse_message(updated_row))

                result_payload = {
                    "message_id": f"interrupted_result_{round_id}_{uuid.uuid4().hex[:8]}",
                    "parent_id": last_assistant_message_id,
                    "session_key": session_key,
                    "agent_id": resolve_agent_id(last_row.get("agent_id") or meta.get("agent_id")),
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
                    "result": result_text,
                    "is_error": True,
                    "timestamp": now_ms,
                }
                raw_rows.append(result_payload)
                JsonFileStore.write_jsonl(log_path, raw_rows)
                refreshed_meta = self._refresh_meta_from_messages(meta, raw_rows)
                self._write_session_meta(meta_path, refreshed_meta)
                repaired_messages.append(parse_message(result_payload))

            return repaired_messages
        except Exception as exc:
            logger.error(
                "❌ 修复未完成轮次失败: key=%s, round=%s, error=%s",
                session_key,
                round_id,
                exc,
                exc_info=True,
            )
            return []

    async def get_session_messages(self, session_key: str) -> List[Message]:
        """获取会话的所有历史消息。"""
        try:
            meta_path = self._find_session_meta_path(session_key)
            meta = JsonFileStore.read_json(meta_path, {}) if meta_path else {}
            compacted_rows = self._load_compacted_message_rows(self._find_message_log_path(session_key))
            active_round_ids = self._get_running_ws_round_ids(session_key)
            if active_round_ids:
                logger.info(
                    "⏭️ 跳过活跃轮次补偿: session=%s, rounds=%s",
                    session_key,
                    ",".join(sorted(active_round_ids)),
                )
            materialized_rows = self._materialize_unfinished_rounds(
                session_key,
                meta,
                compacted_rows,
                active_round_ids=active_round_ids,
            )

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
