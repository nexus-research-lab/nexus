# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：message_formatter.py
# @Date   ：2026/3/13 14:28
# @Author ：leemysw
# 2026/3/13 14:28   Create
# =====================================================

"""Claude 消息格式转换与会话消息处理。"""

import json
import uuid
from dataclasses import asdict
from datetime import datetime
from typing import Any, Dict, List, Optional

from claude_agent_sdk import Message, ResultMessage, SystemMessage, ThinkingBlock, UserMessage
from claude_agent_sdk.types import (
    AssistantMessage,
    ContentBlock,
    StreamEvent,
    TextBlock,
    ToolResultBlock,
    ToolUseBlock,
)

from agent.infra.agent.session_manager import session_manager
from agent.schema.model_message import AMessage
from agent.service.session.session_store import session_store
from agent.utils.logger import logger


class SDKMessageProcessor:
    """Claude Agent SDK 消息处理器。"""

    def __init__(self):
        self.message_type_mapping = {
            AssistantMessage: "assistant",
            UserMessage: "user",
            SystemMessage: "system",
            ResultMessage: "result",
            StreamEvent: "stream",
        }
        self.content_block_mapping = {
            TextBlock: "text",
            ThinkingBlock: "thinking",
            ToolUseBlock: "tool_use",
            ToolResultBlock: "tool_result",
        }

    def process_message(
        self,
        message: Message,
        session_key: str,
        agent_id: str,
        session_id: str,
        round_id: str,
        parent_id: str = None,
    ) -> List[AMessage]:
        """将 Claude SDK 消息转换为 AMessage 列表。"""
        messages = [message]
        if isinstance(message, (AssistantMessage, UserMessage)):
            messages = self._process_assistant_user_message(message)

        a_messages = []
        for msg in messages:
            block_type = None
            if (
                isinstance(msg, (AssistantMessage, UserMessage))
                and hasattr(msg, "content")
                and isinstance(msg.content, list)
                and len(msg.content) > 0
            ):
                if len(msg.content) == 1:
                    block_type = self.content_block_mapping.get(type(msg.content[0]))
                else:
                    block_type = "mixed"

            a_messages.append(
                AMessage(
                    message_type=self.message_type_mapping.get(type(msg)),
                    block_type=block_type,
                    message=msg,
                    message_id=str(uuid.uuid4()),
                    agent_id=agent_id,
                    session_id=session_id,
                    session_key=session_key,
                    round_id=round_id,
                    parent_id=parent_id,
                )
            )

        return a_messages

    @staticmethod
    def _process_assistant_user_message(message: Message) -> List[Message]:
        """规范化 AssistantMessage/UserMessage 的 content。"""
        if isinstance(message.content, str):
            message.content = [TextBlock(text=message.content)]
            return [message]

        if isinstance(message.content, list) and len(message.content) > 0:
            return [message]

        raise ValueError(f"Invalid content type: {type(message.content)}")

    def print_message(self, message: Message, session_id: str = None) -> None:
        """打印 SDK 消息，便于跟踪执行过程。"""
        is_stream_event = isinstance(message, StreamEvent)
        timestamp = datetime.now().strftime("%H:%M:%S")

        if not is_stream_event:
            if session_id:
                print(f"🕐 [{timestamp}] 📋 Session: {session_id} - ", end="")
            else:
                print(f"🕐 [{timestamp}] 📋 Agent Message - ", end="")

        if isinstance(message, AssistantMessage):
            self._print_assistant_message(message)
        elif isinstance(message, UserMessage):
            self._print_user_message(message)
        elif isinstance(message, SystemMessage):
            self._print_system_message(message)
        elif isinstance(message, ResultMessage):
            self._print_result_message(message)
        elif isinstance(message, StreamEvent):
            ...
        else:
            print(f"❓ 未知消息类型: {type(message)}")
            self._print_pretty_json(asdict(message))

        if not is_stream_event:
            print("=" * 80)
            print()

    @staticmethod
    def _print_block(block: ContentBlock) -> None:
        """打印单个内容块。"""
        if isinstance(block, TextBlock):
            print(f"💬 文本: {block.text}")
        elif isinstance(block, ThinkingBlock):
            print(f"🤔 思考: {block.thinking}")
            print(f"🔑 签名: {block.signature}")
        elif isinstance(block, ToolResultBlock):
            print(f"🆔 工具ID: {block.tool_use_id}")
            if block.content:
                print(f"📈 结果: {block.content}")
            if block.is_error:
                print(" ❌ 工具执行错误")
        elif isinstance(block, ToolUseBlock):
            print(f"🔧 工具调用: {block.name}({block.input}) -- {block.id}")

    def _print_user_message(self, message: UserMessage) -> None:
        """打印用户消息。"""
        print("👤 用户消息 (User Message)")
        print("-" * 40)
        if message.parent_tool_use_id:
            print(f"🔗 父工具ID: {message.parent_tool_use_id}")

        content = message.content
        if isinstance(content, str):
            print(f"💬: {content}")
        elif isinstance(content, list):
            if len(content) == 1:
                self._print_block(content[0])
            else:
                for index, block in enumerate(content):
                    print(f"  📝 块 {index + 1}:")
                    self._print_block(block)

    def _print_assistant_message(self, message: AssistantMessage) -> None:
        """打印助手消息。"""
        print(f"🤖 助手回复 (Assistant Message) - 模型: {message.model}")
        print("-" * 40)
        if message.parent_tool_use_id:
            print(f"🔗 父工具ID: {message.parent_tool_use_id}")

        if len(message.content) == 1:
            self._print_block(message.content[0])
            return

        for index, block in enumerate(message.content):
            print(f"  📦 内容块 {index + 1}:")
            self._print_block(block)

    @staticmethod
    def _print_system_message(message: SystemMessage) -> None:
        """打印系统消息。"""
        print(f"⚙️ 系统消息 (System Message) - 类型: {message.subtype}")
        print("-" * 40)

        if not message.data:
            return

        print("📋 数据内容:")
        for key, value in message.data.items():
            print(f"   • {key}: {value}")

    @staticmethod
    def _print_result_message(message: ResultMessage) -> None:
        """打印结果消息。"""
        print("✅ 执行结果 (Result Message)")
        print("-" * 40)
        print("📊 执行统计:")
        print(f"   • 耗时: {message.duration_ms}ms")
        print(f"   • API耗时: {message.duration_api_ms}ms")
        print(f"   • 对话轮数: {message.num_turns}")
        print(f"   • 状态: {'✅ 成功' if not message.is_error else '❌ 失败'}")
        if message.total_cost_usd:
            print(f"   • 成本: ${message.total_cost_usd:.6f}")

    @staticmethod
    def _print_pretty_json(data: Dict[str, Any]) -> None:
        """格式化打印 JSON。"""
        print(json.dumps(data, ensure_ascii=False, indent=2))


