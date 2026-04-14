from __future__ import annotations

import asyncio
from datetime import datetime
from contextlib import asynccontextmanager

import pytest
from sqlalchemy import event
from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from agent.infra.database.async_sqlalchemy import AsyncDatabase, Base
from agent.infra.database.models import load_models


@pytest.fixture
def async_session_factory(tmp_path):
    """为仓储测试准备一个独立的 SQLite 会话工厂。"""
    load_models()
    engine = create_async_engine(f"sqlite+aiosqlite:///{tmp_path / 'automation.db'}")
    _enable_sqlite_foreign_keys(engine)
    asyncio.run(_create_tables(engine))

    session_factory = async_sessionmaker(
        engine,
        class_=AsyncSession,
        expire_on_commit=False,
    )
    asyncio.run(_seed_agent(session_factory, tmp_path))
    yield session_factory

    asyncio.run(engine.dispose())


def _enable_sqlite_foreign_keys(engine):
    """让测试用 SQLite 也严格执行外键约束。"""
    @event.listens_for(engine.sync_engine, "connect")
    def _set_sqlite_foreign_keys(dbapi_connection, _connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("PRAGMA foreign_keys=ON")
        cursor.close()


async def _create_tables(engine):
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)


async def _seed_agent(session_factory, tmp_path):
    from agent.infra.database.models.agent import Agent

    async with session_factory() as session:
        session.add(
            Agent(
                id="nexus",
                slug="nexus",
                name="Nexus",
                description="",
                definition="",
                status="active",
                workspace_path=str(tmp_path / "workspace"),
            )
        )
        await session.commit()


@pytest.fixture
def automation_db(tmp_path):
    """为 store service 测试准备一个独立数据库。"""
    load_models()
    db = AsyncDatabase()
    db.init(f"sqlite+aiosqlite:///{tmp_path / 'automation-store.db'}")
    asyncio.run(db.create_tables())
    asyncio.run(_seed_agent(db.session_factory, tmp_path))
    yield db
    asyncio.run(db.close())


