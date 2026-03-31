#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""@mention 解析器 — 从消息内容中提取被 @ 的 Agent。"""

import re
from typing import Dict, List, Optional


# 基础模式：匹配 @name，name 仅含单词字符和连字符（不含空格、中文）
_MENTION_PATTERN = re.compile(r"@([\w\-]+)")


def parse_mentions(
    content: str,
    agent_names: Optional[List[str]] = None,
) -> List[str]:
    """从消息内容中提取 @mention 的 Agent 名称。

    Args:
        content: 用户发送的消息文本
        agent_names: 可选的合法 Agent 名称列表，用于精确匹配

    Returns:
        匹配到的 Agent 名称列表（去重，保持出现顺序）
    """
    if not content:
        return []

    if agent_names:
        # 有候选列表时，直接用已知名字做精确查找（最长优先避免子串误匹配）
        return _match_known_names(content, agent_names)

    # 无候选列表：退回基础 regex 提取
    raw_mentions = _MENTION_PATTERN.findall(content)
    if not raw_mentions:
        return []

    seen: set[str] = set()
    result: List[str] = []
    for name in raw_mentions:
        name = name.strip()
        if name and name not in seen:
            seen.add(name)
            result.append(name)
    return result


def _match_known_names(content: str, agent_names: List[str]) -> List[str]:
    """基于已知 Agent 名称列表精确匹配 @mention。

    对每个候选名按长度降序尝试匹配 @name，name 后必须跟空格/标点/行尾。
    """
    # 按长度降序，避免短名 "dev" 抢先匹配 "developer" 的前缀
    sorted_names = sorted(agent_names, key=len, reverse=True)
    seen: set[str] = set()
    result: List[str] = []

    for name in sorted_names:
        # @name 后面跟 空白 / 常见标点 / 字符串结尾
        pattern = re.compile(
            r"@" + re.escape(name) + r"(?=[\s，。！？、,.!?;\-:：；]|$)",
            re.IGNORECASE,
        )
        if pattern.search(content):
            canonical = name  # 保留原始大小写
            if canonical not in seen:
                seen.add(canonical)
                result.append(canonical)

    return result


def resolve_mention_agent_ids(
    content: str,
    agent_name_to_id: Dict[str, str],
) -> List[str]:
    """解析 @mention 并返回对应的 agent_id 列表。

    Args:
        content: 用户消息
        agent_name_to_id: Agent 名称到 ID 的映射

    Returns:
        被 @ 的 agent_id 列表（去重）
    """
    mentioned_names = parse_mentions(content, list(agent_name_to_id.keys()))
    return [
        agent_name_to_id[name]
        for name in mentioned_names
        if name in agent_name_to_id
    ]
