import os
import sys
from logging.config import fileConfig
from pathlib import Path

from sqlalchemy import engine_from_config, pool
from sqlalchemy.engine import Connection

from alembic import context

# 添加项目根目录到Python路径
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# 导入当前仓库的 Base 和 ORM 模型
from agent.infra.database.async_sqlalchemy import Base
from agent.infra.database.models import load_models  # noqa: F401


# this is the Alembic Config object, which provides
# access to the values within the .ini file in use.
config = context.config

# Interpret the config file for Python logging.
# This line sets up loggers basically.
if config.config_file_name is not None:
    fileConfig(config.config_file_name)

# add your model's MetaData object here
# for 'autogenerate' support
target_metadata = Base.metadata

# other values from the config, defined by the needs of env.py,
# can be acquired:
# my_important_option = config.get_main_option("my_important_option")
# ... etc.


def _normalize_database_url(database_url: str) -> str:
    """将运行时数据库地址转换为 Alembic 可用的同步地址，并展开 ~。"""
    if database_url.startswith("sqlite+aiosqlite:///"):
        url = database_url.replace("sqlite+aiosqlite:///", "sqlite:///")
    else:
        url = database_url
    # SQLite 不展开 ~，手动展开为绝对路径
    if url.startswith("sqlite:///"):
        db_path = Path(url[len("sqlite:///"):]).expanduser()
        url = f"sqlite:///{db_path}"
    return url


def run_migrations_offline() -> None:
    """Run migrations in 'offline' mode.

    This configures the context with just a URL
    and not an Engine, though an Engine is acceptable
    here as well.  By skipping the Engine creation
    we don't even need a DBAPI to be available.

    Calls to context.execute() here emit the given string to the
    script output.

    """
    url = _normalize_database_url(config.get_main_option("sqlalchemy.url"))
    context.configure(
        url=url,
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
    )

    with context.begin_transaction():
        context.run_migrations()


def do_run_migrations(connection: Connection) -> None:
    context.configure(connection=connection, target_metadata=target_metadata)

    with context.begin_transaction():
        context.run_migrations()
def run_migrations_online() -> None:
    """Run migrations in 'online' mode."""
    # 确保 SQLite 数据库目录存在
    db_url = config.get_main_option("sqlalchemy.url")
    normalized_url = _normalize_database_url(db_url)
    config.set_main_option("sqlalchemy.url", normalized_url)
    if normalized_url.startswith("sqlite:///"):
        db_path = Path(normalized_url.replace("sqlite:///", "")).expanduser()
        db_dir = db_path.parent
        db_dir.mkdir(parents=True, exist_ok=True)

    connectable = engine_from_config(
        config.get_section(config.config_ini_section, {}),
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )

    with connectable.connect() as connection:
        do_run_migrations(connection)
    connectable.dispose()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
