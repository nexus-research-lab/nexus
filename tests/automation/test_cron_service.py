from __future__ import annotations

import asyncio
import importlib
import pathlib
import sys
from datetime import datetime, timezone
from types import ModuleType, SimpleNamespace

PROJECT_ROOT = pathlib.Path(__file__).resolve().parents[2]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

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
        self.failed_ids: list[str] = []
        self.pending_event_ids: list[str] = []

    async def enqueue(self, **fields):
        event_id = str(fields.get("event_id") or f"event-{len(self.calls) + 1}")
        payload = {
            **fields,
            "event_id": event_id,
        }
        self.calls.append(payload)
        self.pending_event_ids.append(event_id)
        return SimpleNamespace(**payload)

    async def mark_failed(self, event_id: str):
        self.failed_ids.append(event_id)
        self.pending_event_ids = [item for item in self.pending_event_ids if item != event_id]
        return None


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


class FakeHeartbeatService:
    """记录真实 heartbeat runtime 入口是否被调用。"""

    def __init__(self, *, wake_error: Exception | None = None) -> None:
        self.start_calls = 0
        self.stop_calls = 0
        self.wake_calls: list[dict[str, object]] = []
        self.wake_error = wake_error

    async def start(self) -> None:
        self.start_calls += 1

    async def stop(self) -> None:
        self.stop_calls += 1

    async def wake(self, *, agent_id: str, mode: str, text: str | None = None):
        self.wake_calls.append(
            {
                "agent_id": agent_id,
                "mode": mode,
                "text": text,
            }
        )
        if self.wake_error is not None:
            raise self.wake_error
        return SimpleNamespace(agent_id=agent_id, mode=mode, scheduled=(mode == "now"))

    def set_channel_register(self, _channel_manager) -> None:
        return None


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
            heartbeat_service=FakeHeartbeatService(),
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
        heartbeat_service = FakeHeartbeatService()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
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
        assert event_queue.calls[0]["payload"]["text"] == "follow up in main session"
        assert "instruction" not in event_queue.calls[0]["payload"]
        assert heartbeat_service.wake_calls == [
            {
                "agent_id": "nexus",
                "mode": "now",
                "text": None,
            }
        ]
        assert await service.list_runs("job-main") == []

    asyncio.run(scenario())


def test_cron_service_run_now_for_main_target_marks_event_failed_when_wake_fails():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner
        from agent.service.automation.cron.cron_service import CronService

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService(wake_error=RuntimeError("wake boom"))
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=FakeOrchestrator(),
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )
        service = CronService(
            store=store,
            runner=runner,
            timer=FakeTimer(),
            id_factory=lambda: "job-main-fail",
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )

        await service.create_job(
            AutomationCronJobCreate(
                name="Main Session Wake Failure",
                agent_id="nexus",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="follow up in main session",
                session_target=AutomationSessionTarget(kind="main", wake_mode="now"),
            )
        )

        try:
            await service.run_now("job-main-fail")
        except RuntimeError as exc:
            assert str(exc) == "wake boom"
        else:
            raise AssertionError("expected wake failure to propagate")

        assert event_queue.failed_ids == ["event-1"]
        assert event_queue.pending_event_ids == []

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
            heartbeat_service=FakeHeartbeatService(),
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


def test_cron_service_run_now_preserves_session_target_semantics():
    async def scenario():
        from agent.service.automation.cron.cron_runner import CronRunner
        from agent.service.automation.cron.cron_service import CronService

        store = FakeCronStore()
        event_queue = FakeSystemEventQueue()
        heartbeat_service = FakeHeartbeatService()
        orchestrator = FakeOrchestrator()
        runner = CronRunner(
            store=store,
            system_event_queue=event_queue,
            heartbeat_service=heartbeat_service,
            agent_run_orchestrator=orchestrator,
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )
        ids = iter(
            [
                "job-main",
                "job-bound",
                "job-named",
                "job-isolated",
                "run-bound",
                "run-named",
                "run-isolated",
            ]
        )
        service = CronService(
            store=store,
            runner=runner,
            timer=FakeTimer(),
            id_factory=lambda: next(ids),
            now_fn=lambda: datetime(2026, 4, 10, 1, 20, tzinfo=timezone.utc),
        )

        await service.create_job(
            AutomationCronJobCreate(
                name="Main",
                agent_id="research",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="main instruction",
                session_target=AutomationSessionTarget(kind="main", wake_mode="now"),
            )
        )
        await service.create_job(
            AutomationCronJobCreate(
                name="Bound",
                agent_id="research",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="bound instruction",
                session_target=AutomationSessionTarget(
                    kind="bound",
                    bound_session_key="agent:research:ws:dm:existing-room",
                ),
            )
        )
        await service.create_job(
            AutomationCronJobCreate(
                name="Named",
                agent_id="research",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="named instruction",
                session_target=AutomationSessionTarget(
                    kind="named",
                    named_session_key="morning-brief",
                ),
            )
        )
        await service.create_job(
            AutomationCronJobCreate(
                name="Isolated",
                agent_id="research",
                schedule=AutomationCronSchedule(kind="every", interval_seconds=60),
                instruction="isolated instruction",
            )
        )

        main_result = await service.run_now("job-main")
        bound_result = await service.run_now("job-bound")
        named_result = await service.run_now("job-named")
        isolated_result = await service.run_now("job-isolated")

        assert main_result.status == "queued_to_main_session"
        assert main_result.run_id is None
        assert main_result.session_key == "agent:research:automation:dm:main"
        assert bound_result.session_key == "agent:research:ws:dm:existing-room"
        assert named_result.session_key == "agent:research:automation:dm:morning-brief"
        assert isolated_result.session_key.startswith("agent:research:automation:dm:cron:")
        assert event_queue.calls[0]["payload"]["text"] == "main instruction"
        assert [ctx.session_key for ctx in orchestrator.calls] == [
            "agent:research:ws:dm:existing-room",
            "agent:research:automation:dm:morning-brief",
            isolated_result.session_key,
        ]

    asyncio.run(scenario())


