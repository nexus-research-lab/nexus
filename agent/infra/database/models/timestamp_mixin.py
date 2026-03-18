# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：timestamp_mixin.py
# @Date   ：2026/3/18 17:02
# @Author ：leemysw
# 2026/3/18 17:02   Create
# =====================================================

"""时间戳字段混入。"""

from datetime import datetime

from sqlalchemy import DateTime, func
from sqlalchemy.orm import Mapped, mapped_column


class TimestampMixin:
    """提供创建和更新时间字段。"""

    created_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False,
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        onupdate=func.now(),
        nullable=False,
    )
