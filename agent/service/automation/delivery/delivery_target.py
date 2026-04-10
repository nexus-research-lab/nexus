# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：delivery_target.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation delivery target 解析。"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Mapping

VALID_DELIVERY_MODES = {"none", "last", "explicit"}


@dataclass(slots=True)
class DeliveryTarget:
    """通道无关的投递目标。"""

    mode: str
    channel: str | None = None
    to: str | None = None
    account_id: str | None = None
    thread_id: str | None = None
    session_key: str | None = None


def resolve_delivery_target(raw_target: DeliveryTarget | Mapping[str, object] | None) -> DeliveryTarget:
    """把外部配置归一化为可路由的投递目标。"""
    if isinstance(raw_target, DeliveryTarget):
        payload = {
            "mode": raw_target.mode,
            "channel": raw_target.channel,
            "to": raw_target.to,
            "account_id": raw_target.account_id,
            "thread_id": raw_target.thread_id,
            "session_key": raw_target.session_key,
        }
    else:
        payload = dict(raw_target or {})

    mode = _normalize_optional_str(payload.get("mode")) or "none"
    if mode not in VALID_DELIVERY_MODES:
        raise ValueError(f"unsupported delivery mode: {mode}")

    if mode == "none":
        return DeliveryTarget(mode="none")
    if mode == "last":
        return DeliveryTarget(mode="last")

    channel = _normalize_optional_str(payload.get("channel"))
    if not channel:
        raise ValueError("explicit delivery target requires channel")

    to = _normalize_optional_str(payload.get("to"))
    session_key = _normalize_optional_str(payload.get("session_key"))
    if channel == "websocket" and session_key and not to:
        to = session_key
    if not to:
        raise ValueError("explicit delivery target requires to")

    if channel == "websocket" and not session_key:
        session_key = to

    return DeliveryTarget(
        mode="explicit",
        channel=channel,
        to=to,
        account_id=_normalize_optional_str(payload.get("account_id")),
        thread_id=_normalize_optional_str(payload.get("thread_id")),
        session_key=session_key,
    )


def _normalize_optional_str(value: object) -> str | None:
    """把空字符串和 None 统一归一化。"""
    if value is None:
        return None
    normalized = str(value).strip()
    return normalized or None
