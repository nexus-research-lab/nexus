from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from types import SimpleNamespace

from agent.schema.model_message import Message


class FakeCronStore:
    """内存版 run store，便于验证 cron runner ledger 行为。"""

    def __init__(self) -> None:
        self.runs: dict[str, SimpleNamespace] = {}

    async def create_run(self, **fields):
        payload = {
            "status": "pending",
            "scheduled_for": None,
            "started_at": None,
            "finished_at": None,
            "attempts": 0,
            "error_message": None,
            **fields,
        }
        row = SimpleNamespace(**payload)
        self.runs[row.run_id] = row
        return row

    async def update_run_status(self, **fields):
        row = self.runs[fields["run_id"]]
        updated = SimpleNamespace(
            run_id=row.run_id,
            job_id=row.job_id,
            status=fields["status"],
            scheduled_for=row.scheduled_for,
            started_at=fields.get("started_at", row.started_at),
            finished_at=fields.get("finished_at", row.finished_at),
            attempts=fields.get("attempts", row.attempts),
            error_message=fields.get("error_message", row.error_message),
        )
        self.runs[row.run_id] = updated
        return updated


class FakeSystemEventQueue:
    """记录 main target 的 system event 入队。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    async def enqueue(self, **fields):
        payload = {
            **fields,
            "event_id": f"event-{len(self.calls) + 1}",
        }
        self.calls.append(payload)
        return SimpleNamespace(**payload)

    async def mark_failed(self, event_id: str):
        del event_id
        return None


class FakeHeartbeatService:
    """记录 cron runner 对 heartbeat 的唤醒请求。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    async def wake(self, *, agent_id: str, mode: str, text: str | None = None):
        self.calls.append({
            "agent_id": agent_id,
            "mode": mode,
            "text": text,
        })
        return SimpleNamespace(agent_id=agent_id, mode=mode, text=text)


class FakeOrchestrator:
    """记录 automation run orchestrator 的入参。"""

    def __init__(self) -> None:
        self.calls: list[object] = []

    async def run_turn(self, ctx):
        self.calls.append(ctx)
        from agent.service.automation.runtime.run_result import AutomationRunResult

        return AutomationRunResult(
            agent_id=ctx.agent_id,
            session_key=ctx.session_key,
            status="success",
            round_id="round-1",
            session_id="session-1",
            message_count=1,
        )


