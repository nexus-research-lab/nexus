# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_cost.py
# @Date   ：2026/03/10 23:40
# @Author ：leemysw
# 2026/03/10 23:40   Create
# =====================================================

"""
成本追踪模型

[INPUT]: 依赖 pydantic
[OUTPUT]: 对外提供成本账本条目和汇总模型
[POS]: schema 模块的成本模型定义，被 cost_repository / API 消费
[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
"""

from datetime import datetime
from typing import Optional

from pydantic import BaseModel, Field


class CostLedgerEntry(BaseModel):
    """单次 result 的成本账本条目。"""

    entry_id: str = Field(..., description="账本条目 ID")
    agent_id: str = Field(..., description="Agent ID")
    session_key: str = Field(..., description="会话路由键")
    session_id: str = Field(default="", description="SDK Session ID")
    round_id: str = Field(..., description="轮次 ID")
    message_id: str = Field(..., description="result 消息 ID")
    subtype: str = Field(default="success", description="结果类型")
    input_tokens: int = Field(default=0, description="输入 token 数")
    output_tokens: int = Field(default=0, description="输出 token 数")
    cache_creation_input_tokens: int = Field(default=0, description="缓存创建 token 数")
    cache_read_input_tokens: int = Field(default=0, description="缓存读取 token 数")
    total_cost_usd: float = Field(default=0.0, description="总成本（USD）")
    duration_ms: int = Field(default=0, description="总耗时（ms）")
    duration_api_ms: int = Field(default=0, description="API 耗时（ms）")
    num_turns: int = Field(default=0, description="轮次内 turns 数")
    created_at: datetime = Field(default_factory=datetime.utcnow, description="创建时间")


class SessionCostSummary(BaseModel):
    """Session 维度的成本汇总。"""

    agent_id: str = Field(..., description="Agent ID")
    session_key: str = Field(..., description="会话路由键")
    session_id: str = Field(default="", description="最近一次 SDK Session ID")
    total_input_tokens: int = Field(default=0, description="累计输入 token")
    total_output_tokens: int = Field(default=0, description="累计输出 token")
    total_tokens: int = Field(default=0, description="累计总 token")
    total_cache_creation_input_tokens: int = Field(default=0, description="累计缓存创建 token")
    total_cache_read_input_tokens: int = Field(default=0, description="累计缓存读取 token")
    total_cost_usd: float = Field(default=0.0, description="累计成本（USD）")
    completed_rounds: int = Field(default=0, description="已完成轮次")
    error_rounds: int = Field(default=0, description="失败轮次")
    last_round_id: Optional[str] = Field(default=None, description="最近一次轮次 ID")
    last_run_duration_ms: Optional[int] = Field(default=None, description="最近一次执行耗时")
    last_run_cost_usd: Optional[float] = Field(default=None, description="最近一次执行成本")
    updated_at: datetime = Field(default_factory=datetime.utcnow, description="更新时间")


class AgentCostSummary(BaseModel):
    """Agent 维度的成本汇总。"""

    agent_id: str = Field(..., description="Agent ID")
    total_input_tokens: int = Field(default=0, description="累计输入 token")
    total_output_tokens: int = Field(default=0, description="累计输出 token")
    total_tokens: int = Field(default=0, description="累计总 token")
    total_cache_creation_input_tokens: int = Field(default=0, description="累计缓存创建 token")
    total_cache_read_input_tokens: int = Field(default=0, description="累计缓存读取 token")
    total_cost_usd: float = Field(default=0.0, description="累计成本（USD）")
    completed_rounds: int = Field(default=0, description="已完成轮次")
    error_rounds: int = Field(default=0, description="失败轮次")
    cost_sessions: int = Field(default=0, description="有成本记录的会话数")
    updated_at: datetime = Field(default_factory=datetime.utcnow, description="更新时间")
