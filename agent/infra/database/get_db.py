# !/usr/bin/env python
# -*- coding: UTF-8 -*-
# =====================================================
# @Project：backend
# @File   ：main_db
# @Date   ：2022/12/18 21:39
# @Author ：leemysw
# @Modify Time      @Author    @Version    @Description
# ------------      -------    --------    ------------
# 2022/12/18 21:39   leemysw      1.0.0         Create

# =====================================================

from functools import lru_cache

from agent.config.config import settings


@lru_cache(maxsize=32)
def get_db(db_type=None, **kwargs):
    if db_type == 'redis':
        from agent.infra.database.get_redis import get_redis_client
        redis_client = get_redis_client(**kwargs)
        return redis_client

    elif db_type == 'aioredis':
        from agent.infra.database.get_redis import get_aioredis_client
        aioredis_client = get_aioredis_client(**kwargs)
        return aioredis_client

    elif db_type == 'duckdb':
        import duckdb
        return duckdb.connect(database=":memory:", read_only=False)


    elif db_type == 'async_sqlite':
        from .async_sqlalchemy import db
        db.init(settings.DATABASE_URL)
        return db


    return None
