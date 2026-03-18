# !/usr/bin/python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：register_exception
# @Date   ：2024/1/22 16:14
# @Author ：leemysw

# 2024/1/22 16:14   Create
# =====================================================

import json
import traceback
from typing import Union

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError, ResponseValidationError, ValidationException
from pydantic_core import ValidationError
from starlette.exceptions import HTTPException as StarletteHTTPException

from agent.utils.constants import TermColors
from agent.utils.logger import logger
from ..common.base_exception import *


async def log_error(
        request: Request,
        exc: Union[ServerException, ValidationError, ValidationException, StarletteHTTPException, Exception]
):
    """记录异常到日志"""

    if issubclass(exc.__class__, ValidationException):
        error_msg = exc.errors()
        response = UnProcessable
    elif issubclass(exc.__class__, ServerException):
        error_msg = exc.errors
        response = exc.resp
    elif issubclass(exc.__class__, ValidationError):
        error_msg = exc.errors()
        response = ServerError
    elif exc.__class__ == StarletteHTTPException:
        error_msg = exc.detail
        response = NotFound
    else:
        error_msg = "[内部异常错误]" + str(exc)
        response = ServerError

    request_id = getattr(request.state, "request_id", "unknown")
    if request_id == "unknown":
        try:
            # 尝试从查询参数获取
            request_id = request.query_params.get("request_id", "unknown")
            
            # 如果还是unknown，尝试从已缓存的body获取（如果有）
            if request_id == "unknown" and hasattr(request.state, "body"):
                body_data = json.loads(request.state.body)
                request_id = body_data.get("request_id", "unknown")
        except Exception:
            request_id = "unknown"

    response.detail = error_msg
    response.request_id = request_id

    logger.error(
        f"{TermColors.RED}Exception  : {error_msg}\n"
        f"====================ERROR======================\n"
        f"RequestId  : {request_id}\n"
        f"Host       : {request.client.host}\n"
        f"URL        : {request.method} {request.url}\n"
        f"UserAgent  : {request.headers.get('user-agent')}\n\n"
        f"{traceback.format_exc()}\n"
        f"===============================================\n{TermColors.RESET}"
    )

    return fail(response)


def register_exception(app: FastAPI) -> None:
    """
    捕获异常
    :param app:
    :return:
    """

    @app.exception_handler(StarletteHTTPException)
    async def http_exception_handler(request, exc: StarletteHTTPException):
        return await log_error(request, exc)

    @app.exception_handler(ValidationError)
    async def inner_validation_exception_handler(request: Request, exc: ResponseValidationError):
        """
        内部参数验证异常
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    @app.exception_handler(ResponseValidationError)
    async def inner_validation_exception_handler(request: Request, exc: ResponseValidationError):
        """
        内部参数验证异常
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    @app.exception_handler(RequestValidationError)
    async def request_validation_exception_handler(request: Request, exc: RequestValidationError):
        """
        请求参数验证异常
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    # 自定义异常捕获
    @app.exception_handler(TokenExpiredException)
    async def user_token_expired_exception_handler(request: Request, exc: TokenExpiredException):
        """
        token过期
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    @app.exception_handler(TokenAuthException)
    async def user_token_exception_handler(request: Request, exc: TokenAuthException):
        """
        用户token异常
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    @app.exception_handler(AuthenticationException)
    async def user_not_found_exception_handler(request: Request, exc: AuthenticationException):
        """
        用户权限不足
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)

    @app.exception_handler(ServerException)
    async def base_exception_handler(request: Request, exc: ServerException):
        """
        服务器内部错误
        :param request:
        :param exc:
        """
        return await log_error(request, exc)

    # 捕获全部异常
    @app.exception_handler(Exception)
    async def all_exception_handler(request: Request, exc: Exception):
        """
        全局所有异常
        :param request:
        :param exc:
        :return:
        """
        return await log_error(request, exc)