sdk_message_processor = SDKMessageProcessor()


class ChatMessageProcessor:
    """单轮聊天消息处理器。"""

    def __init__(self, session_key: str, query: str, round_id: Optional[str] = None, agent_id: str = "main"):
        self.query = query
        self.session_key = session_key
        self.agent_id = agent_id or "main"
        self.subtype: Optional[str] = None
        self.round_id: Optional[str] = round_id
        self.parent_id: Optional[str] = None
        self.session_id: Optional[str] = None
        self.message_count: int = 0
        self.is_streaming: bool = False
        self.is_streaming_tool: bool = False
        self.is_save_user_message: bool = False
        self.stream_message_id: Optional[str] = None
        self.accumulated_thinking: str = ""
        self.accumulated_signature: str = ""
        self.accumulated_content_blocks: list[Any] = []

    async def process_messages(self, response_msg: Message) -> list[AMessage]:
        """处理响应消息并管理消息状态。"""
        sdk_message_processor.print_message(response_msg, self.session_key)
        self.set_subtype(response_msg)
        await self.set_session_id(response_msg)
        await self.save_user_message(self.query)

        messages = sdk_message_processor.process_message(
            message=response_msg,
            session_key=self.session_key,
            agent_id=self.agent_id,
            session_id=self.session_id,
            round_id=self.round_id,
            parent_id=self.parent_id,
        )

        processed_messages = []
        for a_message in messages:
            self.update_stream_state(a_message)

            if a_message.message_type == "stream" and self.is_streaming_tool:
                continue

            if a_message.message_type != "stream":
                self.parent_id = a_message.message_id
                await session_store.save_message(a_message)

            processed_messages.append(a_message)
            self.message_count += 1

        return processed_messages

    async def set_session_id(self, response_msg: Message) -> Optional[str]:
        """处理 session 映射关系。"""
        if self.session_id is None:
            if isinstance(response_msg, SystemMessage):
                self.session_id = response_msg.data.get("session_id", None)
            else:
                raise ValueError("⚠️When session_id is None, response_msg must be a SystemMessage")

            await session_manager.register_sdk_session(session_key=self.session_key, session_id=self.session_id)
            logger.debug(f"🔗建立映射: key={self.session_key} ↔ sdk_session={self.session_id}")

        return self.session_id

    def set_subtype(self, response_msg: Message) -> None:
        """设置消息子类型。"""
        if hasattr(response_msg, "subtype"):
            self.subtype = response_msg.subtype

        if isinstance(response_msg, ResultMessage):
            self.subtype = "success" if response_msg.subtype == "success" else "error"

    def update_stream_state(self, a_message: AMessage) -> None:
        """更新流式处理状态。"""
        if a_message.message_type == "stream" and a_message.message.event["type"] == "message_start":
            self.is_streaming = True
            self.stream_message_id = a_message.message_id
            self.accumulated_thinking = ""
            self.accumulated_signature = ""
            self.accumulated_content_blocks = []

        if self.is_streaming:
            if a_message.message_type == "stream":
                a_message.message_id = self.stream_message_id
                event_type = a_message.message.event["type"]

                if event_type == "content_block_start":
                    if a_message.message.event["content_block"]["type"] == "tool_use":
                        self.is_streaming_tool = True
                elif event_type == "content_block_delta":
                    delta = a_message.message.event.get("delta", {})
                    if delta.get("type") == "thinking_delta":
                        self.accumulated_thinking += delta.get("thinking", "")
                    elif delta.get("type") == "signature_delta":
                        self.accumulated_signature += delta.get("signature", "")

                if self.is_streaming_tool and event_type == "content_block_stop":
                    self.is_streaming_tool = False

            elif a_message.message_type == "assistant":
                if self.stream_message_id:
                    a_message.message_id = self.stream_message_id
                self.parent_id = a_message.message_id

                if isinstance(a_message.message.content, list):
                    a_message.message.content = self._merge_assistant_stream_content(a_message.message.content)

        if a_message.message_type == "stream" and a_message.message.event["type"] == "message_stop":
            self.is_streaming = False
            self.stream_message_id = None
            self.accumulated_content_blocks = []

    def _merge_assistant_stream_content(self, incoming_blocks: list[Any]) -> list[Any]:
        """合并同一条流式 assistant 消息的内容块。"""
        merged_blocks = list(self.accumulated_content_blocks)

        for block in incoming_blocks:
            self._upsert_content_block(merged_blocks, block)

        if self.accumulated_thinking:
            thinking_block = ThinkingBlock(
                thinking=self.accumulated_thinking,
                signature=self.accumulated_signature,
            )
            self._upsert_content_block(merged_blocks, thinking_block)

        self._move_thinking_to_front(merged_blocks)
        self.accumulated_content_blocks = merged_blocks
        return list(merged_blocks)

    @staticmethod
    def _upsert_content_block(content_blocks: list[Any], new_block: Any) -> None:
        """按块类型做幂等更新。"""
        if isinstance(new_block, ThinkingBlock):
            for index, block in enumerate(content_blocks):
                if isinstance(block, ThinkingBlock):
                    content_blocks[index] = new_block
                    return
            content_blocks.insert(0, new_block)
            return

        if isinstance(new_block, ToolUseBlock):
            for index, block in enumerate(content_blocks):
                if isinstance(block, ToolUseBlock) and block.id == new_block.id:
                    content_blocks[index] = new_block
                    return
            content_blocks.append(new_block)
            return

        if isinstance(new_block, ToolResultBlock):
            for index, block in enumerate(content_blocks):
                if isinstance(block, ToolResultBlock) and block.tool_use_id == new_block.tool_use_id:
                    content_blocks[index] = new_block
                    return
            content_blocks.append(new_block)
            return

        if isinstance(new_block, TextBlock):
            for block in content_blocks:
                if isinstance(block, TextBlock) and block.text == new_block.text:
                    return
            content_blocks.append(new_block)
            return

        content_blocks.append(new_block)

    @staticmethod
    def _move_thinking_to_front(content_blocks: list[Any]) -> None:
        """确保 thinking 始终位于首位。"""
        thinking_index: Optional[int] = None
        for index, block in enumerate(content_blocks):
            if isinstance(block, ThinkingBlock):
                thinking_index = index
                break

        if thinking_index is None or thinking_index == 0:
            return

        thinking_block = content_blocks.pop(thinking_index)
        content_blocks.insert(0, thinking_block)

    async def save_user_message(self, content: str):
        """保存用户消息。"""
        if self.is_save_user_message:
            return

        if not self.round_id:
            self.round_id = str(uuid.uuid4())

        user_message = AMessage(
            session_key=self.session_key,
            agent_id=self.agent_id,
            round_id=self.round_id,
            message_id=self.round_id,
            session_id=self.session_id,
            message_type="user",
            block_type="text",
            message=UserMessage(content=content),
        )

        await session_store.save_message(user_message)
        self.is_save_user_message = True
