# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：main_agent_scheduled_task_sdk_server.py
# @Date   ：2026/4/16
# @Author ：Codex
# =====================================================

"""主智能体定时任务 SDK MCP server。"""

from __future__ import annotations

import json
from datetime import datetime
from functools import lru_cache
from typing import Any
from zoneinfo import ZoneInfo

from claude_agent_sdk import create_sdk_mcp_server, tool

from agent.schema.model_automation import (
    AutomationCronSchedule,
    AutomationCronSource,
    AutomationDeliveryTarget,
    AutomationSessionTarget,
)

SERVER_NAME = "nexus_automation"
TOOL_NAMES = (
    "list_scheduled_tasks",
    "create_scheduled_task",
    "update_scheduled_task",
    "delete_scheduled_task",
    "enable_scheduled_task",
    "disable_scheduled_task",
    "run_scheduled_task",
    "get_scheduled_task_runs",
)

_SCHEDULE_SCHEMA = {
    "type": "object",
    "properties": {
        "kind": {"type": "string", "enum": ["every", "cron", "at"]},
        "interval_seconds": {"type": "integer"},
        "cron_expression": {"type": "string"},
        "run_at": {"type": "string"},
        "timezone": {"type": "string"},
    },
    "required": ["kind"],
}
_SESSION_TARGET_SCHEMA = {
    "type": "object",
    "properties": {
        "kind": {"type": "string", "enum": ["isolated", "main", "bound", "named"]},
        "bound_session_key": {"type": "string"},
        "named_session_key": {"type": "string"},
        "wake_mode": {"type": "string", "enum": ["now", "next-heartbeat"]},
    },
    "required": ["kind"],
}
_DELIVERY_SCHEMA = {
    "type": "object",
    "properties": {
        "mode": {"type": "string", "enum": ["none", "last", "explicit"]},
        "channel": {"type": "string"},
        "to": {"type": "string"},
        "account_id": {"type": "string"},
        "thread_id": {"type": "string"},
    },
}
_SOURCE_SCHEMA = {
    "type": "object",
    "properties": {
        "kind": {"type": "string", "enum": ["user_page", "agent", "cli", "system"]},
        "creator_agent_id": {"type": "string"},
        "context_type": {"type": "string", "enum": ["agent", "room"]},
        "context_id": {"type": "string"},
        "context_label": {"type": "string"},
        "session_key": {"type": "string"},
        "session_label": {"type": "string"},
    },
}
_EXECUTION_MODE_SCHEMA = {
    "type": "string",
    "enum": ["main", "existing", "temporary", "dedicated", "current_chat"],
}
_REPLY_MODE_SCHEMA = {
    "type": "string",
    "enum": ["none", "execution", "selected", "current_chat"],
}

_CREATE_TOOL_DESCRIPTION = (
    "创建新的定时任务。若用户没有明确提供执行方式、结果回传方式或时区，"
    "必须先使用 AskUserQuestion 补问，禁止直接套默认参数。"
    "只有非常简单、短文本、一次一条的提醒/播报类任务，才允许默认按 temporary + none 创建。"
    "优先使用和页面一致的 execution_mode / reply_mode 语义，不要直接组合底层 session_target / delivery 细节。"
)

_CONTEXT_HEAVY_KEYWORDS = (
    "总结",
    "汇总",
    "简报",
    "报告",
    "跟进",
    "复盘",
    "检查",
    "分析",
    "研究",
    "整理",
    "回顾",
    "监控",
)


def _format_datetime_for_timezone(value: str, timezone: str) -> str | None:
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        localized = parsed.astimezone(ZoneInfo(timezone))
    except Exception:
        return None
    return localized.strftime("%Y-%m-%d %H:%M:%S %Z")


