# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：file_store
# @Date   ：2025/12/11 18:35
# @Author ：leemysw

# 2025/12/11 18:35   Create
# 临时文件管理器
# =====================================================

import json
import os
import shutil
from datetime import datetime, timedelta, timezone
from functools import lru_cache
from typing import Optional

from agent.core.config import settings
from agent.utils.logger import logger
from agent.utils.snowflake import worker
from agent.utils.utils import cache_path


class TempFile:
    """临时文件管理器，用于管理具有生命周期的临时文件。"""

    def __init__(self, namespace: str = "default", default_ttl_hours: int = 6):
        """初始化临时文件管理器。

        Args:
            namespace: 命名空间，用于隔离不同业务的临时文件
            default_ttl_hours: 默认过期时间（小时）
        """
        self.namespace = namespace

        # 临时文件存储目录
        self.temp_dir = cache_path(settings.CACHE_FILE_DIR, f"temp/{self.namespace}")
        self.default_ttl = timedelta(hours=default_ttl_hours)
        self._last_cleanup_time: Optional[datetime] = None  # <-- Track last cleanup time

        self._ensure_temp_dir()
        self.cleanup_expired()  # 初始时清理一次旧缓存

    def _ensure_temp_dir(self) -> None:
        """确保临时文件目录存在。"""
        if not os.path.exists(self.temp_dir):
            try:
                os.makedirs(self.temp_dir, exist_ok=True)
                logger.debug(f"【TempFile:{self.namespace}】创建临时文件目录: {self.temp_dir}")
            except OSError as e:
                logger.error(f"【TempFile:{self.namespace}】创建临时文件目录失败: {self.temp_dir}, Error: {e}")

    def _get_file_path(self, file_id: str, extension: str = "") -> str:
        """生成文件存储路径。

        Args:
            file_id: 文件ID
            extension: 文件扩展名

        Returns:
            文件完整路径
        """
        if extension and not extension.startswith('.'):
            extension = f'.{extension}'
        return os.path.join(self.temp_dir, f"{file_id}{extension}")

    def _get_meta_path(self, file_id: str) -> str:
        """生成元数据文件路径。"""
        return os.path.join(self.temp_dir, f"{file_id}.meta.json")

    @staticmethod
    def generate_file_id() -> str:
        """生成文件ID。"""
        return worker.get_id()

    def save(self, file_obj: bytes, extension: str = "",
             ttl: Optional[timedelta] = None, file_id: Optional[str] = None) -> Optional[str]:
        """保存临时文件。

        Args:
            file_obj: 文件对象（二进制模式）
            extension: 文件扩展名（如 'pdf', '.jpg'）
            ttl: 过期时间，如果为None则使用默认值
            file_id: 指定文件ID，如果为None则自动生成

        Returns:
            文件ID，失败返回 None
        """

        self._ensure_temp_dir()
        self._check_and_run_cleanup()

        # 生成或使用指定的文件ID
        if file_id is None:
            file_id = self.generate_file_id()

        file_path = self._get_file_path(file_id, extension)
        meta_path = self._get_meta_path(file_id)

        try:
            # 保存文件内容
            with open(file_path, 'wb') as f:
                f.write(file_obj)

            # 计算过期时间
            expires_at = None
            if ttl or self.default_ttl:
                use_ttl = ttl if ttl else self.default_ttl
                expires_at = (datetime.now(timezone.utc) + use_ttl).isoformat()

            # 保存元数据
            metadata = {
                'file_id': file_id,
                'file_path': file_path,
                'extension': extension,
                'created_at': datetime.now(timezone.utc).isoformat(),
                'expires_at': expires_at,
                'file_size': os.path.getsize(file_path)
            }

            with open(meta_path, 'w', encoding='utf-8') as f:
                json.dump(metadata, f, ensure_ascii=False, indent=2)

            logger.debug(f"【TempFile:{self.namespace}】保存临时文件: {file_id} -> {file_path}")
            return file_id

        except Exception as e:
            logger.error(f"【TempFile:{self.namespace}】保存临时文件失败: file_id={file_id}, Error: {e}")
            # 清理可能产生的部分文件
            try:
                if os.path.exists(file_path):
                    os.remove(file_path)
                if os.path.exists(meta_path):
                    os.remove(meta_path)
            except OSError:
                pass
            return None

    def get(self, file_id: str) -> Optional[str]:
        """获取临时文件路径。"""

        meta_path = self._get_meta_path(file_id)
        self._check_and_run_cleanup()

        if not os.path.exists(meta_path):
            return None

        try:
            with open(meta_path, 'r', encoding='utf-8') as f:
                metadata = json.load(f)

            file_path = metadata.get('file_path')

            # 检查文件是否存在
            if file_path and os.path.exists(file_path):
                return file_path
            else:
                # 文件不存在，清理元数据
                self.delete(file_id)
                logger.warning(f"【TempFile:{self.namespace}】临时文件不存在，已清理: {file_id}")
                return None

        except Exception as e:
            logger.error(f"【TempFile:{self.namespace}】获取临时文件失败: file_id={file_id}, Error: {e}")
            return None

    def delete(self, file_id: str) -> bool:
        """删除临时文件。"""

        meta_path = self._get_meta_path(file_id)

        try:
            # 读取元数据获取文件路径
            if os.path.exists(meta_path):
                with open(meta_path, 'r', encoding='utf-8') as f:
                    metadata = json.load(f)
                file_path = metadata.get('file_path')

                # 删除文件
                if file_path and os.path.exists(file_path):
                    os.remove(file_path)

                # 删除元数据
                os.remove(meta_path)
                logger.debug(f"【TempFile:{self.namespace}】删除临时文件: {file_id}")
                return True

            return True  # 文件不存在也视为删除成功

        except Exception as e:
            logger.error(f"【TempFile:{self.namespace}】删除临时文件失败: file_id={file_id}, Error: {e}")
            return False

    def _check_and_run_cleanup(self) -> None:
        """检查是否需要运行清理任务（每6小时清理一次）。"""

        now = datetime.now(timezone.utc)
        if self._last_cleanup_time is None or (now - self._last_cleanup_time) >= timedelta(hours=6):
            logger.info(f"【FileCache:{self.namespace}】触发缓存清理任务...")
            self.cleanup_expired()
            self._last_cleanup_time = now

    def cleanup_expired(self) -> int:
        """清理所有过期的临时文件。"""
        if not os.path.exists(self.temp_dir):
            return 0

        cleaned_count = 0

        try:
            for filename in os.listdir(self.temp_dir):
                if filename.endswith('.meta.json'):
                    meta_path = os.path.join(self.temp_dir, filename)

                    try:
                        with open(meta_path, 'r', encoding='utf-8') as f:
                            metadata = json.load(f)

                        expires_at_str = metadata.get('expires_at')
                        if expires_at_str:
                            expires_at = datetime.fromisoformat(expires_at_str)
                            if datetime.now(timezone.utc) > expires_at:
                                file_id = metadata.get('file_id')
                                if file_id and self.delete(file_id):
                                    cleaned_count += 1

                    except (json.JSONDecodeError, KeyError, FileNotFoundError):
                        continue
                    except Exception as e:
                        logger.warning(f"【TempFile:{self.namespace}】清理文件 {meta_path} 时出错: {e}")

        except Exception as e:
            logger.error(f"【TempFile:{self.namespace}】执行清理任务时出错: {e}")

        if cleaned_count > 0:
            logger.info(f"【TempFile:{self.namespace}】清理了 {cleaned_count} 个过期临时文件")

        return cleaned_count

    def clear_all(self) -> bool:
        """清空命名空间下的所有临时文件。"""
        if os.path.exists(self.temp_dir):
            try:
                shutil.rmtree(self.temp_dir)
                logger.info(f"【TempFile:{self.namespace}】已清空所有临时文件: {self.temp_dir}")
                self._ensure_temp_dir()
                return True
            except OSError as e:
                logger.error(f"【TempFile:{self.namespace}】清空临时文件失败: {self.temp_dir}, Error: {e}")
                return False

        return True

    def exists(self, file_id: str) -> bool:
        """检查临时文件是否存在且未过期。"""
        return self.get(file_id) is not None

    def get_file_info(self, file_id: str) -> Optional[dict]:
        """获取临时文件的元数据信息。"""
        meta_path = self._get_meta_path(file_id)

        if not os.path.exists(meta_path):
            return None

        try:
            with open(meta_path, 'r', encoding='utf-8') as f:
                metadata = json.load(f)

            # 检查是否过期
            expires_at_str = metadata.get('expires_at')
            if expires_at_str:
                expires_at = datetime.fromisoformat(expires_at_str)
                if datetime.now(timezone.utc) > expires_at:
                    return None

            return metadata

        except Exception as e:
            logger.error(f"【TempFile:{self.namespace}】获取文件信息失败: file_id={file_id}, Error: {e}")
            return None


