from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from types import SimpleNamespace

from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationSessionTarget,
)


class FakeCronStore:
    """内存版 cron store，便于覆盖 service 逻辑。"""

    def __init__(self) -> None:
        self.jobs: dict[str, SimpleNamespace] = {}
        self.runs: dict[str, SimpleNamespace] = {}
        self.deleted_job_ids: list[str] = []

    async def get_job(self, job_id: str):
        return self.jobs.get(job_id)

    async def list_jobs(self, agent_id: str | None = None):
        rows = list(self.jobs.values())
        if agent_id is not None:
            rows = [row for row in rows if row.agent_id == agent_id]
        rows.sort(key=lambda item: item.job_id)
        return rows

    async def upsert_job(self, **fields):
        existing = self.jobs.get(fields["job_id"])
        payload = {}
        if existing is not None:
            payload.update(existing.__dict__)
        payload.update(fields)
        row = SimpleNamespace(**payload)
        self.jobs[row.job_id] = row
        return row

    async def delete_job(self, job_id: str) -> None:
        self.deleted_job_ids.append(job_id)
        self.jobs.pop(job_id, None)

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

    async def get_run(self, run_id: str):
        return self.runs.get(run_id)

    async def list_runs_by_job(self, job_id: str):
        rows = [row for row in self.runs.values() if row.job_id == job_id]
        rows.sort(key=lambda item: item.run_id)
        return rows

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


class FakeTimer:
    """记录 timer 同步与删除调用。"""

    def __init__(self) -> None:
        self.sync_calls: list[str] = []
        self.remove_calls: list[str] = []
        self.runtime: dict[str, dict[str, object]] = {}

    async def start(self) -> None:
        return None

    async def stop(self) -> None:
        return None

    async def sync_job(self, job, next_run_at: datetime | None) -> None:
        self.sync_calls.append(job.job_id)
        self.runtime[job.job_id] = {
            "next_run_at": next_run_at,
            "enabled": bool(job.enabled),
        }

    async def remove_job(self, job_id: str) -> None:
        self.remove_calls.append(job_id)
        self.runtime.pop(job_id, None)

    def get_runtime_status(self, job_id: str) -> dict[str, object]:
        return dict(self.runtime.get(job_id, {}))