def _with_display_times(payload: object, timezone: str = "Asia/Shanghai") -> object:
    if isinstance(payload, list):
        return [_with_display_times(item, timezone=timezone) for item in payload]
    if not isinstance(payload, dict):
        return payload

    resolved_timezone = timezone
    schedule = payload.get("schedule")
    if isinstance(schedule, dict):
        schedule_timezone = schedule.get("timezone")
        if isinstance(schedule_timezone, str) and schedule_timezone.strip():
            resolved_timezone = schedule_timezone.strip()

    result = dict(payload)
    for key in ("next_run_at", "last_run_at", "scheduled_for", "started_at", "finished_at"):
        raw_value = result.get(key)
        if isinstance(raw_value, str) and raw_value.strip():
            display_value = _format_datetime_for_timezone(raw_value, resolved_timezone)
            if display_value:
                result[f"{key}_display"] = display_value
    return result


def _json_content(payload: object) -> dict[str, object]:
    normalized_payload = _with_display_times(payload)
    return {
        "content": [
            {
                "type": "text",
                "text": json.dumps(normalized_payload, ensure_ascii=False),
            }
        ]
    }


def _build_schedule(raw_value: object) -> AutomationCronSchedule | None:
    if not isinstance(raw_value, dict):
        return None
    return AutomationCronSchedule(**raw_value)


def _build_session_target(raw_value: object) -> AutomationSessionTarget | None:
    if not isinstance(raw_value, dict):
        return None
    return AutomationSessionTarget(**raw_value)


def _build_delivery(raw_value: object) -> AutomationDeliveryTarget | None:
    if not isinstance(raw_value, dict):
        return None
    return AutomationDeliveryTarget(**raw_value)


def _build_source(raw_value: object) -> AutomationCronSource | None:
    if not isinstance(raw_value, dict):
        return None
    return AutomationCronSource(**raw_value)


def _can_default_to_temporary_none(args: dict[str, Any]) -> bool:
    instruction = str(args.get("instruction") or "").strip()
    if not instruction or len(instruction) > 24:
        return False
    if any(keyword in instruction for keyword in _CONTEXT_HEAVY_KEYWORDS):
        return False
    schedule = args.get("schedule")
    if not isinstance(schedule, dict):
        return False
    kind = str(schedule.get("kind") or "").strip()
    if kind not in {"every", "at", "cron"}:
        return False
    if kind == "every":
        interval_seconds = schedule.get("interval_seconds")
        if not isinstance(interval_seconds, int) or interval_seconds <= 0:
            return False
    if kind == "cron" and not str(schedule.get("cron_expression") or "").strip():
        return False
    if kind == "at" and not str(schedule.get("run_at") or "").strip():
        return False
    return True


def _resolve_session_target_from_mode(
    *,
    execution_mode: str | None,
    named_session_key: str | None,
    selected_session_key: str | None,
    current_session_key: str | None,
) -> AutomationSessionTarget | None:
    if not execution_mode:
        return None
    if execution_mode == "main":
        return AutomationSessionTarget(kind="main", wake_mode="next-heartbeat")
    if execution_mode in {"existing", "current_chat"}:
        bound_session_key = selected_session_key or current_session_key
        if not bound_session_key:
            raise ValueError(
                "execution_mode=existing requires selected_session_key or an active current session. "
                "Use AskUserQuestion to ask which existing session should execute the task."
            )
        return AutomationSessionTarget(kind="bound", bound_session_key=bound_session_key)
    if execution_mode == "temporary":
        return AutomationSessionTarget(kind="isolated", wake_mode="next-heartbeat")
    if execution_mode == "dedicated":
        if not named_session_key:
            raise ValueError(
                "execution_mode=dedicated requires named_session_key. "
                "Use AskUserQuestion to confirm a dedicated session name first."
            )
        return AutomationSessionTarget(kind="named", named_session_key=named_session_key)
    raise ValueError(f"unsupported execution_mode: {execution_mode}")


