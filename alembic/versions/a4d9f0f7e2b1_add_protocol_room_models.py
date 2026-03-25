"""add protocol room models

Revision ID: a4d9f0f7e2b1
Revises: 8f7c592bdd12
Create Date: 2026-03-25 21:10:00.000000
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = "a4d9f0f7e2b1"
down_revision: Union[str, None] = "8f7c592bdd12"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "protocol_definitions",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("slug", sa.String(length=128), nullable=False),
        sa.Column("name", sa.String(length=128), nullable=False),
        sa.Column("description", sa.Text(), nullable=False, server_default=""),
        sa.Column("version", sa.Integer(), nullable=False, server_default="1"),
        sa.Column("coordinator_mode", sa.String(length=64), nullable=False, server_default="main_agent"),
        sa.Column("phases", sa.JSON(), nullable=False, server_default=sa.text("'[]'")),
        sa.Column("channel_policy", sa.JSON(), nullable=False, server_default=sa.text("'[]'")),
        sa.Column("turn_policy", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("action_schemas", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("visibility_resolver", sa.String(length=64), nullable=False, server_default="default"),
        sa.Column("completion_rule", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "uq_protocol_definitions_slug_version",
        "protocol_definitions",
        ["slug", "version"],
        unique=True,
    )

    op.create_table(
        "protocol_runs",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("room_id", sa.String(length=64), nullable=False),
        sa.Column("protocol_definition_id", sa.String(length=64), nullable=False),
        sa.Column("title", sa.String(length=255), nullable=True),
        sa.Column("status", sa.String(length=32), nullable=False, server_default="running"),
        sa.Column("current_phase", sa.String(length=64), nullable=False, server_default="setup"),
        sa.Column("phase_index", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("current_turn_key", sa.String(length=128), nullable=True),
        sa.Column("coordinator_agent_id", sa.String(length=64), nullable=False),
        sa.Column("run_config", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("state", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("completed_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "status IN ('running', 'paused', 'completed', 'terminated')",
            name="ck_protocol_runs_status",
        ),
        sa.ForeignKeyConstraint(["room_id"], ["rooms.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(
            ["protocol_definition_id"],
            ["protocol_definitions.id"],
            ondelete="CASCADE",
        ),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index("idx_protocol_runs_room", "protocol_runs", ["room_id", "created_at"], unique=False)
    op.create_index(
        "idx_protocol_runs_definition",
        "protocol_runs",
        ["protocol_definition_id", "created_at"],
        unique=False,
    )

    op.create_table(
        "channels",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("room_id", sa.String(length=64), nullable=False),
        sa.Column("protocol_run_id", sa.String(length=64), nullable=False),
        sa.Column("slug", sa.String(length=128), nullable=False),
        sa.Column("name", sa.String(length=128), nullable=False),
        sa.Column("channel_type", sa.String(length=32), nullable=False),
        sa.Column("visibility", sa.String(length=32), nullable=False, server_default="public"),
        sa.Column("topic", sa.Text(), nullable=False, server_default=""),
        sa.Column("position", sa.Integer(), nullable=False, server_default="0"),
        sa.Column("metadata", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "channel_type IN ('public', 'scoped', 'direct', 'system')",
            name="ck_channels_type",
        ),
        sa.ForeignKeyConstraint(["room_id"], ["rooms.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(["protocol_run_id"], ["protocol_runs.id"], ondelete="CASCADE"),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index("uq_channels_slug", "channels", ["protocol_run_id", "slug"], unique=True)
    op.create_index(
        "idx_channels_run_position",
        "channels",
        ["protocol_run_id", "position"],
        unique=False,
    )

    op.create_table(
        "channel_members",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("channel_id", sa.String(length=64), nullable=False),
        sa.Column("member_type", sa.String(length=32), nullable=False),
        sa.Column("member_user_id", sa.String(length=64), nullable=True),
        sa.Column("member_agent_id", sa.String(length=64), nullable=True),
        sa.Column("role_label", sa.String(length=64), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "("
            "member_type = 'agent' AND member_agent_id IS NOT NULL AND member_user_id IS NULL"
            ") OR ("
            "member_type = 'user' AND member_user_id IS NOT NULL AND member_agent_id IS NULL"
            ")",
            name="ck_channel_members_target",
        ),
        sa.ForeignKeyConstraint(["channel_id"], ["channels.id"], ondelete="CASCADE"),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "uq_channel_members_agent",
        "channel_members",
        ["channel_id", "member_agent_id"],
        unique=True,
    )
    op.create_index(
        "uq_channel_members_user",
        "channel_members",
        ["channel_id", "member_user_id"],
        unique=True,
    )

    op.create_table(
        "action_requests",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("protocol_run_id", sa.String(length=64), nullable=False),
        sa.Column("channel_id", sa.String(length=64), nullable=True),
        sa.Column("phase_name", sa.String(length=64), nullable=False),
        sa.Column("turn_key", sa.String(length=128), nullable=True),
        sa.Column("action_type", sa.String(length=64), nullable=False),
        sa.Column("status", sa.String(length=32), nullable=False, server_default="pending"),
        sa.Column("requested_by_agent_id", sa.String(length=64), nullable=True),
        sa.Column("allowed_actor_agent_ids", sa.JSON(), nullable=False, server_default=sa.text("'[]'")),
        sa.Column("audience_agent_ids", sa.JSON(), nullable=False, server_default=sa.text("'[]'")),
        sa.Column("input_schema", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("target_scope", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("prompt_text", sa.Text(), nullable=True),
        sa.Column("metadata", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("resolved_at", sa.DateTime(), nullable=True),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "status IN ('pending', 'resolved', 'cancelled')",
            name="ck_action_requests_status",
        ),
        sa.ForeignKeyConstraint(["protocol_run_id"], ["protocol_runs.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(["channel_id"], ["channels.id"], ondelete="SET NULL"),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "idx_action_requests_run_phase",
        "action_requests",
        ["protocol_run_id", "phase_name", "created_at"],
        unique=False,
    )

    op.create_table(
        "action_submissions",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("request_id", sa.String(length=64), nullable=False),
        sa.Column("protocol_run_id", sa.String(length=64), nullable=False),
        sa.Column("channel_id", sa.String(length=64), nullable=True),
        sa.Column("actor_type", sa.String(length=32), nullable=False, server_default="agent"),
        sa.Column("actor_agent_id", sa.String(length=64), nullable=True),
        sa.Column("actor_user_id", sa.String(length=64), nullable=True),
        sa.Column("action_type", sa.String(length=64), nullable=False),
        sa.Column("payload", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("status", sa.String(length=32), nullable=False, server_default="submitted"),
        sa.Column("metadata", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "actor_type IN ('agent', 'user', 'system')",
            name="ck_action_submissions_actor_type",
        ),
        sa.CheckConstraint(
            "status IN ('submitted', 'overridden', 'rejected', 'accepted')",
            name="ck_action_submissions_status",
        ),
        sa.ForeignKeyConstraint(["request_id"], ["action_requests.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(["protocol_run_id"], ["protocol_runs.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(["channel_id"], ["channels.id"], ondelete="SET NULL"),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "idx_action_submissions_request",
        "action_submissions",
        ["request_id", "created_at"],
        unique=False,
    )
    op.create_index(
        "idx_action_submissions_run",
        "action_submissions",
        ["protocol_run_id", "created_at"],
        unique=False,
    )

    op.create_table(
        "run_state_snapshots",
        sa.Column("id", sa.String(length=64), nullable=False),
        sa.Column("protocol_run_id", sa.String(length=64), nullable=False),
        sa.Column("event_seq", sa.Integer(), nullable=False),
        sa.Column("phase_name", sa.String(length=64), nullable=False),
        sa.Column("event_type", sa.String(length=64), nullable=False),
        sa.Column("channel_id", sa.String(length=64), nullable=True),
        sa.Column("actor_agent_id", sa.String(length=64), nullable=True),
        sa.Column("visibility", sa.String(length=32), nullable=False, server_default="public"),
        sa.Column("audience_agent_ids", sa.JSON(), nullable=False, server_default=sa.text("'[]'")),
        sa.Column("headline", sa.String(length=255), nullable=True),
        sa.Column("body", sa.Text(), nullable=True),
        sa.Column("state", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("metadata", sa.JSON(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("created_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column("updated_at", sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.CheckConstraint(
            "visibility IN ('public', 'scoped', 'direct', 'system')",
            name="ck_run_state_snapshots_visibility",
        ),
        sa.ForeignKeyConstraint(["protocol_run_id"], ["protocol_runs.id"], ondelete="CASCADE"),
        sa.ForeignKeyConstraint(["channel_id"], ["channels.id"], ondelete="SET NULL"),
        sa.PrimaryKeyConstraint("id"),
    )
    op.create_index(
        "uq_run_state_snapshots_seq",
        "run_state_snapshots",
        ["protocol_run_id", "event_seq"],
        unique=True,
    )
    op.create_index(
        "idx_run_state_snapshots_run",
        "run_state_snapshots",
        ["protocol_run_id", "created_at"],
        unique=False,
    )


def downgrade() -> None:
    op.drop_index("idx_run_state_snapshots_run", table_name="run_state_snapshots")
    op.drop_index("uq_run_state_snapshots_seq", table_name="run_state_snapshots")
    op.drop_table("run_state_snapshots")

    op.drop_index("idx_action_submissions_run", table_name="action_submissions")
    op.drop_index("idx_action_submissions_request", table_name="action_submissions")
    op.drop_table("action_submissions")

    op.drop_index("idx_action_requests_run_phase", table_name="action_requests")
    op.drop_table("action_requests")

    op.drop_index("uq_channel_members_user", table_name="channel_members")
    op.drop_index("uq_channel_members_agent", table_name="channel_members")
    op.drop_table("channel_members")

    op.drop_index("idx_channels_run_position", table_name="channels")
    op.drop_index("uq_channels_slug", table_name="channels")
    op.drop_table("channels")

    op.drop_index("idx_protocol_runs_definition", table_name="protocol_runs")
    op.drop_index("idx_protocol_runs_room", table_name="protocol_runs")
    op.drop_table("protocol_runs")

    op.drop_index("uq_protocol_definitions_slug_version", table_name="protocol_definitions")
    op.drop_table("protocol_definitions")
