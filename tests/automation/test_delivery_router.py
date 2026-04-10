from __future__ import annotations

import asyncio

import pytest
from sqlalchemy import event
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models import load_models


@pytest.fixture
def async_session_factory(tmp_path):
    """为 delivery router 测试准备独立的 SQLite 会话工厂。"""
    load_models()
    engine = create_async_engine(f"sqlite+aiosqlite:///{tmp_path / 'delivery-router.db'}")
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


class RecordingSender:
    """记录 outbound 调用的测试替身。"""

    def __init__(self) -> None:
        self.calls: list[tuple[object, str]] = []

    async def send_text(self, target, text: str) -> None:
        self.calls.append((target, text))


class StaticMemory:
    """返回固定 last route 的测试替身。"""

    def __init__(self, target=None) -> None:
        self._target = target

    async def get_last_route(self, agent_id: str):
        del agent_id
        return self._target


def test_resolve_delivery_target_defaults_to_none():
    from agent.service.automation.delivery.delivery_target import resolve_delivery_target

    target = resolve_delivery_target(None)

    assert target.mode == "none"
    assert target.channel is None
    assert target.to is None


def test_explicit_delivery_target_requires_channel_and_to():
    from agent.service.automation.delivery.delivery_target import resolve_delivery_target

    target = resolve_delivery_target({"mode": "explicit", "channel": "telegram", "to": "10001"})

    assert target.channel == "telegram"
    assert target.to == "10001"


def test_explicit_delivery_target_rejects_missing_to():
    from agent.service.automation.delivery.delivery_target import resolve_delivery_target

    with pytest.raises(ValueError, match="to"):
        resolve_delivery_target({"mode": "explicit", "channel": "telegram"})


def test_channel_register_get_required_raises_for_missing_channel():
    from agent.service.channels.channel_register import ChannelRegister

    register = ChannelRegister()

    with pytest.raises(LookupError, match="telegram"):
        register.get_required("telegram")


def test_delivery_memory_returns_latest_route(async_session_factory):
    async def scenario():
        from agent.service.automation.delivery.delivery_memory import DeliveryMemory

        memory = DeliveryMemory(session_factory=async_session_factory)
        await memory.remember_route(
            agent_id="nexus",
            channel="telegram",
            to="10001",
            account_id="ops",
            thread_id="42",
        )

        target = await memory.get_last_route("nexus")

        assert target is not None
        assert target.mode == "explicit"
        assert target.channel == "telegram"
        assert target.to == "10001"
        assert target.account_id == "ops"
        assert target.thread_id == "42"

    asyncio.run(scenario())


def test_delivery_router_routes_explicit_target():
    async def scenario():
        from agent.service.automation.delivery.delivery_router import DeliveryRouter

        sender = RecordingSender()
        router = DeliveryRouter(
            memory=StaticMemory(),
            senders={"telegram": sender},
        )

        target = await router.send_text(
            agent_id="nexus",
            text="hello automation",
            target={"mode": "explicit", "channel": "telegram", "to": "10001"},
        )

        assert target.channel == "telegram"
        assert target.to == "10001"
        assert len(sender.calls) == 1
        sent_target, sent_text = sender.calls[0]
        assert sent_target.channel == "telegram"
        assert sent_target.to == "10001"
        assert sent_text == "hello automation"

    asyncio.run(scenario())


def test_delivery_router_routes_to_last_route(async_session_factory):
    async def scenario():
        from agent.service.automation.delivery.delivery_memory import DeliveryMemory
        from agent.service.automation.delivery.delivery_router import DeliveryRouter

        sender = RecordingSender()
        memory = DeliveryMemory(session_factory=async_session_factory)
        await memory.remember_route(agent_id="nexus", channel="discord", to="channel-9")

        router = DeliveryRouter(
            memory=memory,
            senders={"discord": sender},
        )

        target = await router.send_text(
            agent_id="nexus",
            text="heartbeat",
            target={"mode": "last"},
        )

        assert target.channel == "discord"
        assert target.to == "channel-9"
        assert len(sender.calls) == 1
        sent_target, sent_text = sender.calls[0]
        assert sent_target.channel == "discord"
        assert sent_target.to == "channel-9"
        assert sent_text == "heartbeat"

    asyncio.run(scenario())


def test_delivery_router_rejects_last_route_without_memory():
    async def scenario():
        from agent.service.automation.delivery.delivery_router import DeliveryRouter

        router = DeliveryRouter(
            memory=StaticMemory(),
            senders={"telegram": RecordingSender()},
        )

        with pytest.raises(LookupError, match="nexus"):
            await router.send_text(
                agent_id="nexus",
                text="heartbeat",
                target={"mode": "last"},
            )

    asyncio.run(scenario())