def _resolve_delivery_from_mode(
    *,
    reply_mode: str | None,
    execution_mode: str | None,
    selected_reply_session_key: str | None,
    selected_session_key: str | None,
    current_session_key: str | None,
    session_target: AutomationSessionTarget | None,
    source_context_type: str | None,
) -> AutomationDeliveryTarget | None:
    if not reply_mode:
        return None
    if reply_mode == "none":
        return AutomationDeliveryTarget(mode="none")
    if reply_mode == "current_chat":
        if not current_session_key:
            raise ValueError(
                "reply_mode=current_chat requires an active current session. "
                "Use AskUserQuestion to ask which existing session should receive the result."
            )
        return AutomationDeliveryTarget(mode="explicit", channel="websocket", to=current_session_key)
    if reply_mode == "execution":
        if session_target is not None and session_target.kind == "main":
            return AutomationDeliveryTarget(mode="none")
        # 中文注释：页面里的“回到执行会话”不是“总是回当前对话”。
        # 对 temporary / dedicated 这类 automation 会话，结果默认留在执行会话里；
        # 只有 existing，或 Room 语义下需要回到所选 websocket 会话时，才显式投递。
        resolved_execution_mode = execution_mode
        if not resolved_execution_mode and session_target is not None:
            resolved_execution_mode = {
                "bound": "existing",
                "isolated": "temporary",
                "named": "dedicated",
                "main": "main",
            }.get(session_target.kind)
        if resolved_execution_mode in {"temporary", "dedicated"} and source_context_type != "room":
            return AutomationDeliveryTarget(mode="none")
        reply_session_key = selected_session_key or current_session_key
        if not reply_session_key:
            raise ValueError(
                "reply_mode=execution requires selected_session_key or an active current session. "
                "Use AskUserQuestion to ask which execution session should receive the result."
            )
        return AutomationDeliveryTarget(mode="explicit", channel="websocket", to=reply_session_key)
    if reply_mode == "selected":
        if not selected_reply_session_key:
            raise ValueError(
                "reply_mode=selected requires selected_reply_session_key. "
                "Use AskUserQuestion to ask which existing session should receive the result."
            )
        return AutomationDeliveryTarget(mode="explicit", channel="websocket", to=selected_reply_session_key)
    raise ValueError(f"unsupported reply_mode: {reply_mode}")


def _resolve_bound_session_target(
    raw_value: object,
    *,
    current_session_key: str | None,
) -> AutomationSessionTarget | None:
    if not isinstance(raw_value, dict):
        return None
    payload = dict(raw_value)
    if payload.get("kind") == "bound" and not payload.get("bound_session_key") and current_session_key:
        payload["bound_session_key"] = current_session_key
    return AutomationSessionTarget(**payload)


def _resolve_source_context_type(raw_value: object) -> str | None:
    if not isinstance(raw_value, dict):
        return None
    context_type = str(raw_value.get("context_type") or "").strip()
    return context_type or None


def _resolve_delivery_target(
    raw_value: object,
    *,
    current_session_key: str | None,
) -> AutomationDeliveryTarget | None:
    if not isinstance(raw_value, dict):
        return None
    payload = dict(raw_value)
    if payload.get("mode") == "explicit" and not payload.get("to") and current_session_key:
        payload["channel"] = payload.get("channel") or "websocket"
        payload["to"] = current_session_key
    return AutomationDeliveryTarget(**payload)


def _validate_page_semantics(
    *,
    session_target: AutomationSessionTarget | None,
    delivery: AutomationDeliveryTarget | None,
    execution_mode: str | None,
    reply_mode: str | None,
) -> None:
    if delivery and delivery.mode == "last":
        raise ValueError(
            "delivery.mode=last is not supported by the scheduled-task page semantics. "
            "Use AskUserQuestion and choose none/execution/current_chat/selected explicitly."
        )
    normalized_execution_mode = str(execution_mode or "").strip() or None
    normalized_reply_mode = str(reply_mode or "").strip() or None
    if normalized_execution_mode == "main" and normalized_reply_mode not in {None, "", "none"}:
        raise ValueError(
            "execution_mode=main does not support reply_mode under page semantics. "
            "To run independently and send the result back here, use temporary + current_chat."
        )
    if session_target and session_target.kind == "main" and delivery and delivery.mode != "none":
        raise ValueError(
            "session_target.kind=main cannot be combined with delivery.mode!=none under page semantics. "
            "To run independently and send the result back here, use temporary + current_chat."
        )


