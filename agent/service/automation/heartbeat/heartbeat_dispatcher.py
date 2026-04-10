# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：heartbeat_dispatcher.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation heartbeat 执行分发器。"""

from __future__ import annotations

from dataclasses import dataclass

from agent.schema.model_automation import AutomationHeartbeatConfig
from agent.service.automation.delivery.delivery_memory import DeliveryMemory
from agent.service.automation.delivery.delivery_router import DeliveryRouter
from agent.service.automation.heartbeat.heartbeat_prompt import (
    filter_heartbeat_response,
    parse_heartbeat_tasks,
)
from agent.service.automation.runtime.agent_run_orchestrator import AgentRunOrchestrator
from agent.service.automation.runtime.run_context import AutomationRunContext
from agent.service.automation.runtime.system_event_queue import SystemEventQueue
from agent.service.automation.runtime.wake_service import WakeService
from agent.service.session.session_router import build_automation_main_session_key
from agent.service.session.session_store import session_store
from agent.service.workspace.workspace_service import workspace_service


@dataclass(slots=True)
class HeartbeatDispatchResult:
    """单次 heartbeat 分发结果。"""

    dispatched: bool
    delivered: bool
    acknowledged: bool
    text: str = ""


class HeartbeatDispatcher:
    """构造主会话 heartbeat 指令并执行。"""

    def __init__(
        self,
        *,
        orchestrator: AgentRunOrchestrator | None = None,
        delivery_router: DeliveryRouter | None = None,
        system_event_queue: SystemEventQueue | None = None,
        wake_service: WakeService | None = None,
        workspace_reader=None,
        message_store=None,
    ) -> None:
        self._orchestrator = orchestrator or AgentRunOrchestrator()
        self._delivery_router = delivery_router or DeliveryRouter(memory=DeliveryMemory())
        self._system_event_queue = system_event_queue or SystemEventQueue()
        self._wake_service = wake_service or WakeService()
        self._workspace_reader = workspace_reader or workspace_service
        self._message_store = message_store or session_store

    def set_channel_register(self, channel_register) -> None:
        """注入 app 生命周期维护的通道注册表。"""
        self._delivery_router = DeliveryRouter(
            memory=DeliveryMemory(),
            channel_register=channel_register,
        )

    async def dispatch(
        self,
        *,
        agent_id: str,
        config: AutomationHeartbeatConfig,
    ) -> HeartbeatDispatchResult:
        """执行一次主会话 heartbeat。"""
        session_key = build_automation_main_session_key(agent_id)
        events = await self._claim_events(agent_id)
        immediate_wake_requests = self._wake_service.drain_now(agent_id)
        deferred_wake_requests = self._wake_service.list_next_heartbeat(agent_id)
        instruction = await self._build_instruction(
            agent_id,
            events,
            immediate_wake_requests,
            deferred_wake_requests,
        )

        if not instruction.strip():
            self._wake_service.clear(session_key)
            await self._finish_events(events, succeeded=True)
            return HeartbeatDispatchResult(dispatched=False, delivered=False, acknowledged=False)

        result = await self._orchestrator.run_turn(
            AutomationRunContext(
                agent_id=agent_id,
                session_key=session_key,
                instruction=instruction,
                trigger_kind="heartbeat",
                delivery_mode=config.target_mode,
                metadata={"event_ids": [item.event_id for item in events]},
            )
        )
        if not result.ok:
            self._wake_service.clear(session_key)
            await self._finish_events(events, succeeded=False)
            return HeartbeatDispatchResult(dispatched=False, delivered=False, acknowledged=False)

        response_text = await self._extract_result_text(session_key, result.round_id)
        filtered = filter_heartbeat_response(response_text, ack_max_chars=config.ack_max_chars)
        delivered = False
        if filtered.should_deliver and filtered.text.strip() and config.target_mode != "none":
            await self._delivery_router.send_text(
                agent_id=agent_id,
                text=filtered.text,
                target={"mode": config.target_mode},
            )
            delivered = True

        self._wake_service.clear(session_key)
        await self._finish_events(events, succeeded=True)
        return HeartbeatDispatchResult(
            dispatched=True,
            delivered=delivered,
            acknowledged=not filtered.should_deliver,
            text=filtered.text,
        )

    async def _build_instruction(
        self,
        agent_id: str,
        events: list[object],
        immediate_wake_requests: list[object],
        deferred_wake_requests: list[object],
    ) -> str:
        sections: list[str] = []
        heartbeat_text = await self._read_heartbeat_text(agent_id)
        if heartbeat_text:
            tasks = parse_heartbeat_tasks(heartbeat_text)
            if tasks:
                # 中文注释：tasks 模式只抽取 name/prompt，避免把 YAML 样式头部原样喂给主会话。
                task_lines = [
                    item.prompt or item.name or item.interval
                    for item in tasks
                    if (item.prompt or item.name or item.interval)
                ]
                if task_lines:
                    sections.append("Heartbeat tasks:\n" + "\n".join(f"- {item}" for item in task_lines))
            else:
                sections.append(heartbeat_text.strip())

        event_lines = []
        for item in events:
            payload = getattr(item, "payload", {}) or {}
            text = str(payload.get("text") or "").strip()
            event_lines.append(text or getattr(item, "event_type", "heartbeat.event"))
        if event_lines:
            sections.append("System events:\n" + "\n".join(f"- {item}" for item in event_lines))

        wake_lines = []
        for item in [*immediate_wake_requests, *deferred_wake_requests]:
            text = str(getattr(item, "metadata", {}).get("text") or "").strip()
            if text:
                wake_lines.append(text)
            else:
                wake_lines.append(f"wake request ({getattr(item, 'wake_mode', 'unknown')})")
        if wake_lines:
            sections.append("Wake requests:\n" + "\n".join(f"- {item}" for item in wake_lines))
        return "\n\n".join(part for part in sections if part.strip())

    async def _read_heartbeat_text(self, agent_id: str) -> str:
        try:
            return await self._workspace_reader.get_workspace_file(agent_id, "HEARTBEAT.md")
        except FileNotFoundError:
            return ""

    async def _claim_events(self, agent_id: str) -> list[object]:
        pending = await self._system_event_queue.list_pending_events()
        matched = [item for item in pending if ((getattr(item, "payload", {}) or {}).get("agent_id") == agent_id)]
        claimed = []
        for item in matched:
            row = await self._system_event_queue.mark_processing(item.event_id)
            claimed.append(row or item)
        return claimed

    async def _finish_events(self, events: list[object], *, succeeded: bool) -> None:
        for item in events:
            if succeeded:
                await self._system_event_queue.mark_processed(item.event_id)
            else:
                await self._system_event_queue.mark_failed(item.event_id)

    async def _extract_result_text(self, session_key: str, round_id: str | None) -> str:
        messages = await self._message_store.get_session_messages(session_key)
        for message in reversed(messages):
            if round_id and message.round_id != round_id:
                continue
            if message.role == "result" and message.result:
                return message.result
            if message.role != "assistant" or not isinstance(message.content, list):
                continue
            text_parts = [getattr(block, "text", "") for block in message.content if getattr(block, "type", "") == "text"]
            if text_parts:
                return "\n".join(part for part in text_parts if part)
        return ""