def test_cron_job_repository_roundtrip(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_cron_job_sql_repository import (
            AutomationCronJobSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationCronJobSqlRepository(session)
            row = await repo.upsert_job(
                job_id="job-1",
                name="daily",
                agent_id="nexus",
                schedule_kind="every",
            )

            assert row.job_id == "job-1"

            fetched = await repo.get_job("job-1")
            assert fetched is not None
            assert fetched.name == "daily"

    asyncio.run(scenario())


def test_cron_job_delete_cascades_runs(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_cron_job_sql_repository import (
            AutomationCronJobSqlRepository,
        )
        from agent.infra.database.repositories.automation_cron_run_sql_repository import (
            AutomationCronRunSqlRepository,
        )

        async with async_session_factory() as session:
            job_repo = AutomationCronJobSqlRepository(session)
            await job_repo.upsert_job(
                job_id="job-1",
                name="daily",
                agent_id="nexus",
                schedule_kind="every",
            )

            run_repo = AutomationCronRunSqlRepository(session)
            await run_repo.create_run(run_id="run-1", job_id="job-1")
            await job_repo.delete_job("job-1")

            assert await run_repo.get_run("run-1") is None

    asyncio.run(scenario())


def test_sqlite_foreign_keys_are_enforced_in_task2_harness(automation_db):
    async def scenario():
        from agent.infra.database.models.automation_cron_job import AutomationCronJob

        async with automation_db.session() as session:
            session.add(
                AutomationCronJob(
                    job_id="job-fk",
                    name="broken",
                    agent_id="missing-agent",
                    schedule_kind="every",
                    instruction="do something",
                )
            )
            await session.flush()

    with pytest.raises(IntegrityError):
        asyncio.run(scenario())


def test_cron_job_repository_rejects_unknown_fields(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_cron_job_sql_repository import (
            AutomationCronJobSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationCronJobSqlRepository(session)
            await repo.upsert_job(
                job_id="job-1",
                name="daily",
                agent_id="nexus",
                schedule_kind="every",
                unexpected_field="boom",
            )

    with pytest.raises(ValueError):
        asyncio.run(scenario())


def test_cron_run_repository_tracks_status(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_cron_job_sql_repository import (
            AutomationCronJobSqlRepository,
        )
        from agent.infra.database.repositories.automation_cron_run_sql_repository import (
            AutomationCronRunSqlRepository,
        )

        async with async_session_factory() as session:
            job_repo = AutomationCronJobSqlRepository(session)
            await job_repo.upsert_job(
                job_id="job-1",
                name="daily",
                agent_id="nexus",
                schedule_kind="every",
            )

            repo = AutomationCronRunSqlRepository(session)
            run = await repo.create_run(run_id="run-1", job_id="job-1")
            assert run.status == "pending"

            updated = await repo.update_run_status(
                run_id="run-1",
                status="running",
                started_at=datetime(2026, 1, 1, 8, 0, 0),
                attempts=1,
            )
            assert updated is not None
            assert updated.status == "running"

            runs = await repo.list_runs_by_job("job-1")
            assert [item.run_id for item in runs] == ["run-1"]

    asyncio.run(scenario())


def test_cron_run_repository_clears_error_message(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_cron_job_sql_repository import (
            AutomationCronJobSqlRepository,
        )
        from agent.infra.database.repositories.automation_cron_run_sql_repository import (
            AutomationCronRunSqlRepository,
        )

        async with async_session_factory() as session:
            job_repo = AutomationCronJobSqlRepository(session)
            await job_repo.upsert_job(
                job_id="job-1",
                name="daily",
                agent_id="nexus",
                schedule_kind="every",
            )

            repo = AutomationCronRunSqlRepository(session)
            await repo.create_run(run_id="run-1", job_id="job-1", error_message="boom")

            cleared = await repo.update_run_status(
                run_id="run-1",
                status="running",
                error_message=None,
            )
            assert cleared is not None
            assert cleared.error_message is None

    asyncio.run(scenario())


def test_heartbeat_state_store_roundtrip(automation_db, monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_state_store import (
            HeartbeatStateStore,
        )

        import agent.service.automation.heartbeat.heartbeat_state_store as module

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: automation_db)

        store = HeartbeatStateStore()
        row = await store.upsert_state(
            agent_id="nexus",
            enabled=True,
            every_seconds=60,
            target_mode="explicit",
        )
        assert row.agent_id == "nexus"

        fetched = await store.get_state("nexus")
        assert fetched is not None
        assert fetched.enabled is True

    asyncio.run(scenario())


def test_cron_store_service_retries_when_sqlite_is_temporarily_locked(tmp_path, monkeypatch):
    async def scenario():
        from agent.service.automation.cron.cron_store_service import CronStoreService
        import agent.service.automation.cron.cron_store_service as module

        load_models()
        engine = create_async_engine(
            f"sqlite+aiosqlite:///{tmp_path / 'locked.db'}",
            connect_args={"timeout": 0.01},
        )
        _enable_sqlite_foreign_keys(engine)
        await _create_tables(engine)

        session_factory = async_sessionmaker(
            engine,
            class_=AsyncSession,
            expire_on_commit=False,
        )
        await _seed_agent(session_factory, tmp_path)

        class _TestDb:
            @asynccontextmanager
            async def session(self):
                async with session_factory() as session:
                    try:
                        yield session
                    except Exception:
                        await session.rollback()
                        raise
                    finally:
                        await session.close()

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: _TestDb())
        store = CronStoreService(write_retry_attempts=5, write_retry_delay_seconds=0.03)

        async with session_factory() as locking_session:
            await locking_session.execute(text("BEGIN IMMEDIATE"))
            waiting_write = asyncio.create_task(
                store.upsert_job(
                    job_id="job-locked",
                    name="locked",
                    agent_id="nexus",
                    schedule_kind="every",
                )
            )
            await asyncio.sleep(0.08)
            await locking_session.rollback()

        row = await waiting_write
        assert row.job_id == "job-locked"

        await engine.dispose()

    asyncio.run(scenario())


def test_heartbeat_state_store_overwrites_existing_agent_state(automation_db, monkeypatch):
    async def scenario():
        from agent.service.automation.heartbeat.heartbeat_state_store import (
            HeartbeatStateStore,
        )

        import agent.service.automation.heartbeat.heartbeat_state_store as module

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: automation_db)

        store = HeartbeatStateStore()
        first = await store.upsert_state(
            agent_id="nexus",
            enabled=True,
            every_seconds=60,
            target_mode="explicit",
        )
        second = await store.upsert_state(
            agent_id="nexus",
            enabled=False,
            every_seconds=120,
            target_mode="none",
        )

        assert first.state_id == second.state_id
        assert second.enabled is False
        assert second.every_seconds == 120
        assert second.target_mode == "none"

        fetched = await store.get_state("nexus")
        assert fetched is not None
        assert fetched.enabled is False

    asyncio.run(scenario())


def test_delivery_route_repository_returns_latest_route(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_delivery_route_sql_repository import (
            AutomationDeliveryRouteSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationDeliveryRouteSqlRepository(session)
            await repo.upsert_route(route_id="route-1", agent_id="nexus", mode="last")
            await repo.upsert_route(route_id="route-2", agent_id="nexus", mode="explicit")
            await repo.upsert_route(
                route_id="route-1",
                agent_id="nexus",
                mode="explicit",
            )

            latest = await repo.get_latest_route("nexus")
            assert latest is not None
            assert latest.route_id == "route-1"
            assert latest.mode == "explicit"

    asyncio.run(scenario())


def test_delivery_route_repository_rejects_unknown_fields(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_delivery_route_sql_repository import (
            AutomationDeliveryRouteSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationDeliveryRouteSqlRepository(session)
            await repo.upsert_route(
                route_id="route-1",
                agent_id="nexus",
                mode="last",
                unexpected_field="boom",
            )

    with pytest.raises(ValueError):
        asyncio.run(scenario())


def test_system_event_repository_marks_processed_and_failed(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_system_event_sql_repository import (
            AutomationSystemEventSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationSystemEventSqlRepository(session)
            created = await repo.create_event(
                event_id="event-1",
                event_type="cron.job.created",
                payload={"job_id": "job-1"},
            )
            assert created.status == "new"

            pending = await repo.list_pending_events()
            assert [item.event_id for item in pending] == ["event-1"]

            processed = await repo.mark_processed("event-1")
            assert processed is not None
            assert processed.status == "processed"

            second = await repo.create_event(
                event_id="event-2",
                event_type="cron.job.failed",
                payload={"job_id": "job-1"},
            )
            assert second.status == "new"

            failed = await repo.mark_failed("event-2")
            assert failed is not None
            assert failed.status == "failed"

            assert await repo.list_pending_events() == []

    asyncio.run(scenario())


def test_system_event_repository_marks_processing(async_session_factory):
    async def scenario():
        from agent.infra.database.repositories.automation_system_event_sql_repository import (
            AutomationSystemEventSqlRepository,
        )

        async with async_session_factory() as session:
            repo = AutomationSystemEventSqlRepository(session)
            await repo.create_event(
                event_id="event-1",
                event_type="cron.job.created",
                payload={"job_id": "job-1"},
            )

            processing = await repo.mark_processing("event-1")
            assert processing is not None
            assert processing.status == "processing"
            assert await repo.list_pending_events() == []

    asyncio.run(scenario())


def test_cron_store_service_roundtrip(automation_db, monkeypatch):
    async def scenario():
        from agent.service.automation.cron.cron_store_service import CronStoreService

        import agent.service.automation.cron.cron_store_service as module

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: automation_db)

        store = CronStoreService()
        await store.upsert_job(
            job_id="job-1",
            name="daily",
            agent_id="nexus",
            schedule_kind="every",
        )

        jobs = await store.list_jobs()
        assert [job.job_id for job in jobs] == ["job-1"]

    asyncio.run(scenario())
