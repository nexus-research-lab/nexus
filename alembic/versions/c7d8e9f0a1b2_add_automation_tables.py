"""add automation tables

Revision ID: c7d8e9f0a1b2
Revises: 07b5e9f1a2c3
Create Date: 2026-04-09 00:00:00.000000

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "c7d8e9f0a1b2"
down_revision: Union[str, Sequence[str], None] = "07b5e9f1a2c3"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """创建 automation 基础表。"""
    op.create_table(
        "automation_cron_jobs",
        sa.Column("job_id", sa.String(length=64), nullable=False),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column(
            "agent_id",
            sa.String(length=64),
            sa.ForeignKey("agents.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("schedule_kind", sa.String(length=32), nullable=False),
        sa.Column("run_at", sa.String(length=32)),
        sa.Column("interval_seconds", sa.Integer()),
        sa.Column("cron_expression", sa.String(length=255)),
        sa.Column(
            "timezone",
            sa.String(length=64),
            nullable=False,
            server_default=sa.text("'Asia/Shanghai'"),
        ),
        sa.Column("instruction", sa.Text(), nullable=False),
        sa.Column(
            "session_target_kind",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'isolated'"),
        ),
        sa.Column("bound_session_key", sa.String(length=255)),
        sa.Column("named_session_key", sa.String(length=255)),
        sa.Column(
            "wake_mode",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'next-heartbeat'"),
        ),
        sa.Column(
            "delivery_mode",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'none'"),
        ),
        sa.Column("delivery_channel", sa.String(length=64)),
        sa.Column("delivery_to", sa.String(length=255)),
        sa.Column("delivery_account_id", sa.String(length=64)),
        sa.Column("delivery_thread_id", sa.String(length=255)),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("1")),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.CheckConstraint(
            "schedule_kind IN ('every', 'cron', 'at')",
            name="ck_automation_cron_jobs_schedule_kind",
        ),
        sa.CheckConstraint(
            "session_target_kind IN ('isolated', 'main', 'bound', 'named')",
            name="ck_automation_cron_jobs_session_target_kind",
        ),
        sa.CheckConstraint(
            "wake_mode IN ('now', 'next-heartbeat')",
            name="ck_automation_cron_jobs_wake_mode",
        ),
        sa.CheckConstraint(
            "delivery_mode IN ('none', 'last', 'explicit')",
            name="ck_automation_cron_jobs_delivery_mode",
        ),
        sa.PrimaryKeyConstraint("job_id"),
    )
    op.create_index("idx_automation_cron_jobs_agent", "automation_cron_jobs", ["agent_id"])

    op.create_table(
        "automation_cron_runs",
        sa.Column("run_id", sa.String(length=64), nullable=False),
        sa.Column(
            "job_id",
            sa.String(length=64),
            sa.ForeignKey("automation_cron_jobs.job_id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "status",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'pending'"),
        ),
        sa.Column("scheduled_for", sa.DateTime()),
        sa.Column("started_at", sa.DateTime()),
        sa.Column("finished_at", sa.DateTime()),
        sa.Column("attempts", sa.Integer(), nullable=False, server_default=sa.text("0")),
        sa.Column("error_message", sa.Text()),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.CheckConstraint(
            "status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')",
            name="ck_automation_cron_runs_status",
        ),
        sa.PrimaryKeyConstraint("run_id"),
    )
    op.create_index("idx_automation_cron_runs_job", "automation_cron_runs", ["job_id"])
    op.create_index("idx_automation_cron_runs_status", "automation_cron_runs", ["status"])

    op.create_table(
        "automation_heartbeat_states",
        sa.Column("state_id", sa.String(length=64), nullable=False),
        sa.Column(
            "agent_id",
            sa.String(length=64),
            sa.ForeignKey("agents.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("0")),
        sa.Column("every_seconds", sa.Integer(), nullable=False, server_default=sa.text("1800")),
        sa.Column(
            "target_mode",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'none'"),
        ),
        sa.Column("ack_max_chars", sa.Integer(), nullable=False, server_default=sa.text("300")),
        sa.Column("last_heartbeat_at", sa.DateTime()),
        sa.Column("last_ack_at", sa.DateTime()),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.CheckConstraint(
            "target_mode IN ('none', 'last', 'explicit')",
            name="ck_automation_heartbeat_states_target_mode",
        ),
        sa.PrimaryKeyConstraint("state_id"),
        sa.UniqueConstraint("agent_id", name="uq_automation_heartbeat_states_agent"),
    )

    op.create_table(
        "automation_delivery_routes",
        sa.Column("route_id", sa.String(length=64), nullable=False),
        sa.Column(
            "agent_id",
            sa.String(length=64),
            sa.ForeignKey("agents.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "mode",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'none'"),
        ),
        sa.Column("channel", sa.String(length=64)),
        sa.Column("to", sa.String(length=255)),
        sa.Column("account_id", sa.String(length=64)),
        sa.Column("thread_id", sa.String(length=255)),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("1")),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.CheckConstraint(
            "mode IN ('none', 'last', 'explicit')",
            name="ck_automation_delivery_routes_mode",
        ),
        sa.PrimaryKeyConstraint("route_id"),
    )
    op.create_index(
        "idx_automation_delivery_routes_agent",
        "automation_delivery_routes",
        ["agent_id"],
    )

    op.create_table(
        "automation_system_events",
        sa.Column("event_id", sa.String(length=64), nullable=False),
        sa.Column("event_type", sa.String(length=64), nullable=False),
        sa.Column("source_type", sa.String(length=64)),
        sa.Column("source_id", sa.String(length=64)),
        sa.Column("payload", sa.JSON(), nullable=False),
        sa.Column(
            "status",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'new'"),
        ),
        sa.Column("processed_at", sa.DateTime()),
        sa.Column("created_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.CheckConstraint(
            "status IN ('new', 'processing', 'processed', 'failed')",
            name="ck_automation_system_events_status",
        ),
        sa.PrimaryKeyConstraint("event_id"),
    )
    op.create_index("idx_automation_system_events_type", "automation_system_events", ["event_type"])
    op.create_index("idx_automation_system_events_status", "automation_system_events", ["status"])
    op.create_index("idx_automation_system_events_created", "automation_system_events", ["created_at"])


def downgrade() -> None:
    """删除 automation 基础表。"""
    op.drop_index("idx_automation_system_events_created", table_name="automation_system_events")
    op.drop_index("idx_automation_system_events_status", table_name="automation_system_events")
    op.drop_index("idx_automation_system_events_type", table_name="automation_system_events")
    op.drop_table("automation_system_events")

    op.drop_index("idx_automation_delivery_routes_agent", table_name="automation_delivery_routes")
    op.drop_table("automation_delivery_routes")

    op.drop_table("automation_heartbeat_states")

    op.drop_index("idx_automation_cron_runs_status", table_name="automation_cron_runs")
    op.drop_index("idx_automation_cron_runs_job", table_name="automation_cron_runs")
    op.drop_table("automation_cron_runs")

    op.drop_index("idx_automation_cron_jobs_agent", table_name="automation_cron_jobs")
    op.drop_table("automation_cron_jobs")
