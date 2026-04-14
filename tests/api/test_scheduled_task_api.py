from __future__ import annotations

import sys
from pathlib import Path

from fastapi import FastAPI
from fastapi.testclient import TestClient


def _ensure_repo_root_on_path() -> None:
    """确保直接运行 pytest 时也能导入 repo 内包。"""
    repo_root = Path(__file__).resolve().parents[2]
    repo_root_str = str(repo_root)
    if repo_root_str not in sys.path:
        sys.path.insert(0, repo_root_str)


class FakePayload:
    """提供与 AModel 兼容的最小导出接口。"""

    def __init__(self, **fields) -> None:
        self._fields = fields

    def model_dump(self, mode: str = "python") -> dict[str, object]:
        del mode
        return dict(self._fields)


class FakeScheduledTaskService:
    """记录 scheduled task API 调用。"""

    def __init__(self) -> None:
        self.calls: list[tuple[str, tuple, dict]] = []

    async def list_tasks(self, *, agent_id: str | None = None):
        self.calls.append(("list_tasks", (), {"agent_id": agent_id}))
        return [
            FakePayload(
                job_id="job-1",
                agent_id=agent_id or "nexus",
                name="Daily sync",
                enabled=True,
            )
        ]

    async def create_task(self, payload):
        data = payload.model_dump(mode="json")
        self.calls.append(("create_task", (), {"payload": data}))
        return FakePayload(job_id="job-1", **data)

    async def update_task(
        self,
        job_id: str,
        *,
        name=None,
        schedule=None,
        instruction=None,
        session_target=None,
        delivery=None,
        enabled=None,
    ):
        self.calls.append(
            (
                "update_task",
                (job_id,),
                {
                    "name": name,
                    "schedule": None if schedule is None else schedule.model_dump(mode="json"),
                    "instruction": instruction,
                    "session_target": None
                    if session_target is None
                    else session_target.model_dump(mode="json"),
                    "delivery": None if delivery is None else delivery.model_dump(mode="json"),
                    "enabled": enabled,
                },
            )
        )
        return FakePayload(job_id=job_id, name=name or "updated", enabled=enabled)

    async def delete_task(self, job_id: str):
        self.calls.append(("delete_task", (job_id,), {}))

    async def run_task_now(self, job_id: str):
        self.calls.append(("run_task_now", (job_id,), {}))
        return FakePayload(job_id=job_id, run_id="run-1", status="queued")

    async def update_task_status(self, job_id: str, *, enabled: bool):
        self.calls.append(("update_task_status", (job_id,), {"enabled": enabled}))
        return FakePayload(job_id=job_id, enabled=enabled)

    async def list_task_runs(self, job_id: str):
        self.calls.append(("list_task_runs", (job_id,), {}))
        return [FakePayload(run_id="run-1", job_id=job_id, status="success")]


def _build_api_app(monkeypatch, service: FakeScheduledTaskService) -> FastAPI:
    monkeypatch.setenv("ENV_FILE", "/dev/null")
    monkeypatch.setenv("DEBUG", "false")
    _ensure_repo_root_on_path()
    import agent.api.capability.api_scheduled_task as scheduled_task_module

    monkeypatch.setattr(scheduled_task_module, "scheduled_task_service", service)

    app = FastAPI()
    app.include_router(scheduled_task_module.router, prefix="/agent/v1")
    return app


def test_list_scheduled_tasks_route_returns_repo_response_shape(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.get("/agent/v1/capability/scheduled/tasks", params={"agent_id": "nexus"})

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": [
            {
                "job_id": "job-1",
                "agent_id": "nexus",
                "name": "Daily sync",
                "enabled": True,
            }
        ],
    }
    assert service.calls == [("list_tasks", (), {"agent_id": "nexus"})]


