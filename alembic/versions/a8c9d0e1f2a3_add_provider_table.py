# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：a8c9d0e1f2a3_add_provider_table.py
# @Date   ：2026/04/14 10:14
# @Author ：leemysw
# 2026/04/14 10:14   Create
# =====================================================

"""add provider table

Revision ID: a8c9d0e1f2a3
Revises: 1c2d3e4f5a6b
Create Date: 2026-04-14 10:14:00.000000
"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

# revision identifiers, used by Alembic.
revision: str = "a8c9d0e1f2a3"
down_revision: Union[str, Sequence[str], None] = "1c2d3e4f5a6b"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """把运行时 provider 字段落到新表结构。"""
    with op.batch_alter_table("runtimes") as batch_op:
        batch_op.alter_column(
            "model",
            new_column_name="provider",
            existing_type=sa.String(length=128),
            type_=sa.String(length=32),
            existing_nullable=True,
            nullable=True,
        )

    op.create_table(
        "provider",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("provider", sa.String(length=64), nullable=False),
        sa.Column("display_name", sa.String(length=128), nullable=False),
        sa.Column("auth_token", sa.Text(), nullable=False, server_default=""),
        sa.Column("base_url", sa.Text(), nullable=False, server_default=""),
        sa.Column("model", sa.String(length=255), nullable=False, server_default=""),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("1")),
        sa.Column("is_default", sa.Boolean(), nullable=False, server_default=sa.text("0")),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("provider"),
    )
    op.create_index("ix_provider_provider", "provider", ["provider"], unique=True)


def downgrade() -> None:
    """回退 Provider 表和运行时字段。"""
    op.drop_index("ix_provider_provider", table_name="provider")
    op.drop_table("provider")

    with op.batch_alter_table("runtimes") as batch_op:
        batch_op.alter_column(
            "provider",
            new_column_name="model",
            existing_type=sa.String(length=32),
            type_=sa.String(length=128),
            existing_nullable=True,
            nullable=True,
        )
