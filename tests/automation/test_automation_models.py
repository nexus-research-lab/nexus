from __future__ import annotations

import pytest
from pydantic import ValidationError

from agent.infra.database.models.automation_cron_job import AutomationCronJob
from agent.infra.database.models.automation_delivery_route import (
    AutomationDeliveryRoute,
)
from agent.infra.database.models.automation_heartbeat_state import (
    AutomationHeartbeatState,
)
from agent.schema.model_automation import (
    AutomationCronJobCreate,
    AutomationCronSchedule,
    AutomationDeliveryTarget,
    AutomationHeartbeatConfig,
    AutomationSessionTarget,
)


def test_cron_job_create_defaults_delivery_and_status():
    job = AutomationCronJobCreate(
        name="daily brief",
        agent_id="nexus",
        schedule=AutomationCronSchedule(kind="every", interval_seconds=3600),
        instruction="summarize overnight updates",
    )

    assert job.delivery.mode == "none"
    assert job.session_target.kind == "isolated"
    assert job.enabled is True


def test_heartbeat_config_defaults_to_silent_delivery():
    config = AutomationHeartbeatConfig(agent_id="nexus")

    assert config.enabled is False
    assert config.target_mode == "none"
    assert config.ack_max_chars == 300


def test_automation_schedule_and_delivery_kinds_match_expected_shapes():
    every = AutomationCronSchedule(kind="every", interval_seconds=3600)
    delivery = AutomationDeliveryTarget(mode="last")
    session_target = AutomationSessionTarget(kind="main", wake_mode="now")
    heartbeat = AutomationHeartbeatConfig(agent_id="nexus", target_mode="explicit")

    assert every.kind == "every"
    assert delivery.mode == "last"
    assert session_target.kind == "main"
    assert session_target.wake_mode == "now"
    assert heartbeat.target_mode == "explicit"


def test_invalid_schedule_kind_rejected():
    with pytest.raises(ValidationError):
        AutomationCronSchedule(kind="hourly", interval_seconds=3600)


@pytest.mark.parametrize(
    "payload",
    [
        dict(kind="every"),
        dict(kind="every", interval_seconds=0),
        dict(
            kind="every",
            interval_seconds=60,
            run_at="2026-01-01T00:00:00Z",
            cron_expression="* * * * *",
        ),
        dict(kind="at"),
        dict(kind="at", run_at="2026-01-01T00:00:00Z", interval_seconds=60),
        dict(kind="cron"),
        dict(kind="cron", cron_expression="* * * * *", run_at="2026-01-01T00:00:00Z"),
        dict(kind="cron", cron_expression="* * * * *", timezone=None),
    ],
)
def test_invalid_cron_schedule_shapes_are_rejected(payload: dict[str, object]):
    with pytest.raises(ValidationError):
        AutomationCronSchedule(**payload)


@pytest.mark.parametrize(
    "payload",
    [
        dict(agent_id="nexus", every_seconds=0),
        dict(agent_id="nexus", ack_max_chars=-1),
    ],
)
def test_invalid_heartbeat_config_values_are_rejected(payload: dict[str, object]):
    with pytest.raises(ValidationError):
        AutomationHeartbeatConfig(**payload)


def test_agent_scoped_automation_tables_cascade_to_agents():
    cron_job_foreign_keys = list(AutomationCronJob.__table__.c.agent_id.foreign_keys)
    heartbeat_foreign_keys = list(AutomationHeartbeatState.__table__.c.agent_id.foreign_keys)
    delivery_route_foreign_keys = list(AutomationDeliveryRoute.__table__.c.agent_id.foreign_keys)

    assert cron_job_foreign_keys[0].target_fullname == "agents.id"
    assert cron_job_foreign_keys[0].ondelete == "CASCADE"
    assert heartbeat_foreign_keys[0].target_fullname == "agents.id"
    assert heartbeat_foreign_keys[0].ondelete == "CASCADE"
    assert delivery_route_foreign_keys[0].target_fullname == "agents.id"
    assert delivery_route_foreign_keys[0].ondelete == "CASCADE"
