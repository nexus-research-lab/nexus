from __future__ import annotations

import asyncio
import importlib
import pathlib
import sys
from types import ModuleType, SimpleNamespace

import pytest
from typer.testing import CliRunner

PROJECT_ROOT = pathlib.Path(__file__).resolve().parents[2]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from agent.schema.model_automation import AutomationCronSource, AutomationSessionTarget


class FakeScheduledTaskService:
    """记录定时任务调用。"""

    def __init__(self) -> None:
        self.calls: list[tuple[str, tuple, dict]] = []

    async def set_task_enabled(self, job_id: str, *, enabled: bool):
        self.calls.append(("set_task_enabled", (job_id,), {"enabled": enabled}))
        return FakePayload(job_id=job_id, enabled=enabled)

    async def create_task(self, payload):
        self.calls.append(("create_task", (), {"payload": payload}))
        return FakePayload(job_id="job-1")


class FakeSessionStore:
    """按 session_key 返回预设 session。"""

    def __init__(self, sessions: dict[str, object] | None = None) -> None:
        self.sessions = dict(sessions or {})

    async def get_session_info(self, session_key: str):
        return self.sessions.get(session_key)


class FakePayload:
    """提供与 AModel 兼容的最小导出接口。"""

    def __init__(self, **fields) -> None:
        self._fields = fields

    def model_dump(self, mode: str = "python") -> dict[str, object]:
        del mode
        return dict(self._fields)


def _prepare_env(monkeypatch) -> None:
    monkeypatch.setenv("ENV_FILE", "/dev/null")
    monkeypatch.setenv("DEBUG", "false")


def _load_orchestration_module(monkeypatch):
    _prepare_env(monkeypatch)
    for module_name in (
        "agent.service.agent.main_agent_orchestration_service",
        "agent.service.agent.agent_service",
        "agent.service.capability.scheduled.scheduled_task_service",
        "agent.service.capability.skills.skill_service",
        "agent.service.room.room_service",
        "agent.service.workspace.workspace_service",
        "agent.service.session.session_store",
        "agent.schema.model_agent",
    ):
        monkeypatch.delitem(sys.modules, module_name, raising=False)

    model_agent_module = ModuleType("agent.schema.model_agent")
    model_agent_module.AAgent = type("AAgent", (), {})
    model_agent_module.AgentOptions = type(
        "AgentOptions",
        (),
        {
            "__init__": lambda self, **kwargs: self.__dict__.update(kwargs),
        },
    )
    monkeypatch.setitem(sys.modules, "agent.schema.model_agent", model_agent_module)

    agent_service_module = ModuleType("agent.service.agent.agent_service")
    agent_service_module.agent_service = SimpleNamespace()
    monkeypatch.setitem(sys.modules, "agent.service.agent.agent_service", agent_service_module)

    scheduled_task_module = ModuleType("agent.service.capability.scheduled.scheduled_task_service")
    scheduled_task_module.scheduled_task_service = FakeScheduledTaskService()
    monkeypatch.setitem(
        sys.modules,
        "agent.service.capability.scheduled.scheduled_task_service",
        scheduled_task_module,
    )

    skill_service_module = ModuleType("agent.service.capability.skills.skill_service")
    skill_service_module.skill_service = SimpleNamespace()
    monkeypatch.setitem(sys.modules, "agent.service.capability.skills.skill_service", skill_service_module)

    room_service_module = ModuleType("agent.service.room.room_service")
    room_service_module.room_service = SimpleNamespace()
    monkeypatch.setitem(sys.modules, "agent.service.room.room_service", room_service_module)

    workspace_service_module = ModuleType("agent.service.workspace.workspace_service")
    workspace_service_module.workspace_service = SimpleNamespace()
    monkeypatch.setitem(sys.modules, "agent.service.workspace.workspace_service", workspace_service_module)

    session_store_module = ModuleType("agent.service.session.session_store")
    session_store_module.session_store = FakeSessionStore()
    monkeypatch.setitem(sys.modules, "agent.service.session.session_store", session_store_module)

    return importlib.import_module("agent.service.agent.main_agent_orchestration_service")


def test_create_scheduled_task_rejects_session_bound_to_other_agent(monkeypatch):
    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        fake_scheduled_task_service = FakeScheduledTaskService()
        fake_session_store = FakeSessionStore(
            sessions={
                "agent:writer:ws:dm:launcher-app-writer": SimpleNamespace(
                    session_key="agent:writer:ws:dm:launcher-app-writer",
                    agent_id="writer",
                ),
            }
        )
        monkeypatch.setattr(module, "scheduled_task_service", fake_scheduled_task_service)
        monkeypatch.setattr(module, "session_store", fake_session_store)

        try:
            await service.create_scheduled_task(
                name="daily brief",
                agent_id="research",
                instruction="summarize updates",
                session_key="agent:writer:ws:dm:launcher-app-writer",
                schedule_kind="every",
                interval_seconds=60,
            )
        except ValueError as exc:
            assert "does not belong" in str(exc)
        else:
            raise AssertionError("expected mismatched session ownership to be rejected")

        assert fake_scheduled_task_service.calls == []

    asyncio.run(scenario())


