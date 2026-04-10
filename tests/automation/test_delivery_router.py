from __future__ import annotations

import asyncio
from pathlib import Path

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


class DeliveryContractChannel:
    """实现 outbound contract 的通道替身。"""

    channel_type = "telegram"

    def __init__(self) -> None:
        self.calls: list[dict[str, str | None]] = []

    async def start(self) -> None:
        return None

    async def stop(self) -> None:
        return None

    async def send_delivery_text(
        self,
        *,
        to: str,
        text: str,
        account_id: str | None = None,
        thread_id: str | None = None,
    ) -> None:
        self.calls.append(
            {
                "to": to,
                "text": text,
                "account_id": account_id,
                "thread_id": thread_id,
            }
        )


class PlainChannel:
    """不实现 outbound contract 的通道替身。"""

    channel_type = "telegram"

    async def start(self) -> None:
        return None

    async def stop(self) -> None:
        return None


class ActiveMessageRecorder:
    """记录 websocket 在线推送走的是消息路径还是事件路径。"""

    def __init__(self) -> None:
        self.messages = []
        self.events = []

    async def send_message(self, message) -> None:
        self.messages.append(message)

    async def send_stream_message(self, message) -> None:
        raise AssertionError(f"unexpected stream push: {message}")

    async def send_event_message(self, event) -> None:
        self.events.append(event)

    async def send(self, message) -> None:
        from agent.service.channels.message_sender import MessageSender

        await MessageSender.send(self, message)


class FallbackEventRecorder:
    """记录 websocket fallback 发送。"""

    def __init__(self) -> None:
        self.events = []

    async def send_message(self, message) -> None:
        raise AssertionError(f"unexpected fallback message: {message}")

    async def send_stream_message(self, message) -> None:
        raise AssertionError(f"unexpected fallback stream: {message}")

    async def send_event_message(self, event) -> None:
        self.events.append(event)


class FailFirstEventRecorder:
    """首条 event 推送失败，但后续推送继续工作的替身。"""

    def __init__(self) -> None:
        self.events = []
        self.failures = 0

    async def send_message(self, message) -> None:
        raise AssertionError(f"unexpected direct message push: {message}")

    async def send_stream_message(self, message) -> None:
        raise AssertionError(f"unexpected stream push: {message}")

    async def send_event_message(self, event) -> None:
        self.events.append(event)
        if self.failures == 0:
            self.failures += 1
            raise RuntimeError("simulated first push failure")

    async def send(self, message) -> None:
        from agent.service.channels.message_sender import MessageSender

        await MessageSender.send(self, message)


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


def test_delivery_router_lazily_resolves_channel_delivery_contract():
    async def scenario():
        from agent.service.automation.delivery.delivery_router import DeliveryRouter
        from agent.service.channels.channel_register import ChannelRegister

        register = ChannelRegister()
        channel = DeliveryContractChannel()
        register.register(channel)
        router = DeliveryRouter(
            memory=StaticMemory(),
            channel_register=register,
        )

        target = await router.send_text(
            agent_id="nexus",
            text="hello lazy channel",
            target={"mode": "explicit", "channel": "telegram", "to": "10001"},
        )

        assert target.channel == "telegram"
        assert channel.calls == [
            {
                "to": "10001",
                "text": "hello lazy channel",
                "account_id": None,
                "thread_id": None,
            }
        ]

    asyncio.run(scenario())


def test_delivery_router_rejects_channel_without_delivery_contract():
    async def scenario():
        from agent.service.automation.delivery.delivery_router import DeliveryRouter
        from agent.service.channels.channel_register import ChannelRegister

        register = ChannelRegister()
        register.register(PlainChannel())
        router = DeliveryRouter(
            memory=StaticMemory(),
            channel_register=register,
        )

        with pytest.raises(TypeError, match="send_delivery_text"):
            await router.send_text(
                agent_id="nexus",
                text="hello lazy channel",
                target={"mode": "explicit", "channel": "telegram", "to": "10001"},
            )

    asyncio.run(scenario())


def test_delivery_router_routes_to_last_route(async_session_factory):
    async def scenario():
        from agent.service.automation.delivery.delivery_memory import DeliveryMemory
        from agent.service.automation.delivery.delivery_router import DeliveryRouter

        sender = RecordingSender()
        memory = DeliveryMemory(session_factory=async_session_factory)
        await memory.remember_route(agent_id="nexus", channel="discord", to="9001")

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
        assert target.to == "9001"
        assert len(sender.calls) == 1
        sent_target, sent_text = sender.calls[0]
        assert sent_target.channel == "discord"
        assert sent_target.to == "9001"
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


