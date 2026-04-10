from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient


class FakePayload:
    """提供与 AModel 兼容的最小导出接口。"""

    def __init__(self, **fields) -> None:
        self._fields = fields

    def model_dump(self, mode: str = "python") -> dict[str, object]:
        del mode
        return dict(self._fields)


class FakeHeartbeatService:
    """记录 API 调用并返回固定数据。"""

    def __init__(self) -> None:
        self.calls: list[tuple[str, tuple, dict]] = []

    async def get_status(self, agent_id: str):
        self.calls.append(("get_status", (agent_id,), {}))
        return FakePayload(
            agent_id=agent_id,
            enabled=True,
            every_seconds=60,
            target_mode="last",
            ack_max_chars=120,
            running=True,
            pending_wake=False,
            next_run_at="2026-04-10T08:30:00Z",
            last_heartbeat_at=None,
            last_ack_at=None,
        )

    async def wake(self, *, agent_id: str, mode: str, text: str | None = None):
        self.calls.append(("wake", (), {"agent_id": agent_id, "mode": mode, "text": text}))
        return FakePayload(
            agent_id=agent_id,
            mode=mode,
            scheduled=(mode == "now"),
        )


def _build_api_app(monkeypatch, service: FakeHeartbeatService) -> FastAPI:
    monkeypatch.setenv("ENV_FILE", "/dev/null")
    monkeypatch.setenv("DEBUG", "false")
    import agent.api.automation.api_heartbeat as heartbeat_module

    monkeypatch.setattr(heartbeat_module, "heartbeat_service", service)

    app = FastAPI()
    app.include_router(heartbeat_module.router, prefix="/agent/v1")
    return app


def test_get_heartbeat_status_route_returns_repo_response_shape(monkeypatch):
    service = FakeHeartbeatService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.get("/agent/v1/automation/heartbeat/nexus")

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {
            "agent_id": "nexus",
            "enabled": True,
            "every_seconds": 60,
            "target_mode": "last",
            "ack_max_chars": 120,
            "running": True,
            "pending_wake": False,
            "next_run_at": "2026-04-10T08:30:00Z",
            "last_heartbeat_at": None,
            "last_ack_at": None,
        },
    }
    assert service.calls == [("get_status", ("nexus",), {})]


def test_post_heartbeat_wake_route_delegates_to_service(monkeypatch):
    service = FakeHeartbeatService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.post(
            "/agent/v1/automation/heartbeat/nexus/wake",
            json={
                "mode": "now",
                "text": "check disk pressure",
            },
        )

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {
            "agent_id": "nexus",
            "mode": "now",
            "scheduled": True,
        },
    }
    assert service.calls == [
        (
            "wake",
            (),
            {
                "agent_id": "nexus",
                "mode": "now",
                "text": "check disk pressure",
            },
        )
    ]
