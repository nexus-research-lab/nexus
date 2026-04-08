# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：connector.py
# @Date   ：2026/3/31
# @Author ：Codex
# 2026/3/31   Create
# =====================================================

"""Connector 用户授权连接 ORM 模型。"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models.timestamp_mixin import TimestampMixin


class ConnectorConnection(TimestampMixin, Base):
    """用户对某个连接器的授权连接记录。"""

    __tablename__ = "connector_connections"

    # 连接器 ID（如 gmail, github）作为主键
    connector_id: Mapped[str] = mapped_column(String(128), primary_key=True)
    # 连接状态：connected / disconnected / expired
    state: Mapped[str] = mapped_column(String(32), default="disconnected", nullable=False)
    # 授权凭证，JSON 格式（当前明文存储，后续需加密，见 connector_service.py）
    credentials: Mapped[str] = mapped_column(Text, default="", nullable=False)
    # 授权方式
    auth_type: Mapped[str] = mapped_column(String(32), default="oauth2", nullable=False)
    # OAuth 发起时生成的 state，用于回调校验
    oauth_state: Mapped[str | None] = mapped_column(String(255), nullable=True)
    oauth_state_expires_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