async def _resolve_source_snapshot(
    *,
    service,
    raw_source: object,
    agent_id: str,
    current_session_key: str | None,
    current_session_label: str | None,
) -> AutomationCronSource | None:
    source = _build_source(raw_source)
    current_agent_name = agent_id
    try:
        agent_payload = await service.get_agent(agent_id)
        current_agent_name = str(agent_payload.get("name") or agent_id)
    except Exception:
        current_agent_name = agent_id

    if source is None:
        return AutomationCronSource(
            kind="agent",
            context_type="agent",
            context_id=agent_id,
            context_label=current_agent_name,
            session_key=current_session_key,
            session_label=current_session_label or ("当前对话" if current_session_key else None),
        )

    if source.context_type == "agent" and source.context_id and not source.context_label:
        source.context_label = current_agent_name if source.context_id == agent_id else source.context_id
    elif source.context_type == "agent" and not source.context_id:
        source.context_id = agent_id
        source.context_label = current_agent_name

    if not source.session_key and current_session_key:
        source.session_key = current_session_key
    if not source.session_label and current_session_key:
        source.session_label = current_session_label or "当前对话"
    return source


def _require_explicit_create_fields(args: dict[str, Any]) -> None:
    schedule = args.get("schedule")
    if not isinstance(schedule, dict):
        raise ValueError("schedule is required")

    missing_fields: list[str] = []
    allow_simple_defaults = _can_default_to_temporary_none(args)
    has_session_target = isinstance(args.get("session_target"), dict) or bool(str(args.get("execution_mode") or "").strip())
    has_delivery = isinstance(args.get("delivery"), dict) or bool(str(args.get("reply_mode") or "").strip())
    if not has_session_target and not allow_simple_defaults:
        missing_fields.append("session_target")
    if not has_delivery and not allow_simple_defaults:
        missing_fields.append("delivery")
    timezone_value = schedule.get("timezone")
    if not isinstance(timezone_value, str) or not timezone_value.strip():
        missing_fields.append("schedule.timezone")

    if missing_fields:
        missing_text = ", ".join(missing_fields)
        raise ValueError(
            f"missing required scheduling fields: {missing_text}. "
            "Do not assume defaults; use AskUserQuestion to confirm them with the user first."
        )


def _apply_simple_defaults(args: dict[str, Any]) -> dict[str, Any]:
    payload = dict(args)
    if not _can_default_to_temporary_none(payload):
        return payload
    if not isinstance(payload.get("session_target"), dict) and not str(payload.get("execution_mode") or "").strip():
        payload["execution_mode"] = "temporary"
    if not isinstance(payload.get("delivery"), dict) and not str(payload.get("reply_mode") or "").strip():
        payload["reply_mode"] = "none"
    return payload