def test_websocket_delivery_persists_history_and_pushes_message(tmp_path, monkeypatch):
    async def scenario():
        from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender
        from agent.service.channels.ws.ws_session_replay_registry import (
            ws_session_replay_registry,
        )
        from agent.service.permission.permission_runtime_context import (
            permission_runtime_context,
        )
        from agent.service.session.session_router import build_session_key
        from agent.service.session.session_store import session_store
        from agent.service.session.session_repository import session_repository

        workspace_path = tmp_path / "ws-agent"
        workspace_path.mkdir(parents=True, exist_ok=True)

        async def resolve_workspace_path(_agent_id: str) -> Path:
            return workspace_path

        monkeypatch.setattr(
            session_repository,
            "_resolve_workspace_path",
            resolve_workspace_path,
        )
        monkeypatch.setattr(
            session_repository,
            "_iter_known_workspace_paths",
            lambda: [workspace_path],
        )

        session_key = build_session_key(
            channel="ws",
            chat_type="dm",
            ref="delivery-test",
            agent_id="nexus",
        )
        created = await session_store.create_session_by_key(
            session_key=session_key,
            channel_type="websocket",
            chat_type="dm",
        )
        assert created is not None

        active_sender = ActiveMessageRecorder()
        fallback_sender = FallbackEventRecorder()
        monkeypatch.setattr(
            permission_runtime_context,
            "resolve_session_sender",
            lambda key: active_sender if key == session_key else None,
        )
        ws_session_replay_registry._session_sequences.clear()
        ws_session_replay_registry._session_replay_buffers.clear()

        sender = WsSessionRoutingSender(fallback_sender)
        await sender.send_text(session_key=session_key, text="durable websocket delivery")

        messages = await session_store.get_session_messages(session_key)

        assert [message.role for message in messages] == ["assistant", "result"]
        assistant_message, result_message = messages
        assert assistant_message.session_key == session_key
        assert len(assistant_message.content or []) == 1
        assert assistant_message.content[0].type == "text"
        assert assistant_message.content[0].text == "durable websocket delivery"
        assert result_message.session_key == session_key
        assert result_message.subtype == "success"
        assert result_message.result == "durable websocket delivery"

        assert active_sender.messages == []
        assert len(active_sender.events) == 2
        assert [event.session_seq for event in active_sender.events] == [1, 2]
        assert active_sender.events[0].event_type == "message"
        assert active_sender.events[0].data["role"] == "assistant"
        assert active_sender.events[0].data["content"] == [{"type": "text", "text": "durable websocket delivery"}]
        assert active_sender.events[1].event_type == "message"
        assert active_sender.events[1].data["role"] == "result"
        assert active_sender.events[1].data["result"] == "durable websocket delivery"
        assert fallback_sender.events == []

    asyncio.run(scenario())


def test_websocket_delivery_buffers_later_events_after_first_live_push_failure(tmp_path, monkeypatch):
    async def scenario():
        from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender
        from agent.service.channels.ws.ws_session_replay_registry import (
            ws_session_replay_registry,
        )
        from agent.service.permission.permission_runtime_context import (
            permission_runtime_context,
        )
        from agent.service.session.session_router import build_session_key
        from agent.service.session.session_store import session_store
        from agent.service.session.session_repository import session_repository

        workspace_path = tmp_path / "ws-agent-fail-once"
        workspace_path.mkdir(parents=True, exist_ok=True)

        async def resolve_workspace_path(_agent_id: str) -> Path:
            return workspace_path

        monkeypatch.setattr(
            session_repository,
            "_resolve_workspace_path",
            resolve_workspace_path,
        )
        monkeypatch.setattr(
            session_repository,
            "_iter_known_workspace_paths",
            lambda: [workspace_path],
        )

        session_key = build_session_key(
            channel="ws",
            chat_type="dm",
            ref="delivery-fail-once",
            agent_id="nexus",
        )
        created = await session_store.create_session_by_key(
            session_key=session_key,
            channel_type="websocket",
            chat_type="dm",
        )
        assert created is not None

        active_sender = FailFirstEventRecorder()
        fallback_sender = FallbackEventRecorder()
        monkeypatch.setattr(
            permission_runtime_context,
            "resolve_session_sender",
            lambda key: active_sender if key == session_key else None,
        )
        ws_session_replay_registry._session_sequences.clear()
        ws_session_replay_registry._session_replay_buffers.clear()

        sender = WsSessionRoutingSender(fallback_sender)
        await sender.send_text(session_key=session_key, text="deliver despite first push failure")

        replay_buffer = list(
            ws_session_replay_registry._session_replay_buffers.get(session_key, ())
        )

        assert [event.session_seq for event in replay_buffer] == [1, 2]
        assert replay_buffer[0].data["role"] == "assistant"
        assert replay_buffer[1].data["role"] == "result"
        assert [event.session_seq for event in active_sender.events] == [1, 2]
        assert fallback_sender.events == []

    asyncio.run(scenario())
