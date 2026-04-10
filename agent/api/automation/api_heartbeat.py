# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：api_heartbeat.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Heartbeat automation API。"""

from fastapi import APIRouter

from agent.infra.schemas.model_cython import AModel
from agent.infra.server.common import resp
from agent.schema.model_automation import AutomationSessionWakeMode
from agent.service.automation.heartbeat.heartbeat_service import heartbeat_service

router = APIRouter(prefix="/automation/heartbeat", tags=["automation"])


class HeartbeatWakeRequest(AModel):
    """手动 wake 请求。"""

    mode: AutomationSessionWakeMode = "now"
    text: str | None = None


@router.get("/{agent_id}")
async def get_heartbeat(agent_id: str):
    """读取指定 agent 的 heartbeat 状态。"""
    data = await heartbeat_service.get_status(agent_id)
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))


@router.post("/{agent_id}/wake")
async def wake_heartbeat(agent_id: str, payload: HeartbeatWakeRequest):
    """手动触发一次 heartbeat wake。"""
    data = await heartbeat_service.wake(
        agent_id=agent_id,
        mode=payload.mode,
        text=payload.text,
    )
    return resp.ok(resp.Resp(data=data.model_dump(mode="json")))
