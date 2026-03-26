"""add room runtime fields

Revision ID: c31e8f6d4a20
Revises: a4d9f0f7e2b1
Create Date: 2026-03-26 16:30:00.000000
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = "c31e8f6d4a20"
down_revision: Union[str, None] = "a4d9f0f7e2b1"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column("rooms", sa.Column("mode", sa.String(length=32), nullable=False, server_default="open"))
    op.add_column("rooms", sa.Column("runtime_status", sa.String(length=32), nullable=False, server_default="created"))
    op.add_column("rooms", sa.Column("active_run_id", sa.String(length=64), nullable=True))
    op.add_column("rooms", sa.Column("orchestrator_agent_id", sa.String(length=64), nullable=False, server_default="main"))
    op.add_column("rooms", sa.Column("ruleset_slug", sa.String(length=128), nullable=True))
    op.add_column("rooms", sa.Column("goal", sa.Text(), nullable=False, server_default=""))
    op.add_column("rooms", sa.Column("runtime_state", sa.JSON(), nullable=False, server_default=sa.text("'{}'")))
    op.add_column("rooms", sa.Column("capabilities", sa.JSON(), nullable=False, server_default=sa.text("'{}'")))
    op.create_check_constraint("ck_rooms_mode", "rooms", "mode IN ('open', 'protocol')")
    op.create_check_constraint(
        "ck_rooms_runtime_status",
        "rooms",
        "runtime_status IN ('created', 'running', 'paused', 'finished', 'error')",
    )

    op.add_column("members", sa.Column("member_source", sa.String(length=32), nullable=False, server_default="existing"))
    op.add_column("members", sa.Column("member_role", sa.String(length=128), nullable=True))
    op.add_column("members", sa.Column("member_status", sa.String(length=32), nullable=False, server_default="listening"))
    op.add_column("members", sa.Column("member_visibility_scope", sa.JSON(), nullable=False, server_default=sa.text("'[]'")))
    op.add_column("members", sa.Column("workspace_binding", sa.Boolean(), nullable=False, server_default=sa.false()))
    op.create_check_constraint("ck_members_source", "members", "member_source IN ('existing', 'ephemeral')")
    op.create_check_constraint(
        "ck_members_status",
        "members",
        "member_status IN ('listening', 'speaking', 'thinking', 'working', 'waiting', 'blocked', 'done')",
    )


def downgrade() -> None:
    op.drop_constraint("ck_members_status", "members", type_="check")
    op.drop_constraint("ck_members_source", "members", type_="check")
    op.drop_column("members", "workspace_binding")
    op.drop_column("members", "member_visibility_scope")
    op.drop_column("members", "member_status")
    op.drop_column("members", "member_role")
    op.drop_column("members", "member_source")

    op.drop_constraint("ck_rooms_runtime_status", "rooms", type_="check")
    op.drop_constraint("ck_rooms_mode", "rooms", type_="check")
    op.drop_column("rooms", "capabilities")
    op.drop_column("rooms", "runtime_state")
    op.drop_column("rooms", "goal")
    op.drop_column("rooms", "ruleset_slug")
    op.drop_column("rooms", "orchestrator_agent_id")
    op.drop_column("rooms", "active_run_id")
    op.drop_column("rooms", "runtime_status")
    op.drop_column("rooms", "mode")
