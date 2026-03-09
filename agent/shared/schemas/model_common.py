# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_interface
# @Date   ：2024/12/6 14:51
# @Author ：leemysw

# 2024/12/6 14:51   Create
# =====================================================

from typing import List, Literal, Optional, Union

from pydantic import ConfigDict, Field, model_validator

from agent.shared.schemas.model_cython import AModel
from agent.shared.schemas.model_file import Image


class Text(AModel):
    text: str
    type: Literal["text"] = "text"
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "text": "This is a test text",
            "type": "text"
        }
    })


class Multi(AModel):
    text: Optional[str] = Field(default=None, description="text input")
    image: Optional[str] = Field(default=None, description="base64 encoded image string or image url")
    type: Literal["multi"] = Field(default="multi", description="input type")

    @model_validator(mode="after")
    def check_input(self):
        if self.text is None and self.image is None:
            raise ValueError("At least one of texts or image should be provided")

        return self

    model_config = ConfigDict(json_schema_extra={
        "example": {
            "text": "This is a test text",
            "image": "base64_image_string",
            "type": "multi"
        }
    })


INPUT_TYPE = Union[str, Text, Image, Multi, List[Union[str, Text, Image, Multi]]]
