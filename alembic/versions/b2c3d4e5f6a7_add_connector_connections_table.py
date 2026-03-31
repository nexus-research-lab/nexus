"""add connector_connections table

Revision ID: b2c3d4e5f6a7
Revises: a1b2c3d4e5f6
Create Date: 2026-03-31 12:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'b2c3d4e5f6a7'
down_revision: Union[str, None] = 'a1b2c3d4e5f6'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'connector_connections',
        sa.Column('connector_id', sa.String(length=128), nullable=False),
        sa.Column('state', sa.String(length=32), nullable=False, server_default=sa.text("'disconnected'")),
        sa.Column('credentials', sa.Text(), nullable=False, server_default=sa.text("''")),
        sa.Column('auth_type', sa.String(length=32), nullable=False, server_default=sa.text("'oauth2'")),
        sa.Column('created_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), server_default=sa.func.now(), nullable=False),
        sa.PrimaryKeyConstraint('connector_id'),
    )


def downgrade() -> None:
    op.drop_table('connector_connections')
