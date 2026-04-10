from __future__ import annotations

import asyncio
import sys
from datetime import datetime, timedelta, timezone
from types import SimpleNamespace

import pytest


class FakeStateStore:
    """返回固定状态的测试替身。"""

    def __init__(self, row=None, enabled_rows=None) -> None:
        self.row = row
        self.enabled_rows = list(enabled_rows or [])

    async def get_state(self, agent_id: str):
        del agent_id
        return self.row

    async def list_enabled_states(self):
        return list(self.enabled_rows)


class FakeEventQueue:
    """记录系统事件入队参数。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []
        self.pending_events: list[object] = []
        self.processing_ids: list[str] = []
        self.processed_ids: list[str] = []
        self.failed_ids: list[str] = []

    async def enqueue(self, **fields):
        self.calls.append(fields)
        event = SimpleNamespace(
            event_id=fields.get("event_id", f"event-{len(self.calls)}"),
            event_type=fields.get("event_type"),
            source_type=fields.get("source_type"),
            source_id=fields.get("source_id"),
            payload=dict(fields.get("payload") or {}),
        )
        self.pending_events.append(event)
        return event

    async def list_pending_events(self):
        return list(self.pending_events)

    async def mark_processing(self, event_id: str):
        self.processing_ids.append(event_id)
        for item in self.pending_events:
            if getattr(item, "event_id", None) == event_id:
                return item
        return None

    async def mark_processed(self, event_id: str):
        self.processed_ids.append(event_id)
        return None

    async def mark_failed(self, event_id: str):
        self.failed_ids.append(event_id)
        return None


class FakeScheduler:
    """记录调度层调用的替身。"""

    def __init__(self) -> None:
        self.start_calls = 0
        self.stop_calls = 0
        self.sync_calls: list[tuple[str, object]] = []
        self.request_calls: list[tuple[str, str]] = []
        self.runtime = {}

    async def start(self) -> None:
        self.start_calls += 1

    async def stop(self) -> None:
        self.stop_calls += 1

    async def sync_agent(self, agent_id: str, config) -> None:
        self.sync_calls.append((agent_id, config))

    async def request_wake(self, *, agent_id: str, mode: str):
        self.request_calls.append((agent_id, mode))
        return {
            "agent_id": agent_id,
            "mode": mode,
            "scheduled": mode == "now",
        }

    def get_runtime_status(self, agent_id: str) -> dict[str, object]:
        return dict(self.runtime.get(agent_id, {}))


class FakeDispatcher:
    """记录 dispatcher 调用。"""

    def __init__(self) -> None:
        self.calls: list[tuple[str, object]] = []

    async def dispatch(self, *, agent_id: str, config):
        self.calls.append((agent_id, config))
        return None


class FakeWakeService:
    """记录 wake bookkeeping 的替身。"""

    def __init__(self) -> None:
        self.now_calls: list[dict[str, object]] = []
        self.next_calls: list[dict[str, object]] = []
        self._now_requests: list[SimpleNamespace] = []
        self._next_requests: list[SimpleNamespace] = []

    def request_now(self, *, agent_id: str, session_key: str, metadata=None):
        request = SimpleNamespace(
            agent_id=agent_id,
            session_key=session_key,
            wake_mode="now",
            metadata=dict(metadata or {}),
        )
        self.now_calls.append(
            {
                "agent_id": agent_id,
                "session_key": session_key,
                "metadata": dict(metadata or {}),
            }
        )
        self._now_requests.append(request)
        return request

    def request_next_heartbeat(self, *, agent_id: str, session_key: str, metadata=None):
        request = SimpleNamespace(
            agent_id=agent_id,
            session_key=session_key,
            wake_mode="next-heartbeat",
            metadata=dict(metadata or {}),
        )
        self.next_calls.append(
            {
                "agent_id": agent_id,
                "session_key": session_key,
                "metadata": dict(metadata or {}),
            }
        )
        self._next_requests.append(request)
        return request

    def drain_now(self, agent_id: str | None = None):
        matched = [item for item in self._now_requests if agent_id is None or item.agent_id == agent_id]
        self._now_requests = [item for item in self._now_requests if item not in matched]
        return matched

    def list_next_heartbeat(self, agent_id: str | None = None):
        return [item for item in self._next_requests if agent_id is None or item.agent_id == agent_id]

    def clear(self, session_key: str) -> None:
        self._now_requests = [item for item in self._now_requests if item.session_key != session_key]
        self._next_requests = [item for item in self._next_requests if item.session_key != session_key]


class FakeWorkspaceReader:
    """返回固定 HEARTBEAT.md 内容。"""

    def __init__(self, text: str = "") -> None:
        self.text = text

    async def get_workspace_file(self, agent_id: str, path: str) -> str:
        assert agent_id
        assert path == "HEARTBEAT.md"
        return self.text


class FakeOrchestrator:
    """记录 dispatcher 生成的 heartbeat 指令。"""

    def __init__(self, result=None) -> None:
        self.calls: list[object] = []
        self.result = result or SimpleNamespace(ok=True, round_id="round-1")

    async def run_turn(self, ctx):
        self.calls.append(ctx)
        return self.result


class FakeMessageStore:
    """返回固定的主会话结果消息。"""

    def __init__(self, messages=None) -> None:
        self.messages = list(messages or [])

    async def get_session_messages(self, session_key: str):
        del session_key
        return list(self.messages)


class FakeDeliveryRouter:
    """记录 dispatcher 是否尝试外发。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    async def send_text(self, *, agent_id: str, text: str, target):
        self.calls.append(
            {
                "agent_id": agent_id,
                "text": text,
                "target": target,
            }
        )
        return target