class FakeSystemEventQueue:
    """记录 main session 入队事件。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    async def enqueue(self, **fields):
        self.calls.append(fields)
        return SimpleNamespace(**fields)


class FakeWakeService:
    """记录 wake 请求。"""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    def request(
        self,
        *,
        agent_id: str,
        session_key: str,
        wake_mode: str,
        metadata: dict[str, object] | None = None,
    ):
        payload = {
            "agent_id": agent_id,
            "session_key": session_key,
            "wake_mode": wake_mode,
            "metadata": dict(metadata or {}),
        }
        self.calls.append(payload)
        return SimpleNamespace(**payload)


class FakeOrchestrator:
    """记录 automation run 调用。"""

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
            message_count=2,
        )


def test_cron_service_create_pause_resume_list_and_delete_jobs():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner
        from agent.service.automation.cron.cron_service import CronService

        store = FakeCronStore()
        timer = FakeTimer()
        runner = CronRunner(
            store=store,
            system_event_queue=FakeSystemEventQueue(),
            wake_service=FakeWakeService(),
            agent_run_orchestrator=FakeOrchestrator(),
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )
        service = CronService(
            store=store,
            runner=runner,
            timer=timer,
            id_factory=lambda: "job-1",
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )

        created = await service.create_job(
            AutomationCronJobCreate(
                name="Morning Brief",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=300),
                instruction="summarize updates",
            )
        )
        paused = await service.set_job_enabled("job-1", enabled=False)
        resumed = await service.set_job_enabled("job-1", enabled=True)
        jobs = await service.list_jobs(agent_id="nexus")
        await service.delete_job("job-1")

        assert created.job_id == "job-1"
        assert created.next_run_at == datetime(2026, 4, 10, 1, 25, tzinfo=timezone.utc)
        assert paused.enabled is False
        assert resumed.enabled is True
        assert [job.job_id for job in jobs] == ["job-1"]
        assert timer.sync_calls == ["job-1", "job-1", "job-1"]
        assert timer.remove_calls == ["job-1"]
        assert store.deleted_job_ids == ["job-1"]

    asyncio.run(scenario())


def test_cron_service_run_now_for_main_target_enqueues_system_event_and_wake():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner
        from agent.service.automation.cron.cron_service import CronService

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        wake_service = FakeWakeService()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            wake_service=wake_service,
            agent_run_orchestrator=FakeOrchestrator(),
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )
        service = CronService(
            store=store,
            runner=runner,
            timer=FakeTimer(),
            id_factory=lambda: "job-main",
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )

        await service.create_job(
            AutomationCronJobCreate(
                name="Main Session Job",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="follow up in main session",
                session_target=AutomationSessionTarget(kind="main", wake_mode="now"),
            )
        )

        result = await service.run_now("job-main")

        assert result.status == "queued_to_main_session"
        assert result.run_id is None
        assert event_queue.calls[0]["event_type"] == "cron.trigger"
        assert event_queue.calls[0]["payload"]["instruction"] == "follow up in main session"
        assert wake_service.calls == [
            {
                "agent_id": "nexus",
                "session_key": "agent:nexus:automation:dm:main",
                "wake_mode": "now",
                "metadata": {
                    "job_id": "job-main",
                    "trigger_kind": "manual",
                },
            }
        ]
        assert await service.list_runs("job-main") == []

    asyncio.run(scenario())


def test_cron_service_run_now_routes_non_main_targets_through_orchestrator_and_tracks_runs():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner
        from agent.service.automation.cron.cron_service import CronService

        store = FakeCronStore()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=FakeSystemEventQueue(),
            wake_service=FakeWakeService(),
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )
        ids = iter(["job-iso", "job-bound", "job-named", "run-iso", "run-bound", "run-named"])
        service = CronService(
            store=store,
            runner=runner,
            timer=FakeTimer(),
            id_factory=lambda: next(ids),
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )

        await service.create_job(
            AutomationCronJobCreate(
                name="Isolated",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="isolated run",
            )
        )
        await service.create_job(
            AutomationCronJobCreate(
                name="Bound",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="bound run",
                session_target=AutomationSessionTarget(
                    kind="bound",
                    bound_session_key="agent:nexus:ws:dm:bound-room",
                ),
            )
        )
        await service.create_job(
            AutomationCronJobCreate(
                name="Named",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="named run",
                session_target=AutomationSessionTarget(
                    kind="named",
                    named_session_key="nightly-ops",
                ),
            )
        )

        isolated_result = await service.run_now("job-iso")
        bound_result = await service.run_now("job-bound")
        named_result = await service.run_now("job-named")
        isolated_runs = await service.list_runs("job-iso")
        bound_runs = await service.list_runs("job-bound")
        named_runs = await service.list_runs("job-named")

        assert [ctx.session_key for ctx in orchestrator.calls] == [
            "agent:nexus:automation:dm:cron:job-iso:run-iso",
            "agent:nexus:ws:dm:bound-room",
            "agent:nexus:automation:dm:nightly-ops",
        ]
        assert isolated_result.run_id == "run-iso"
        assert isolated_result.session_key == "agent:nexus:automation:dm:cron:job-iso:run-iso"
        assert bound_result.run_id == "run-bound"
        assert bound_result.session_key == "agent:nexus:ws:dm:bound-room"
        assert named_result.run_id == "run-named"
        assert named_result.session_key == "agent:nexus:automation:dm:nightly-ops"
        assert isolated_runs[0].status == "succeeded"
        assert isolated_runs[0].attempts == 1
        assert bound_runs[0].status == "succeeded"
        assert named_runs[0].status == "succeeded"

    asyncio.run(scenario())
