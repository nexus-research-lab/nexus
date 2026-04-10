# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cron_schedule.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation cron 调度计算。"""

from __future__ import annotations

from collections.abc import Mapping
from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo


def compute_next_run_at(schedule: Mapping[str, object] | object, now_ts: int) -> int | None:
    """按 schedule 定义计算下次触发时间戳。"""
    schedule_dict = _as_schedule_dict(schedule)
    kind = str(schedule_dict["kind"])
    if kind == "every":
        return now_ts + int(schedule_dict["interval_seconds"])
    if kind == "at":
        run_at_ts = _parse_run_at(schedule_dict)
        return run_at_ts if run_at_ts >= now_ts else None
    if kind == "cron":
        return _compute_cron_next_run(schedule_dict, now_ts)
    raise ValueError(f"unsupported cron schedule kind: {kind}")


def compute_next_run_datetime(
    schedule: Mapping[str, object] | object,
    now: datetime,
) -> datetime | None:
    """返回 UTC datetime 版本的下次触发时间。"""
    normalized_now = _ensure_utc(now)
    next_run_at = compute_next_run_at(schedule, int(normalized_now.timestamp()))
    if next_run_at is None:
        return None
    return datetime.fromtimestamp(next_run_at, tz=timezone.utc)


def _compute_cron_next_run(schedule: Mapping[str, object], now_ts: int) -> int | None:
    expression = str(schedule["cron_expression"])
    timezone_name = str(schedule.get("timezone") or "Asia/Shanghai")
    timezone_info = ZoneInfo(timezone_name)
    minute_values, hour_values, day_values, month_values, weekday_values = _parse_expression(expression)

    now = datetime.fromtimestamp(now_ts, tz=timezone.utc)
    candidate = now.replace(second=0, microsecond=0) + timedelta(minutes=1)
    limit = candidate + timedelta(days=366)

    while candidate <= limit:
        localized = candidate.astimezone(timezone_info)
        if (
            localized.minute in minute_values
            and localized.hour in hour_values
            and localized.month in month_values
            and _match_day(localized, day_values, weekday_values)
        ):
            return int(candidate.timestamp())
        candidate += timedelta(minutes=1)
    return None


def _parse_expression(expression: str) -> tuple[set[int], set[int], set[int], set[int], set[int]]:
    parts = expression.split()
    if len(parts) != 5:
        raise ValueError("cron_expression must contain 5 fields")
    return (
        _parse_field(parts[0], 0, 59),
        _parse_field(parts[1], 0, 23),
        _parse_field(parts[2], 1, 31),
        _parse_field(parts[3], 1, 12),
        _parse_field(parts[4], 0, 6, normalize_weekday=True),
    )


def _parse_field(
    raw_value: str,
    min_value: int,
    max_value: int,
    *,
    normalize_weekday: bool = False,
) -> set[int]:
    values: set[int] = set()
    for chunk in raw_value.split(","):
        values.update(
            _parse_chunk(
                chunk.strip(),
                min_value,
                max_value,
                normalize_weekday=normalize_weekday,
            )
        )
    if not values:
        raise ValueError(f"invalid cron field: {raw_value}")
    return values


def _parse_chunk(
    chunk: str,
    min_value: int,
    max_value: int,
    *,
    normalize_weekday: bool,
) -> set[int]:
    if chunk == "*":
        return set(range(min_value, max_value + 1))

    base = chunk
    step = 1
    if "/" in chunk:
        base, step_text = chunk.split("/", 1)
        step = int(step_text)
        if step <= 0:
            raise ValueError("cron field step must be greater than 0")

    if base in {"", "*"}:
        start = min_value
        end = max_value
    elif "-" in base:
        start_text, end_text = base.split("-", 1)
        start = _normalize_number(int(start_text), normalize_weekday=normalize_weekday)
        end = _normalize_number(int(end_text), normalize_weekday=normalize_weekday)
    else:
        value = _normalize_number(int(base), normalize_weekday=normalize_weekday)
        _assert_range(value, min_value, max_value)
        return {value}

    _assert_range(start, min_value, max_value)
    _assert_range(end, min_value, max_value)
    if end < start:
        raise ValueError("cron field range end must be greater than or equal to start")
    return set(range(start, end + 1, step))


def _normalize_number(value: int, *, normalize_weekday: bool) -> int:
    # 中文注释：cron 的 weekday 允许 0/7 都表示 Sunday，
    # 这里统一折叠到 Python `weekday()` 对齐的 0-6 语义。
    if normalize_weekday:
        if value == 7:
            return 6
        if value == 0:
            return 6
        return value - 1
    return value


def _assert_range(value: int, min_value: int, max_value: int) -> None:
    if value < min_value or value > max_value:
        raise ValueError(f"cron field value {value} is out of range")


def _match_day(localized: datetime, day_values: set[int], weekday_values: set[int]) -> bool:
    all_days = len(day_values) == 31
    all_weekdays = len(weekday_values) == 7
    day_match = localized.day in day_values
    weekday_match = localized.weekday() in weekday_values
    if all_days and all_weekdays:
        return True
    if all_days:
        return weekday_match
    if all_weekdays:
        return day_match
    return day_match or weekday_match


def _parse_run_at(schedule: Mapping[str, object]) -> int:
    value = datetime.fromisoformat(str(schedule["run_at"]))
    if value.tzinfo is None:
        timezone_name = str(schedule.get("timezone") or "Asia/Shanghai")
        value = value.replace(tzinfo=ZoneInfo(timezone_name))
    return int(value.astimezone(timezone.utc).timestamp())


def _ensure_utc(value: datetime) -> datetime:
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def _as_schedule_dict(schedule: Mapping[str, object] | object) -> dict[str, object]:
    if isinstance(schedule, Mapping):
        return dict(schedule)
    if hasattr(schedule, "model_dump"):
        return dict(schedule.model_dump(mode="python"))
    return dict(vars(schedule))
