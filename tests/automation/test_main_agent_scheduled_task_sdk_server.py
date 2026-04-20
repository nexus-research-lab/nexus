from __future__ import annotations

import asyncio
import json

from mcp.types import CallToolRequest, CallToolRequestParams, ListToolsRequest

from agent.service.agent.main_agent_scheduled_task_sdk_server import (
    create_main_agent_scheduled_task_sdk_server,
)


class FakeOrchestrationService:
    """记录 SDK tool 对主智能体编排 service 的调用。"""

    def __init__(self) -> None:
        self.calls: list[tuple[str, dict]] = []

    async def list_scheduled_tasks(self, agent_id: str | None = None):
        self.calls.append(("list_scheduled_tasks", {"agent_id": agent_id}))
        return [{
            "job_id": "job-1",
            "agent_id": agent_id or "nexus",
            "schedule": {"kind": "every", "interval_seconds": 300, "timezone": "Asia/Shanghai"},
            "next_run_at": "2026-04-16T01:50:04Z",
        }]

    async def create_scheduled_task(
        self,
        *,
        name: str,
        agent_id: str,
        instruction: str,
        session_target=None,
        source=None,
        delivery=None,
        schedule_kind: str,
        interval_seconds: int | None = None,
        cron_expression: str | None = None,
        run_at: str | None = None,
        timezone: str = "Asia/Shanghai",
        enabled: bool = True,
    ):
        self.calls.append(
            (
                "create_scheduled_task",
                {
                    "name": name,
                    "agent_id": agent_id,
                    "instruction": instruction,
                    "session_target": session_target,
                    "source": source,
                    "delivery": delivery,
                    "schedule_kind": schedule_kind,
                    "interval_seconds": interval_seconds,
                    "cron_expression": cron_expression,
                    "run_at": run_at,
                    "timezone": timezone,
                    "enabled": enabled,
                },
            )
        )
        return {"job_id": "job-1"}

    async def get_agent(self, agent_id: str):
        return {"agent_id": agent_id, "name": "Agent"}

    async def update_scheduled_task(
        self,
        *,
        job_id: str,
        name: str | None = None,
        agent_id: str | None = None,
        instruction: str | None = None,
        schedule=None,
        session_target=None,
        delivery=None,
        source=None,
        enabled: bool | None = None,
    ):
        self.calls.append(
            (
                "update_scheduled_task",
                {
                    "job_id": job_id,
                    "name": name,
                    "agent_id": agent_id,
                    "instruction": instruction,
                    "schedule": schedule,
                    "session_target": session_target,
                    "delivery": delivery,
                    "source": source,
                    "enabled": enabled,
                },
            )
        )
        return {"job_id": job_id}


def test_sdk_server_lists_scheduled_task_tools():
    async def scenario():
        config = create_main_agent_scheduled_task_sdk_server(service=FakeOrchestrationService())
        server = config["instance"]
        handler = server.request_handlers[ListToolsRequest]

        result = await handler(ListToolsRequest(method="tools/list"))
        tool_names = [tool.name for tool in result.root.tools]

        assert "list_scheduled_tasks" in tool_names
        assert "create_scheduled_task" in tool_names
        assert "run_scheduled_task" in tool_names

    asyncio.run(scenario())


def test_sdk_server_list_tool_adds_localized_display_time():
    async def scenario():
        config = create_main_agent_scheduled_task_sdk_server(service=FakeOrchestrationService())
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(name="list_scheduled_tasks", arguments={}),
            )
        )

        payload = json.loads(result.root.content[0].text)
        assert payload[0]["next_run_at"] == "2026-04-16T01:50:04Z"
        assert payload[0]["next_run_at_display"] == "2026-04-16 09:50:04 CST"

    asyncio.run(scenario())


def test_sdk_server_create_tool_delegates_to_orchestration_service():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "morning brief",
                        "agent_id": "research",
                        "instruction": "summarize updates",
                        "schedule": {"kind": "every", "interval_seconds": 300, "timezone": "Asia/Shanghai"},
                        "session_target": {"kind": "main", "wake_mode": "next-heartbeat"},
                        "delivery": {"mode": "none"},
                        "enabled": True,
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert service.calls[0][0] == "create_scheduled_task"
        assert payload["schedule_kind"] == "every"
        assert payload["interval_seconds"] == 300
        assert payload["session_target"].kind == "main"
        assert json.loads(result.root.content[0].text) == {"job_id": "job-1"}

    asyncio.run(scenario())


def test_sdk_server_create_tool_rejects_missing_explicit_fields():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "morning brief",
                        "agent_id": "research",
                        "instruction": "summarize updates",
                        "schedule": {"kind": "every", "interval_seconds": 300},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "AskUserQuestion" in result.root.content[0].text
        assert "schedule.timezone" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_create_tool_defaults_simple_task_to_temporary_and_no_reply():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_agent_id="71b0d4147311",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "instruction": "说你好",
                        "schedule": {"kind": "every", "interval_seconds": 60, "timezone": "Asia/Shanghai"},
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert payload["agent_id"] == "71b0d4147311"
        assert payload["session_target"].kind == "isolated"
        assert payload["delivery"].mode == "none"

    asyncio.run(scenario())


