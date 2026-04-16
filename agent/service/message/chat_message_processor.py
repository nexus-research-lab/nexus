# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：chat_message_processor.py
# @Date   ：2026/03/17 20:08
# @Author ：leemysw
# 2026/03/17 20:08   Create
# =====================================================

"""单轮聊天消息处理器。"""

from __future__ import annotations

import uuid
from typing import Awaitable, Callable, Optional

from claude_agent_sdk import Message as SDKMessage
from claude_agent_sdk import ResultMessage, SystemMessage
from claude_agent_sdk.types import AssistantMessage, StreamEvent, UserMessage

from agent.schema.model_message import Message, StreamMessage
from agent.service.message.assistant_segment import AssistantSegment
from agent.service.permission.permission_error_codes import infer_permission_error_code
from agent.service.message.sdk_message_logger import message_log
from agent.service.message.sdk_message_mapper import SdkMessageMapper
from agent.service.session.session_manager import session_manager
from agent.service.session.session_router import resolve_agent_id
from agent.service.session.session_store import session_store
from agent.utils.logger import logger

PersistMessageCallback = Callable[[Message], Awaitable[None]]
RegisterSessionCallback = Callable[[str, str], Awaitable[None]]


class ChatMessageProcessor:
    """负责将 SDK 消息转换为前后端统一消息协议。"""

    def __init__(
        self,
        session_key: str,
        query: str,
        display_query: Optional[str] = None,
        round_id: Optional[str] = None,
        agent_id: str = None,
        session_id: Optional[str] = None,
        persist_message: Optional[PersistMessageCallback] = None,
        register_session: Optional[RegisterSessionCallback] = None,
    ) -> None:
        self.query = query
        self.display_query = display_query or query
        self.session_key = session_key
        self.agent_id = resolve_agent_id(agent_id)
        self.round_id = round_id or str(uuid.uuid4())
        self.session_id = session_id
        # 中文注释：subtype 只记录本轮最终 result 的终态，
        # 不能再被 api_retry 这类过程性 system subtype 污染。
        self.subtype: Optional[str] = None
        self.message_count = 0
        self._is_user_message_saved = False
        self._last_assistant_message_id: Optional[str] = None
        self._segment = AssistantSegment()
        self._persist_message = persist_message or self._default_persist_message
        self._register_session = register_session or self._default_register_session

    async def process_messages(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """处理单条 SDK 消息。"""
        if not isinstance(response_msg, StreamEvent):
            logger.debug(
                "📨 SDK message: session=%s type=%s payload=%s",
                self.session_key,
                type(response_msg).__name__,
                SdkMessageMapper.to_plain_dict(response_msg),
            )
        message_log.print_message(response_msg)
        await self._register_session_id(response_msg)
        if not isinstance(response_msg, SystemMessage):
            await self._save_user_message()

        messages = self._dispatch(response_msg)
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        if isinstance(response_msg, ResultMessage):
            self.subtype = self._normalize_result_subtype(payload.get("subtype"))

        for message in messages:
            if isinstance(message, Message):
                await self._persist_message(message)
            self.message_count += 1
        return messages

    async def _register_session_id(self, response_msg: SDKMessage) -> None:
        """在 system message 中建立 session 映射。"""
        if self.session_id or not isinstance(response_msg, SystemMessage):
            return
        session_id = (SdkMessageMapper.to_plain_dict(response_msg).get("data") or {}).get("session_id")
        if not session_id:
            return
        self.session_id = session_id
        await self._register_session(self.session_key, session_id)
        logger.debug("🔗建立映射: key=%s ↔ sdk_session=%s", self.session_key, session_id)

    async def _save_user_message(self) -> None:
        """只在当前轮次首次收到非 system 消息时写入用户消息。"""
        if self._is_user_message_saved:
            return
        user_message = Message(
            message_id=self.round_id,
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            role="user",
            content=self.display_query,
        )
        await self._persist_message(user_message)
        self._is_user_message_saved = True

    @staticmethod
    async def _default_persist_message(message: Message) -> None:
        """默认消息落库逻辑。"""
        await session_store.save_message(message)

    @staticmethod
    async def _default_register_session(session_key: str, session_id: str) -> None:
        """默认会话映射落库逻辑。"""
        await session_manager.register_sdk_session(
            session_key=session_key,
            session_id=session_id,
        )

    def _dispatch(self, response_msg: SDKMessage) -> list[Message | StreamMessage]:
        """按 SDK 消息类型分发。"""
        if isinstance(response_msg, SystemMessage):
            return self._handle_system_message(response_msg)
        if isinstance(response_msg, StreamEvent):
            return self._handle_stream_event(response_msg)
        if isinstance(response_msg, AssistantMessage):
            return self._handle_assistant_message(response_msg)
        if isinstance(response_msg, UserMessage):
            return self._handle_tool_result_message(response_msg)
        if isinstance(response_msg, ResultMessage):
            return [self._build_result_message(response_msg)]
        raise ValueError(f"Unsupported SDK message type: {type(response_msg)}")

    def _handle_stream_event(self, response_msg: StreamEvent) -> list[Message | StreamMessage]:
        """处理流式事件，仅负责实时 patch。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        event = payload.get("event") if isinstance(payload.get("event"), dict) else payload
        event_type = str(event.get("type") or "")
        if not event_type:
            return []

        if event_type == "message_start":
            message = event.get("message") or {}
            self._segment.start(
                message_id=message.get("id"),
                model=message.get("model"),
                usage=message.get("usage"),
            )
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_start",
                    message={"model": self._segment.model},
                    usage=self._segment.usage,
                )
            ]

        if event_type == "content_block_start":
            index = event.get("index")
            if not isinstance(index, int):
                return []
            block = SdkMessageMapper.to_plain_block(event.get("content_block"))
            self._segment.apply_block(index, block)
            if block.get("type") == "tool_use":
                return []
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="content_block_start",
                    index=index,
                    content_block=block,
                )
            ]

        if event_type == "content_block_delta":
            index = event.get("index")
            if not isinstance(index, int) or not self._segment.apply_delta(index, event.get("delta") or {}):
                return []
            current_block = self._segment.content[index]
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="content_block_delta",
                    index=index,
                    content_block=current_block,
                )
            ]

        if event_type == "message_delta":
            delta = event.get("delta") or {}
            self._segment.update_message_meta(
                stop_reason=delta.get("stop_reason"),
                usage=event.get("usage"),
            )
            messages: list[Message | StreamMessage] = [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_delta",
                    message={"stop_reason": self._segment.stop_reason},
                    usage=self._segment.usage,
                )
            ]
            if self._segment.has_content() and self._segment.stop_reason:
                assistant_message = self._segment.build_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    parent_id=self.round_id,
                    is_complete=True,
                )
                self._remember_assistant_message(assistant_message)
                messages.append(assistant_message)
            return messages

        if event_type == "message_stop":
            return [
                self._segment.build_stream_message(
                    session_key=self.session_key,
                    agent_id=self.agent_id,
                    round_id=self.round_id,
                    session_id=self.session_id,
                    stream_type="message_stop",
                )
            ]

        return []

    def _handle_assistant_message(self, response_msg: AssistantMessage) -> list[Message]:
        """使用 SDK assistant 快照补齐当前段内容。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        content = SdkMessageMapper.normalize_content_blocks(payload.get("content"))
        if not content:
            return []
        self._segment.replace_from_snapshot(
            content=content,
            model=payload.get("model"),
            usage=payload.get("usage"),
            stop_reason=payload.get("stop_reason"),
        )
        assistant_message = self._segment.build_message(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=str(payload.get("parent_tool_use_id") or self.round_id),
            # 中文注释：这里处理的是 SDK 给出的完整 assistant 快照，
            # 不是逐 token 的流式片段。即使 stop_reason 未显式带回，
            # 也必须按终态落库，否则固定 session 的历史重载会把旧回复误判成仍在执行。
            is_complete=True,
        )
        self._remember_assistant_message(assistant_message)
        return [assistant_message]

    def _handle_system_message(self, response_msg: SystemMessage) -> list[Message]:
        """处理可见 system 事件与任务进度事件。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        subtype = str(payload.get("subtype") or "")
        data = payload.get("data") if isinstance(payload.get("data"), dict) else {}
        if subtype != "task_progress" or not data:
            visible_system_message = self._build_visible_system_message(
                payload=payload,
                subtype=subtype,
            )
            return [visible_system_message] if visible_system_message else []

        task_id = str(data.get("task_id") or "")
        if not task_id:
            return []

        self._segment.append_task_progress(
            {
                "type": "task_progress",
                "task_id": task_id,
                "tool_use_id": data.get("tool_use_id"),
                "description": str(data.get("description") or ""),
                "last_tool_name": data.get("last_tool_name"),
                "usage": data.get("usage") if isinstance(data.get("usage"), dict) else {},
            }
        )
        message = self._segment.build_message(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self.round_id,
            is_complete=False,
        )
        self._remember_assistant_message(message)
        return [message]

    @staticmethod
    def _normalize_result_subtype(subtype: object) -> str:
        """把 SDK result subtype 归一化为前端可识别的终态。"""
        normalized_subtype = str(subtype or "success")
        if normalized_subtype in {"success", "error", "interrupted"}:
            return normalized_subtype
        return "error"

    def _build_visible_system_message(
        self,
        payload: dict,
        subtype: str,
    ) -> Message | None:
        """把可见的 SDK system 事件转换为统一消息。"""
        metadata = {
            "subtype": subtype,
            "uuid": payload.get("uuid"),
        }

        if subtype == "api_retry":
            content = self._format_api_retry_message(payload)
            metadata.update({
                "attempt": payload.get("attempt"),
                "max_retries": payload.get("max_retries"),
                "retry_delay_ms": payload.get("retry_delay_ms"),
                "error_status": payload.get("error_status"),
                "error": payload.get("error"),
            })
        elif subtype == "task_started":
            content = self._format_task_started_message(payload)
            data = payload.get("data") if isinstance(payload.get("data"), dict) else {}
            metadata.update({
                "task_id": data.get("task_id") or payload.get("task_id"),
                "task_type": data.get("task_type") or payload.get("task_type"),
                "tool_use_id": data.get("tool_use_id") or payload.get("tool_use_id"),
            })
        else:
            return None

        return Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self._current_parent_id(),
            role="system",
            content=content,
            metadata={key: value for key, value in metadata.items() if value is not None},
        )

    def _format_api_retry_message(self, payload: dict) -> str:
        """把 SDK api_retry 事件格式化为用户可读文本。"""
        attempt = payload.get("attempt")
        max_retries = payload.get("max_retries")
        retry_delay_ms = payload.get("retry_delay_ms")
        error_status = str(payload.get("error_status") or "").strip()
        error = str(payload.get("error") or "").strip()

        if isinstance(attempt, int) and isinstance(max_retries, int) and max_retries > 0:
            retry_progress_text = f"第 {attempt}/{max_retries} 次重试"
        elif isinstance(attempt, int):
            retry_progress_text = f"第 {attempt} 次重试"
        else:
            retry_progress_text = "下一次重试"

        if isinstance(retry_delay_ms, (int, float)):
            delay_text = (
                f"{retry_delay_ms / 1000:.1f} 秒后"
                if retry_delay_ms >= 1000
                else f"{int(retry_delay_ms)} 毫秒后"
            )
        else:
            delay_text = "稍后"

        reason_parts: list[str] = []
        if error_status and error_status.lower() != "none":
            reason_parts.append(f"状态 {error_status}")
        if error and error.lower() != "none":
            reason_parts.append(error)

        reason_text = f"（原因：{'，'.join(reason_parts)}）" if reason_parts else ""
        return f"接口调用失败，系统将在 {delay_text} 自动进行{retry_progress_text}{reason_text}。"

    def _format_task_started_message(self, payload: dict) -> str:
        """把 task_started 事件格式化为用户可读文本。"""
        data = payload.get("data") if isinstance(payload.get("data"), dict) else {}
        description = str(data.get("description") or payload.get("description") or "").strip()
        task_type = str(data.get("task_type") or payload.get("task_type") or "").strip()

        task_type_label = {
            "local_agent": "子代理",
            "local_bash": "本地任务",
        }.get(task_type, "后台任务")

        if description:
            return f"已启动{task_type_label}：{description}"
        return f"已启动{task_type_label}"

    def _handle_tool_result_message(self, response_msg: UserMessage) -> list[Message]:
        """将工具结果回填到当前 assistant 段。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        content = SdkMessageMapper.normalize_content_blocks(payload.get("content"))
        if not content or not all(block.get("type") == "tool_result" for block in content):
            return []
        content = [self._enrich_tool_result_block(block) for block in content]
        self._segment.append_tool_results(content)
        message = self._segment.build_message(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self.round_id,
            is_complete=True,
        )
        self._remember_assistant_message(message)
        return [message]

    def _enrich_tool_result_block(self, block: dict) -> dict:
        """给权限类工具结果补齐结构化错误码。"""
        enriched_block = dict(block)
        if not enriched_block.get("is_error"):
            return enriched_block

        tool_use_id = str(enriched_block.get("tool_use_id") or "").strip()
        if not tool_use_id:
            return enriched_block

        tool_name = self._segment.find_tool_name(tool_use_id)
        error_code = infer_permission_error_code(
            tool_name=tool_name,
            message=enriched_block.get("content"),
        )
        if error_code:
            enriched_block["error_code"] = error_code
        return enriched_block

    def _build_result_message(self, response_msg: ResultMessage) -> Message:
        """构建结果消息。"""
        payload = SdkMessageMapper.to_plain_dict(response_msg)
        subtype = str(payload.get("subtype") or "success")
        normalized_subtype = self._normalize_result_subtype(subtype)
        return Message(
            message_id=str(uuid.uuid4()),
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            session_id=self.session_id,
            parent_id=self._current_parent_id(),
            role="result",
            subtype=normalized_subtype,
            duration_ms=int(payload.get("duration_ms") or 0),
            duration_api_ms=int(payload.get("duration_api_ms") or 0),
            num_turns=int(payload.get("num_turns") or 0),
            total_cost_usd=payload.get("total_cost_usd"),
            usage=payload.get("usage"),
            result=payload.get("result"),
            is_error=bool(payload.get("is_error", normalized_subtype != "success")),
        )

    def _current_parent_id(self) -> str:
        """返回当前消息应该关联的父节点。"""
        return self._segment.message_id or self._last_assistant_message_id or self.round_id

    def _remember_assistant_message(self, message: Message) -> None:
        """记录最近一条 assistant 消息 ID。"""
        self._last_assistant_message_id = message.message_id
