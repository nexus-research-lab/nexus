from __future__ import annotations

import asyncio
from types import SimpleNamespace


class FakeClient:
    """可控的 SDK client 替身。"""

    def __init__(self, messages, *, query_error: Exception | None = None) -> None:
        self._messages = list(messages)
        self._query_error = query_error
        self.query_calls: list[str] = []

    async def query(self, instruction: str) -> None:
        self.query_calls.append(instruction)
        if self._query_error is not None:
            raise self._query_error

    async def receive_messages(self):
        for item in self._messages:
            yield item


class FakeAgentRuntime:
    """记录 get_or_create_client 调用的运行时替身。"""

    def __init__(self, client: FakeClient) -> None:
        self.client = client
        self.calls: list[dict[str, object]] = []

    async def get_or_create_client(self, **kwargs):
        self.calls.append(kwargs)
        return self.client


class FakeSessionStore:
    """记录 session_store 调用的替身。"""

    def __init__(self, existing_session=None) -> None:
        self._existing_session = existing_session
        self.created_calls: list[dict[str, object]] = []
        self.saved_messages: list[object] = []

    async def get_session_info(self, session_key: str):
        del session_key
        return self._existing_session

    async def create_session_by_key(self, **kwargs):
        self.created_calls.append(kwargs)
        self._existing_session = SimpleNamespace(
            session_key=kwargs["session_key"],
            agent_id="nexus",
            status="active",
            session_id=None,
        )
        return self._existing_session

    async def save_message(self, message) -> bool:
        self.saved_messages.append(message)
        return True


class FakeSender:
    """记录自动化实时推送的 sender。"""

    def __init__(self) -> None:
        self.sent_messages: list[object] = []

    async def send(self, message) -> None:
        self.sent_messages.append(message)


class FakeProcessor:
    """按预设终态消费 SDK 消息。"""

    outcomes: list[str | None] = []
    success_session_id: str | None = "sdk-session-1"
    emitted_messages: list[list[object]] = []
    instances: list["FakeProcessor"] = []

    def __init__(
        self,
        *,
        session_key: str,
        query: str,
        display_query: str | None = None,
        agent_id: str,
        session_id: str | None = None,
    ) -> None:
        self.session_key = session_key
        self.query = query
        self.display_query = display_query
        self.agent_id = agent_id
        self.round_id = "round-1"
        self.session_id = session_id
        self.subtype = None
        self.message_count = 0
        self.messages: list[object] = []
        type(self).instances.append(self)

    async def process_messages(self, response_msg) -> list[object]:
        self.messages.append(response_msg)
        self.message_count += 1
        outcome = type(self).outcomes[self.message_count - 1]
        if outcome is not None:
            self.subtype = outcome
            if outcome == "success" and type(self).success_session_id is not None:
                self.session_id = type(self).success_session_id
        if len(type(self).emitted_messages) >= self.message_count:
            return type(self).emitted_messages[self.message_count - 1]
        return []


def test_build_automation_main_session_key_uses_structured_gateway_format():
    from agent.service.session.session_router import (
        build_automation_main_session_key,
        is_automation_main_session_key,
        is_automation_session_key,
    )

    session_key = build_automation_main_session_key(agent_id="nexus")

    assert session_key == "agent:nexus:automation:dm:main"
    assert is_automation_session_key(session_key) is True
    assert is_automation_main_session_key(session_key) is True


