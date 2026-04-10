from __future__ import annotations

import asyncio

import pytest
from sqlalchemy import event
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from agent.infra.database.async_sqlalchemy import AsyncDatabase, Base
from agent.infra.database.models import load_models


@pytest.fixture
def automation_db(tmp_path):
    """为系统事件队列测试准备一个独立数据库。"""
    load_models()
    db = AsyncDatabase()
    db.init(f"sqlite+aiosqlite:///{tmp_path / 'automation-system-event.db'}")
    asyncio.run(db.create_tables())
    asyncio.run(_seed_agent(db.session_factory, tmp_path))
    yield db
    asyncio.run(db.close())


@pytest.fixture
def async_session_factory(tmp_path):
    """为 SQL 仓储测试准备独立 SQLite 会话工厂。"""
    load_models()
    engine = create_async_engine(f"sqlite+aiosqlite:///{tmp_path / 'automation-system-event-repo.db'}")
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
    """让测试用 SQLite 严格执行外键约束。"""

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


def test_system_event_queue_roundtrip(automation_db, monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.system_event_queue import SystemEventQueue
        import agent.service.automation.runtime.system_event_queue as module

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: automation_db)

        queue = SystemEventQueue()
        created = await queue.enqueue(
            event_type="cron.job.created",
            source_type="cron",
            source_id="job-1",
            payload={"job_id": "job-1"},
        )

        assert created.status == "new"

        pending = await queue.list_pending_events()
        assert [item.event_id for item in pending] == [created.event_id]

        processing = await queue.mark_processing(created.event_id)
        assert processing is not None
        assert processing.status == "processing"
        assert await queue.list_pending_events() == []

        processed = await queue.mark_processed(created.event_id)
        assert processed is not None
        assert processed.status == "processed"
        assert processed.processed_at is not None

    asyncio.run(scenario())


def test_system_event_queue_can_mark_failed(automation_db, monkeypatch):
    async def scenario():
        from agent.service.automation.runtime.system_event_queue import SystemEventQueue
        import agent.service.automation.runtime.system_event_queue as module

        monkeypatch.setattr(module, "get_db", lambda *_args, **_kwargs: automation_db)

        queue = SystemEventQueue()
        created = await queue.enqueue(
            event_id="event-2",
            event_type="cron.job.failed",
            payload={"job_id": "job-2"},
        )

        failed = await queue.mark_failed(created.event_id)

        assert failed is not None
        assert failed.status == "failed"
        assert failed.processed_at is not None
        assert await queue.list_pending_events() == []

    asyncio.run(scenario())
