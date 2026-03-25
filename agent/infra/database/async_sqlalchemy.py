# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：async_sqlalchemy
# @Date   ：2025/8/30 16:00
# @Author ：leemysw
# 2025/8/30 16:00   Create
# =====================================================

import json
from pathlib import Path
from contextlib import asynccontextmanager
from typing import AsyncGenerator

from sqlalchemy.ext.asyncio import async_sessionmaker, AsyncSession, create_async_engine
from sqlalchemy.orm import DeclarativeBase, declared_attr

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

        self._ensure_sqlite_directory(database_url)

        self.engine = create_async_engine(
            database_url,
            # echo=settings.DEBUG if hasattr(settings, 'DEBUG') else False,
            echo=False,
            future=True,
            # JSON 字段写入数据库时保留中文，不转义为 \uXXXX
            json_serializer=lambda obj: json.dumps(obj, ensure_ascii=False),
        )

        self.session_factory = async_sessionmaker(
            self.engine,
            class_=AsyncSession,
            expire_on_commit=False
        )

        logger.info(f"Database initialized: {database_url}")

    @staticmethod
    def _ensure_sqlite_directory(database_url: str) -> None:
        """确保 SQLite 数据文件目录存在。"""
        sqlite_prefix = "sqlite+aiosqlite:///"
        if not database_url.startswith(sqlite_prefix):
            return

        db_path = database_url.replace(sqlite_prefix, "", 1)
        db_dir = Path(db_path).expanduser().resolve().parent
        db_dir.mkdir(parents=True, exist_ok=True)


    async def create_tables(self):
        """创建所有表"""
        from agent.infra.database.models import load_models

        load_models()
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
