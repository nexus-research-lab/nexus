# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：client_base
# @Date   ：2024/9/13 15:51
# @Author ：leemysw

# 2024/9/13 15:51   Create
# =====================================================

import os
import ssl
from abc import ABC, abstractmethod

import aiohttp
import certifi
import requests
from aiohttp import ClientTimeout

from agent.core.config import settings
from agent.shared.server.common.base_exception import ServerException
from agent.utils.logger import logger


class BaseClient(ABC):
    @staticmethod
    def _invoke(url: str, method: str = "POST", **kwargs):
        try:
            proxy = os.getenv("HTTP_PROXY", None)
            if proxy and url.startswith("https"):
                kwargs["proxies"] = {"http": proxy, "https": proxy}
            if "headers" not in kwargs:
                kwargs["headers"] = {'Content-Type': 'application/json'}
            if kwargs.get("timeout", None):
                kwargs["timeout"] = settings.HTTP_TIMEOUT

            response = requests.request(method, url, **kwargs)
            if response.status_code != 200:
                raise ServerException(f"API request failed with state: {response.status_code}, error: {response.text}")
            return response.json()
        except Exception as e:
            logger.error(f"Failed to invoke API: {url}, error: {e}")
            raise e

    @staticmethod
    async def _async_invoke(url: str, method: str = "POST", **kwargs):
        try:
            proxy = os.getenv("HTTP_PROXY", None)
            if proxy and url.startswith("https"):
                kwargs["proxy"] = proxy
            if "headers" not in kwargs:
                kwargs["headers"] = {'Content-Type': 'application/json'}
            if "timeout" not in kwargs:
                kwargs["timeout"] = ClientTimeout(total=settings.HTTP_TIMEOUT)
            elif isinstance(kwargs["timeout"], (int, float)):
                kwargs["timeout"] = ClientTimeout(total=kwargs["timeout"])

            if url.startswith("https"):
                ssl_context = ssl.create_default_context(cafile=certifi.where())
                kwargs["ssl"] = ssl_context

            async with aiohttp.ClientSession() as session:
                async with session.request(method, url, **kwargs) as response:
                    if response.status != 200:
                        error_msg = await response.text()
                        raise ServerException(f"API request failed with state: {response.status}, error: {error_msg}")
                    return await response.json()
        except Exception as e:
            raise e

    @abstractmethod
    def invoke(self, **kwargs):
        ...

    @abstractmethod
    async def async_invoke(self, **kwargs):
        ...
