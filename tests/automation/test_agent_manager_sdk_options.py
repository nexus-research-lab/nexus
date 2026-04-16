from __future__ import annotations

import asyncio
from pathlib import Path
from types import SimpleNamespace

from agent.schema.model_agent import AAgent, AgentOptions
from agent.schema.model_provider_config import ProviderRuntimeConfig
from agent.service.agent.agent_manager import AgentManager
from agent.service.agent.main_agent_profile import MainAgentProfile


class _FakeWorkspace:
    def __init__(self) -> None:
        self.path = Path("/tmp/fake-agent")

    def read_file(self, _name: str) -> str:
        return ""


async def _fake_resolve_runtime_config(_provider):
    return ProviderRuntimeConfig(
        provider="test",
        display_name="Test",
        auth_token="token",
        base_url="https://example.invalid",
        model="test-model",
    )


def test_build_sdk_options_keeps_empty_allowed_tools(monkeypatch):
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
        monkeypatch.setattr(
            "agent.service.agent.agent_manager.provider_config_service.resolve_runtime_config",
            _fake_resolve_runtime_config,
        )

        options = await manager.build_sdk_options("agent-1")

        assert options["allowed_tools"] == []

    asyncio.run(scenario())


def test_build_sdk_options_backfills_default_tools_when_missing(monkeypatch):
    async def fake_get_agent(_agent_id: str):
        return AAgent(
            agent_id="agent-1",
            name="planner",
            workspace_path="/tmp/fake-agent",
            options=AgentOptions(permission_mode="default"),
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
        monkeypatch.setattr(
            "agent.service.agent.agent_manager.provider_config_service.resolve_runtime_config",
            _fake_resolve_runtime_config,
        )

        options = await manager.build_sdk_options("agent-1")

        assert "Skill" in options["allowed_tools"]
        assert "Bash" in options["allowed_tools"]

    asyncio.run(scenario())


def test_build_sdk_options_injects_scheduled_task_sdk_server_for_main_agent(monkeypatch):
    async def fake_get_agent(_agent_id: str):
        return AAgent(
            agent_id=MainAgentProfile.AGENT_ID,
            name=MainAgentProfile.AGENT_ID,
            workspace_path="/tmp/fake-agent",
            options=AgentOptions(permission_mode="default"),
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
        monkeypatch.setattr(
            "agent.service.agent.agent_manager.provider_config_service.resolve_runtime_config",
            _fake_resolve_runtime_config,
        )

        options = await manager.build_sdk_options(MainAgentProfile.AGENT_ID)

        assert "mcp_servers" in options
        assert "nexus_automation" in options["mcp_servers"]
        assert options["mcp_servers"]["nexus_automation"]["type"] == "sdk"
        assert "list_scheduled_tasks" in options["allowed_tools"]
        assert "create_scheduled_task" in options["allowed_tools"]

    asyncio.run(scenario())


def test_build_sdk_options_injects_scheduled_task_sdk_server_for_regular_agent(monkeypatch):
    async def fake_get_agent(_agent_id: str):
        return AAgent(
            agent_id="agent-1",
            name="planner",
            workspace_path="/tmp/fake-agent",
            options=AgentOptions(permission_mode="default"),
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
        monkeypatch.setattr(
            "agent.service.agent.agent_manager.provider_config_service.resolve_runtime_config",
            _fake_resolve_runtime_config,
        )

        options = await manager.build_sdk_options("agent-1")

        assert "nexus_automation" in (options.get("mcp_servers") or {})
        assert "list_scheduled_tasks" in options["allowed_tools"]
        assert "create_scheduled_task" in options["allowed_tools"]

    asyncio.run(scenario())
