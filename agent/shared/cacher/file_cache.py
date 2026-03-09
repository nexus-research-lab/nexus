# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：cache.py
# @Date   ：2025/4/24 14:58
# @Author ：leemysw

# 2025/4/24 14:58   Create
# =====================================================

import hashlib
import json
import os
import shutil
import time
from datetime import date, datetime, timedelta, timezone
from functools import lru_cache
from typing import Any, Optional

from agent.core.config import settings
from agent.utils.logger import logger
from agent.utils.utils import cache_path


class FileCache:
    def __init__(self, namespace: str = "default", default_ttl_days: int = settings.DEFAULT_CACHE_TTL_DAYS):
        """初始化通用文件缓存管理器。

        Args:
            namespace: 缓存命名空间 (e.g. 'ocr_results', 'user_sessions')
            default_ttl_days: 默认的缓存清理周期（天）
        """
        self.namespace = namespace
        self.enable_cache = settings.ENABLE_CACHE

        # 使用命名空间创建更具体的缓存基础目录
        self.base_cache_dir = cache_path(settings.CACHE_FILE_DIR, f"namespace/{self.namespace}")
        self.cleanup_ttl = timedelta(days=default_ttl_days)
        self._last_cleanup_date: Optional[date] = None  # <-- Track last cleanup date

        # 不再强制按日期分子目录存储，但清理时仍按日期目录清理
        self._ensure_base_dir()
        self._cleanup_old_caches()  # 初始时清理一次旧缓存

    def _ensure_base_dir(self) -> None:
        """确保基础缓存目录存在。"""
        if not os.path.exists(self.base_cache_dir):
            try:
                os.makedirs(self.base_cache_dir, exist_ok=True)
            except OSError as e:
                logger.error(f"【FileCache:{self.namespace}】创建缓存目录失败: {self.base_cache_dir}, Error: {e}")

    def _get_cache_filepath(self, key: str) -> str:
        """根据 key 生成缓存文件路径。"""
        # 使用 key 的哈希值作为文件名，避免特殊字符问题
        key_hash = hashlib.md5(key.encode('utf-8')).hexdigest()
        # 将所有缓存文件直接放在基础目录下，简化查找
        return os.path.join(self.base_cache_dir, f"{key_hash}.json")

    def _check_and_run_cleanup(self) -> None:
        """检查是否需要运行每日清理任务。"""
        if not self.enable_cache:
            return
        today = date.today()
        if self._last_cleanup_date != today:
            logger.info(f"【FileCache:{self.namespace}】触发每日缓存清理任务...")
            self._cleanup_old_caches()
            self._last_cleanup_date = today

    def _cleanup_old_caches(self) -> None:
        """清理过期的缓存文件（基于文件修改时间）。"""
        if not self.enable_cache or not os.path.exists(self.base_cache_dir):
            return

        now_ts = time.time()
        cutoff_ts = now_ts - self.cleanup_ttl.total_seconds()

        cleaned_count = 0
        try:
            for filename in os.listdir(self.base_cache_dir):
                if filename.endswith(".json"):
                    filepath = os.path.join(self.base_cache_dir, filename)
                    try:
                        # 检查文件的最后修改时间
                        file_mtime = os.path.getmtime(filepath)
                        if file_mtime < cutoff_ts:
                            os.remove(filepath)
                            cleaned_count += 1
                    except FileNotFoundError:
                        continue  # 文件可能在迭代过程中已被删除
                    except Exception as e:
                        logger.warning(f"【FileCache:{self.namespace}】清理文件 {filepath} 时出错: {e}")
            if cleaned_count > 0:
                logger.info(
                    f"【FileCache:{self.namespace}】清理了 {cleaned_count} 个超过 {self.cleanup_ttl.days} 天的旧缓存文件。")
        except Exception as e:
            logger.error(f"【FileCache:{self.namespace}】执行缓存清理时出错: {e}")

    @staticmethod
    def generate_key(content: str) -> str:
        """根据参数生成缓存键。

        Args:
            content: 要缓存的值

        Returns:
            缓存键 (字符串)
        """

        return hashlib.md5(content.encode('utf-8')).hexdigest()

    @staticmethod
    def generate_key_from_file(file_path: str) -> str:
        """根据文件路径生成缓存键。

        Args:
            file_path: 文件路径

        Returns:
            缓存键 (字符串)
        """

        if not os.path.exists(file_path):
            raise FileNotFoundError(f"文件不存在: {file_path}")

        md5_hash = hashlib.md5()
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                md5_hash.update(chunk)

        return md5_hash.hexdigest()

    def set(self, key: str, value: Any, ttl: Optional[timedelta] = None) -> None:
        """将键值对存入缓存，并可设置过期时间。

        Args:
            key: 缓存键 (字符串)
            value: 要缓存的值 (任何可 JSON 序列化的对象)
            ttl: 过期时间 (timedelta 对象)。如果为 None，则永不过期（除非被清理任务删除）。
        """
        self._check_and_run_cleanup()  # <-- Add cleanup check

        filepath = self._get_cache_filepath(key)
        self._ensure_base_dir()  # 确保目录存在

        expires_at = None
        if ttl:
            expires_at = (datetime.now(timezone.utc) + ttl).isoformat()

        cache_data = {
            'value': value,
            'expires_at': expires_at,  # 存储 ISO 格式的 UTC 时间字符串
            'created_at': datetime.now(timezone.utc).isoformat()
        }

        try:
            with open(filepath, "w", encoding="utf-8") as f:
                json.dump(cache_data, f, ensure_ascii=False, indent=4)  # indent for readability
            logger.debug(f"【FileCache:{self.namespace}】保存缓存: {key} -> {filepath}")
        except IOError as e:
            logger.error(f"【FileCache:{self.namespace}】写入缓存文件失败: {filepath}, Error: {e}")
        except TypeError as e:
            logger.error(f"【FileCache:{self.namespace}】缓存值无法JSON序列化: key={key}, Error: {e}")

    def get(self, key: str) -> Optional[Any]:
        """根据键获取缓存值。如果缓存不存在或已过期，则返回 None。

        Args:
            key: 缓存键

        Returns:
            缓存的值，如果不存在或过期则返回 None
        """
        self._check_and_run_cleanup()  # <-- Add cleanup check

        if not self.enable_cache:
            return None

        filepath = self._get_cache_filepath(key)

        if not os.path.exists(filepath):
            return None

        try:
            with open(filepath, "r", encoding="utf-8") as f:
                cache_data = json.load(f)

            expires_at_str = cache_data.get('expires_at')
            if expires_at_str:
                expires_at = datetime.fromisoformat(expires_at_str)
                # 确保比较时使用带时区的当前时间
                if datetime.now(timezone.utc) > expires_at:
                    # 缓存已过期，删除文件并返回 None
                    try:
                        os.remove(filepath)
                        logger.debug(f"【FileCache:{self.namespace}】删除过期缓存: {key}")
                    except OSError as e:
                        logger.warning(f"【FileCache:{self.namespace}】删除过期缓存文件失败: {filepath}, Error: {e}")
                    return None

            # logger.debug(f"【FileCache:{self.namespace}】命中缓存: {key}")
            return cache_data.get('value')

        except (json.JSONDecodeError, KeyError, TypeError, ValueError, FileNotFoundError) as e:
            logger.warning(f"【FileCache:{self.namespace}】读取或解析缓存文件失败: {filepath}, Error: {e}. 可能需要清理。")
            # 如果文件有问题，尝试删除它
            try:
                if os.path.exists(filepath):
                    os.remove(filepath)
            except OSError:
                pass  # 忽略删除错误
            return None
        except Exception as e:
            logger.error(f"【FileCache:{self.namespace}】获取缓存时发生未知错误: key={key}, Error: {e}")
            return None

    def delete(self, key: str) -> bool:
        """删除指定的缓存键。

        Args:
            key: 要删除的缓存键

        Returns:
            如果成功删除或键不存在，返回 True；如果删除失败，返回 False。
        """
        # Note: Cleanup is not triggered on delete
        if not self.enable_cache:
            return True  # 缓存未启用，视为删除成功

        filepath = self._get_cache_filepath(key)
        if os.path.exists(filepath):
            try:
                os.remove(filepath)
                logger.debug(f"【FileCache:{self.namespace}】删除缓存: {key}")
                return True
            except OSError as e:
                logger.error(f"【FileCache:{self.namespace}】删除缓存文件失败: {filepath}, Error: {e}")
                return False
        return True  # 文件不存在，也视为删除成功

    def clear_namespace(self) -> bool:
        """清空当前命名空间下的所有缓存。"""
        # Note: Cleanup is not triggered on clear_namespace
        if not self.enable_cache:
            return True

        if os.path.exists(self.base_cache_dir):
            try:
                shutil.rmtree(self.base_cache_dir)
                logger.info(f"【FileCache:{self.namespace}】已清空命名空间缓存目录: {self.base_cache_dir}")
                self._ensure_base_dir()  # 重新创建基础目录
                return True
            except OSError as e:
                logger.error(f"【FileCache:{self.namespace}】清空命名空间缓存失败: {self.base_cache_dir}, Error: {e}")
                return False
        return True  # 目录不存在，视为清空成功


