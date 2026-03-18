#!/usr/bin/env python
# -*- coding: UTF-8 -*-
# =====================================================
# @Project：algorithm
# @File   ：database_redis
# @Date   ：2020/11/25 19:07
# @Author ：leemysw
# @Modify Time      @Author    @Version    @Description
# ------------      -------    --------    ------------
# 2020/11/25 19:07   leemysw      1.0.0         Create

# =====================================================

from functools import lru_cache
from typing import Optional, Union

import redis
from redis import asyncio as aioredis
from redis.asyncio.cluster import RedisCluster as AioRedisCluster
from redis.cluster import ClusterNode, RedisCluster

from agent.core.config import settings


@lru_cache(maxsize=32)
def get_redis_client(
        host: Optional[str] = None,
        port: Optional[int] = None,
        password: Optional[str] = None,
        db: Optional[int] = None,
        decode_responses: bool = True,
        cluster_enabled: Optional[bool] = None,
        cluster_nodes: Optional[str] = None,
        use_pool: Optional[bool] = None,
        max_connections: Optional[int] = None,
        socket_timeout: Optional[int] = None,
        socket_connect_timeout: Optional[int] = None,
        socket_keepalive: Optional[bool] = None
) -> Union[redis.Redis, RedisCluster]:
    """
    获取同步Redis客户端（单机或集群）

    Args:
        host: Redis主机地址，默认使用settings.REDIS_HOST
        port: Redis端口，默认使用settings.REDIS_PORT
        password: Redis密码，默认使用settings.REDIS_PASSWD
        db: 数据库索引，默认使用settings.REDIS_DB
        decode_responses: 是否自动解码响应为字符串，默认False
        cluster_enabled: 是否启用集群模式，默认使用settings.REDIS_CLUSTER_ENABLED
        cluster_nodes: 集群节点，格式"host1:port1,host2:port2"，默认使用settings.REDIS_CLUSTER_NODES
        use_pool: 是否使用连接池，默认使用settings.REDIS_POOL
        max_connections: 连接池最大连接数，默认使用settings.REDIS_MAX_CONNECTIONS
        socket_timeout: Socket超时时间(秒)，默认使用settings.REDIS_SOCKET_TIMEOUT
        socket_connect_timeout: 连接超时时间(秒)，默认使用settings.REDIS_SOCKET_CONNECT_TIMEOUT
        socket_keepalive: 是否保持连接，默认使用settings.REDIS_SOCKET_KEEPALIVE

    Returns:
        同步Redis客户端实例（redis.Redis或RedisCluster）
    """

    # 使用传入参数或默认配置
    host = host or settings.REDIS_HOST  # noqa
    port = port or settings.REDIS_PORT
    password = password or settings.REDIS_PASSWD
    db = db if db is not None else settings.REDIS_DB
    cluster_enabled = cluster_enabled if cluster_enabled is not None else settings.REDIS_CLUSTER_ENABLED
    cluster_nodes = cluster_nodes or settings.REDIS_CLUSTER_NODES
    use_pool = use_pool if use_pool is not None else settings.REDIS_POOL
    max_connections = max_connections or settings.REDIS_MAX_CONNECTIONS
    socket_timeout = socket_timeout or settings.REDIS_SOCKET_TIMEOUT
    socket_connect_timeout = socket_connect_timeout or settings.REDIS_SOCKET_CONNECT_TIMEOUT
    socket_keepalive = socket_keepalive if socket_keepalive is not None else settings.REDIS_SOCKET_KEEPALIVE

    # 集群模式
    if cluster_enabled and cluster_nodes:
        # 解析集群节点
        nodes = []
        for node in cluster_nodes.split(','):
            node_host, node_port = node.strip().split(':')
            nodes.append(ClusterNode(host=node_host, port=int(node_port)))

        client = RedisCluster(
            startup_nodes=nodes,
            password=password if password else None,
            decode_responses=decode_responses,
            max_connections=max_connections,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout
        )

        return client

    # 单机模式
    if use_pool:  # noqa
        # 连接池模式

        pool = redis.ConnectionPool(
            host=host,
            port=port,
            password=password if password else None,
            db=db,
            encoding="utf-8",
            decode_responses=decode_responses,
            max_connections=max_connections,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout,
            socket_keepalive=socket_keepalive
        )
        client = redis.Redis.from_pool(connection_pool=pool)
    else:
        # 直接连接
        client = redis.Redis(
            host=host,
            port=port,
            password=password if password else None,
            db=db,
            encoding="utf-8",
            decode_responses=decode_responses,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout
        )

    return client