class FakeDeliveryRouter:
    """记录 cron 任务完成后的 delivery 投递。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    async def send_text(self, *, agent_id: str, text: str, target):
        self.calls.append(
            {
                "agent_id": agent_id,
                "text": text,
                "target": dict(target or {}),
            }
        )
        return None


class FakeMessageStore:
    """返回会话消息，供 cron runner 抽取结果文本。"""

    def __init__(self, messages_by_session: dict[str, list[Message]] | None = None) -> None:
        self._messages_by_session = messages_by_session or {}

    async def get_session_messages(self, session_key: str) -> list[Message]:
        return list(self._messages_by_session.get(session_key, []))


def _build_job(
    *,
    kind: str,
    bound_session_key: str | None = None,
    named_session_key: str | None = None,
    wake_mode: str = "next-heartbeat",
):
    return SimpleNamespace(
        job_id="cron-job-1",
        agent_id="research",
        instruction="summarize updates",
        session_target_kind=kind,
        bound_session_key=bound_session_key,
        named_session_key=named_session_key,
        wake_mode=wake_mode,
        delivery_mode="none",
        delivery_channel=None,
        delivery_to=None,
        delivery_account_id=None,
        delivery_thread_id=None,
    )


def _build_result_message(*, session_key: str, round_id: str, result: str) -> Message:
    return Message(
        message_id=f"msg-{round_id}",
        session_key=session_key,
        agent_id="research",
        round_id=round_id,
        session_id="session-1",
        role="result",
        subtype="success",
        result=result,
    )


def test_cron_runner_main_target_queues_system_event_and_returns_queued_status():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )

        result = await runner.run_job(
            _build_job(kind="main", wake_mode="now"),
            trigger_kind="cron",
            scheduled_for=datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )

        assert result.status == "queued_to_main_session"
        assert result.run_id is None
        assert result.session_key == "agent:research:automation:dm:main"
        assert event_queue.calls == [
            {
                "event_type": "cron.trigger",
                "source_type": "cron",
                "source_id": "cron-job-1",
                "payload": {
                    "job_id": "cron-job-1",
                    "agent_id": "research",
                    "text": "summarize updates",
                    "session_target_kind": "main",
                    "trigger_kind": "cron",
                },
                "event_id": "event-1",
            }
        ]
        assert heartbeat_service.calls == [
            {
                "agent_id": "research",
                "mode": "now",
                "text": None,
            }
        ]
        assert orchestrator.calls == []
        assert store.runs == {}

    asyncio.run(scenario())


def test_cron_runner_bound_target_reuses_existing_session_key():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )

        result = await runner.run_job(
            _build_job(
                kind="bound",
                bound_session_key="agent:research:ws:dm:existing-room",
            ),
            run_id="run-bound-1",
            trigger_kind="manual",
            scheduled_for=datetime(2026, 4, 13, 8, 1, tzinfo=timezone.utc),
        )

        assert result.status == "succeeded"
        assert result.run_id == "run-bound-1"
        assert result.session_key == "agent:research:ws:dm:existing-room"
        assert result.round_id == "round-1"
        assert result.session_id == "session-1"
        assert result.message_count == 1
        assert [ctx.session_key for ctx in orchestrator.calls] == [
            "agent:research:ws:dm:existing-room"
        ]
        assert [ctx.trigger_kind for ctx in orchestrator.calls] == ["manual"]
        assert event_queue.calls == []
        assert heartbeat_service.calls == []

    asyncio.run(scenario())


def test_cron_runner_named_target_uses_named_automation_session_key():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )

        result = await runner.run_job(
            _build_job(
                kind="named",
                named_session_key="morning-brief",
            ),
            run_id="run-named-1",
            trigger_kind="cron",
            scheduled_for=datetime(2026, 4, 13, 8, 2, tzinfo=timezone.utc),
        )

        assert result.status == "succeeded"
        assert result.run_id == "run-named-1"
        assert result.session_key == "agent:research:automation:dm:morning-brief"
        assert result.round_id == "round-1"
        assert result.session_id == "session-1"
        assert result.message_count == 1
        assert [ctx.session_key for ctx in orchestrator.calls] == [
            "agent:research:automation:dm:morning-brief"
        ]
        assert event_queue.calls == []
        assert heartbeat_service.calls == []

    asyncio.run(scenario())


def test_cron_runner_isolated_target_uses_run_scoped_automation_session_key():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )

        result = await runner.run_job(
            _build_job(kind="isolated"),
            run_id="run-isolated-1",
            trigger_kind="cron",
            scheduled_for=datetime(2026, 4, 13, 8, 3, tzinfo=timezone.utc),
        )

        assert result.status == "succeeded"
        assert result.run_id == "run-isolated-1"
        assert result.session_key.startswith("agent:research:automation:dm:cron:")
        assert result.session_key.endswith(":run-isolated-1")
        assert result.round_id == "round-1"
        assert result.session_id == "session-1"
        assert result.message_count == 1
        assert [ctx.session_key for ctx in orchestrator.calls] == [result.session_key]
        assert event_queue.calls == []
        assert heartbeat_service.calls == []

    asyncio.run(scenario())


def test_cron_runner_non_main_target_delivers_result_text_when_delivery_is_configured():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner

        store = FakeCronStore()
        delivery_router = FakeDeliveryRouter()
        message_store = FakeMessageStore(
            {
                "agent:research:automation:dm:morning-brief": [
                    _build_result_message(
                        session_key="agent:research:automation:dm:morning-brief",
                        round_id="round-1",
                        result="daily summary",
                    )
                ]
            }
        )
        runner = CronRunner(
            store=store,
            system_event_queue=FakeSystemEventQueue(),
            heartbeat_service=FakeHeartbeatService(),
            agent_run_orchestrator=FakeOrchestrator(),
            delivery_router=delivery_router,
            message_store=message_store,
            now_fn=lambda: datetime(2026, 4, 13, 8, 0, tzinfo=timezone.utc),
        )
        job = _build_job(
            kind="named",
            named_session_key="morning-brief",
        )
        job.delivery_mode = "explicit"
        job.delivery_channel = "websocket"
        job.delivery_to = "agent:research:ws:dm:reply-room"

        result = await runner.run_job(
            job,
            run_id="run-named-1",
            trigger_kind="cron",
            scheduled_for=datetime(2026, 4, 13, 8, 2, tzinfo=timezone.utc),
        )

        assert result.status == "succeeded"
        assert delivery_router.calls == [
            {
                "agent_id": "research",
                "text": "daily summary",
                    "target": {
                        "mode": "explicit",
                        "channel": "websocket",
                        "to": "agent:research:ws:dm:reply-room",
                        "account_id": None,
                        "thread_id": None,
                    },
                }
            ]

    asyncio.run(scenario())
