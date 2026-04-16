# !/usr/bin/env bash
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：activator.sh
# @Date   ：2026/04/04 12:45
# @Author ：leemysw
# 2026/04/04 12:45   Create
# =====================================================

set -e

cat << 'EOF'
<memory-review-reminder>
开始当前任务前，先判断是否需要复用过去经验：
- 这是复杂任务、返工任务或曾经失败过的任务吗？
- 用户刚刚表达了新偏好、规则或纠正吗？
- 这次执行里是否可能产生可复用的新经验？

如果是：
1. 先查看近期 memory 和记忆文件相关记录
2. 任务结束后，把 [LRN] / [ERR] / [FEAT] / [REF] 写入今日日记
3. 可长期复用的规则，提升到 SOUL.md / AGENTS.md / TOOLS.md / MEMORY.md
</memory-review-reminder>
EOF
