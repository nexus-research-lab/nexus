# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：test_session_manager_runtime_refresh.py
# @Date   ：2026/04/16 10:32
# @Author ：leemysw
# 2026/04/16 10:32   Create
# =====================================================

from __future__ import annotations

import asyncio
from pathlib import Path
from types import SimpleNamespace

from agent.service.session.session_manager import SessionManager


class FakeClient:
    """会话刷新测试用 client。"""

    def __init__(self) -> None:
        self.options = SimpleNamespace(
            cwd=Path("/tmp"),
            env={"ANTHROPIC_BASE_URL": "https://glm.example.com"},
            system_prompt="prompt-a",
            allowed_tools=["Bash"],
            disallowed_tools=[],
            max_turns=None,
            max_thinking_tokens=None,
            setting_sources=["project"],
            tools=None,
            fallback_model=None,
            betas=[],
            sandbox=None,
            thinking=None,
            effort=None,
            output_format=None,
            mcp_servers={},
            permission_mode="default",
            model="glm-5-turbo",
            resume=None,
        )
        self._query = SimpleNamespace(_closed=False)
        self.permission_mode_updates: list[str] = []
        self.model_updates: list[str | None] = []
        self.disconnect_calls = 0

    async def set_permission_mode(self, mode: str) -> None:
        self.permission_mode_updates.append(mode)

    async def set_model(self, model: str | None) -> None:
        self.model_updates.append(model)

    async def disconnect(self) -> None:
        self.disconnect_calls += 1
        self._query = None


def test_update_session_options_hot_updates_permission_mode(monkeypatch):
    async def fake_build_sdk_options(_agent_id: str):
        return {
            "cwd": "/tmp",
            "env": {"ANTHROPIC_BASE_URL": "https://glm.example.com"},
            "system_prompt": "prompt-a",
            "allowed_tools": ["Bash"],
            "setting_sources": ["project"],
            "permission_mode": "bypassPermissions",
            "model": "glm-5-turbo",
        }

    async def scenario():
        manager = SessionManager()
        client = FakeClient()
        manager._sessions["agent:test:ws:dm:main"] = client

        monkeypatch.setattr(
            "agent.service.agent.agent_manager.agent_manager",
            SimpleNamespace(build_sdk_options=fake_build_sdk_options),
        )

        updated = await manager.update_session_options("agent:test:ws:dm:main", "agent-1")

        assert updated is True
        assert client.permission_mode_updates == ["bypassPermissions"]
        assert client.options.permission_mode == "bypassPermissions"
        assert await manager.get_session("agent:test:ws:dm:main") is client
        assert await manager.get_reusable_session("agent:test:ws:dm:main") is client
        assert manager.needs_reconnect("agent:test:ws:dm:main") is False

    asyncio.run(scenario())


def test_update_session_options_keeps_client_visible_when_provider_requires_reconnect(monkeypatch):
    async def fake_build_sdk_options(_agent_id: str):
        return {
            "cwd": "/tmp",
            "env": {"ANTHROPIC_BASE_URL": "https://kimi.example.com"},
            "system_prompt": "prompt-a",
            "allowed_tools": ["Bash"],
            "setting_sources": ["project"],
            "permission_mode": "default",
            "model": "glm-5-turbo",
        }

    async def scenario():
        manager = SessionManager()
        client = FakeClient()
        manager._sessions["agent:test:ws:dm:main"] = client

        monkeypatch.setattr(
            "agent.service.agent.agent_manager.agent_manager",
            SimpleNamespace(build_sdk_options=fake_build_sdk_options),
        )

        updated = await manager.update_session_options("agent:test:ws:dm:main", "agent-1")

        assert updated is True
        assert await manager.get_session("agent:test:ws:dm:main") is client
        assert await manager.get_reusable_session("agent:test:ws:dm:main") is None
        assert manager.needs_reconnect("agent:test:ws:dm:main") is True

    asyncio.run(scenario())


def test_prepare_session_reconnect_reuses_same_client_object():
    async def scenario():
        manager = SessionManager()
        client = FakeClient()
        manager._sessions["agent:test:ws:dm:main"] = client
        manager._reconnect_sessions.add("agent:test:ws:dm:main")

        refreshed = await manager.prepare_session_reconnect(
            session_key="agent:test:ws:dm:main",
            can_use_tool=None,
            session_id="sdk-session-1",
            session_options={
                "cwd": "/tmp",
                "env": {"ANTHROPIC_BASE_URL": "https://kimi.example.com"},
                "system_prompt": "prompt-b",
                "allowed_tools": ["Bash"],
                "setting_sources": ["project"],
                "permission_mode": "bypassPermissions",
                "model": "kimi-k2.5",
            },
        )

        assert refreshed is client
        assert client.disconnect_calls == 1
        assert client.options.resume == "sdk-session-1"
        assert client.options.model == "kimi-k2.5"
        assert manager.needs_reconnect("agent:test:ws:dm:main") is False

    asyncio.run(scenario())
