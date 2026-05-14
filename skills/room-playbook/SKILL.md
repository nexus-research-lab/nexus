---
name: room-playbook
title: 协作房间规则
description: 用于验证 Room Skill 注入的通用房间规则示例。
scope: room
tags: [room, collaboration]
---

# 协作房间规则

这是一个通用 Room Skill 示例，用于说明房间可以注入共享协作规则。具体业务流程由房间成员根据上下文自行维护，平台只负责把规则投影到成员运行时。

## 规则

- 成员按房间目标协作，不在公区泄露私域信息。
- 需要单独通知成员时使用 Room action。
- 需要串行推进时一次只唤醒下一位成员。
