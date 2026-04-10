from __future__ import annotations

from datetime import datetime, timezone


def _ts(value: str) -> int:
    return int(datetime.fromisoformat(value).timestamp())


def test_compute_next_run_at_for_every_schedule():
    from agent.service.automation.cron.cron_schedule import compute_next_run_at

    next_run_at = compute_next_run_at(
        {
            "kind": "every",
            "interval_seconds": 300,
            "timezone": "Asia/Shanghai",
        },
        _ts("2026-04-10T01:20:00+00:00"),
    )

    assert next_run_at == _ts("2026-04-10T01:25:00+00:00")


def test_compute_next_run_at_for_at_schedule_returns_none_after_due_time():
    from agent.service.automation.cron.cron_schedule import compute_next_run_at

    next_run_at = compute_next_run_at(
        {
            "kind": "at",
            "run_at": "2026-04-10T09:00:00+08:00",
            "timezone": "Asia/Shanghai",
        },
        _ts("2026-04-10T01:01:00+00:00"),
    )

    assert next_run_at is None


def test_compute_next_run_at_for_cron_schedule_respects_timezone():
    from agent.service.automation.cron.cron_schedule import compute_next_run_at

    next_run_at = compute_next_run_at(
        {
            "kind": "cron",
            "cron_expression": "30 9 * * *",
            "timezone": "Asia/Shanghai",
        },
        _ts("2026-04-10T01:20:00+00:00"),
    )

    assert next_run_at == _ts("2026-04-10T01:30:00+00:00")


def test_compute_next_run_at_for_cron_schedule_supports_step_minutes():
    from agent.service.automation.cron.cron_schedule import compute_next_run_at

    next_run_at = compute_next_run_at(
        {
            "kind": "cron",
            "cron_expression": "*/15 * * * *",
            "timezone": "UTC",
        },
        _ts("2026-04-10T01:16:00+00:00"),
    )

    assert next_run_at == _ts("2026-04-10T01:30:00+00:00")


def test_compute_next_run_at_for_cron_schedule_rolls_to_next_day():
    from agent.service.automation.cron.cron_schedule import compute_next_run_at

    next_run_at = compute_next_run_at(
        {
            "kind": "cron",
            "cron_expression": "30 9 * * *",
            "timezone": "Asia/Shanghai",
        },
        _ts("2026-04-10T01:31:00+00:00"),
    )

    expected = datetime(2026, 4, 11, 1, 30, tzinfo=timezone.utc)
    assert next_run_at == int(expected.timestamp())