def test_sdk_server_create_tool_rejects_context_heavy_task_without_modes():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "morning brief",
                        "agent_id": "research",
                        "instruction": "总结今天项目进展",
                        "schedule": {"kind": "every", "interval_seconds": 300, "timezone": "Asia/Shanghai"},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "AskUserQuestion" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_create_tool_uses_current_session_for_bound_and_explicit_delivery():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_session_key="agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8",
            current_session_label="New Chat",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "agent_id": "71b0d4147311",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "session_target": {"kind": "bound"},
                        "delivery": {"mode": "explicit", "channel": "websocket"},
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert payload["session_target"].kind == "bound"
        assert payload["session_target"].bound_session_key == "agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8"
        assert payload["delivery"].mode == "explicit"
        assert payload["delivery"].to == "agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8"
        assert payload["source"].context_label == "Agent"
        assert payload["source"].session_label == "New Chat"

    asyncio.run(scenario())


def test_sdk_server_create_tool_supports_ui_style_execution_and_reply_modes():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_session_key="agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8",
            current_session_label="New Chat",
            current_agent_id="71b0d4147311",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "execution_mode": "existing",
                        "reply_mode": "execution",
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert payload["agent_id"] == "71b0d4147311"
        assert payload["session_target"].kind == "bound"
        assert payload["delivery"].mode == "explicit"
        assert payload["source"].session_label == "New Chat"

    asyncio.run(scenario())


def test_sdk_server_create_tool_maps_temporary_execution_reply_to_none_for_agent_page_semantics():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_session_key="agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8",
            current_session_label="New Chat",
            current_agent_id="71b0d4147311",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "execution_mode": "temporary",
                        "reply_mode": "execution",
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert payload["session_target"].kind == "isolated"
        assert payload["delivery"].mode == "none"

    asyncio.run(scenario())


def test_sdk_server_create_tool_rejects_main_execution_with_current_chat_reply():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_session_key="agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8",
            current_session_label="New Chat",
            current_agent_id="71b0d4147311",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "execution_mode": "main",
                        "reply_mode": "current_chat",
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "temporary + current_chat" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_create_tool_rejects_last_delivery_mode_for_page_parity():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "agent_id": "71b0d4147311",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "session_target": {"kind": "main", "wake_mode": "next-heartbeat"},
                        "delivery": {"mode": "last"},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "page semantics" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_create_tool_rejects_main_session_target_with_explicit_delivery():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="create_scheduled_task",
                    arguments={
                        "name": "test",
                        "agent_id": "71b0d4147311",
                        "instruction": "学狗叫",
                        "schedule": {"kind": "every", "interval_seconds": 1, "timezone": "Asia/Shanghai"},
                        "session_target": {"kind": "main", "wake_mode": "next-heartbeat"},
                        "delivery": {"mode": "explicit", "channel": "websocket", "to": "agent:71b0d4147311:ws:dm:chat"},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "temporary + current_chat" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_update_tool_supports_ui_style_execution_and_reply_modes():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(
            service=service,
            current_session_key="agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8",
            current_session_label="New Chat",
            current_agent_id="71b0d4147311",
        )
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="update_scheduled_task",
                    arguments={
                        "job_id": "job-1",
                        "execution_mode": "existing",
                        "reply_mode": "execution",
                    },
                ),
            )
        )

        payload = service.calls[0][1]
        assert service.calls[0][0] == "update_scheduled_task"
        assert payload["session_target"].kind == "bound"
        assert payload["session_target"].bound_session_key == "agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8"
        assert payload["delivery"].mode == "explicit"
        assert payload["delivery"].to == "agent:71b0d4147311:ws:dm:cb1f9937234148a984c0e3d803c38ac8"

    asyncio.run(scenario())


def test_sdk_server_update_tool_rejects_main_session_target_with_explicit_delivery():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="update_scheduled_task",
                    arguments={
                        "job_id": "job-1",
                        "session_target": {"kind": "main", "wake_mode": "next-heartbeat"},
                        "delivery": {"mode": "explicit", "channel": "websocket", "to": "agent:71b0d4147311:ws:dm:chat"},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "temporary + current_chat" in result.root.content[0].text

    asyncio.run(scenario())


def test_sdk_server_update_tool_rejects_last_delivery_mode_for_page_parity():
    async def scenario():
        service = FakeOrchestrationService()
        config = create_main_agent_scheduled_task_sdk_server(service=service)
        server = config["instance"]
        handler = server.request_handlers[CallToolRequest]

        result = await handler(
            CallToolRequest(
                method="tools/call",
                params=CallToolRequestParams(
                    name="update_scheduled_task",
                    arguments={
                        "job_id": "job-1",
                        "delivery": {"mode": "last"},
                    },
                ),
            )
        )

        assert service.calls == []
        assert result.root.isError is True
        assert "page semantics" in result.root.content[0].text

    asyncio.run(scenario())
