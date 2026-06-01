---
name: goal-manager
description: 当用户明确要求启动、设定、创建、继续、完成或阻塞当前会话的 Goal，或系统/开发者明确要求启用 Goal 长程执行时使用。先加载本 skill，再调用 nexus_goal MCP 工具；不要用 /goal 文本命令。
---

# goal-manager

你负责把用户对当前会话的长程目标需求稳定转换成 `nexus_goal` 工具调用。Goal 是会话级长程目标，不是普通待办、定时提醒或 Room action。

## 必须遵守

1. 用户明确要求“启动 Goal、设定目标、开启长期目标、持续完成 X、把 X 作为本轮目标”时，先调用 `get_goal` 判断当前会话是否已有 Goal，再按需调用 `create_goal`。
2. 只有用户或系统/开发者明确要求 Goal 时才创建；不要从普通问题、一次性任务、闲聊、自动标题或常规协作里推断 Goal。
3. 不再使用 `/goal`、`/goal pause`、`/goal resume` 这类文本命令；产品入口是 UI 的“启动 Goal”和 `nexus_goal` 工具。
4. Goal 属于当前会话。`nexus_goal` 工具会自动绑定当前 session，不要向用户索要 session_key，也不要自己拼 session_key。
5. `token_budget` 只有在用户明确给出预算时才传；用户没有说预算就不要设置。
6. 当前会话已有未结束 Goal 时，不要创建第二个 Goal；先说明已有目标，必要时让用户在面板清理/完成后再创建新的。
7. 只有目标确实完成且没有剩余必要工作时，才调用 `update_goal` 标记 `complete`。
8. 只有同一个阻塞条件在连续 Goal 续跑中重复出现，且没有用户输入或外部状态变化就无法推进时，才调用 `update_goal` 标记 `blocked`；不要因为一次不确定、需要澄清或暂时停顿就标记阻塞。
9. 暂停、恢复、清理、预算限制和用量限制由用户或系统控制，不要用模型工具模拟这些状态。
10. 用户要“提醒我、每天/每周、定时做某事”时，使用 `scheduled-task-manager` 和 `nexus_automation`，不要把定时任务创建成 Goal。

## 工具顺序

### 查看当前 Goal

当用户问“现在目标是什么、进展如何、有没有 Goal”时：

```json
{}
```

调用 `get_goal` 后，用工具结果里的 `goal`、`remainingTokens` 回答；没有 Goal 时直接说明当前会话未启动 Goal。

### 创建 Goal

适用：

- 用户说“启动 Goal：完成 X”
- 用户说“接下来持续帮我完成 X”
- 系统/开发者明确要求本会话进入 Goal 模式

流程：

1. 调用 `get_goal`。
2. 如果 `goal` 为 `null`，调用 `create_goal`。
3. 如果已有 Goal，说明当前目标，不创建新 Goal。

示例：

```json
{
  "objective": "完成 Nexus Goal 功能与 Codex 行为对齐，并验证关键路径",
  "token_budget": 200000
}
```

没有明确预算时：

```json
{
  "objective": "完成 Nexus Goal 功能与 Codex 行为对齐，并验证关键路径"
}
```

### 完成 Goal

适用：

- 目标已完成
- 所有必要验证已做完
- 没有剩余必须继续处理的问题

完成前先做一次简短但真实的完成审计：从 objective 和用户最新要求中提取必须满足的范围、交付物、验证命令、文件或运行状态，用当前事实逐项确认。不要因为已有进展、测试看起来相关、预算接近耗尽或准备停止而标记完成；只有当前证据证明完整目标成立时才调用工具。

调用：

```json
{
  "status": "complete"
}
```

工具成功后只发送一条简短最终回复，然后停止并等待用户输入；不要继续调用工具或开启新工作。如果工具结果包含 `completionBudgetReport`，按结果里的 `goal.tokensUsed`、`goal.tokenBudget` 和 `goal.timeUsedSeconds` 简短报告最终用量。

### 阻塞 Goal

适用：

- 同一个阻塞条件已经连续出现
- 没有用户输入、权限、外部系统修复或外部状态变化就无法继续
- 继续自动重试没有意义

调用：

```json
{
  "status": "blocked"
}
```

阻塞前应先把具体缺口告诉用户。一次性澄清问题优先直接问用户，不要立刻把 Goal 置为 blocked。

## 判断边界

创建 Goal：

- “把修复发送失败作为当前 Goal”
- “接下来持续检查并改到通过为止”
- “启动一个目标：完成这个分支的 Goal 对齐”
- “继续这个 Goal，直到和 Codex 几乎一致”

不创建 Goal：

- “帮我看一下这个报错”
- “写个函数”
- “明天提醒我开会”
- “每天发新闻给我”
- “总结一下这段对话”
- “创建一个 Room”

需要定时任务时转用 `scheduled-task-manager`；需要 Room 协作时转用 `nexus-manager` 或 Room skill。

## 回复要求

- 创建成功后，用一句话确认当前 Goal，不解释底层工具。
- 已有 Goal 时，说明已有目标并给出下一步选择。
- 完成或阻塞后，按工具结果简短说明状态。
- 不向用户展示 JSON 参数，除非用户明确要求看调用细节。
