# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：f6a7b8c9d0e1_drop_activity_events_table.py
# @Date   ：2026/04/01 23:45
# @Author ：leemysw
# 2026/04/01 23:45   Create
# =====================================================

"""drop activity events table

Revision ID: f6a7b8c9d0e1
Revises: e5f6a7b8c9d0
Create Date: 2026-04-01 23:45:00.000000
"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

# revision identifiers, used by Alembic.
revision: str = "f6a7b8c9d0e1"
down_revision: Union[str, Sequence[str], None] = "e5f6a7b8c9d0"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """移除 activity 事件表。"""
    bind = op.get_bind()
    inspector = sa.inspect(bind)

    # 兼容已被手工清理或不完整环境，避免重复删除失败。
    if "activity_events" in inspector.get_table_names():
        op.drop_table("activity_events")


def downgrade() -> None:
    """恢复 activity 事件表。"""
    bind = op.get_bind()
    inspector = sa.inspect(bind)

    # 仅在表不存在时恢复，避免降级过程中重复建表。
    if "activity_events" in inspector.get_table_names():
        return

    op.create_table(
        "activity_events",
        sa.Column("id", sa.String(32), primary_key=True, comment="事件 ID（Snowflake）"),
        sa.Column("event_type", sa.String(50), nullable=False, comment="事件类型"),
        sa.Column("actor_type", sa.String(20), nullable=False, comment="执行者类型"),
        sa.Column("actor_id", sa.String(32), nullable=True, comment="执行者 ID"),
        sa.Column("target_type", sa.String(20), nullable=True, comment="目标类型"),
        sa.Column("target_id", sa.String(32), nullable=True, comment="目标 ID"),
        sa.Column("summary", sa.String(500), nullable=True, comment="事件摘要"),
        sa.Column("metadata", sa.JSON(), nullable=True, comment="事件元数据"),
        sa.Column(
            "created_at",
            sa.DateTime(),
            server_default=sa.func.now(),
            nullable=False,
            comment="创建时间",
        ),
    )
    op.create_index(
        "ix_activity_events_event_type",
        "activity_events",
        ["event_type"],
        unique=False,
    )