def test_create_scheduled_task_route_delegates_payload(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)
    payload = {
        "name": "Daily sync",
        "agent_id": "nexus",
        "schedule": {"kind": "every", "interval_seconds": 300, "timezone": "Asia/Shanghai"},
        "instruction": "summarize status",
        "session_target": {"kind": "main", "wake_mode": "next-heartbeat"},
        "delivery": {"mode": "none"},
        "source": {
            "context_type": "agent",
            "context_id": "nexus",
            "context_label": "Nexus",
            "session_key": "agent:nexus:ws:dm:launcher-app-nexus",
            "session_label": "主会话",
        },
        "enabled": True,
    }
    expected_payload = {
        "name": "Daily sync",
        "agent_id": "nexus",
        "schedule": {
            "kind": "every",
            "run_at": None,
            "interval_seconds": 300,
            "cron_expression": None,
            "timezone": "Asia/Shanghai",
        },
        "instruction": "summarize status",
        "session_target": {
            "kind": "main",
            "bound_session_key": None,
            "named_session_key": None,
            "wake_mode": "next-heartbeat",
        },
        "delivery": {
            "mode": "none",
            "channel": None,
            "to": None,
            "account_id": None,
            "thread_id": None,
        },
        "source": {
            "kind": "user_page",
            "creator_agent_id": None,
            "context_type": "agent",
            "context_id": "nexus",
            "context_label": "Nexus",
            "session_key": "agent:nexus:ws:dm:launcher-app-nexus",
            "session_label": "主会话",
        },
        "enabled": True,
    }

    with TestClient(app) as client:
        response = client.post("/agent/v1/capability/scheduled/tasks", json=payload)

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {"job_id": "job-1", **expected_payload},
    }
    assert service.calls == [("create_task", (), {"payload": expected_payload})]


def test_patch_scheduled_task_route_updates_job(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.patch(
            "/agent/v1/capability/scheduled/tasks/job-1",
            json={
                "name": "Nightly sync",
                "schedule": {"kind": "cron", "cron_expression": "0 * * * *", "timezone": "Asia/Shanghai"},
                "instruction": "refresh cache",
                "session_target": {"kind": "named", "named_session_key": "ops", "wake_mode": "now"},
                "delivery": {"mode": "last"},
                "enabled": False,
            },
        )

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {
            "job_id": "job-1",
            "name": "Nightly sync",
            "enabled": False,
        },
    }
    assert service.calls == [
        (
            "update_task",
            ("job-1",),
            {
                "name": "Nightly sync",
                "schedule": {
                    "kind": "cron",
                    "run_at": None,
                    "interval_seconds": None,
                    "cron_expression": "0 * * * *",
                    "timezone": "Asia/Shanghai",
                },
                "instruction": "refresh cache",
                "session_target": {
                    "kind": "named",
                    "bound_session_key": None,
                    "named_session_key": "ops",
                    "wake_mode": "now",
                },
                "delivery": {
                    "mode": "last",
                    "channel": None,
                    "to": None,
                    "account_id": None,
                    "thread_id": None,
                },
                "enabled": False,
            },
        )
    ]


def test_delete_scheduled_task_route_returns_job_id(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.delete("/agent/v1/capability/scheduled/tasks/job-1")

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {"job_id": "job-1"},
    }
    assert service.calls == [("delete_task", ("job-1",), {})]


def test_run_scheduled_task_route_returns_service_payload(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.post("/agent/v1/capability/scheduled/tasks/job-1/run")

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {
            "job_id": "job-1",
            "run_id": "run-1",
            "status": "queued",
        },
    }
    assert service.calls == [("run_task_now", ("job-1",), {})]


def test_patch_scheduled_task_status_route_updates_enabled_state(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.patch(
            "/agent/v1/capability/scheduled/tasks/job-1/status",
            json={"enabled": False},
        )

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": {
            "job_id": "job-1",
            "enabled": False,
        },
    }
    assert service.calls == [("update_task_status", ("job-1",), {"enabled": False})]


def test_list_scheduled_task_runs_route_returns_repo_response_shape(monkeypatch):
    service = FakeScheduledTaskService()
    app = _build_api_app(monkeypatch, service)

    with TestClient(app) as client:
        response = client.get("/agent/v1/capability/scheduled/tasks/job-1/runs")

    assert response.status_code == 200
    assert response.json() == {
        "code": "0000",
        "message": "success",
        "success": True,
        "data": [
            {
                "run_id": "run-1",
                "job_id": "job-1",
                "status": "success",
            }
        ],
    }
    assert service.calls == [("list_task_runs", ("job-1",), {})]
