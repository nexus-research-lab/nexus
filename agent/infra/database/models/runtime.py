# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：runtime.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""Runtime ORM 模型。"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import ForeignKey, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin

if TYPE_CHECKING:
    from agent.infra.database.models.agent import Agent
    from agent.infra.database.models.session import Session


class Runtime(TimestampMixin, Base):
    """Agent 的运行时配置。"""

    __tablename__ = "runtimes"

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    agent_id: Mapped[str] = mapped_column(
        String(64),
        ForeignKey("agents.id", ondelete="CASCADE"),
        unique=True,
        nullable=False,
    )
    # 中文注释：空 provider 表示跟随当前默认 Provider。
    provider: Mapped[str | None] = mapped_column(String(32))
    permission_mode: Mapped[str | None] = mapped_column(String(64))
    allowed_tools_json: Mapped[str] = mapped_column(Text, default="[]", nullable=False)
    disallowed_tools_json: Mapped[str] = mapped_column(Text, default="[]", nullable=False)
    mcp_servers_json: Mapped[str] = mapped_column(Text, default="{}", nullable=False)
    max_turns: Mapped[int | None] = mapped_column(Integer)
    max_thinking_tokens: Mapped[int | None] = mapped_column(Integer)
    setting_sources_json: Mapped[str] = mapped_column(Text, default="[]", nullable=False)
    runtime_version: Mapped[int] = mapped_column(Integer, default=1, nullable=False)

    agent: Mapped["Agent"] = relationship(back_populates="runtime")
    sessions: Mapped[list["Session"]] = relationship(back_populates="runtime")
