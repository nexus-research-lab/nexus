# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_file
# @Date   ：2024/12/6 13:56
# @Author ：leemysw

# 2024/12/6 13:56   Create
# =====================================================

from typing import Literal, Union

from pydantic import ConfigDict, Field, model_validator

from agent.core.config import settings
from agent.shared.schemas.model_cython import AModel
from agent.shared.server.common.base_exception import ServerException


class BaseFile(AModel):
    file_name: str = Field(..., description="文件名称")
    file_bnid: Union[int, str] = Field(..., description="文件业务编号")
    deploy_id: Union[int, str] = Field(default="", description="部署编号")
    user_bnid: Union[int, str] = Field(default="", description="用户数据库id")
    cache_path: str = Field(default="", description="本地缓存路径", exclude=True)


class UrlFile(BaseFile):
    file_url: str = Field(..., description="文件URL")

    def to_file(self):
        return File(data=self.file_url, type="url")

    model_config = ConfigDict(json_schema_extra={
        "example": {
            "file_name": "example.pdf",
            "file_bnid": 202412061356000001,
            "user_bnid": 1784472530828918784,
            "deploy_id": "123456",
            "file_url": "",
        },
    })


class OSSFile(BaseFile):
    file_path: str = Field(..., description="文件路径")
    file_bucket: str = Field(..., description="文件桶")
    oss: Literal["minio", "tencent_cos", "local"] = Field(settings.DEFAULT_OSS_CLIENT, description="OSS类型")

    def to_file(self):
        return File(data=self, type="oss")

    model_config = ConfigDict(json_schema_extra={
        "example": {
            "file_bnid": 202412061356000001,
            "file_name": "example.txt",
            "file_path": "/file/4390-ae79-76bb13962b0b.xlsx",
            "file_bucket": "agent-file",
            "user_bnid": 1784472530828918784,
            "oss": "minio",
            "deploy_id": "123456"
        }
    })


class File(AModel):
    data: Union[str, OSSFile]
    type: Literal["url", "oss", "base64"]
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "data": OSSFile.model_config["json_schema_extra"]["example"],
            "type": "oss"
        }
    })

    @model_validator(mode="after")
    def check_type(self):
        if self.type == "url" and not self.data.startswith("http"):
            raise ServerException("url must start with http or https")
        if self.type == "base64" and not isinstance(self.data, str):
            raise ServerException("base64 data must be a string")
        if self.type == "base64" and not self.data.startswith("data:"):
            raise ServerException("base64 data must start with data:")
        if self.type == "oss" and not isinstance(self.data, OSSFile):
            raise ServerException("oss data must be an OSSFile object")

        return self


class Audio(AModel):
    audio: str = Field(..., description="base64 encoded audio string(data:audio) or audio url")
    type: Literal["audio"] = "audio"
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "audio": "base64_audio_string or audio_url",
            "type": "audio"
        }
    })


class Image(AModel):
    image: str = Field(..., description="base64 encoded image string or image url")
    type: Literal["image"] = "image"
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "image": "base64_image_string or image_url",
            "type": "image"
        }
    })
