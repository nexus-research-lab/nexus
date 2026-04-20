from __future__ import annotations

import asyncio
from types import SimpleNamespace


def test_get_agent_sessions_filters_automation_sessions(monkeypatch):
    async def fake_get_agent(_agent_id: str):
        return SimpleNamespace(agent_id="agent-1")

    async def fake_get_all_sessions():
        return [
            SimpleNamespace(
                session_key="agent:agent-1:ws:dm:launcher-app-agent-1",
                agent_id="agent-1",
            ),
            SimpleNamespace(
                session_key="agent:agent-1:automation:dm:main",
                agent_id="agent-1",
            ),
        ]

    async def scenario():
        from agent.service.agent.agent_service import AgentService

        service = AgentService()
        monkeypatch.setattr(service, "get_agent", fake_get_agent)
        monkeypatch.setattr(
            "agent.service.agent.agent_service.session_store.get_all_sessions",
            fake_get_all_sessions,
        )

        sessions = await service.get_agent_sessions("agent-1")

        assert [session.session_key for session in sessions] == [
            "agent:agent-1:ws:dm:launcher-app-agent-1",
        ]

    asyncio.run(scenario())
