# Room Skill 编写指南

## 1. 文档定位

本文面向 Skill 作者，只说明如何把业务协作规则写成可执行的 Room Skill。

通信字段、可见性和唤醒约束以 [Room 协作协议](./room-collaboration-spec.md) 为准；本文不复制协议实现细节。

## 2. 什么时候使用 Room Skill

适用于需要多个 Agent 在同一 conversation 中协作的任务，例如：

- 明确的主持、执行、复核或汇总分工。
- 先私下收集，再公开发布结论。
- 需要约定发言顺序、交接、沉默和收口条件。

只影响一个 Agent 的规则应写成普通 Agent Skill。

## 3. Skill 必须写清楚什么

### 3.1 启用条件

写明适用任务、触发条件和不适用场景。不要把所有 Room 都默认当成同一种业务流程。

### 3.2 成员职责

为每类成员写一个可观察的职责：谁发起、谁执行、谁复核、谁汇总、谁负责最终公开结论。

### 3.3 公开协作

公开区只发布所有成员都需要知道的事实：

- 目标和已确认的决策。
- 不需要持久责任与验收的一次性会话请求。
- 阶段性进展、阻塞和最终结论。

未确认的草稿、重复确认和无意义附和不要公开。需要行动时才写非代码的 @成员；描述计划、举例或总结状态时使用普通名称，不要制造唤醒。

`@成员` 只表示 conversation wake/handoff，不创建 Work Item、Assignment、Submission 或 Acceptance，也不表示发言顺序。需要持久 owner、依赖、交付与验收时必须使用 WorkGraph，并由 `assign_work` 建立责任。多个 Agent 并行时，Skill 不应假设谁先完成；平台会按 source slot 独立派发，公区按实际发布顺序展示。

### 3.4 私下协作

私下发送适合：

- 指定成员才能处理的请求。
- 不宜公开的背景或中间意见。
- 需要先收集、再由指定成员汇总的内容。

私下信息不会自动公开。公开时只转述必要结论，不复制敏感正文。

### 3.5 交接与收口

Skill 必须说明：

- 什么条件下把工作交给下一位成员。
- 谁接收汇总结果。
- 最终结论由谁公开。
- 没有新工作时如何停止。
- 超时、失败或成员不可用时由谁介入。

平台不会替 Skill 判断讨论是否结束，也不会自动把控制权交还主持人。

## 4. 工具与回复选择

| 场景 | 做法 |
| --- | --- |
| 普通公开发言 | 直接输出 final reply，不调用 Room 工具。 |
| 一次性公区会话请求或唤醒 | 在 final reply 中写真实的非代码 @成员；它不建立结构化责任。 |
| 可追责交付、依赖或验收 | 使用 WorkGraph 与 `assign_work`；后续以 `submit_work` / `review_work` 回交，不用 `@` 冒充。 |
| 私下发送或多人收集 | 使用 send_directed_message，明确 recipients、是否唤醒和 reply_route。 |
| 私下结果交给主持人 | 使用 reply_route=private，必要时设置 wake_policy=immediate。 |
| 私下结果需要主持人自然公开推进 | 在 private route 上设置 next_reply_route=public。 |
| 私域/tool-driven 流程需要额外广播独立事实 | 使用 publish_public_message 一次；普通公区发言不要用它。 |
| 被唤醒但没有新工作 | 输出 <nexus_room_no_reply/>，不要制造公区消息。 |

工具中的 Room、conversation、source agent 和因果字段由 runtime 注入，Skill 不应伪造或拼接。

## 5. Room Skill 加载

Room Skill 复用 Skill 的目录、frontmatter 和正文格式，但由 Room runtime 在启动成员时直接加载。运行时只注入去掉 frontmatter 后的完整正文，正文是 Room 协作规则的唯一真相源。

`description` 只用于技能目录和管理界面展示；不要再维护额外的运行时规则字段。

## 6. 发布前检查

- 是否写明 scope: room。
- 是否有明确的成员职责和最终汇总者。
- 是否区分公开事实与私下上下文。
- 是否只在需要行动时 @成员。
- 是否为 directed message 写清 recipients、wake_policy 和 reply_route。
- 是否说明私下结果如何回到公区。
- 是否有停止、超时和失败收口规则。
- 是否删除了平台无法验证的流程状态描述。

一句话：Skill 负责“为什么协作、谁做什么、何时结束”；Room 平台负责“消息给谁、何时运行、回复去哪里”。
