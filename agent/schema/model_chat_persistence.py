# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_chat_persistence.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Room / Conversation 持久化模型。"""

from __future__ import annotations

from datetime import datetime
from typing import Optional

from pydantic import AliasChoices, BaseModel, Field


class PersistenceModel(BaseModel):
    """持久化模型基类。"""

    model_config = {"from_attributes": True}


class MemberRecord(PersistenceModel):
    """房间成员记录。"""

    id: str = Field(..., description="成员记录 ID")
    room_id: str = Field(..., description="所属房间")
    member_type: str = Field(..., description="成员类型")
    member_user_id: Optional[str] = Field(default=None, description="用户成员")
    member_agent_id: Optional[str] = Field(default=None, description="Agent 成员")
    member_source: str = Field(default="existing", description="成员来源")
    member_role: Optional[str] = Field(default=None, description="成员角色")
    member_status: str = Field(default="listening", description="成员状态")
    member_visibility_scope: list[str] = Field(default_factory=list, description="成员可见范围")
    workspace_binding: bool = Field(default=False, description="是否绑定 workspace")
    joined_at: Optional[datetime] = Field(default=None, description="加入时间")


class RoomRecord(PersistenceModel):
    """房间记录。"""

    id: str = Field(..., description="房间 ID")
    room_type: str = Field(..., description="房间类型")
    name: Optional[str] = Field(default=None, description="房间名称")
    description: str = Field(default="", description="房间描述")
    mode: str = Field(default="open", description="房间模式")
    runtime_status: str = Field(default="created", description="运行时状态")
    active_run_id: Optional[str] = Field(default=None, description="当前运行实例 ID")
    orchestrator_agent_id: str = Field(default="", description="主持 agent ID")
    ruleset_slug: Optional[str] = Field(default=None, description="规则集标识")
    goal: str = Field(default="", description="房间目标")
    runtime_state: dict = Field(default_factory=dict, description="运行时状态")
    capabilities: dict = Field(
        default_factory=dict,
        description="能力开关",
        validation_alias=AliasChoices("capabilities", "capabilities_json"),
    )
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class RoomAggregate(PersistenceModel):
    """房间聚合。"""

    room: RoomRecord = Field(..., description="房间实体")
    members: list[MemberRecord] = Field(default_factory=list, description="成员列表")


class ConversationRecord(PersistenceModel):
    """对话记录。"""

    id: str = Field(..., description="对话 ID")
    room_id: str = Field(..., description="所属房间")
    conversation_type: str = Field(..., description="对话类型")
    title: Optional[str] = Field(default=None, description="标题")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ConversationContextAggregate(PersistenceModel):
    """房间对话上下文聚合。"""

    room: RoomRecord = Field(..., description="房间实体")
    members: list[MemberRecord] = Field(default_factory=list, description="成员列表")
    conversation: ConversationRecord = Field(..., description="对话实体")
    sessions: list["SessionRecord"] = Field(default_factory=list, description="运行时会话")


class SessionRecord(PersistenceModel):
    """运行时会话记录。"""

    id: str = Field(..., description="会话 ID")
    conversation_id: str = Field(..., description="所属对话")
    agent_id: str = Field(..., description="所属 Agent")
    runtime_id: str = Field(..., description="所属 Runtime")
    version_no: int = Field(default=1, description="版本号")
    branch_key: str = Field(default="main", description="多活分支")
    is_primary: bool = Field(default=True, description="是否主版本")
    sdk_session_id: Optional[str] = Field(default=None, description="SDK 会话 ID")
    status: str = Field(default="active", description="会话状态")
    last_activity_at: Optional[datetime] = Field(default=None, description="最近活动时间")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class MessageRecord(PersistenceModel):
    """消息索引记录。"""

    id: str = Field(..., description="消息 ID")
    conversation_id: str = Field(..., description="所属对话")
    session_id: Optional[str] = Field(default=None, description="所属会话")
    sender_type: str = Field(..., description="发送方类型")
    sender_user_id: Optional[str] = Field(default=None, description="用户发送方")
    sender_agent_id: Optional[str] = Field(default=None, description="Agent 发送方")
    kind: str = Field(..., description="消息类型")
    content_preview: Optional[str] = Field(default=None, description="预览内容")
    jsonl_path: str = Field(..., description="JSONL 路径")
    jsonl_offset: Optional[int] = Field(default=None, description="JSONL 偏移")
    round_id: Optional[str] = Field(default=None, description="轮次标识")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class RoundRecord(PersistenceModel):
    """轮次索引记录。"""

    id: str = Field(..., description="轮次记录 ID")
    session_id: str = Field(..., description="所属会话")
    round_id: str = Field(..., description="外部轮次 ID")
    trigger_message_id: str = Field(..., description="触发消息 ID")
    status: str = Field(..., description="轮次状态")
    started_at: Optional[datetime] = Field(default=None, description="开始时间")
    finished_at: Optional[datetime] = Field(default=None, description="结束时间")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


ConversationContextAggregate.model_rebuild()
