# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：0f1e2d3c4b5a_drop_skill_tables.py
# @Date   ：2026/04/02 12:05
# @Author ：leemysw
# 2026/04/02 12:05   Create
# =====================================================

"""drop skill tables

Revision ID: 0f1e2d3c4b5a
Revises: f6a7b8c9d0e1
Create Date: 2026-04-02 12:05:00.000000
"""

from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

# revision identifiers, used by Alembic.
revision: str = "0f1e2d3c4b5a"
down_revision: Union[str, Sequence[str], None] = "f6a7b8c9d0e1"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """移除 skill 专用表。"""
    bind = op.get_bind()
    inspector = sa.inspect(bind)
    table_names = set(inspector.get_table_names())

    if "agent_skills" in table_names:
        op.drop_table("agent_skills")
    if "pool_skills" in table_names:
        op.drop_table("pool_skills")


def downgrade() -> None:
    """恢复旧的 skill 专用表。"""
    op.create_table(
        "pool_skills",
        sa.Column("name", sa.String(length=256), nullable=False),
        sa.Column("installed", sa.Boolean(), nullable=False, server_default=sa.text("0")),
        sa.Column("global_enabled", sa.Boolean(), nullable=False, server_default=sa.text("1")),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint("name"),
    )
    op.create_table(
        "agent_skills",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("agent_id", sa.String(length=64), nullable=False),
        sa.Column("skill_name", sa.String(length=256), nullable=False),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint("id"),
        sa.ForeignKeyConstraint(["agent_id"], ["agents.id"], ondelete="CASCADE"),
        sa.UniqueConstraint("agent_id", "skill_name", name="uq_agent_skill"),
    )
    op.create_index("ix_agent_skills_agent_id", "agent_skills", ["agent_id"])
    op.create_index("ix_agent_skills_skill_name", "agent_skills", ["skill_name"])
