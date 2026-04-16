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
from functools import lru_cache
from typing import Any

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


def _json_content(payload: object) -> dict[str, object]:
    return {
        "content": [
            {
                "type": "text",
                "text": json.dumps(payload, ensure_ascii=False),
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


def create_main_agent_scheduled_task_sdk_server(service=None):
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
        "创建新的定时任务。",
        {
            "type": "object",
            "properties": {
                "name": {"type": "string"},
                "agent_id": {"type": "string"},
                "instruction": {"type": "string"},
                "schedule": _SCHEDULE_SCHEMA,
                "session_target": _SESSION_TARGET_SCHEMA,
                "delivery": _DELIVERY_SCHEMA,
                "source": _SOURCE_SCHEMA,
                "enabled": {"type": "boolean"},
            },
            "required": ["name", "agent_id", "instruction", "schedule"],
        },
    )
    async def create_scheduled_task_tool(args: dict[str, Any]):
        schedule = _build_schedule(args.get("schedule"))
        if schedule is None:
            raise ValueError("schedule is required")
        payload = await get_service().create_scheduled_task(
            name=str(args["name"]),
            agent_id=str(args["agent_id"]),
            instruction=str(args["instruction"]),
            session_target=_build_session_target(args.get("session_target")),
            source=_build_source(args.get("source")),
            delivery=_build_delivery(args.get("delivery")),
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
        "更新已有定时任务。",
        {
            "type": "object",
            "properties": {
                "job_id": {"type": "string"},
                "name": {"type": "string"},
                "agent_id": {"type": "string"},
                "instruction": {"type": "string"},
                "schedule": _SCHEDULE_SCHEMA,
                "session_target": _SESSION_TARGET_SCHEMA,
                "delivery": _DELIVERY_SCHEMA,
                "source": _SOURCE_SCHEMA,
                "enabled": {"type": "boolean"},
            },
            "required": ["job_id"],
        },
    )
    async def update_scheduled_task_tool(args: dict[str, Any]):
        payload = await get_service().update_scheduled_task(
            job_id=str(args["job_id"]),
            name=args.get("name"),
            agent_id=args.get("agent_id"),
            instruction=args.get("instruction"),
            schedule=_build_schedule(args.get("schedule")),
            session_target=_build_session_target(args.get("session_target")),
            delivery=_build_delivery(args.get("delivery")),
            source=_build_source(args.get("source")),
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
