# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_automation.py
# @Date   ：2026/4/9
# @Author ：Codex
# 2026/4/9   Create
# =====================================================

"""Automation 域数据模型。"""

from __future__ import annotations

from typing import Literal

from pydantic import Field
from pydantic import model_validator

from agent.infra.schemas.model_cython import AModel

# 中文注释：这些 Literal 把调度、会话和投递模式限定在受控枚举内，避免上层直接塞任意字符串。
AutomationDeliveryMode = Literal["none", "last", "explicit"]
AutomationSessionTargetKind = Literal["isolated", "main", "bound", "named"]
AutomationSessionWakeMode = Literal["now", "next-heartbeat"]
AutomationCronScheduleKind = Literal["every", "cron", "at"]
AutomationHeartbeatTargetMode = Literal["none", "last", "explicit"]


class AutomationDeliveryTarget(AModel):
    mode: AutomationDeliveryMode = "none"
    channel: str | None = None
    to: str | None = None
    account_id: str | None = None
    thread_id: str | None = None


class AutomationSessionTarget(AModel):
    kind: AutomationSessionTargetKind = "isolated"
    bound_session_key: str | None = None
    named_session_key: str | None = None
    wake_mode: AutomationSessionWakeMode = "next-heartbeat"


class AutomationCronSchedule(AModel):
    kind: AutomationCronScheduleKind
    run_at: str | None = None
    interval_seconds: int | None = None
    cron_expression: str | None = None
    timezone: str = "Asia/Shanghai"

    @model_validator(mode="after")
    def validate_shape(self) -> "AutomationCronSchedule":
        """按调度类型校验必填字段，避免无效组合流入下游。"""
        if self.kind == "every":
            if self.interval_seconds is None or self.interval_seconds <= 0:
                raise ValueError("interval_seconds must be greater than 0 when kind is every")
            if self.run_at is not None or self.cron_expression is not None:
                raise ValueError("run_at and cron_expression must be empty when kind is every")
        elif self.kind == "at":
            if not self.run_at:
                raise ValueError("run_at is required when kind is at")
            if self.interval_seconds is not None or self.cron_expression is not None:
                raise ValueError("interval_seconds and cron_expression must be empty when kind is at")
        elif self.kind == "cron":
            if not self.cron_expression:
                raise ValueError("cron_expression is required when kind is cron")
            if self.run_at is not None or self.interval_seconds is not None:
                raise ValueError("run_at and interval_seconds must be empty when kind is cron")
        return self


class AutomationCronJobCreate(AModel):
    name: str
    agent_id: str
    schedule: AutomationCronSchedule
    instruction: str
    session_target: AutomationSessionTarget = Field(default_factory=AutomationSessionTarget)
    delivery: AutomationDeliveryTarget = Field(default_factory=AutomationDeliveryTarget)
    enabled: bool = True


class AutomationHeartbeatConfig(AModel):
    agent_id: str
    enabled: bool = False
    every_seconds: int = 1800
    target_mode: AutomationHeartbeatTargetMode = "none"
    ack_max_chars: int = 300

    @model_validator(mode="after")
    def validate_values(self) -> "AutomationHeartbeatConfig":
        """心跳参数必须保持可执行区间。"""
        if self.every_seconds <= 0:
            raise ValueError("every_seconds must be greater than 0")
        if self.ack_max_chars < 0:
            raise ValueError("ack_max_chars must be greater than or equal to 0")
        return self
