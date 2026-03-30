# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：skill.py
# @Date   ：2026/3/31
# @Author ：leemysw
# 2026/3/31   Create
# =====================================================

"""Skill 资源池与 Agent-Skill 关联 ORM 模型。"""

from __future__ import annotations

from sqlalchemy import Boolean, ForeignKey, String, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class PoolSkill(TimestampMixin, Base):
    """全局技能池条目 —— 记录 skill 的安装与启用状态。"""

    __tablename__ = "pool_skills"

    # 以 skill name 为主键，保证唯一
    name: Mapped[str] = mapped_column(String(256), primary_key=True)
    installed: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    global_enabled: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)


class AgentSkill(TimestampMixin, Base):
    """Agent 维度的 Skill 安装关联。"""

    __tablename__ = "agent_skills"
    __table_args__ = (
        UniqueConstraint("agent_id", "skill_name", name="uq_agent_skill"),
    )

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
    )
    skill_name: Mapped[str] = mapped_column(String(256), nullable=False, index=True)
