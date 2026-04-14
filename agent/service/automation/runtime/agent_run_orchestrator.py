# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：agent_run_orchestrator.py
# @Date   ：2026/4/10
# @Author ：Codex
# 2026/4/10   Create
# =====================================================

"""Automation 单轮执行编排器。"""

from __future__ import annotations

from datetime import datetime

from agent.schema.model_message import EventMessage, Message, StreamMessage
from agent.service.agent.agent_runtime import agent_runtime
from agent.service.automation.runtime.run_context import AutomationRunContext
from agent.service.automation.runtime.run_result import AutomationRunResult
from agent.service.channels.message_sender import MessageSender
from agent.service.channels.ws.ws_session_routing_sender import WsSessionRoutingSender
from agent.service.message.chat_message_processor import ChatMessageProcessor
from agent.service.permission.strategy.permission_auto import AutoAllowPermissionStrategy
from agent.service.session.session_manager import session_manager
from agent.service.session.session_router import parse_session_key, resolve_agent_id
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class _NoopMessageSender(MessageSender):
    """自动化链路未配置实时发送时的空 sender。"""

    async def send_message(self, message: Message) -> None:
        del message

    async def send_stream_message(self, message: StreamMessage) -> None:
        del message

    async def send_event_message(self, event: EventMessage) -> None:
        del event


class AgentRunOrchestrator:
    """串行执行 automation session 的单轮对话。"""

    def __init__(
        self,
        *,
        runtime=None,
        store=None,
        processor_cls=None,
        sender: MessageSender | None = None,
    ) -> None:
        self._runtime = runtime or agent_runtime
        self._session_store = store or session_store
        self._processor_cls = processor_cls or ChatMessageProcessor
        self._sender = sender or WsSessionRoutingSender(_NoopMessageSender())

    async def run_turn(self, ctx: AutomationRunContext) -> AutomationRunResult:
        """执行一次 automation turn，并把消息落入现有会话存储。"""
        async with session_manager.get_lock(ctx.session_key):
            return await self._run_locked(ctx)

    async def _run_locked(self, ctx: AutomationRunContext) -> AutomationRunResult:
        metadata = self._build_result_metadata(ctx)
        model_prompt, display_prompt = self._build_prompts(ctx)
        session_info = await self._session_store.get_session_info(ctx.session_key)
        if session_info is None:
            session_info = await self._create_session(ctx)
            if session_info is None:
                resolved_agent_id = resolve_agent_id(ctx.agent_id)
                return AutomationRunResult(
                    agent_id=resolved_agent_id,
                    session_key=ctx.session_key,
                    status="error",
                    error_message="failed to create automation session",
                    metadata=metadata,
                )
        resolved_agent_id = resolve_agent_id(session_info.agent_id if session_info else ctx.agent_id)
        known_session_id = session_info.session_id if session_info else None

        processor = self._processor_cls(
            session_key=ctx.session_key,
            query=model_prompt,
            display_query=display_prompt,
            agent_id=resolved_agent_id,
            session_id=known_session_id,
        )

        try:
            if ctx.trigger_kind == "cron":
                user_message = Message(
                    message_id=processor.round_id,
                    session_key=ctx.session_key,
                    agent_id=resolved_agent_id,
                    round_id=processor.round_id,
                    session_id=known_session_id,
                    role="user",
                    content=display_prompt,
                )
                await self._session_store.save_message(user_message)
                await self._sender.send(user_message)
                processor._is_user_message_saved = True

            client = await self._runtime.get_or_create_client(
                session_key=ctx.session_key,
                agent_id=resolved_agent_id,
                permission_strategy=AutoAllowPermissionStrategy(),
                resume_session_id=known_session_id,
                resolved_agent_id=resolved_agent_id,
            )
            await client.query(model_prompt)

            async for sdk_message in client.receive_messages():
                processed_messages = await processor.process_messages(sdk_message)
                for message in processed_messages:
                    await self._sender.send(message)
                if processor.subtype in {"success", "error"}:
                    break
        except Exception as exc:
            logger.warning("⚠️ automation run 失败: session=%s error=%s", ctx.session_key, exc)
            return AutomationRunResult(
                agent_id=resolved_agent_id,
                session_key=ctx.session_key,
                status="error",
                round_id=processor.round_id,
                session_id=processor.session_id,
                message_count=processor.message_count,
                error_message=str(exc),
                metadata=metadata,
            )

        if processor.subtype not in {"success", "error"}:
            return AutomationRunResult(
                agent_id=resolved_agent_id,
                session_key=ctx.session_key,
                status="error",
                round_id=processor.round_id,
                session_id=processor.session_id,
                message_count=processor.message_count,
                error_message="agent run ended without terminal result",
                metadata=metadata,
            )

        return AutomationRunResult(
            agent_id=resolved_agent_id,
            session_key=ctx.session_key,
            status=processor.subtype,
            round_id=processor.round_id,
            session_id=processor.session_id,
            message_count=processor.message_count,
            metadata=metadata,
        )

    async def _create_session(self, ctx: AutomationRunContext):
        parsed = parse_session_key(ctx.session_key)
        channel_type = parsed.get("channel") or "automation"
        chat_type = parsed.get("chat_type") or "dm"
        return await self._session_store.create_session_by_key(
            session_key=ctx.session_key,
            channel_type=channel_type,
            chat_type=chat_type,
            title="Automation Run",
        )

    @staticmethod
    def _build_result_metadata(ctx: AutomationRunContext) -> dict[str, object]:
        """把触发上下文压平到返回结果，便于后续调度层复用。"""
        metadata = dict(ctx.metadata)
        metadata.setdefault("trigger_kind", ctx.trigger_kind)
        metadata.setdefault("delivery_mode", ctx.delivery_mode)
        return metadata

    @staticmethod
    def _build_prompts(ctx: AutomationRunContext) -> tuple[str, str]:
        if ctx.trigger_kind != "cron":
            return ctx.instruction, ctx.instruction

        job_name = str(ctx.metadata.get("job_name") or "").strip() or "task"
        time_label = AgentRunOrchestrator._format_trigger_time(ctx.metadata.get("scheduled_for"))
        display_prompt = f"[cron:{job_name}|{time_label}]\ntask: {ctx.instruction}"
        model_prompt = "\n".join([
            "[AUTO TASK]",
            f"name:{job_name}",
            f"time:{time_label}",
            "rule:execute TASK only; do not mention auto metadata.",
            "TASK:",
            ctx.instruction,
        ])
        return model_prompt, display_prompt

    @staticmethod
    def _format_trigger_time(raw_value: object) -> str:
        if isinstance(raw_value, str) and raw_value.strip():
            try:
                parsed = datetime.fromisoformat(raw_value.strip())
                return parsed.strftime("%H:%M")
            except ValueError:
                pass
        return "auto"
