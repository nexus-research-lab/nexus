# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：permission_runtime.py
# @Date   ：2026/3/11 15:43
# @Author ：leemysw
# 2026/3/11 15:43   Create
# =====================================================

"""权限运行时辅助对象。

[INPUT]: 依赖 claude_agent_sdk 的 PermissionUpdate/PermissionRuleValue
[OUTPUT]: 对外提供权限请求状态、建议序列化与风险摘要能力
[POS]: channel 模块的权限辅助层，被 WebSocket 权限策略消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from __future__ import annotations

import asyncio
from dataclasses import asdict, dataclass, is_dataclass
from datetime import datetime, timedelta
from typing import Any, Iterable, Optional

from claude_agent_sdk import PermissionUpdate
from claude_agent_sdk.types import PermissionRuleValue


@dataclass
class PendingPermissionRequest:
    """挂起中的权限请求。"""

    request_id: str
    session_key: str
    tool_name: str
    input_data: dict[str, Any]
    event: asyncio.Event
    expires_at: datetime


class PermissionUpdateCodec:
    """权限更新对象与前端 JSON 之间的转换器。"""

    @classmethod
    def serialize_updates(
            cls,
            updates: Optional[Iterable[Any]],
    ) -> list[dict[str, Any]]:
        """将 SDK 返回的权限建议转换为可序列化结构。"""
        serialized: list[dict[str, Any]] = []
        for update in updates or []:
            normalized = cls._serialize_single(update)
            if normalized:
                serialized.append(normalized)
        return serialized

    @classmethod
    def deserialize_updates(
            cls,
            updates: Optional[list[dict[str, Any]]],
    ) -> list[PermissionUpdate]:
        """将前端回传的权限更新恢复为 SDK 对象。"""
        result: list[PermissionUpdate] = []
        for item in updates or []:
            update_type = item.get("type")
            if not update_type:
                continue

            rules: list[PermissionRuleValue] | None = None
            raw_rules = item.get("rules")
            if isinstance(raw_rules, list):
                rules = []
                for raw_rule in raw_rules:
                    if not isinstance(raw_rule, dict):
                        continue
                    tool_name = raw_rule.get("toolName") or raw_rule.get("tool_name")
                    if not tool_name:
                        continue
                    rules.append(
                        PermissionRuleValue(
                            tool_name=tool_name,
                            rule_content=raw_rule.get("ruleContent") or raw_rule.get("rule_content"),
                        )
                    )

            result.append(
                PermissionUpdate(
                    type=update_type,
                    rules=rules,
                    behavior=item.get("behavior"),
                    mode=item.get("mode"),
                    directories=item.get("directories"),
                    destination=item.get("destination"),
                )
            )
        return result

    @classmethod
    def _serialize_single(cls, update: Any) -> dict[str, Any] | None:
        """序列化单个权限更新。"""
        if isinstance(update, PermissionUpdate):
            return update.to_dict()

        if is_dataclass(update):
            raw = asdict(update)
            return cls._normalize_update_dict(raw)

        if isinstance(update, dict):
            return cls._normalize_update_dict(update)

        return None

    @classmethod
    def _normalize_update_dict(cls, raw: dict[str, Any]) -> dict[str, Any]:
        """统一权限更新字段命名。"""
        result: dict[str, Any] = {"type": raw.get("type")}
        for key in ("behavior", "mode", "destination"):
            if raw.get(key) is not None:
                result[key] = raw.get(key)

        if raw.get("directories") is not None:
            result["directories"] = raw.get("directories")

        rules = raw.get("rules")
        if isinstance(rules, list):
            normalized_rules = []
            for rule in rules:
                if isinstance(rule, PermissionRuleValue):
                    normalized_rules.append(
                        {
                            "toolName": rule.tool_name,
                            "ruleContent": rule.rule_content,
                        }
                    )
                    continue

                if is_dataclass(rule):
                    rule = asdict(rule)

                if isinstance(rule, dict):
                    tool_name = rule.get("toolName") or rule.get("tool_name")
                    if not tool_name:
                        continue
                    normalized_rules.append(
                        {
                            "toolName": tool_name,
                            "ruleContent": rule.get("ruleContent") or rule.get("rule_content"),
                        }
                    )

            if normalized_rules:
                result["rules"] = normalized_rules

        return result


class PermissionRequestPresenter:
    """构建发送给前端的权限请求展示数据。"""

    READ_ONLY_TOOLS = {
        "Read",
        "Glob",
        "Grep",
        "LS",
        "WebFetch",
        "WebSearch",
        "Skill",
    }
    EDIT_TOOLS = {"Edit", "Write", "NotebookEdit", "TodoWrite"}
    EXECUTE_TOOLS = {"Bash", "KillShell", "Task", "TaskOutput"}
    INTERACTIVE_TOOLS = {"AskUserQuestion", "EnterPlanMode", "ExitPlanMode"}

    @classmethod
    def build_payload(
            cls,
            request: PendingPermissionRequest,
            suggestion_updates: list[dict[str, Any]],
    ) -> dict[str, Any]:
        """构建前端展示所需的权限请求载荷。"""
        risk_level, risk_label = cls._resolve_risk(request.tool_name)
        return {
            "request_id": request.request_id,
            "tool_name": request.tool_name,
            "tool_input": request.input_data,
            "risk_level": risk_level,
            "risk_label": risk_label,
            "summary": cls._summarize_input(request.tool_name, request.input_data),
            "suggestions": suggestion_updates,
            "expires_at": request.expires_at.isoformat(),
        }

    @classmethod
    def _resolve_risk(cls, tool_name: str) -> tuple[str, str]:
        """根据工具名给出风险级别。"""
        if tool_name in cls.READ_ONLY_TOOLS:
            return "low", "只读"
        if tool_name in cls.EDIT_TOOLS:
            return "medium", "写入"
        if tool_name in cls.EXECUTE_TOOLS:
            return "high", "执行"
        if tool_name in cls.INTERACTIVE_TOOLS:
            return "medium", "交互"
        return "high", "敏感"

    @classmethod
    def _summarize_input(cls, tool_name: str, input_data: dict[str, Any]) -> str:
        """提取适合在权限面板显示的摘要。"""
        if tool_name == "Bash":
            command = input_data.get("command")
            if isinstance(command, str) and command.strip():
                return command.strip()

        for key in ("file_path", "path", "target_file", "cwd", "url", "query"):
            value = input_data.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()

        if tool_name == "AskUserQuestion":
            questions = input_data.get("questions")
            if isinstance(questions, list) and questions:
                first_question = questions[0]
                if isinstance(first_question, dict):
                    question_text = first_question.get("question")
                    if isinstance(question_text, str) and question_text.strip():
                        return question_text.strip()

        for key in ("description", "task", "prompt"):
            value = input_data.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()

        return tool_name

    @staticmethod
    def build_expiry(timeout_seconds: float) -> datetime:
        """根据超时时间生成过期时间。"""
        return datetime.now() + timedelta(seconds=timeout_seconds)
