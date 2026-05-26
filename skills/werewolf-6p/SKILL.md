---
name: werewolf-6p
title: 六人狼人杀调试本子
description: 面向 1 名主持 Agent 和 6 名玩家 Agent 的 Room 通讯验证规则。
scope: room
tags: [room, game, werewolf]
---

# 六人狼人杀调试本子

本本子只用于验证 Room 通讯机制，不把业务状态交给平台固化。主持 Agent 自己维护局面，Room 只负责公区、私域投递、受众夜聊、请求回复和上下文投影。

## 思考预算

- 主持 Agent 只思考当前阶段、需要发出的 action、下一步等待谁回复；不要在思考里复述完整规则、完整角色表或失败备选方案。
- 玩家 Agent 只基于自己可见信息发言；不要长篇复盘隐藏信息。
- 单条公开发言控制在 120 字以内；私域请求控制在 160 字以内；私有记录控制在 12 行以内。
- 每次只推进一个闭合步骤。不要一次性创建后续多个阶段的 action。

## 人数与角色

- 1 名主持 Agent：负责发身份、收夜间行动、公布天亮、组织发言和投票。
- 6 名玩家 Agent：2 狼人、1 预言家、1 女巫、2 平民。
- 角色应由主持 Agent 私下随机或按测试需要指定，并通过 `private_message --wake-policy none` 发给每名玩家。
- 主持 Agent 用 `private_note` 维护最小状态：轮次、存活、死亡、角色、女巫药水、当前等待项。

## 胜负

- 好人胜利：两名狼人全部出局。
- 狼人胜利：平民全部死亡，或神职全部死亡。
- 每次天亮公布、投票出局后都检查胜负；未结束才进入下一阶段。

## 夜晚流程

按顺序闭合：

1. 狼人行动：给两名狼人发送 `private_message --audience-agent-id ... --wake-policy immediate` 开启夜聊；再对一名狼人提交者发送 `request-reply --reply-target sender_private`，要求只回复击杀目标名字。
2. 预言家行动：收到狼人击杀目标后，对预言家发送 `request-reply --reply-target sender_private`，要求只回复查验对象名字；主持再用 `private_message --wake-policy none` 回告“好人/狼人”。
3. 女巫行动：对女巫发送 `request-reply --reply-target sender_private`，告知本夜被杀者和药水剩余，要求只回复“救/不救；毒/不毒 玩家名”。
4. 天亮：主持根据击杀、解药、毒药结算死亡，只在公区公布死亡结果，不公开身份和夜间私聊内容。

## 白天流程

- 主持按固定顺序逐个唤醒玩家发言，使用 `request-reply --reply-target public_feed`。
- 所有存活玩家发言结束后，主持逐个收投票，或在调试时指定一名主持汇总者收集投票。
- 投票结果只公布出局者和票型，不公布未公开身份。

## Room action 约束

- 狼人夜聊使用 audience 私域；狼人之间的讨论只投影给两名狼人。
- 需要某人给主持提交决定时，使用 `request_reply`，不要让玩家再调用 CLI。
- 玩家收到 `request_reply` 时，最终回复就是答案；不要创建新的 Room action，除非请求明确要求转发给第三方。
- 主持收到私域回复后，只更新私有状态并发起下一步；公开输出只用于阶段公告、死亡公告、发言和投票结果。
