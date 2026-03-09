# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：redis_cache
# @Date   ：2025/11/12 17:58
# @Author ：leemysw
#
# 2025/11/12 17:58   Create
# =====================================================

import json
from functools import lru_cache
from typing import Optional, Union

import redis
from redis.cluster import RedisCluster as RedisCluster

from agent.core.config import settings


class RedisCache:
    """同步Redis缓存封装类"""

    def __init__(
            self,
            redis_client: Union[redis.Redis, RedisCluster],
            expire: Optional[int] = None,
            prefix: Optional[str] = None
    ):
        """
        初始化同步Redis缓存

        Args:
            redis_client: 同步Redis客户端实例(必须)
            expire: 默认过期时间(秒)
            prefix: key前缀
        """
        self._client = redis_client

        self.expire = expire
        self.nx = False
        self.xx = False
        self.prefix = f"{settings.PROJECT_NAME}:algorithm:{prefix}" if prefix else f"{settings.PROJECT_NAME}:algorithm"


    @staticmethod
    def to_text(value, encoding="utf-8"):
        """将值转换为文本"""
        if not value:
            return None
        if isinstance(value, str):
            return value
        if isinstance(value, bytes):
            return value.decode(encoding)
        return str(value)

    def key_name(self, key: str) -> str:
        """生成带前缀的key名称"""
        if not self.prefix:
            return key
        if key.startswith(self.prefix):
            return key
        return f"{self.prefix}:{key}"

    def set(self, name: str, value: str, ex: Optional[int] = None) -> bool:
        """设置缓存"""
        name = self.key_name(name)
        ex = ex or self.expire
        return self._client.set(name, value, ex=ex, nx=self.nx, xx=self.xx)

    def set_json(self, name: str, value: dict, ex: Optional[int] = None) -> bool:
        """设置JSON缓存"""
        name = self.key_name(name)
        json_str = json.dumps(value, ensure_ascii=False)
        return self.set(name, json_str, ex=ex)

    def get(self, name: str) -> Optional[str]:
        """获取缓存"""
        name = self.key_name(name)
        ret = self._client.get(name)
        return self.to_text(ret)

    def get_json(self, name: str) -> Optional[dict]:
        """获取JSON缓存"""
        name = self.key_name(name)
        ret = self.get(name)
        if ret:
            try:
                return json.loads(ret)
            except json.JSONDecodeError:
                return None
        return None

    def hget(self, name: str, key: str) -> Optional[str]:
        """获取Hash字段"""
        name = self.key_name(name)
        ret = self._client.hget(name, key)
        return self.to_text(ret)

    def hset(self, name: str, key: str, value: str) -> int:
        """设置Hash字段"""
        name = self.key_name(name)
        return self._client.hset(name, key, value)

    def delete(self, *names: str) -> int:
        """删除缓存"""
        keys = [self.key_name(name) for name in names]
        return self._client.delete(*keys)

    def exists(self, name: str) -> bool:
        """检查key是否存在"""
        name = self.key_name(name)
        result = self._client.exists(name)
        return result > 0

    def expire_key(self, name: str, seconds: int) -> bool:
        """设置key过期时间"""
        name = self.key_name(name)
        return self._client.expire(name, seconds)

    def ttl(self, name: str) -> int:
        """获取key剩余过期时间"""
        name = self.key_name(name)
        return self._client.ttl(name)


@lru_cache()
def get_cache_instance(prefix: str=None) -> RedisCache:
    """获取缓存管理器实例。
    Args:
        prefix: 缓存key前缀

    Returns:
        缓存管理器实例
    """
    from agent.shared.database.get_redis import get_redis_client
    redis_client = get_redis_client()
    return RedisCache(redis_client, prefix=prefix)