@lru_cache(maxsize=32)
def get_aioredis_client(
        host: Optional[str] = None,
        port: Optional[int] = None,
        password: Optional[str] = None,
        db: Optional[int] = None,
        decode_responses: bool = True,
        cluster_enabled: Optional[bool] = None,
        cluster_nodes: Optional[str] = None,
        use_pool: Optional[bool] = None,
        max_connections: Optional[int] = None,
        socket_timeout: Optional[int] = None,
        socket_connect_timeout: Optional[int] = None,
        socket_keepalive: Optional[bool] = None
) -> Union[aioredis.Redis, AioRedisCluster]:
    """
    获取Redis客户端（单机或集群）

    Args:
        host: Redis主机地址，默认使用settings.REDIS_HOST
        port: Redis端口，默认使用settings.REDIS_PORT
        password: Redis密码，默认使用settings.REDIS_PASSWD
        db: 数据库索引，默认使用settings.REDIS_DB
        decode_responses: 是否自动解码响应为字符串，默认False
        cluster_enabled: 是否启用集群模式，默认使用settings.REDIS_CLUSTER_ENABLED
        cluster_nodes: 集群节点，格式"host1:port1,host2:port2"，默认使用settings.REDIS_CLUSTER_NODES
        use_pool: 是否使用连接池，默认使用settings.REDIS_POOL
        max_connections: 连接池最大连接数，默认使用settings.REDIS_MAX_CONNECTIONS
        socket_timeout: Socket超时时间(秒)，默认使用settings.REDIS_SOCKET_TIMEOUT
        socket_connect_timeout: 连接超时时间(秒)，默认使用settings.REDIS_SOCKET_CONNECT_TIMEOUT
        socket_keepalive: 是否保持连接，默认使用settings.REDIS_SOCKET_KEEPALIVE

    Returns:
        Redis客户端实例（aioredis.Redis或RedisCluster）
    """

    # 使用传入参数或默认配置
    host = host or settings.REDIS_HOST
    port = port or settings.REDIS_PORT
    password = password or settings.REDIS_PASSWD
    db = db if db is not None else settings.REDIS_DB
    cluster_enabled = cluster_enabled if cluster_enabled is not None else settings.REDIS_CLUSTER_ENABLED
    cluster_nodes = cluster_nodes or settings.REDIS_CLUSTER_NODES
    use_pool = use_pool if use_pool is not None else settings.REDIS_POOL
    max_connections = max_connections or settings.REDIS_MAX_CONNECTIONS
    socket_timeout = socket_timeout or settings.REDIS_SOCKET_TIMEOUT
    socket_connect_timeout = socket_connect_timeout or settings.REDIS_SOCKET_CONNECT_TIMEOUT
    socket_keepalive = socket_keepalive if socket_keepalive is not None else settings.REDIS_SOCKET_KEEPALIVE

    # 集群模式
    if cluster_enabled and cluster_nodes:
        # 解析集群节点
        nodes = []
        for node in cluster_nodes.split(','):
            node_host, node_port = node.strip().split(':')
            nodes.append({"host": node_host, "port": int(node_port)})

        aioredis_client = AioRedisCluster(
            startup_nodes=nodes,
            password=password if password else None,
            decode_responses=decode_responses,
            max_connections=max_connections,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout
        )
        return aioredis_client

    if use_pool:  # noqa
        # 连接池模式
        pool = aioredis.ConnectionPool(
            host=host,
            port=port,
            password=password if password else None,
            db=db,
            encoding="utf-8",
            decode_responses=decode_responses,
            max_connections=max_connections,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout,
            socket_keepalive=socket_keepalive
        )
        client = aioredis.Redis.from_pool(connection_pool=pool)
    else:
        # 直接连接
        client = aioredis.Redis(
            host=host,
            port=port,
            password=password if password else None,
            db=db,
            encoding="utf-8",
            decode_responses=decode_responses,
            socket_timeout=socket_timeout,
            socket_connect_timeout=socket_connect_timeout
        )

    return client
