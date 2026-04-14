# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：provider_config.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""Provider 配置 ORM 模型。"""

from __future__ import annotations

from sqlalchemy import Boolean, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class ProviderConfig(TimestampMixin, Base):
    """Provider 运行时配置。"""

    __tablename__ = "provider"

    id: Mapped[str] = mapped_column(String(64), primary_key=True)
    provider: Mapped[str] = mapped_column(String(64), unique=True, nullable=False, index=True)
    display_name: Mapped[str] = mapped_column(String(128), nullable=False)
    auth_token: Mapped[str] = mapped_column(Text, default="", nullable=False)
    base_url: Mapped[str] = mapped_column(Text, default="", nullable=False)
    model: Mapped[str] = mapped_column(String(255), default="", nullable=False)
    enabled: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
    is_default: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
