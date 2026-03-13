# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：protocol_adapter.py
# @Date   ：2026/02/27 15:21
# @Author ：leemysw
# 2026/02/27 15:21   Create
# =====================================================

"""会话协议适配器。

将内部 `AMessage` 统一转换为前端可直接消费的协议：
1. WebSocket 实时事件（conversation_event）。
2. 历史快照消息（Message[]）。
"""

import uuid
from dataclasses import asdict, is_dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from agent.schema.model_message import AEvent, AMessage
from agent.schema.model_workspace_event import WorkspaceEvent


class ProtocolAdapter:
    """会话协议适配器。

    该类是后端消息协议的唯一出口，负责消除 SDK 原始结构差异，
    并生成稳定的前端消息结构与事件结构。
    """

    def __init__(self):
        self._session_seq: Dict[str, int] = {}
        self._tool_use_owner: Dict[str, Dict[str, str]] = {}

    def build_ws_event(self, message: AMessage) -> Optional[AEvent]:
        """将内部消息转换为 WebSocket conversation_event。"""
        agent_id = self._resolve_agent_id(message.session_key, message.agent_id)
        base_data: Dict[str, Any] = {
            "event_id": str(uuid.uuid4()),
            "seq": self._next_seq(message.session_key),
            "turn_id": message.round_id,
        }

        if message.message_type == "stream":
            delta = self._convert_stream_delta(message)
            if not delta:
                return None
            base_data["kind"] = "message_delta"
            base_data["delta"] = delta
        else:
            snapshot = self._convert_message_snapshot(message)
            if not snapshot:
                return None
            base_data["kind"] = "message_upsert"
            base_data["message"] = snapshot

        return AEvent(
            event_type="conversation_event",
            agent_id=agent_id,
            session_id=message.session_id,
            data=base_data,
        )

    def build_history_messages(self, messages: List[AMessage]) -> List[Dict[str, Any]]:
        """将历史消息转换为稳定快照列表。"""
        history: List[Dict[str, Any]] = []
        self._tool_use_owner = {}

        for record in messages:
            snapshot = self._convert_message_snapshot(record)
            if not snapshot:
                continue
            self._apply_snapshot(history, snapshot)

        return history

    def build_workspace_event(self, event: WorkspaceEvent) -> AEvent:
        """将 WorkspaceEvent 转换为 WebSocket 事件。"""
        return AEvent(
            event_type="workspace_event",
            agent_id=event.agent_id,
            data={
                **event.model_dump(),
                "timestamp": event.timestamp.isoformat(),
            },
            timestamp=event.timestamp,
        )

    def _apply_snapshot(self, history: List[Dict[str, Any]], snapshot: Dict[str, Any]) -> None:
        """对历史列表应用快照（upsert + tool_result 合并）。"""
        if snapshot.get("role") == "assistant" and snapshot.get("is_tool_result"):
            if self._merge_tool_result_snapshot(history, snapshot):
                return

        message_id = snapshot.get("message_id")
        if message_id:
            for idx, item in enumerate(history):
                if item.get("message_id") != message_id:
                    continue
                if item.get("role") == "assistant" and snapshot.get("role") == "assistant":
                    merged = dict(snapshot)
                    merged["content"] = self._merge_assistant_content(
                        item.get("content", []),
                        snapshot.get("content", []),
                    )
                    history[idx] = merged
                else:
                    history[idx] = snapshot
                return

        history.append(snapshot)

    def _merge_tool_result_snapshot(self, history: List[Dict[str, Any]], tool_result: Dict[str, Any]) -> bool:
        """将 tool_result 合并到对应 assistant 消息。"""
        content = tool_result.get("content", [])
        if not isinstance(content, list):
            return False

        target_message_id = tool_result.get("target_message_id")
        if target_message_id:
            for idx, item in enumerate(history):
                if item.get("message_id") != target_message_id:
                    continue
                if item.get("role") != "assistant":
                    break
                merged = dict(item)
                merged["content"] = self._merge_assistant_content(item.get("content", []), content)
                history[idx] = merged
                return True

        tool_use_id = None
        for block in content:
            if isinstance(block, dict) and block.get("type") == "tool_result":
                tool_use_id = block.get("tool_use_id")
                break
        if not tool_use_id:
            return False

        for idx in range(len(history) - 1, -1, -1):
            item = history[idx]
            if item.get("role") != "assistant":
                continue
            blocks = item.get("content", [])
            if not isinstance(blocks, list):
                continue
            has_tool_use = any(
                isinstance(block, dict) and block.get("type") == "tool_use" and block.get("id") == tool_use_id
                for block in blocks
            )
            if not has_tool_use:
                continue
            merged = dict(item)
            merged["content"] = self._merge_assistant_content(blocks, content)
            history[idx] = merged
            return True

        return False

    def _convert_message_snapshot(self, message: AMessage) -> Optional[Dict[str, Any]]:
        """将内部消息转换为前端快照消息。"""
        if message.message_type in ("system", "stream"):
            return None

        payload = self._to_dict(message.message)
        base = {
            "message_id": message.message_id,
            "round_id": message.round_id,
            "agent_id": self._resolve_agent_id(message.session_key, message.agent_id),
            "session_id": message.session_id,
            "parent_id": message.parent_id,
            "timestamp": self._to_timestamp_ms(message.timestamp),
        }

        if message.message_type == "assistant":
            blocks = self._normalize_content_blocks(payload.get("content"))
            self._index_tool_use_owner(message.session_key, message.message_id, blocks)
            snapshot = {
                **base,
                "role": "assistant",
                "content": blocks,
                "model": payload.get("model"),
                "stop_reason": payload.get("stop_reason"),
                "usage": self._normalize_usage(payload.get("usage")),
                "parent_tool_use_id": payload.get("parent_tool_use_id"),
            }
            return snapshot

        if message.message_type == "user":
            if message.block_type == "tool_result":
                blocks = self._normalize_content_blocks(payload.get("content"))
                target_message_id = self._resolve_target_message_id(message.session_key, blocks)
                snapshot = {
                    **base,
                    "role": "assistant",
                    "content": blocks,
                    "is_tool_result": True,
                    "parent_tool_use_id": payload.get("parent_tool_use_id"),
                }
                if target_message_id:
                    snapshot["target_message_id"] = target_message_id
                return snapshot

            text = self._extract_user_text(payload.get("content"))
            return {
                **base,
                "role": "user",
                "content": text,
                "parent_tool_use_id": payload.get("parent_tool_use_id"),
            }

        if message.message_type == "result":
            subtype = payload.get("subtype", "success")
            normalized_subtype = "success" if subtype == "success" else "error"
            usage = self._normalize_usage(payload.get("usage"))
            return {
                **base,
                "role": "result",
                "subtype": normalized_subtype,
                "duration_ms": payload.get("duration_ms", 0),
                "duration_api_ms": payload.get("duration_api_ms", 0),
                "num_turns": payload.get("num_turns", 0),
                "total_cost_usd": payload.get("total_cost_usd"),
                "usage": usage,
                "result": payload.get("result"),
                "isError": payload.get("is_error", False),
            }

        return None

    def _convert_stream_delta(self, message: AMessage) -> Optional[Dict[str, Any]]:
        """将内部 stream 消息转换为前端 delta。"""
        payload = self._to_dict(message.message)
        event = payload.get("event")
        if not isinstance(event, dict):
            return None

        delta: Dict[str, Any] = {
            "message_id": message.message_id,
            "type": event.get("type"),
            "index": event.get("index"),
            "delta": event.get("delta"),
            "message": event.get("message"),
            "usage": event.get("usage"),
        }
        content_block = event.get("content_block")
        if content_block is not None:
            delta["content_block"] = self._normalize_block(content_block)
        return delta

    def _resolve_target_message_id(self, session_key: str, blocks: List[Dict[str, Any]]) -> Optional[str]:
        """根据 tool_use_id 反查 assistant message_id。"""
        tool_result_block = next(
            (
                block
                for block in blocks
                if isinstance(block, dict) and block.get("type") == "tool_result" and block.get("tool_use_id")
            ),
            None,
        )
        if not tool_result_block:
            return None

        owner_mapping = self._tool_use_owner.get(session_key, {})
        return owner_mapping.get(tool_result_block["tool_use_id"])

    def _index_tool_use_owner(self, session_key: str, message_id: str, blocks: List[Dict[str, Any]]) -> None:
        """建立 tool_use_id 到 assistant message_id 的索引。"""
        owner_mapping = self._tool_use_owner.setdefault(session_key, {})
        for block in blocks:
            if block.get("type") == "tool_use" and block.get("id"):
                owner_mapping[str(block["id"])] = message_id

    def _next_seq(self, session_key: str) -> int:
        """获取并递增会话内事件序号。"""
        current = self._session_seq.get(session_key, 0) + 1
        self._session_seq[session_key] = current
        return current

    def _resolve_agent_id(self, session_key: str, fallback: str) -> str:
        """返回前端使用的会话路由键。"""
        return session_key or fallback

    def _normalize_usage(self, usage: Any) -> Optional[Dict[str, Any]]:
        """统一 usage 字段命名。"""
        raw = self._to_dict(usage)
        if not raw:
            return None
        normalized = {
            "input_tokens": raw.get("input_tokens", 0),
            "output_tokens": raw.get("output_tokens", 0),
            "cache_read_input_tokens": raw.get("cache_read_input_tokens"),
            "cache_creation_input_tokens": raw.get("cache_creation_input_tokens"),
        }
        normalized.update(raw)
        return normalized

    def _extract_user_text(self, content: Any) -> str:
        """提取用户文本内容。"""
        if isinstance(content, str):
            return content
        blocks = self._normalize_content_blocks(content)
        for block in blocks:
            if block.get("type") == "text":
                return str(block.get("text", ""))
        return ""

    def _normalize_content_blocks(self, content: Any) -> List[Dict[str, Any]]:
        """规范化内容块列表。"""
        if content is None:
            return []
        if isinstance(content, str):
            return [{"type": "text", "text": content}]
        if not isinstance(content, list):
            return []

        normalized: List[Dict[str, Any]] = []
        for raw_block in content:
            normalized.append(self._normalize_block(raw_block))
        return normalized

    def _normalize_block(self, block: Any) -> Dict[str, Any]:
        """规范化单个内容块。"""
        raw = self._to_dict(block)
        if not isinstance(raw, dict):
            return {"type": "text", "text": str(raw)}

        if raw.get("type"):
            return raw
        if "thinking" in raw:
            raw["type"] = "thinking"
            return raw
        if "text" in raw:
            raw["type"] = "text"
            return raw
        if "tool_use_id" in raw:
            raw["type"] = "tool_result"
            return raw
        if "id" in raw and "name" in raw and "input" in raw:
            raw["type"] = "tool_use"
            return raw

        raw["type"] = "text"
        raw["text"] = str(raw.get("text", ""))
        return raw

    @staticmethod
    def _merge_assistant_content(existing: Any, incoming: Any) -> List[Dict[str, Any]]:
        """合并 assistant 内容块，保证幂等。"""
        existing_blocks = existing if isinstance(existing, list) else []
        incoming_blocks = incoming if isinstance(incoming, list) else []
        merged: List[Dict[str, Any]] = [dict(block) for block in existing_blocks if isinstance(block, dict)]
        index_map: Dict[str, int] = {}

        for idx, block in enumerate(merged):
            key = ProtocolAdapter._content_block_key(block)
            if key:
                index_map[key] = idx

        for block in incoming_blocks:
            if not isinstance(block, dict):
                continue
            key = ProtocolAdapter._content_block_key(block)
            if not key:
                merged.append(dict(block))
                continue
            existing_index = index_map.get(key)
            if existing_index is None:
                merged.append(dict(block))
                index_map[key] = len(merged) - 1
            else:
                merged[existing_index] = dict(block)

        thinking_index = next((i for i, block in enumerate(merged) if block.get("type") == "thinking"), -1)
        if thinking_index > 0:
            thinking_block = merged.pop(thinking_index)
            merged.insert(0, thinking_block)
        return merged

    @staticmethod
    def _content_block_key(block: Dict[str, Any]) -> Optional[str]:
        """生成内容块幂等键。"""
        block_type = block.get("type")
        if block_type == "thinking":
            return "thinking"
        if block_type == "tool_use" and block.get("id"):
            return f"tool_use:{block['id']}"
        if block_type == "tool_result" and block.get("tool_use_id"):
            return f"tool_result:{block['tool_use_id']}"
        if block_type == "text" and block.get("text"):
            return f"text:{block['text']}"
        return None

    @staticmethod
    def _to_timestamp_ms(value: Optional[datetime]) -> int:
        """将 datetime 转为毫秒时间戳。"""
        if not value:
            return int(datetime.now(timezone.utc).timestamp() * 1000)
        if value.tzinfo is None:
            value = value.replace(tzinfo=timezone.utc)
        return int(value.timestamp() * 1000)

    @staticmethod
    def _to_dict(value: Any) -> Dict[str, Any]:
        """将对象尽可能转换为字典。"""
        if value is None:
            return {}
        if isinstance(value, dict):
            return value
        if hasattr(value, "model_dump"):
            return value.model_dump()
        if is_dataclass(value):
            return asdict(value)
        if hasattr(value, "__dict__"):
            return dict(value.__dict__)
        return {}
