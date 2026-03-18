# !/usr/bin/env python
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：model_cython
# @Date   ：2025/6/26 16:45
# @Author ：leemysw

# 2025/6/26 16:45   Create
# =====================================================

from pydantic import BaseModel, ConfigDict


class _CyFunctionDetectorMeta(type):
    def __instancecheck__(self, instance):
        return instance.__class__.__name__ == 'cython_function_or_method'


class CyFunctionDetector(metaclass=_CyFunctionDetectorMeta):
    pass


class AModel(BaseModel):
    model_config = ConfigDict(
        ignored_types=(CyFunctionDetector,)
    )