class FailingDeliveryRouter(FakeDeliveryRouter):
    """模拟投递阶段失败。"""

    async def send_text(self, *, agent_id: str, text: str, target):
        await super().send_text(agent_id=agent_id, text=text, target=target)
        raise RuntimeError("delivery boom")


def _prepare_service_import(monkeypatch) -> None:
    monkeypatch.setenv("ENV_FILE", "/dev/null")
    monkeypatch.setenv("DEBUG", "false")
    sys.modules.pop("agent.config.config", None)
    sys.modules.pop("agent.service.automation.heartbeat.heartbeat_service", None)


def test_heartbeat_service_returns_status_merged_from_store_and_scheduler(monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        last_heartbeat_at = datetime(2026, 4, 10, 8, 0, tzinfo=timezone.utc)
        last_ack_at = datetime(2026, 4, 10, 8, 1, tzinfo=timezone.utc)
        next_run_at = datetime(2026, 4, 10, 8, 30, tzinfo=timezone.utc)
        scheduler = FakeScheduler()
        scheduler.runtime["nexus"] = {
            "running": True,
            "next_run_at": next_run_at,
            "pending_wake": False,
        }
        service = HeartbeatService(
            state_store=FakeStateStore(
                SimpleNamespace(
                    agent_id="nexus",
                    enabled=True,
                    every_seconds=60,
                    target_mode="last",
                    ack_max_chars=120,
                    last_heartbeat_at=last_heartbeat_at,
                    last_ack_at=last_ack_at,
                )
            ),
            scheduler=scheduler,
            dispatcher=FakeDispatcher(),
            system_event_queue=FakeEventQueue(),
        )

        status = await service.get_status("nexus")

        assert status.agent_id == "nexus"
        assert status.enabled is True
        assert status.every_seconds == 60
        assert status.target_mode == "last"
        assert status.ack_max_chars == 120
        assert status.running is True
        assert status.pending_wake is False
        assert status.next_run_at == next_run_at
        assert status.last_heartbeat_at == last_heartbeat_at
        assert status.last_ack_at == last_ack_at
        assert scheduler.sync_calls
        synced_agent_id, synced_config = scheduler.sync_calls[0]
        assert synced_agent_id == "nexus"
        assert synced_config.every_seconds == 60

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_service_wake_enqueues_event_text_and_delegates_to_scheduler(monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        scheduler = FakeScheduler()
        event_queue = FakeEventQueue()
        service = HeartbeatService(
            state_store=FakeStateStore(),
            scheduler=scheduler,
            dispatcher=FakeDispatcher(),
            system_event_queue=event_queue,
        )

        wake = await service.wake(
            agent_id="nexus",
            mode="now",
            text="check disk pressure",
        )

        assert scheduler.request_calls == [("nexus", "now")]
        assert wake.agent_id == "nexus"
        assert wake.mode == "now"
        assert wake.scheduled is True
        assert event_queue.calls == [
            {
                "event_type": "heartbeat.wake",
                "source_type": "heartbeat",
                "source_id": "nexus",
                "payload": {
                    "agent_id": "nexus",
                    "text": "check disk pressure",
                    "wake_mode": "now",
                },
            }
        ]

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_service_start_preloads_enabled_states(monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        scheduler = FakeScheduler()
        service = HeartbeatService(
            state_store=FakeStateStore(
                enabled_rows=[
                    SimpleNamespace(
                        agent_id="enabled-agent",
                        enabled=True,
                        every_seconds=90,
                        target_mode="last",
                        ack_max_chars=80,
                        last_heartbeat_at=None,
                        last_ack_at=None,
                    )
                ]
            ),
            scheduler=scheduler,
            dispatcher=FakeDispatcher(),
            system_event_queue=FakeEventQueue(),
        )

        await service.start()

        assert scheduler.start_calls == 1
        assert scheduler.sync_calls
        synced_agent_id, synced_config = scheduler.sync_calls[0]
        assert synced_agent_id == "enabled-agent"
        assert synced_config.enabled is True
        assert synced_config.every_seconds == 90
        assert synced_config.target_mode == "last"

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_service_rejects_persisted_explicit_target_mode(monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        service = HeartbeatService(
            state_store=FakeStateStore(
                row=SimpleNamespace(
                    agent_id="nexus",
                    enabled=True,
                    every_seconds=60,
                    target_mode="explicit",
                    ack_max_chars=120,
                    last_heartbeat_at=None,
                    last_ack_at=None,
                ),
                enabled_rows=[
                    SimpleNamespace(
                        agent_id="nexus",
                        enabled=True,
                        every_seconds=60,
                        target_mode="explicit",
                        ack_max_chars=120,
                        last_heartbeat_at=None,
                        last_ack_at=None,
                    )
                ]
            ),
            scheduler=FakeScheduler(),
            dispatcher=FakeDispatcher(),
            system_event_queue=FakeEventQueue(),
        )

        await service.start()

        status = await service.get_status("nexus")
        wake = await service.wake(agent_id="nexus", mode="now")

        assert status.target_mode == "none"
        assert status.delivery_error == "heartbeat target_mode=explicit is not supported in Task 6 runtime"
        assert wake.scheduled is True

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_service_wake_records_wake_service_bookkeeping(monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        scheduler = FakeScheduler()
        wake_service = FakeWakeService()
        service = HeartbeatService(
            state_store=FakeStateStore(),
            scheduler=scheduler,
            dispatcher=FakeDispatcher(),
            system_event_queue=FakeEventQueue(),
            wake_service=wake_service,
        )

        await service.wake(agent_id="nexus", mode="now", text="ping")
        await service.wake(agent_id="nexus", mode="next-heartbeat")

        assert wake_service.now_calls == [
            {
                "agent_id": "nexus",
                "session_key": "agent:nexus:automation:dm:main",
                "metadata": {"text": "ping"},
            }
        ]
        assert wake_service.next_calls == [
            {
                "agent_id": "nexus",
                "session_key": "agent:nexus:automation:dm:main",
                "metadata": {},
            }
        ]
        assert scheduler.request_calls == [
            ("nexus", "now"),
            ("nexus", "next-heartbeat"),
        ]

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_dispatcher_consumes_wake_bookkeeping(monkeypatch):
    async def scenario():
        from agent.schema.model_automation import AutomationHeartbeatConfig
        from agent.service.automation.heartbeat.heartbeat_dispatcher import HeartbeatDispatcher

        wake_service = FakeWakeService()
        wake_service.request_now(
            agent_id="nexus",
            session_key="agent:nexus:automation:dm:main",
            metadata={"text": "urgent ping"},
        )
        wake_service.request_next_heartbeat(
            agent_id="nexus",
            session_key="agent:nexus:automation:dm:main",
            metadata={"text": "follow up soon"},
        )
        orchestrator = FakeOrchestrator(
            result=SimpleNamespace(ok=True, round_id="round-1")
        )
        dispatcher = HeartbeatDispatcher(
            orchestrator=orchestrator,
            delivery_router=FakeDeliveryRouter(),
            system_event_queue=FakeEventQueue(),
            wake_service=wake_service,
            workspace_reader=FakeWorkspaceReader(""),
            message_store=FakeMessageStore(
                messages=[SimpleNamespace(round_id="round-1", role="result", result="HEARTBEAT_OK")]
            ),
        )

        result = await dispatcher.dispatch(
            agent_id="nexus",
            config=AutomationHeartbeatConfig(
                agent_id="nexus",
                enabled=True,
                every_seconds=60,
                target_mode="none",
            ),
        )

        assert result.acknowledged is True
        assert orchestrator.calls
        assert "urgent ping" in orchestrator.calls[0].instruction
        assert "follow up soon" in orchestrator.calls[0].instruction
        assert wake_service.drain_now("nexus") == []
        assert wake_service.list_next_heartbeat("nexus") == []

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_dispatcher_cleans_up_after_delivery_failure(monkeypatch):
    async def scenario():
        from agent.schema.model_automation import AutomationHeartbeatConfig
        from agent.service.automation.heartbeat.heartbeat_dispatcher import HeartbeatDispatcher

        wake_service = FakeWakeService()
        wake_service.request_next_heartbeat(
            agent_id="nexus",
            session_key="agent:nexus:automation:dm:main",
            metadata={"text": "follow up soon"},
        )
        event_queue = FakeEventQueue()
        event_queue.pending_events = [
            SimpleNamespace(
                event_id="evt-1",
                event_type="heartbeat.wake",
                payload={"agent_id": "nexus", "text": "queued event"},
            )
        ]
        dispatcher = HeartbeatDispatcher(
            orchestrator=FakeOrchestrator(
                result=SimpleNamespace(ok=True, round_id="round-1")
            ),
            delivery_router=FailingDeliveryRouter(),
            system_event_queue=event_queue,
            wake_service=wake_service,
            workspace_reader=FakeWorkspaceReader(""),
            message_store=FakeMessageStore(
                messages=[SimpleNamespace(round_id="round-1", role="result", result="alert: deliver this")]
            ),
        )

        with pytest.raises(RuntimeError, match="delivery boom"):
            await dispatcher.dispatch(
                agent_id="nexus",
                config=AutomationHeartbeatConfig(
                    agent_id="nexus",
                    enabled=True,
                    every_seconds=60,
                    target_mode="last",
                ),
            )

        assert event_queue.processing_ids == ["evt-1"]
        assert event_queue.processed_ids == ["evt-1"]
        assert event_queue.failed_ids == []
        assert wake_service.list_next_heartbeat("nexus") == []

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_dispatcher_does_not_duplicate_wake_text_from_queue_and_bookkeeping(monkeypatch):
    async def scenario():
        from agent.schema.model_automation import AutomationHeartbeatConfig
        from agent.service.automation.heartbeat.heartbeat_dispatcher import HeartbeatDispatcher
        from agent.service.automation.heartbeat.heartbeat_service import HeartbeatService

        wake_service = FakeWakeService()
        event_queue = FakeEventQueue()
        scheduler = FakeScheduler()
        service = HeartbeatService(
            state_store=FakeStateStore(),
            scheduler=scheduler,
            dispatcher=FakeDispatcher(),
            system_event_queue=event_queue,
            wake_service=wake_service,
        )
        await service.wake(agent_id="nexus", mode="now", text="urgent ping")

        orchestrator = FakeOrchestrator(
            result=SimpleNamespace(ok=True, round_id="round-1")
        )
        dispatcher = HeartbeatDispatcher(
            orchestrator=orchestrator,
            delivery_router=FakeDeliveryRouter(),
            system_event_queue=event_queue,
            wake_service=wake_service,
            workspace_reader=FakeWorkspaceReader(""),
            message_store=FakeMessageStore(
                messages=[SimpleNamespace(round_id="round-1", role="result", result="HEARTBEAT_OK")]
            ),
        )

        result = await dispatcher.dispatch(
            agent_id="nexus",
            config=AutomationHeartbeatConfig(
                agent_id="nexus",
                enabled=True,
                every_seconds=60,
                target_mode="none",
            ),
        )

        assert result.acknowledged is True
        assert orchestrator.calls
        assert orchestrator.calls[0].instruction.count("urgent ping") == 1

    _prepare_service_import(monkeypatch)
    asyncio.run(scenario())


def test_heartbeat_scheduler_dispatches_immediate_wake_requests():
    async def scenario():
        from agent.schema.model_automation import AutomationHeartbeatConfig
        from agent.service.automation.heartbeat.heartbeat_scheduler import HeartbeatScheduler

        dispatcher = FakeDispatcher()
        scheduler = HeartbeatScheduler(
            dispatcher=dispatcher,
            tick_seconds=0.01,
        )
        await scheduler.start()
        await scheduler.sync_agent(
            "nexus",
            AutomationHeartbeatConfig(
                agent_id="nexus",
                enabled=True,
                every_seconds=60,
                target_mode="none",
            ),
        )

        wake = await scheduler.request_wake(agent_id="nexus", mode="now")
        await asyncio.sleep(0.05)
        runtime = scheduler.get_runtime_status("nexus")
        await scheduler.stop()

        assert wake["scheduled"] is True
        assert dispatcher.calls
        dispatched_agent_id, dispatched_config = dispatcher.calls[0]
        assert dispatched_agent_id == "nexus"
        assert dispatched_config.agent_id == "nexus"
        assert runtime["running"] is True
        assert runtime["last_dispatch_reason"] == "wake-now"
        assert runtime["pending_wake"] is False

    asyncio.run(scenario())


def test_heartbeat_scheduler_dispatches_next_heartbeat_on_due_tick():
    async def scenario():
        from agent.schema.model_automation import AutomationHeartbeatConfig
        from agent.service.automation.heartbeat.heartbeat_scheduler import HeartbeatScheduler

        dispatcher = FakeDispatcher()
        scheduler = HeartbeatScheduler(
            dispatcher=dispatcher,
            tick_seconds=0.01,
        )
        await scheduler.start()
        await scheduler.sync_agent(
            "nexus",
            AutomationHeartbeatConfig(
                agent_id="nexus",
                enabled=True,
                every_seconds=1,
                target_mode="none",
            ),
        )

        await scheduler.request_wake(agent_id="nexus", mode="next-heartbeat")
        scheduler._states["nexus"].next_run_at = datetime.now(timezone.utc) - timedelta(seconds=1)
        await asyncio.sleep(0.05)
        runtime = scheduler.get_runtime_status("nexus")
        await scheduler.stop()

        assert dispatcher.calls
        assert runtime["last_dispatch_reason"] == "heartbeat"
        assert runtime["pending_wake"] is False
        assert runtime["next_run_at"] > datetime.now(timezone.utc)

    asyncio.run(scenario())
