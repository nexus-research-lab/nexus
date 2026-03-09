# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：base_resp
# @Date   ：2024/1/22 23:24
# @Author ：leemysw

# 2024/1/22 23:24   Create
# =====================================================

from enum import Enum
from typing import Union

from fastapi import status as http_status
from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse, Response

from agent.shared.schemas.model_cython import AModel
from agent.utils.constants import TermColors
from agent.utils.logger import logger

__all__ = [
    'ok',
    'fail',
    'Resp',
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


def ok(response: Resp, data=None) -> Response:
    logger.info(
        f"\n\n=====================DONE========================\n"
        f"{response.info_dict}\n"
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
        f"{response.info_dict}\n"
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
