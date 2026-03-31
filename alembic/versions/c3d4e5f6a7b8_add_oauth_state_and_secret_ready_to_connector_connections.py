"""add oauth state columns to connector_connections

Revision ID: c3d4e5f6a7b8
Revises: b2c3d4e5f6a7
Create Date: 2026-03-31 15:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'c3d4e5f6a7b8'
down_revision: Union[str, None] = 'b2c3d4e5f6a7'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column(
        'connector_connections',
        sa.Column('oauth_state', sa.String(length=255), nullable=True),
    )
    op.add_column(
        'connector_connections',
        sa.Column('oauth_state_expires_at', sa.DateTime(), nullable=True),
    )


def downgrade() -> None:
    op.drop_column('connector_connections', 'oauth_state_expires_at')
    op.drop_column('connector_connections', 'oauth_state')
