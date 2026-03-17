# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：sdk_message_mapper.py
# @Date   ：2026/03/17 20:05
# @Author ：leemysw
# 2026/03/17 20:05   Create
# =====================================================

"""SDK 消息映射器。"""

from __future__ import annotations

from typing import Any, Dict

from claude_agent_sdk.types import TextBlock, ThinkingBlock, ToolResultBlock, ToolUseBlock


class SdkMessageMapper:
    """负责 SDK 消息与内容块的轻量映射。"""

    @classmethod
    def to_plain_dict(cls, payload: Any) -> Dict[str, Any]:
        """将 SDK 对象转换为普通字典。"""
        if payload is None:
            return {}
        if isinstance(payload, dict):
            return dict(payload)
        if hasattr(payload, "model_dump"):
            return payload.model_dump(mode="json", exclude_none=True)
        if hasattr(payload, "__dict__"):
            return dict(payload.__dict__)
        return {}

    @classmethod
    def normalize_content_blocks(cls, content: Any) -> list[Dict[str, Any]]:
        """将 SDK 内容统一转换为普通字典列表。"""
        if isinstance(content, str):
            return [{"type": "text", "text": content}]
        if isinstance(content, list):
            return [cls.to_plain_block(block) for block in content]
        if content is None:
            return []
        return [{"type": "text", "text": str(content)}]

    @classmethod
    def to_plain_block(cls, block: Any) -> Dict[str, Any]:
        """将 SDK block 转换为普通字典。"""
        if isinstance(block, dict):
            return cls._normalize_block_type(dict(block))
        if isinstance(block, TextBlock):
            return {"type": "text", "text": block.text}
        if isinstance(block, ThinkingBlock):
            return {
                "type": "thinking",
                "thinking": block.thinking,
                "signature": getattr(block, "signature", None),
            }
        if isinstance(block, ToolUseBlock):
            return {
                "type": "tool_use",
                "id": block.id,
                "name": block.name,
                "input": block.input or {},
            }
        if isinstance(block, ToolResultBlock):
            return {
                "type": "tool_result",
                "tool_use_id": block.tool_use_id,
                "content": block.content,
                "is_error": bool(getattr(block, "is_error", False)),
            }

        plain_block = cls.to_plain_dict(block)
        if plain_block:
            return cls._normalize_block_type(plain_block)
        return {"type": "text", "text": str(block)}

    @staticmethod
    def _normalize_block_type(block: Dict[str, Any]) -> Dict[str, Any]:
        """兼容 SDK 新增的服务端工具块类型。"""
        normalized = dict(block)
        block_type = normalized.get("type")
        if block_type == "server_tool_use":
            normalized["type"] = "tool_use"
        elif block_type == "server_tool_result":
            normalized["type"] = "tool_result"
        return normalized
