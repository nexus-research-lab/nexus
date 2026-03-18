# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_resp
# @Date   ：2024/1/22 23:24
# @Author ：leemysw

# 2024/1/22 23:24   Create
# =====================================================

from enum import Enum
from typing import Any, Union

from fastapi import status as http_status
from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse, Response

from agent.infra.schemas.model_cython import AModel
from agent.utils.constants import TermColors
from agent.utils.logger import logger

__all__ = [
    'ok',
    'fail',
    'Resp',
    'build_log_payload',
    'Unauthorized',
    'FORBIDDEN',
    'NotFound',
    'UnProcessable',
    'ServerError',
    'IndexingStatusCode'
]


# Indexing Code 1xxxx
# Indexing Cancel Failed 10099
# Indexing Cancel Task NotFound 10091
# Retrieval Code 2xxxx

class IndexingStatusCode(Enum):
    TASK_NOT_FOUND = "10091"
    CANCEL_FAILED = "10099"
    SUCCESS = "0000"


class Resp(AModel):
    code: Union[int, str] = "0000"
    message: str = "success"
    success: bool = True
    http_status: int = http_status.HTTP_200_OK
    detail: str = ""
    request_id: Union[str, int] = ""
    data: Union[list, dict, str] = None
    info_data: str = ""

    def set_detail(self, detail: str):
        self.detail = detail

    @property
    def resp_dict(self):
        return {
            'code': self.code,
            'message': self.message,
            'success': self.success,
            'data': self.data,
        }

    @property
    def info_dict(self):
        return {
            'request_id': self.request_id,
            'code': self.code,
            'message': self.message,
            'success': self.success,
            'data': self.info_data if self.info_data else self.data,
        }


MAX_LOG_STRING_LENGTH = 20
MAX_LOG_COLLECTION_ITEMS = 5
TRUNCATED_MARKER = "...<truncated>"


def _truncate_string(value: str, max_length: int = MAX_LOG_STRING_LENGTH) -> str:
    """裁剪过长字符串，避免日志被大字段刷屏。"""
    if len(value) <= max_length:
        return value
    return f"{value[:max_length]}{TRUNCATED_MARKER}(len={len(value)})"


def _clip_payload(value: Any) -> Any:
    """递归裁剪日志载荷中的大字段。"""
    if isinstance(value, str):
        return _truncate_string(value)

    if isinstance(value, dict):
        clipped_items = list(value.items())[:MAX_LOG_COLLECTION_ITEMS]
        result = {key: _clip_payload(item) for key, item in clipped_items}
        if len(value) > MAX_LOG_COLLECTION_ITEMS:
            result["__truncated__"] = (
                f"dict_items={len(value)}, kept={MAX_LOG_COLLECTION_ITEMS}"
            )
        return result

    if isinstance(value, list):
        result = [_clip_payload(item) for item in value[:MAX_LOG_COLLECTION_ITEMS]]
        if len(value) > MAX_LOG_COLLECTION_ITEMS:
            result.append(
                f"{TRUNCATED_MARKER}(list_items={len(value)}, kept={MAX_LOG_COLLECTION_ITEMS})"
            )
        return result

    if isinstance(value, tuple):
        result = tuple(_clip_payload(item) for item in value[:MAX_LOG_COLLECTION_ITEMS])
        if len(value) > MAX_LOG_COLLECTION_ITEMS:
            return result + (
                f"{TRUNCATED_MARKER}(tuple_items={len(value)}, kept={MAX_LOG_COLLECTION_ITEMS})",
            )
        return result

    return value


def build_log_payload(response: Resp) -> dict[str, Any]:
    """构建用于日志输出的裁剪后响应内容。"""
    payload = dict(response.info_dict)
    payload["data"] = _clip_payload(payload.get("data"))
    return payload


def ok(response: Resp, data=None) -> Response:
    logger.info(
        f"\n\n=====================DONE========================\n"
        f"{build_log_payload(response)}\n"
    )
    if not data:
        data = response.resp_dict
    return JSONResponse(
        status_code=http_status.HTTP_200_OK,
        content=jsonable_encoder(data)
    )


def fail(response: Resp) -> Response:
    response.data = {"request_id": response.request_id, "detail": response.detail}
    response.message = "failed"
    response.success = False
    if response.code == "0000":
        response.code = "9999"

    logger.error(
        f"{TermColors.RED}\n\n=====================DONE========================\n"
        f"{build_log_payload(response)}\n"
    )

    return JSONResponse(
        status_code=response.http_status,
        content=jsonable_encoder(response.resp_dict)
    )


Unauthorized = Resp(
    code="401",
    message="权限拒绝",
    success=False,
    http_status=http_status.HTTP_401_UNAUTHORIZED
)
FORBIDDEN = Resp(
    code="403",
    message="权限不足",
    success=False,
    http_status=http_status.HTTP_403_FORBIDDEN
)
NotFound = Resp(
    code="404",
    message="资源不存在",
    success=False,
    http_status=http_status.HTTP_404_NOT_FOUND
)
UnProcessable = Resp(
    code="422",
    message="请求参数错误",
    success=False,
    http_status=http_status.HTTP_422_UNPROCESSABLE_ENTITY
)
ServerError = Resp(
    code="500",
    message="系统调用异常",
    success=False,
    http_status=http_status.HTTP_500_INTERNAL_SERVER_ERROR
)