def create_main_agent_scheduled_task_sdk_server(
    service=None,
    *,
    current_session_key: str | None = None,
    current_session_label: str | None = None,
    current_agent_id: str | None = None,
):
    """构建主智能体可直接调用的定时任务工具集。"""
    def get_service():
        if service is not None:
            return service
        from agent.service.agent.main_agent_orchestration_service import (
            main_agent_orchestration_service,
        )

        return main_agent_orchestration_service

    @tool(
        "list_scheduled_tasks",
        "列出某个智能体或全部定时任务。",
        {"type": "object", "properties": {"agent_id": {"type": "string"}}},
    )
    async def list_scheduled_tasks_tool(args: dict[str, Any]):
        return _json_content(await get_service().list_scheduled_tasks(agent_id=args.get("agent_id") or None))

    @tool(
        "create_scheduled_task",
        _CREATE_TOOL_DESCRIPTION,
        {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "agent_id": {"type": "string"},
                "instruction": {"type": "string"},
                "schedule": _SCHEDULE_SCHEMA,
                "execution_mode": _EXECUTION_MODE_SCHEMA,
                "reply_mode": _REPLY_MODE_SCHEMA,
                "named_session_key": {"type": "string"},
                "selected_session_key": {"type": "string"},
                "selected_reply_session_key": {"type": "string"},
                "session_target": _SESSION_TARGET_SCHEMA,
                "delivery": _DELIVERY_SCHEMA,
                "source": _SOURCE_SCHEMA,
                "enabled": {"type": "boolean"},
            },
            "required": ["name", "instruction", "schedule"],
        },
    )
    async def create_scheduled_task_tool(args: dict[str, Any]):
        normalized_args = _apply_simple_defaults(args)
        _require_explicit_create_fields(normalized_args)
        schedule = _build_schedule(normalized_args.get("schedule"))
        if schedule is None:
            raise ValueError("schedule is required")
        resolved_agent_id = str(normalized_args.get("agent_id") or current_agent_id or "").strip()
        if not resolved_agent_id:
            raise ValueError("agent_id is required")
        session_target = _resolve_bound_session_target(
            normalized_args.get("session_target"),
            current_session_key=current_session_key,
        ) or _resolve_session_target_from_mode(
            execution_mode=str(normalized_args.get("execution_mode") or "").strip() or None,
            named_session_key=str(normalized_args.get("named_session_key") or "").strip() or None,
            selected_session_key=str(normalized_args.get("selected_session_key") or "").strip() or None,
            current_session_key=current_session_key,
        )
        delivery = _resolve_delivery_target(
            normalized_args.get("delivery"),
            current_session_key=current_session_key,
        ) or _resolve_delivery_from_mode(
            reply_mode=str(normalized_args.get("reply_mode") or "").strip() or None,
            execution_mode=str(normalized_args.get("execution_mode") or "").strip() or None,
            selected_reply_session_key=str(normalized_args.get("selected_reply_session_key") or "").strip() or None,
            selected_session_key=str(normalized_args.get("selected_session_key") or "").strip() or None,
            current_session_key=current_session_key,
            session_target=session_target,
            source_context_type=_resolve_source_context_type(normalized_args.get("source")),
        )
        _validate_page_semantics(
            session_target=session_target,
            delivery=delivery,
            execution_mode=str(normalized_args.get("execution_mode") or "").strip() or None,
            reply_mode=str(normalized_args.get("reply_mode") or "").strip() or None,
        )
        payload = await get_service().create_scheduled_task(
            name=str(args["name"]),
            agent_id=resolved_agent_id,
            instruction=str(normalized_args["instruction"]),
            session_target=session_target,
            source=await _resolve_source_snapshot(
                service=get_service(),
                raw_source=normalized_args.get("source"),
                agent_id=resolved_agent_id,
                current_session_key=current_session_key,
                current_session_label=current_session_label,
            ),
            delivery=delivery,
            schedule_kind=schedule.kind,
            interval_seconds=schedule.interval_seconds,
            cron_expression=schedule.cron_expression,
            run_at=schedule.run_at,
            timezone=schedule.timezone,
            enabled=bool(args.get("enabled", True)),
        )
        return _json_content(payload)

    @tool(
        "update_scheduled_task",
        "更新已有定时任务。优先使用和页面一致的 execution_mode / reply_mode 语义；不要混用页面没有的 last 模式。",
        {
            "type": "object",
            "properties": {
                "job_id": {"type": "string"},
                "name": {"type": "string"},
                "agent_id": {"type": "string"},
                "instruction": {"type": "string"},
                "schedule": _SCHEDULE_SCHEMA,
                "execution_mode": _EXECUTION_MODE_SCHEMA,
                "reply_mode": _REPLY_MODE_SCHEMA,
                "named_session_key": {"type": "string"},
                "selected_session_key": {"type": "string"},
                "selected_reply_session_key": {"type": "string"},
                "session_target": _SESSION_TARGET_SCHEMA,
                "delivery": _DELIVERY_SCHEMA,
                "source": _SOURCE_SCHEMA,
                "enabled": {"type": "boolean"},
            },
            "required": ["job_id"],
        },
    )
    async def update_scheduled_task_tool(args: dict[str, Any]):
        schedule = _build_schedule(args.get("schedule")) if "schedule" in args else None
        resolved_agent_id = str(args.get("agent_id") or current_agent_id or "").strip() or None
        session_target = _resolve_bound_session_target(
            args.get("session_target"),
            current_session_key=current_session_key,
        ) or _resolve_session_target_from_mode(
            execution_mode=str(args.get("execution_mode") or "").strip() or None,
            named_session_key=str(args.get("named_session_key") or "").strip() or None,
            selected_session_key=str(args.get("selected_session_key") or "").strip() or None,
            current_session_key=current_session_key,
        )
        delivery = _resolve_delivery_target(
            args.get("delivery"),
            current_session_key=current_session_key,
        ) or _resolve_delivery_from_mode(
            reply_mode=str(args.get("reply_mode") or "").strip() or None,
            execution_mode=str(args.get("execution_mode") or "").strip() or None,
            selected_reply_session_key=str(args.get("selected_reply_session_key") or "").strip() or None,
            selected_session_key=str(args.get("selected_session_key") or "").strip() or None,
            current_session_key=current_session_key,
            session_target=session_target,
            source_context_type=_resolve_source_context_type(args.get("source")),
        )
        _validate_page_semantics(
            session_target=session_target,
            delivery=delivery,
            execution_mode=str(args.get("execution_mode") or "").strip() or None,
            reply_mode=str(args.get("reply_mode") or "").strip() or None,
        )
        payload = await get_service().update_scheduled_task(
            job_id=str(args["job_id"]),
            name=args.get("name"),
            agent_id=resolved_agent_id,
            instruction=args.get("instruction"),
            schedule=schedule,
            session_target=session_target,
            delivery=delivery,
            source=await _resolve_source_snapshot(
                service=get_service(),
                raw_source=args.get("source"),
                agent_id=resolved_agent_id or current_agent_id or "",
                current_session_key=current_session_key,
                current_session_label=current_session_label,
            ) if ("source" in args or resolved_agent_id or current_agent_id) else None,
            enabled=args.get("enabled"),
        )
        return _json_content(payload)

    @tool("delete_scheduled_task", "删除定时任务。", {"job_id": str})
    async def delete_scheduled_task_tool(args: dict[str, Any]):
        return _json_content(await get_service().delete_scheduled_task(str(args["job_id"])))

    @tool("enable_scheduled_task", "启用定时任务。", {"job_id": str})
    async def enable_scheduled_task_tool(args: dict[str, Any]):
        return _json_content(await get_service().set_scheduled_task_enabled(str(args["job_id"]), enabled=True))

    @tool("disable_scheduled_task", "禁用定时任务。", {"job_id": str})
    async def disable_scheduled_task_tool(args: dict[str, Any]):
        return _json_content(await get_service().set_scheduled_task_enabled(str(args["job_id"]), enabled=False))

    @tool("run_scheduled_task", "立即运行一次定时任务。", {"job_id": str})
    async def run_scheduled_task_tool(args: dict[str, Any]):
        return _json_content(await get_service().run_scheduled_task(str(args["job_id"])))

    @tool("get_scheduled_task_runs", "读取定时任务运行记录。", {"job_id": str})
    async def get_scheduled_task_runs_tool(args: dict[str, Any]):
        return _json_content(await get_service().get_scheduled_task_runs(str(args["job_id"])))

    return create_sdk_mcp_server(
        name=SERVER_NAME,
        tools=[
            list_scheduled_tasks_tool,
            create_scheduled_task_tool,
            update_scheduled_task_tool,
            delete_scheduled_task_tool,
            enable_scheduled_task_tool,
            disable_scheduled_task_tool,
            run_scheduled_task_tool,
            get_scheduled_task_runs_tool,
        ],
    )


@lru_cache(maxsize=1)
def get_main_agent_scheduled_task_sdk_server():
    """缓存主智能体定时任务工具 server，避免重复构建。"""
    return create_main_agent_scheduled_task_sdk_server()
