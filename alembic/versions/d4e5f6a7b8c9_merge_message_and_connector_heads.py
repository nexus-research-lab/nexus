"""merge message and connector migration heads

Revision ID: d4e5f6a7b8c9
Revises: b6c9d2e4f1a7, c3d4e5f6a7b8
Create Date: 2026-04-02 09:50:00.000000

"""

from typing import Sequence, Union


revision: str = "d4e5f6a7b8c9"
down_revision: Union[str, Sequence[str], None] = (
    "b6c9d2e4f1a7",
    "c3d4e5f6a7b8",
)
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """合并 connector/message 两条迁移分支。"""


def downgrade() -> None:
    """回滚 merge 节点本身无需额外操作。"""
