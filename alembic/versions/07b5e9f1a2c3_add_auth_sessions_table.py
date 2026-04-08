# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：07b5e9f1a2c3_add_auth_sessions_table.py
# @Date   ：2026/04/08 12:02
# @Author ：leemysw
# 2026/04/08 12:02   Create
# =====================================================

"""add auth sessions table

Revision ID: 07b5e9f1a2c3
Revises: 0f1e2d3c4b5a
Create Date: 2026-04-08 12:02:00.000000
"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

# revision identifiers, used by Alembic.
revision: str = "07b5e9f1a2c3"
down_revision: Union[str, Sequence[str], None] = "0f1e2d3c4b5a"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """创建浏览器登录会话表。"""
    op.create_table(
        "auth_sessions",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("session_token_hash", sa.String(length=64), nullable=False),
        sa.Column("username", sa.String(length=128), nullable=False),
        sa.Column("expires_at", sa.DateTime(), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "uq_auth_sessions_token_hash",
        "auth_sessions",
        ["session_token_hash"],
        unique=True,
    )
    op.create_index(
        "idx_auth_sessions_expires_at",
        "auth_sessions",
        ["expires_at"],
        unique=False,
    )
    op.create_index(
        "idx_auth_sessions_username",
        "auth_sessions",
        ["username"],
        unique=False,
    )


def downgrade() -> None:
    """删除浏览器登录会话表。"""
    op.drop_index("idx_auth_sessions_username", table_name="auth_sessions")
    op.drop_index("idx_auth_sessions_expires_at", table_name="auth_sessions")
    op.drop_index("uq_auth_sessions_token_hash", table_name="auth_sessions")
    op.drop_table("auth_sessions")
