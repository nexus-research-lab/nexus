# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：async_sqlalchemy
# @Date   ：2025/8/30 16:00
# @Author ：leemysw
# 2025/8/30 16:00   Create
# =====================================================

import json
from contextlib import asynccontextmanager
from pathlib import Path
from typing import AsyncGenerator

from sqlalchemy import event
from sqlalchemy.exc import OperationalError
from sqlalchemy.ext.asyncio import async_sessionmaker, AsyncSession, create_async_engine
from sqlalchemy.orm import DeclarativeBase, declared_attr
from sqlalchemy.pool import NullPool

from agent.config.config import settings
from agent.utils.logger import logger


class Base(DeclarativeBase):
    """SQLAlchemy基础模型类"""

    @declared_attr.directive
    def __tablename__(cls) -> str:
        """自动生成表名"""
        return cls.__name__.lower()


class AsyncDatabase:
    """异步数据库管理类"""

    def __init__(self):
        self.engine = None
        self.session_factory = None

    def init(self, database_url: str = None):
        """初始化数据库连接"""
        if database_url is None:
            # 默认使用SQLite，可以配置为其他数据库
            database_url = settings.DATABASE_URL

        # SQLite 不展开 ~，需要手动解析为绝对路径
        if ":///" in database_url:
            scheme = database_url.split(":///")[0]
            db_path = str(Path(database_url[len(scheme) + len(":///"):]).expanduser())
            database_url = f"{scheme}:///{db_path}"

        connect_args = None
        engine_kwargs = {}
        if self._is_sqlite_url(database_url):
            # 中文注释：SQLite 在并发写入时容易触发 locked，必须开启超时等待。
            connect_args = {"timeout": 30}
            # 中文注释：本地 SQLite 不应长期复用连接池，避免瞬时 I/O 故障把坏连接放大到整个进程。
            engine_kwargs["poolclass"] = NullPool

        self.engine = create_async_engine(
            database_url,
            # echo=settings.DEBUG if hasattr(settings, 'DEBUG') else False,
            echo=False,
            future=True,
            # JSON 字段写入数据库时保留中文，不转义为 \uXXXX
            json_serializer=lambda obj: json.dumps(obj, ensure_ascii=False),
            connect_args=connect_args,
            **engine_kwargs,
        )

        if self._is_sqlite_url(database_url):
            # SQLite 默认不会启用外键约束，必须在每个连接上显式打开。
            @event.listens_for(self.engine.sync_engine, "connect")
            def _set_sqlite_foreign_keys(dbapi_connection, _connection_record):
                cursor = dbapi_connection.cursor()
                cursor.execute("PRAGMA foreign_keys=ON")
                # 中文注释：启用 WAL + busy_timeout，减少并发写导致的 database is locked。
                cursor.execute("PRAGMA journal_mode=WAL")
                cursor.execute("PRAGMA synchronous=NORMAL")
                cursor.execute("PRAGMA busy_timeout=5000")
                cursor.close()

        self.session_factory = async_sessionmaker(
            self.engine,
            class_=AsyncSession,
            expire_on_commit=False
        )

        logger.info(f"Database initialized: {database_url}")

    @staticmethod
    def _is_sqlite_url(database_url: str) -> bool:
        """判断当前数据库是否为 SQLite。"""
        return database_url.startswith("sqlite")

    @staticmethod
    def _is_sqlite_disk_io_error(error: Exception) -> bool:
        """判断是否为 SQLite disk I/O 故障。"""
        return "disk i/o error" in str(error).lower()

    async def _recover_from_sqlite_disk_io_error(self) -> None:
        """遇到 SQLite I/O 故障时主动回收引擎，避免继续复用坏连接。"""
        if self.engine is None:
            return
        logger.warning("⚠️ 检测到 SQLite disk I/O error，主动回收数据库引擎连接")
        await self.engine.dispose()


    async def create_tables(self):
        """创建所有表"""
        async with self.engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)

    async def drop_tables(self):
        """删除所有表"""
        async with self.engine.begin() as conn:
            await conn.run_sync(Base.metadata.drop_all)

    @asynccontextmanager
    async def session(self) -> AsyncGenerator[AsyncSession, None]:
        """获取数据库会话"""
        if self.session_factory is None:
            raise RuntimeError("Database not initialized. Call init() first.")

        async with self.session_factory() as session:
            try:
                yield session
            except OperationalError as exc:
                await session.rollback()
                if self._is_sqlite_disk_io_error(exc):
                    await self._recover_from_sqlite_disk_io_error()
                raise
            except Exception:
                await session.rollback()
                raise
            finally:
                await session.close()

    async def close(self):
        """关闭数据库连接"""
        if self.engine:
            await self.engine.dispose()


# 全局数据库实例
db = AsyncDatabase()


# 便捷函数
def get_db_session():
    """获取数据库会话的依赖函数"""
    return db.session()


async def close_database():
    """关闭数据库连接"""
    await db.close()
