"""add scheduled task source fields

Revision ID: 1c2d3e4f5a6b
Revises: 514af30e4585
Create Date: 2026-04-14 10:40:00.000000

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = "1c2d3e4f5a6b"
down_revision: Union[str, Sequence[str], None] = "514af30e4585"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """为定时任务补来源快照字段。"""
    op.add_column(
        "automation_cron_jobs",
        sa.Column(
            "source_kind",
            sa.String(length=32),
            nullable=False,
            server_default=sa.text("'system'"),
        ),
    )
    op.add_column("automation_cron_jobs", sa.Column("source_creator_agent_id", sa.String(length=64)))
    op.add_column("automation_cron_jobs", sa.Column("source_context_type", sa.String(length=32)))
    op.add_column("automation_cron_jobs", sa.Column("source_context_id", sa.String(length=255)))
    op.add_column("automation_cron_jobs", sa.Column("source_context_label", sa.String(length=255)))
    op.add_column("automation_cron_jobs", sa.Column("source_session_key", sa.String(length=255)))
    op.add_column("automation_cron_jobs", sa.Column("source_session_label", sa.String(length=255)))
    op.create_check_constraint(
        "ck_automation_cron_jobs_source_kind",
        "automation_cron_jobs",
        "source_kind IN ('user_page', 'agent', 'cli', 'system')",
    )
    op.create_check_constraint(
        "ck_automation_cron_jobs_source_context_type",
        "automation_cron_jobs",
        "source_context_type IS NULL OR source_context_type IN ('agent', 'room')",
    )


def downgrade() -> None:
    """回退定时任务来源快照字段。"""
    op.drop_constraint("ck_automation_cron_jobs_source_context_type", "automation_cron_jobs", type_="check")
    op.drop_constraint("ck_automation_cron_jobs_source_kind", "automation_cron_jobs", type_="check")
    op.drop_column("automation_cron_jobs", "source_session_label")
    op.drop_column("automation_cron_jobs", "source_session_key")
    op.drop_column("automation_cron_jobs", "source_context_label")
    op.drop_column("automation_cron_jobs", "source_context_id")
    op.drop_column("automation_cron_jobs", "source_context_type")
    op.drop_column("automation_cron_jobs", "source_creator_agent_id")
    op.drop_column("automation_cron_jobs", "source_kind")