@lru_cache()
def get_cache_instance(namespace: str = "default",
                       default_ttl_days: int = settings.DEFAULT_CACHE_TTL_DAYS) -> FileCache:
    """获取缓存管理器实例。
    Args:
        namespace: 缓存命名空间 (e.g. 'ocr_results', 'user_sessions')
        default_ttl_days: 默认的缓存清理周期（天）
    Returns:
        缓存管理器实例
    """
    return FileCache(namespace=namespace, default_ttl_days=default_ttl_days)

# 示例用法 (可以放在其他地方或测试文件中)
# if __name__ == '__main__':
#     # 假设 settings.ENABLE_CACHE = True 和 settings.CACHE_FILE_DIR 已配置
#     # settings.ENABLE_CACHE = True
#     # settings.CACHE_FILE_DIR = "./.cache" # 示例路径
#
#     my_cache = FileCache(namespace="my_app_data")
#
#     # 设置缓存，有效期 1 小时
#     my_cache.set("user:123:profile", {"name": "Alice", "email": "alice@example.com"}, ttl=timedelta(hours=1))
#
#     # 设置永久缓存 (或直到被清理任务删除)
#     my_cache.set("app:config", {"theme": "dark", "language": "en"})
#
#     # 获取缓存
#     user_profile = my_cache.get("user:123:profile")
#     if user_profile:
#         print("获取到用户配置:", user_profile)
#     else:
#         print("用户配置缓存未找到或已过期")
#
#     app_config = my_cache.get("app:config")
#     print("获取到应用配置:", app_config)
#
#     # 删除缓存
#     my_cache.delete("user:123:profile")
#     print("删除后再次获取用户配置:", my_cache.get("user:123:profile"))
#
#     # 清理旧缓存（通常不需要手动调用，初始化时会调用）
#     # my_cache._cleanup_old_caches()
#
#     # 清空命名空间
#     # my_cache.clear_namespace()
#     # print("清空命名空间后获取应用配置:", my_cache.get("app:config"))
