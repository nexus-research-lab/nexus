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
        return [{"job_id": "job-1", "agent_id": agent_id or "nexus"}]

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
