# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_agent_persistence.py
# @Date   ：2026/3/18 23:56
# @Author ：leemysw
# 2026/3/18 23:56   Create
# =====================================================

"""Agent 持久化模型。"""

from __future__ import annotations

from datetime import datetime
from typing import Optional

from pydantic import BaseModel, Field


class PersistenceModel(BaseModel):
    """持久化模型基类。"""

    model_config = {"from_attributes": True}


class AgentRecord(PersistenceModel):
    """Agent 持久化记录。"""

    id: str = Field(..., description="Agent ID")
    slug: str = Field(..., description="稳定标识")
    name: str = Field(..., description="显示名称")
    description: str = Field(default="", description="对外描述")
    definition: str = Field(default="", description="核心定义")
    status: str = Field(default="active", description="状态")
    workspace_path: str = Field(..., description="工作区路径")
    # 身份标识字段
    avatar: Optional[str] = Field(default=None, description="头像标识（emoji 或图标名称）")
    vibe_tags: Optional[list] = Field(default=None, description="氛围标签列表")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class ProfileRecord(PersistenceModel):
    """Profile 持久化记录。"""

    id: str = Field(..., description="Profile ID")
    agent_id: str = Field(..., description="所属 Agent")
    display_name: str = Field(..., description="显示名")
    avatar_url: Optional[str] = Field(default=None, description="头像地址")
    headline: str = Field(default="", description="一句话简介")
    profile_markdown: str = Field(default="", description="详细介绍")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class RuntimeRecord(PersistenceModel):
    """Runtime 持久化记录。"""

    id: str = Field(..., description="Runtime ID")
    agent_id: str = Field(..., description="所属 Agent")
    provider: Optional[str] = Field(default=None, description="Provider")
    permission_mode: Optional[str] = Field(default=None, description="权限模式")
    allowed_tools_json: str = Field(default="[]", description="工具白名单")
    disallowed_tools_json: str = Field(default="[]", description="工具黑名单")
    mcp_servers_json: str = Field(default="{}", description="MCP 配置")
    max_turns: Optional[int] = Field(default=None, description="最大轮次")
    max_thinking_tokens: Optional[int] = Field(default=None, description="思考 token 上限")
    setting_sources_json: str = Field(default="[]", description="配置来源")
    runtime_version: int = Field(default=1, description="运行时版本")
    created_at: Optional[datetime] = Field(default=None, description="创建时间")
    updated_at: Optional[datetime] = Field(default=None, description="更新时间")


class AgentAggregate(PersistenceModel):
    """Agent 聚合视图。"""

    agent: AgentRecord = Field(..., description="Agent 实体")
    profile: ProfileRecord = Field(..., description="身份信息")
    runtime: RuntimeRecord = Field(..., description="运行时配置")


class CreateAgentAggregate(PersistenceModel):
    """创建 Agent 聚合输入。"""

    agent: AgentRecord = Field(..., description="Agent 实体")
    profile: ProfileRecord = Field(..., description="身份信息")
    runtime: RuntimeRecord = Field(..., description="运行时配置")
