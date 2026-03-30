"""add skill pool and agent skill tables

Revision ID: a1b2c3d4e5f6
Revises: a3b7e1c4d5f6
Create Date: 2026-03-31 00:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'a1b2c3d4e5f6'
down_revision: Union[str, None] = 'a3b7e1c4d5f6'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'pool_skills',
        sa.Column('name', sa.String(length=256), nullable=False),
        sa.Column('installed', sa.Boolean(), nullable=False, server_default=sa.text('0')),
        sa.Column('global_enabled', sa.Boolean(), nullable=False, server_default=sa.text('1')),
        sa.Column('created_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint('name'),
    )

    op.create_table(
        'agent_skills',
        sa.Column('id', sa.String(length=64), nullable=False),
        sa.Column('agent_id', sa.String(length=64), nullable=False),
        sa.Column('skill_name', sa.String(length=256), nullable=False),
        sa.Column('created_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint('id'),
        sa.ForeignKeyConstraint(['agent_id'], ['agents.id'], ondelete='CASCADE'),
        sa.UniqueConstraint('agent_id', 'skill_name', name='uq_agent_skill'),
    )
    op.create_index('ix_agent_skills_agent_id', 'agent_skills', ['agent_id'])
    op.create_index('ix_agent_skills_skill_name', 'agent_skills', ['skill_name'])


def downgrade() -> None:
    op.drop_index('ix_agent_skills_skill_name', table_name='agent_skills')
    op.drop_index('ix_agent_skills_agent_id', table_name='agent_skills')
    op.drop_table('agent_skills')
    op.drop_table('pool_skills')