def test_agent_run_orchestrator_creates_missing_session_and_stops_on_success(monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
        from agent.service.automation.runtime.run_context import AutomationRunContext
        import agent.service.automation.runtime.agent_run_orchestrator as module

        FakeProcessor.instances = []
        FakeProcessor.outcomes = [None, "success", "error"]
        FakeProcessor.success_session_id = "sdk-session-1"
        FakeProcessor.emitted_messages = []
        client = FakeClient(messages=["system", "assistant", "result"])
        runtime = FakeAgentRuntime(client)
        store = FakeSessionStore(existing_session=None)

        monkeypatch.setattr(module, "agent_runtime", runtime)
        monkeypatch.setattr(module, "session_store", store)
        monkeypatch.setattr(module, "ChatMessageProcessor", FakeProcessor)

        orchestrator = AgentRunOrchestrator()
        result = await orchestrator.run_turn(
            AutomationRunContext(
                agent_id="nexus",
                session_key="agent:nexus:automation:dm:main",
                instruction="summarize status",
                trigger_kind="cron",
            )
        )

        assert store.created_calls == [
            {
                "session_key": "agent:nexus:automation:dm:main",
                "channel_type": "automation",
                "chat_type": "dm",
                "title": "Automation Run",
            }
        ]
        assert runtime.calls[0]["session_key"] == "agent:nexus:automation:dm:main"
        assert runtime.calls[0]["agent_id"] == "nexus"
        assert client.query_calls == [
            "[AUTO TASK]\nname:task\ntime:auto\nrule:execute TASK only; do not mention auto metadata.\nTASK:\nsummarize status"
        ]
        assert FakeProcessor.instances[0].messages == ["system", "assistant"]
        assert FakeProcessor.instances[0].query == (
            "[AUTO TASK]\nname:task\ntime:auto\nrule:execute TASK only; do not mention auto metadata.\nTASK:\nsummarize status"
        )
        assert FakeProcessor.instances[0].display_query == "[cron:task|auto]\ntask: summarize status"
        assert result.ok is True
        assert result.status == "success"
        assert result.message_count == 2
        assert result.session_id == "sdk-session-1"

    asyncio.run(scenario())


def test_agent_run_orchestrator_returns_error_result_when_query_fails(monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
        from agent.service.automation.runtime.run_context import AutomationRunContext
        import agent.service.automation.runtime.agent_run_orchestrator as module

        FakeProcessor.instances = []
        FakeProcessor.outcomes = []
        FakeProcessor.success_session_id = "sdk-session-1"
        FakeProcessor.emitted_messages = []
        runtime = FakeAgentRuntime(FakeClient(messages=[], query_error=RuntimeError("boom")))
        store = FakeSessionStore(
            existing_session=SimpleNamespace(
                session_key="agent:nexus:automation:dm:main",
                agent_id="nexus",
                status="active",
                session_id="sdk-existing",
            )
        )

        monkeypatch.setattr(module, "agent_runtime", runtime)
        monkeypatch.setattr(module, "session_store", store)
        monkeypatch.setattr(module, "ChatMessageProcessor", FakeProcessor)

        orchestrator = AgentRunOrchestrator()
        result = await orchestrator.run_turn(
            AutomationRunContext(
                agent_id="nexus",
                session_key="agent:nexus:automation:dm:main",
                instruction="summarize status",
                trigger_kind="heartbeat",
            )
        )

        assert store.created_calls == []
        assert result.ok is False
        assert result.status == "error"
        assert result.error_message == "boom"
        assert result.message_count == 0
        assert FakeProcessor.instances[0].messages == []

    asyncio.run(scenario())


def test_agent_run_orchestrator_reuses_persisted_agent_and_session_metadata(monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
        from agent.service.automation.runtime.run_context import AutomationRunContext
        import agent.service.automation.runtime.agent_run_orchestrator as module

        FakeProcessor.instances = []
        FakeProcessor.outcomes = ["success"]
        FakeProcessor.success_session_id = None
        FakeProcessor.emitted_messages = []
        runtime = FakeAgentRuntime(FakeClient(messages=["result"]))
        store = FakeSessionStore(
            existing_session=SimpleNamespace(
                session_key="agent:nexus:automation:dm:main",
                agent_id="persisted-agent",
                status="active",
                session_id="sdk-existing",
            )
        )

        monkeypatch.setattr(module, "agent_runtime", runtime)
        monkeypatch.setattr(module, "session_store", store)
        monkeypatch.setattr(module, "ChatMessageProcessor", FakeProcessor)

        orchestrator = AgentRunOrchestrator()
        result = await orchestrator.run_turn(
            AutomationRunContext(
                agent_id="ctx-agent",
                session_key="agent:nexus:automation:dm:main",
                instruction="reuse persisted session",
                trigger_kind="heartbeat",
            )
        )

        assert store.created_calls == []
        assert runtime.calls[0]["agent_id"] == "persisted-agent"
        assert FakeProcessor.instances[0].agent_id == "persisted-agent"
        assert FakeProcessor.instances[0].session_id == "sdk-existing"
        assert result.ok is True
        assert result.session_id == "sdk-existing"

    asyncio.run(scenario())


def test_agent_run_orchestrator_pushes_processed_messages_to_sender(monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
        from agent.service.automation.runtime.run_context import AutomationRunContext
        import agent.service.automation.runtime.agent_run_orchestrator as module

        FakeProcessor.instances = []
        FakeProcessor.outcomes = ["success"]
        FakeProcessor.success_session_id = None
        FakeProcessor.emitted_messages = [["automation-message", "automation-result"]]
        runtime = FakeAgentRuntime(FakeClient(messages=["result"]))
        store = FakeSessionStore(
            existing_session=SimpleNamespace(
                session_key="agent:nexus:automation:dm:main",
                agent_id="nexus",
                status="active",
                session_id="sdk-existing",
            )
        )
        sender = FakeSender()

        monkeypatch.setattr(module, "agent_runtime", runtime)
        monkeypatch.setattr(module, "session_store", store)
        monkeypatch.setattr(module, "ChatMessageProcessor", FakeProcessor)

        orchestrator = AgentRunOrchestrator(sender=sender)
        result = await orchestrator.run_turn(
            AutomationRunContext(
                agent_id="nexus",
                session_key="agent:nexus:automation:dm:main",
                instruction="push updates",
                trigger_kind="cron",
            )
        )

        assert result.ok is True
        assert sender.sent_messages[0].role == "user"
        assert sender.sent_messages[0].content == "[cron:task|auto]\ntask: push updates"
        assert sender.sent_messages[1:] == ["automation-message", "automation-result"]
        assert store.saved_messages[0].role == "user"

    asyncio.run(scenario())


def test_agent_run_orchestrator_wraps_cron_instruction_for_session_history(monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
        from agent.service.automation.runtime.run_context import AutomationRunContext
        import agent.service.automation.runtime.agent_run_orchestrator as module

        FakeProcessor.instances = []
        FakeProcessor.outcomes = ["success"]
        FakeProcessor.success_session_id = None
        FakeProcessor.emitted_messages = []
        client = FakeClient(messages=["result"])
        runtime = FakeAgentRuntime(client)
        store = FakeSessionStore(
            existing_session=SimpleNamespace(
                session_key="agent:nexus:automation:dm:main",
                agent_id="nexus",
                status="active",
                session_id="sdk-existing",
            )
        )

        monkeypatch.setattr(module, "agent_runtime", runtime)
        monkeypatch.setattr(module, "session_store", store)
        monkeypatch.setattr(module, "ChatMessageProcessor", FakeProcessor)

        orchestrator = AgentRunOrchestrator()
        result = await orchestrator.run_turn(
            AutomationRunContext(
                agent_id="nexus",
                session_key="agent:nexus:automation:dm:main",
                instruction="say hello",
                trigger_kind="cron",
                metadata={
                    "job_name": "test",
                    "scheduled_for": "2026-04-14T16:39:00+08:00",
                },
            )
        )

        assert result.ok is True
        assert runtime.client.query_calls == [
            "[AUTO TASK]\nname:test\ntime:16:39\nrule:execute TASK only; do not mention auto metadata.\nTASK:\nsay hello"
        ]
        assert FakeProcessor.instances[0].query == (
            "[AUTO TASK]\nname:test\ntime:16:39\nrule:execute TASK only; do not mention auto metadata.\nTASK:\nsay hello"
        )
        assert FakeProcessor.instances[0].display_query == "[cron:test|16:39]\ntask: say hello"

    asyncio.run(scenario())
