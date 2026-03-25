# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_protocol.py
# @Date   ：2026/3/25 21:10
# @Author ：OpenAI
# =====================================================

"""Protocol Room 持久化与 API 模型。"""

from __future__ import annotations

from datetime import datetime
from typing import Any, Optional

from pydantic import BaseModel, Field

from agent.schema.model_chat_persistence import MemberRecord, RoomRecord


class PersistenceModel(BaseModel):
    """持久化模型基类。"""

    model_config = {"from_attributes": True}


class ProtocolDefinitionRecord(PersistenceModel):
    """协议定义记录。"""

    id: str = Field(..., description="协议定义 ID")
    slug: str = Field(..., description="协议标识")
    name: str = Field(..., description="协议名称")
    description: str = Field(default="", description="协议描述")
    version: int = Field(default=1, description="协议版本")
    coordinator_mode: str = Field(default="main_agent", description="协调者模式")
    phases: list[str] = Field(default_factory=list, description="阶段列表")
    channel_policy: list[dict[str, Any]] = Field(default_factory=list, description="频道策略")
    turn_policy: dict[str, Any] = Field(default_factory=dict, description="发言/轮次策略")
    action_schemas: dict[str, Any] = Field(default_factory=dict, description="动作 schema")
    visibility_resolver: str = Field(default="default", description="可见性解析策略")
    completion_rule: dict[str, Any] = Field(default_factory=dict, description="完成规则")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ProtocolRunRecord(PersistenceModel):
    """协议运行记录。"""

    id: str = Field(..., description="运行 ID")
    room_id: str = Field(..., description="所属 room")
    protocol_definition_id: str = Field(..., description="协议定义 ID")
    title: Optional[str] = Field(default=None, description="运行标题")
    status: str = Field(default="running", description="运行状态")
    current_phase: str = Field(default="setup", description="当前阶段")
    phase_index: int = Field(default=0, description="阶段索引")
    current_turn_key: Optional[str] = Field(default=None, description="当前轮次 key")
    coordinator_agent_id: str = Field(..., description="协调者 agent")
    run_config: dict[str, Any] = Field(default_factory=dict, description="运行配置")
    state: dict[str, Any] = Field(default_factory=dict, description="运行状态")
    completed_at: Optional[datetime] = Field(default=None, description="完成时间")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ChannelRecord(PersistenceModel):
    """协议频道记录。"""

    id: str = Field(..., description="频道 ID")
    room_id: str = Field(..., description="所属 room")
    protocol_run_id: str = Field(..., description="所属 run")
    slug: str = Field(..., description="频道 slug")
    name: str = Field(..., description="频道名称")
    channel_type: str = Field(..., description="频道类型")
    visibility: str = Field(..., description="可见性类型")
    topic: str = Field(default="", description="频道主题")
    position: int = Field(default=0, description="排序位置")
    metadata: dict[str, Any] = Field(default_factory=dict, description="频道元数据")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ChannelMemberRecord(PersistenceModel):
    """协议频道成员记录。"""

    id: str = Field(..., description="频道成员 ID")
    channel_id: str = Field(..., description="频道 ID")
    member_type: str = Field(..., description="成员类型")
    member_user_id: Optional[str] = Field(default=None, description="用户成员")
    member_agent_id: Optional[str] = Field(default=None, description="agent 成员")
    role_label: Optional[str] = Field(default=None, description="角色标签")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ChannelAggregate(PersistenceModel):
    """频道聚合。"""

    channel: ChannelRecord = Field(..., description="频道实体")
    members: list[ChannelMemberRecord] = Field(default_factory=list, description="频道成员")


class ActionRequestRecord(PersistenceModel):
    """动作请求记录。"""

    id: str = Field(..., description="请求 ID")
    protocol_run_id: str = Field(..., description="所属 run")
    channel_id: Optional[str] = Field(default=None, description="关联频道")
    phase_name: str = Field(..., description="所属阶段")
    turn_key: Optional[str] = Field(default=None, description="轮次 key")
    action_type: str = Field(..., description="动作类型")
    status: str = Field(default="pending", description="请求状态")
    requested_by_agent_id: Optional[str] = Field(default=None, description="请求发起者")
    allowed_actor_agent_ids: list[str] = Field(default_factory=list, description="允许执行的 agent")
    audience_agent_ids: list[str] = Field(default_factory=list, description="可见 audience")
    input_schema: dict[str, Any] = Field(default_factory=dict, description="动作 schema")
    target_scope: dict[str, Any] = Field(default_factory=dict, description="目标范围")
    prompt_text: Optional[str] = Field(default=None, description="请求提示文案")
    metadata: dict[str, Any] = Field(default_factory=dict, description="请求元数据")
    resolved_at: Optional[datetime] = Field(default=None, description="结束时间")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ActionSubmissionRecord(PersistenceModel):
    """动作提交记录。"""

    id: str = Field(..., description="提交 ID")
    request_id: str = Field(..., description="动作请求 ID")
    protocol_run_id: str = Field(..., description="所属 run")
    channel_id: Optional[str] = Field(default=None, description="关联频道")
    actor_type: str = Field(default="agent", description="提交者类型")
    actor_agent_id: Optional[str] = Field(default=None, description="提交者 agent")
    actor_user_id: Optional[str] = Field(default=None, description="提交者 user")
    action_type: str = Field(..., description="动作类型")
    payload: dict[str, Any] = Field(default_factory=dict, description="提交载荷")
    status: str = Field(default="submitted", description="提交状态")
    metadata: dict[str, Any] = Field(default_factory=dict, description="提交元数据")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class RunStateSnapshotRecord(PersistenceModel):
    """运行态快照/事件记录。"""

    id: str = Field(..., description="快照 ID")
    protocol_run_id: str = Field(..., description="所属 run")
    event_seq: int = Field(..., description="事件序号")
    phase_name: str = Field(..., description="阶段名称")
    event_type: str = Field(..., description="事件类型")
    channel_id: Optional[str] = Field(default=None, description="关联频道")
    actor_agent_id: Optional[str] = Field(default=None, description="行为 actor")
    visibility: str = Field(default="public", description="可见性")
    audience_agent_ids: list[str] = Field(default_factory=list, description="可见 audience")
    headline: Optional[str] = Field(default=None, description="事件标题")
    body: Optional[str] = Field(default=None, description="事件正文")
    state: dict[str, Any] = Field(default_factory=dict, description="状态片段")
    metadata: dict[str, Any] = Field(default_factory=dict, description="事件元数据")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ProtocolRunDetail(PersistenceModel):
    """协议运行详情。"""

    room: RoomRecord = Field(..., description="所属 room")
    members: list[MemberRecord] = Field(default_factory=list, description="room 成员")
    definition: ProtocolDefinitionRecord = Field(..., description="协议定义")
    run: ProtocolRunRecord = Field(..., description="运行记录")
    channels: list[ChannelAggregate] = Field(default_factory=list, description="频道列表")
    action_requests: list[ActionRequestRecord] = Field(default_factory=list, description="动作请求")
    action_submissions: list[ActionSubmissionRecord] = Field(default_factory=list, description="动作提交")
    snapshots: list[RunStateSnapshotRecord] = Field(default_factory=list, description="按视角过滤后的快照")
    viewer_agent_id: Optional[str] = Field(default=None, description="当前视角 agent")


class ProtocolRunListItem(PersistenceModel):
    """room 下协议运行列表项。"""

    run: ProtocolRunRecord = Field(..., description="运行记录")
    definition: ProtocolDefinitionRecord = Field(..., description="协议定义")
