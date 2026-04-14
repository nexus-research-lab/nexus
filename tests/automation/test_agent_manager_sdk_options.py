from __future__ import annotations

import asyncio
from pathlib import Path
from types import SimpleNamespace

from agent.schema.model_agent import AAgent, AgentOptions
from agent.service.agent.agent_manager import AgentManager


class _FakeWorkspace:
    def __init__(self) -> None:
        self.path = Path("/tmp/fake-agent")

    def read_file(self, _name: str) -> str:
        return ""


def test_build_sdk_options_backfills_default_tools_for_empty_allowed_tools(monkeypatch):
    async def fake_get_agent(_agent_id: str):
        return AAgent(
            agent_id="agent-1",
            name="planner",
            workspace_path="/tmp/fake-agent",
            options=AgentOptions(permission_mode="default", allowed_tools=[]),
            status="active",
        )

    async def fake_get_workspace(_agent_id: str):
        return _FakeWorkspace()

    async def fake_load_connector_mcp_servers():
        return {}

    async def scenario():
        manager = AgentManager()
        monkeypatch.setattr(
            "agent.service.agent.agent_manager.agent_repository",
            SimpleNamespace(get_agent=fake_get_agent),
        )
        monkeypatch.setattr(manager._workspace_registry, "get_agent_workspace", fake_get_workspace)
        monkeypatch.setattr(manager, "_load_connector_mcp_servers", fake_load_connector_mcp_servers)

        options = await manager.build_sdk_options("agent-1")

        assert "Skill" in options["allowed_tools"]
        assert "Bash" in options["allowed_tools"]

    asyncio.run(scenario())