@lru_cache()
def get_temp_file_manager(namespace: str = "files",
                          default_ttl_hours: int = 24) -> TempFile:
    """获取临时文件管理器实例。

    Args:
        namespace: 命名空间
        default_ttl_hours: 默认过期时间（小时）

    Returns:
        临时文件管理器实例
    """
    return TempFile(namespace=namespace, default_ttl_hours=default_ttl_hours)

# 示例用法
# from io import BytesIO
#
# # 创建临时文件管理器
# temp_mgr = TempFile(namespace="upload_files", default_ttl_hours=2)
#
# # 1. 保存上传的文件
# with open("/path/to/uploaded.pdf", "rb") as f:
#     file_id = temp_mgr.save(f, extension="pdf", ttl=timedelta(hours=6))
#     print(f"文件已保存，ID: {file_id}")
#
# # 2. 保存内存中的数据
# image_data = BytesIO(b'fake image data...')
# file_id2 = temp_mgr.save(image_data, extension="jpg")
# print(f"图片已保存，ID: {file_id2}")
#
# # 3. 获取文件路径
# file_path = temp_mgr.get(file_id)
# if file_path:
#     print(f"文件路径: {file_path}")
#     with open(file_path, 'rb') as f:
#         content = f.read()
#         print(f"文件大小: {len(content)} bytes")
#
# # 4. 检查文件是否存在
# if temp_mgr.exists(file_id):
#     print("文件存在且有效")
#
# # 5. 获取文件信息
# info = temp_mgr.get_file_info(file_id)
# if info:
#     print(f"文件信息: {info}")
#
# # 6. 清理过期文件
# cleaned = temp_mgr.cleanup_expired()
# print(f"清理了 {cleaned} 个过期文件")
#
# # 7. 删除指定文件
# temp_mgr.delete(file_id2)
# print(f"已删除文件: {file_id2}")
#
# # 8. 清空所有临时文件
# # temp_mgr.clear_all()