def test_app_lifespan_starts_and_stops_cron_runtime(monkeypatch):
    async def scenario():
        original_router_module = sys.modules.get("agent.api.router")
        heartbeat = FakeHeartbeatService()
        cron = FakeHeartbeatService()
        channel_manager = SimpleNamespace(stop_all=lambda: asyncio.sleep(0))

        async def register_channels() -> None:
            return None

        with monkeypatch.context() as module_patch:
            module_patch.setenv("ENV_FILE", "/dev/null")
            module_patch.setenv("DEBUG", "false")
            for module_name in (
                "agent.app",
                "agent.api.router",
                "agent.config.config",
                "agent.service.channels.channel_register",
                "agent.service.agent.agent_service",
                "agent.infra.server.register",
                "agent.utils.logger",
                "agent.service.automation.heartbeat.heartbeat_service",
                "agent.service.automation.cron.cron_service",
                "agent.service.session.session_repository",
                "agent.service.session.cost_repository",
            ):
                module_patch.delitem(sys.modules, module_name, raising=False)

            api_router_module = ModuleType("agent.api.router")
            api_router_module.api_router = SimpleNamespace(routes=[])
            module_patch.setitem(sys.modules, "agent.api.router", api_router_module)

            config_module = ModuleType("agent.config.config")
            config_module.settings = SimpleNamespace(
                DEBUG=False,
                PROJECT_NAME="Nexus",
                ENABLE_SWAGGER_DOC=False,
            )
            module_patch.setitem(sys.modules, "agent.config.config", config_module)

            channel_register_module = ModuleType("agent.service.channels.channel_register")
            channel_register_module.ChannelRegister = lambda: SimpleNamespace(
                stop_all=lambda: asyncio.sleep(0),
                start_all=lambda: asyncio.sleep(0),
                register=lambda *_args, **_kwargs: None,
            )
            module_patch.setitem(
                sys.modules,
                "agent.service.channels.channel_register",
                channel_register_module,
            )

            agent_service_module = ModuleType("agent.service.agent.agent_service")
            agent_service_module.agent_service = SimpleNamespace()
            module_patch.setitem(
                sys.modules,
                "agent.service.agent.agent_service",
                agent_service_module,
            )

            register_module = ModuleType("agent.infra.server.register")
            register_module.register_exception = lambda _app: None
            register_module.register_hook = lambda _app: None
            register_module.register_middleware = lambda _app: None
            module_patch.setitem(sys.modules, "agent.infra.server.register", register_module)

            logger_module = ModuleType("agent.utils.logger")
            logger_module.logger = SimpleNamespace(
                info=lambda *_args, **_kwargs: None,
            )
            module_patch.setitem(sys.modules, "agent.utils.logger", logger_module)

            heartbeat_module = ModuleType("agent.service.automation.heartbeat.heartbeat_service")
            heartbeat_module.heartbeat_service = FakeHeartbeatService()
            module_patch.setitem(
                sys.modules,
                "agent.service.automation.heartbeat.heartbeat_service",
                heartbeat_module,
            )

            cron_module = ModuleType("agent.service.automation.cron.cron_service")
            cron_module.get_cron_service = lambda: FakeHeartbeatService()
            module_patch.setitem(
                sys.modules,
                "agent.service.automation.cron.cron_service",
                cron_module,
            )

            session_repository_module = ModuleType("agent.service.session.session_repository")
            session_repository_module.session_repository = SimpleNamespace(
                ensure_ready=lambda: None
            )
            module_patch.setitem(
                sys.modules,
                "agent.service.session.session_repository",
                session_repository_module,
            )

            cost_repository_module = ModuleType("agent.service.session.cost_repository")
            cost_repository_module.cost_repository = SimpleNamespace(
                ensure_ready=lambda: None
            )
            module_patch.setitem(
                sys.modules,
                "agent.service.session.cost_repository",
                cost_repository_module,
            )

            app_module = importlib.import_module("agent.app")
            module_patch.setattr(app_module, "heartbeat_service", heartbeat)
            module_patch.setattr(app_module, "get_cron_service", lambda: cron)
            module_patch.setattr(app_module, "channel_manager", channel_manager)
            module_patch.setattr(app_module, "_register_channels", register_channels)

            async with app_module.lifespan(SimpleNamespace()):
                assert heartbeat.start_calls == 1
                assert cron.start_calls == 1

        assert heartbeat.stop_calls == 1
        assert cron.stop_calls == 1
        if original_router_module is None:
            assert "agent.api.router" not in sys.modules
        else:
            assert sys.modules.get("agent.api.router") is original_router_module

    asyncio.run(scenario())
