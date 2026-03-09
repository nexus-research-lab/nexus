# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：handler
# @Date   ：2024/7/19 14:08
# @Author ：leemysw

# 2024/7/19 14:08   Create
# =====================================================

import logging
from typing import Optional, Union

from fastapi import Request
from pydantic import BaseModel

from agent.shared.server.schemas.base_models import BaseSchema
from agent.utils.logger import logger


class BaseHandler:

    @staticmethod
    def get_base_schema(request: Request, request_data: Union[BaseModel, dict]) -> Optional[BaseSchema]:
        if isinstance(request_data, BaseModel):
            try:
                return BaseSchema(**request_data.model_dump())
            except:
                logging.warning(f"Error in get_base_schema: {request.url} ")
                return None
        return None

    @staticmethod
    def log_request_data(request: Request, request_data: Union[BaseModel, dict]):
        if isinstance(request_data, BaseModel):
            request_data = request_data.model_dump()
        else:
            request_data = request_data
        logger.info(
            f"\n\n====================Request======================\n"
            f"Host       : {request.client.host}\n"
            f"URL        : {request.method} {request.url}\n"
            f"DATA       : {request_data}\n"
        )