def test_set_scheduled_task_enabled_delegates_to_capability_service(monkeypatch):
    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        fake_scheduled_task_service = FakeScheduledTaskService()
        monkeypatch.setattr(module, "scheduled_task_service", fake_scheduled_task_service)

        enabled = await service.set_scheduled_task_enabled("job-1", enabled=True)
        disabled = await service.set_scheduled_task_enabled("job-1", enabled=False)

        assert enabled == {"job_id": "job-1", "enabled": True}
        assert disabled == {"job_id": "job-1", "enabled": False}
        assert fake_scheduled_task_service.calls == [
            ("set_task_enabled", ("job-1",), {"enabled": True}),
            ("set_task_enabled", ("job-1",), {"enabled": False}),
        ]

    asyncio.run(scenario())


def test_create_scheduled_task_accepts_named_session_target(monkeypatch):
    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        fake_scheduled_task_service = FakeScheduledTaskService()
        monkeypatch.setattr(module, "scheduled_task_service", fake_scheduled_task_service)

        result = await service.create_scheduled_task(
            name="morning brief",
            agent_id="research",
            instruction="summarize updates",
            session_target=AutomationSessionTarget(
                kind="named",
                named_session_key="morning-brief",
            ),
            schedule_kind="every",
            interval_seconds=300,
        )

        assert result == {"job_id": "job-1"}
        assert fake_scheduled_task_service.calls[0][0] == "create_task"
        payload = fake_scheduled_task_service.calls[0][2]["payload"]
        assert payload.agent_id == "research"
        assert payload.session_target.kind == "named"
        assert payload.session_target.named_session_key == "morning-brief"
        assert payload.session_target.bound_session_key is None
        assert payload.source.kind == "agent"
        assert payload.source.context_type == "agent"
        assert payload.source.context_id == "research"

    asyncio.run(scenario())


def test_create_scheduled_task_accepts_main_session_target(monkeypatch):
    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        fake_scheduled_task_service = FakeScheduledTaskService()
        monkeypatch.setattr(module, "scheduled_task_service", fake_scheduled_task_service)

        result = await service.create_scheduled_task(
            name="main follow up",
            agent_id="research",
            instruction="follow up in main session",
            session_target=AutomationSessionTarget(
                kind="main",
                wake_mode="now",
            ),
            schedule_kind="every",
            interval_seconds=300,
        )

        assert result == {"job_id": "job-1"}
        assert fake_scheduled_task_service.calls[0][0] == "create_task"
        payload = fake_scheduled_task_service.calls[0][2]["payload"]
        assert payload.agent_id == "research"
        assert payload.session_target.kind == "main"
        assert payload.session_target.wake_mode == "now"
        assert payload.session_target.bound_session_key is None
        assert payload.session_target.named_session_key is None
        assert payload.source.kind == "agent"

    asyncio.run(scenario())


def test_create_scheduled_task_accepts_explicit_source(monkeypatch):
    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        fake_scheduled_task_service = FakeScheduledTaskService()
        monkeypatch.setattr(module, "scheduled_task_service", fake_scheduled_task_service)

        result = await service.create_scheduled_task(
            name="cli follow up",
            agent_id="research",
            instruction="follow up from cli",
            session_target=AutomationSessionTarget(
                kind="main",
                wake_mode="now",
            ),
            source=AutomationCronSource(
                kind="cli",
                creator_agent_id="nexus",
                context_type="agent",
                context_id="research",
                context_label="Research",
            ),
            schedule_kind="every",
            interval_seconds=300,
        )

        assert result == {"job_id": "job-1"}
        payload = fake_scheduled_task_service.calls[0][2]["payload"]
        assert payload.source.kind == "cli"
        assert payload.source.creator_agent_id == "nexus"
        assert payload.source.context_label == "Research"

    asyncio.run(scenario())


