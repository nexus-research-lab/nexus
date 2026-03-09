# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：models
# @Date   ：2024/11/21 09:56
# @Author ：leemysw

# 2024/11/21 09:56   Create
# =====================================================

from typing import Optional

from pydantic import ConfigDict, Field

from agent.shared.schemas.model_cython import AModel


class BaseSchema(AModel):
    user_id: Optional[str] = Field(None, description='用户ID')
    request_id: Optional[str] = Field(None, description='请求ID')
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "user_id": "1234567890",
            "trace_id": "1234567890",
            "request_id": "1234567890",
        }
    })