def test_cli_enable_and_disable_scheduled_task_commands_forward_enabled_flag(monkeypatch):
    _prepare_env(monkeypatch)
    for module_name in (
        "agent.cli.command",
        "agent.service.agent.main_agent_profile",
        "agent.config.config",
    ):
        monkeypatch.delitem(sys.modules, module_name, raising=False)

    config_module = ModuleType("agent.config.config")
    config_module.settings = SimpleNamespace(
        DEFAULT_AGENT_ID="nexus",
        MAIN_AGENT_NAME="nexus",
    )
    monkeypatch.setitem(sys.modules, "agent.config.config", config_module)

    profile_module = ModuleType("agent.service.agent.main_agent_profile")

    class FakeMainAgentProfile:
        @classmethod
        def display_name(cls) -> str:
            return "nexus"

        @classmethod
        def display_label(cls) -> str:
            return "Nexus"

    profile_module.MainAgentProfile = FakeMainAgentProfile
    monkeypatch.setitem(sys.modules, "agent.service.agent.main_agent_profile", profile_module)

    module = importlib.import_module("agent.cli.command")
    calls: list[tuple[str, bool]] = []

    async def fake_set_enabled(job_id: str, *, enabled: bool):
        calls.append((job_id, enabled))
        return {"job_id": job_id, "enabled": enabled}

    def run_service_call(service_call):
        service = SimpleNamespace(set_scheduled_task_enabled=fake_set_enabled)
        asyncio.run(service_call(service))

    app = module.build_typer_app(
        run_service_call=run_service_call,
        parse_agent_ids=lambda value: value.split(","),
        configure_output=lambda **_kwargs: None,
    )
    runner = CliRunner()

    enable_result = runner.invoke(app, ["enable_scheduled_task", "--job-id", "job-1"])
    disable_result = runner.invoke(app, ["disable_scheduled_task", "--job-id", "job-2"])

    assert enable_result.exit_code == 0
    assert disable_result.exit_code == 0
    assert calls == [("job-1", True), ("job-2", False)]


def test_automation_session_target_rejects_reserved_named_session_key():
    with pytest.raises(ValueError, match="named_session_key 'main' is reserved"):
        AutomationSessionTarget(kind="named", named_session_key="main")


def test_cli_create_scheduled_task_rejects_conflicting_session_target_options(monkeypatch):
    _prepare_env(monkeypatch)
    for module_name in (
        "agent.cli.command",
        "agent.service.agent.main_agent_profile",
        "agent.config.config",
    ):
        monkeypatch.delitem(sys.modules, module_name, raising=False)

    config_module = ModuleType("agent.config.config")
    config_module.settings = SimpleNamespace(
        DEFAULT_AGENT_ID="nexus",
        MAIN_AGENT_NAME="nexus",
    )
    monkeypatch.setitem(sys.modules, "agent.config.config", config_module)

    profile_module = ModuleType("agent.service.agent.main_agent_profile")

    class FakeMainAgentProfile:
        @classmethod
        def display_name(cls) -> str:
            return "nexus"

        @classmethod
        def display_label(cls) -> str:
            return "Nexus"

    profile_module.MainAgentProfile = FakeMainAgentProfile
    monkeypatch.setitem(sys.modules, "agent.service.agent.main_agent_profile", profile_module)

    module = importlib.import_module("agent.cli.command")

    def run_service_call(service_call):
        del service_call

    app = module.build_typer_app(
        run_service_call=run_service_call,
        parse_agent_ids=lambda value: value.split(","),
        configure_output=lambda **_kwargs: None,
    )
    runner = CliRunner()

    result = runner.invoke(
        app,
        [
            "create_scheduled_task",
            "--name",
            "job",
            "--agent-id",
            "research",
            "--instruction",
            "do work",
            "--main",
            "--session-key",
            "agent:research:ws:dm:room",
        ],
    )

    assert result.exit_code != 0
    assert "cannot be combined" in result.output
    assert "ValidationError" not in result.output

    second_result = runner.invoke(
        app,
        [
            "create_scheduled_task",
            "--name",
            "job",
            "--agent-id",
            "research",
            "--instruction",
            "do work",
            "--session-key",
            "agent:research:ws:dm:room",
            "--named-session-key",
            "nightly",
        ],
    )

    assert second_result.exit_code != 0
    assert "cannot be combined" in second_result.output
    assert "ValidationError" not in second_result.output


def test_create_agent_defaults_include_skill_tool(monkeypatch):
    async def fake_create_agent(*, name: str, options, avatar=None, description=None, vibe_tags=None):
        del avatar, description, vibe_tags
        return SimpleNamespace(
            agent_id="agent-1",
            name=name,
            workspace_path="/tmp/agent-1",
            options=options,
            status="active",
        )

    async def scenario():
        module = _load_orchestration_module(monkeypatch)
        service = module.MainAgentOrchestrationService()
        monkeypatch.setattr(module, "agent_service", SimpleNamespace(create_agent=fake_create_agent))

        result = await service.create_agent(name="planner", provider="glm")

        assert result["agent_id"] == "agent-1"
        assert result["provider"] == "glm"
        assert "Skill" in result["allowed_tools"]

    asyncio.run(scenario())
