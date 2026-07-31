---
name: goal-manager
description: 当用户明确要求启动、设定、创建、继续、纠正、完成或阻塞当前会话的 Goal，或系统/开发者明确要求启用 Goal 长程执行时使用。创建前必须先确认 objective 已具备足够的可执行信息；缺少会改变结果的关键信息时先提问并等待，禁止创建占位 Goal。先加载本 skill，再按需调用 mcp__nexus_goal__get_goal/create_goal/retarget_goal/audit_objective_alignment/update_goal；不要用 /goal 文本命令。
---

# goal-manager

你负责把用户对当前会话的长程目标需求稳定转换成 `nexus_goal` MCP 工具调用。Goal 是会话级长程目标，不是普通待办、定时提醒或 Room action。

Skill 只负责加载这份使用说明，不会替你读取、创建、完成或阻塞 Goal。加载本 skill 后，必须继续调用当前模型工具列表中可见的 Goal MCP 工具；不要把“skill 文档里没有工具面板”误判成 Goal MCP 工具不可用。

## 工具名

Nexus 中 Goal MCP 工具在模型可见工具列表里通常带完整 MCP 前缀：

- `mcp__nexus_goal__get_goal`
- `mcp__nexus_goal__create_goal`
- `mcp__nexus_goal__retarget_goal`
- `mcp__nexus_goal__audit_objective_alignment`
- `mcp__nexus_goal__update_goal`

如果运行时暴露的是 Codex/plain-tool 裸名 `get_goal`、`create_goal`、`retarget_goal`、`audit_objective_alignment`、`update_goal`，它们是同一组 Goal 工具。优先调用当前工具列表中实际可见的名字；不要因为裸名不存在就放弃，先找对应的 `mcp__nexus_goal__*` 工具。

判断工具是否可用，只看当前模型可见工具列表，不看 skill 文档本身。完成目标时，如果 `mcp__nexus_goal__update_goal` 可见，下一步必须调用它；只有运行时实际暴露为裸名时才调用裸 `update_goal`。

## 必须遵守

1. 用户明确要求 Goal 只是进入创建判断的必要条件，不是立即调用 `create_goal` 的指令。普通问题、一次性任务、闲聊、自动标题或常规协作仍然不能推断为 Goal。
   自适应持久化不走本 Skill 的猜测性 `create_goal`：只有受管 Execution context 明确开放 `promote_execution_to_goal`，并带有后端验证过的跨边界证据时，才调用该 Execution 工具。复杂度、Plan 长度、Room 或子智能体参与本身都不是证据。
2. 创建前先检查当前上下文能否形成可直接执行的 objective。至少要明确目标交付物，以及会实质改变结果的范围、对象或受众、约束和验收标准；只检查与当前任务实际相关的项，不机械索要无关信息。
3. 若缺少的信息会改变实际交付物或执行路径，先向用户提出最少必要的澄清问题并等待回答。信息足够前禁止调用 `create_goal`，禁止先创建“写一篇作文”“完成这个项目”之类的宽泛或占位 Goal，再靠后续追问或 `retarget_goal` 补齐。
4. 能从当前对话、文件或可靠上下文确定的信息不要重复询问。信息足够后，把已确认的关键要求合并成完整、具体的 objective，再调用 `mcp__nexus_goal__get_goal`（或裸名 `get_goal`）判断当前会话是否已有 Goal，并按需调用 `mcp__nexus_goal__create_goal`（或裸名 `create_goal`）。
5. 不再使用 `/goal`、`/goal pause`、`/goal resume` 这类文本命令；产品入口是 UI 的“启动 Goal”和 `mcp__nexus_goal__*` 工具。
6. Goal 属于当前会话。`mcp__nexus_goal__*` 工具会自动绑定当前 session，不要向用户索要 session_key，也不要自己拼 session_key。
7. `token_budget` 只有在用户明确给出预算时才传；用户没有说预算就不要设置。
8. 当前会话已有未结束 Goal 时，不要创建第二个 Goal。只有用户明确纠正或替换当前 active Goal 的 objective 时，调用 `mcp__nexus_goal__retarget_goal`（或裸名 `retarget_goal`）更新同一个 Goal；绝不能先完成旧 Goal 再创建新 Goal。
9. 只有目标确实完成且没有剩余必要工作时，才在当前 round 调用 `mcp__nexus_goal__audit_objective_alignment`（或裸名 `audit_objective_alignment`）提交逐条证据；仅当返回 `aligned`，才紧接着调用 `mcp__nexus_goal__update_goal`（或裸名 `update_goal`）标记 `complete`。
10. 只有同一个阻塞条件在连续 Goal 续跑中重复出现，且没有用户输入或外部状态变化就无法推进时，才调用 `mcp__nexus_goal__update_goal`（或裸名 `update_goal`）标记 `blocked`；不要因为一次不确定、需要澄清或暂时停顿就标记阻塞。
11. 暂停、恢复、清理、预算限制和用量限制由用户或系统控制，不要用模型工具模拟这些状态。
12. 用户要“提醒我、每天/每周、定时做某事”时，直接使用 `nexus_automation` 定时任务工具，不要把定时任务创建成 Goal。

## Room Goal 负责人

当运行时明确告诉你当前 Goal 是 Room Goal，且你是负责人 Agent 时：

- Goal 属于整个 Room，不是你的私人会话目标；你负责推进、协调、验收和最终标记完成。
- 多成员 Room Goal 的可见协作是完成条件，不是可选优化；如果房间可见历史中还没有非负责人成员对当前 Goal 的实质贡献，负责人本轮必须先公开 `@成员名` 分派一个具体交付物。
- 普通协作分派发公开 Room 消息，并在消息中 `@成员名` 指定一个要立刻行动的 Agent，让用户能看到负责人如何调度。
- 公开 `@` 必须说明清楚交付物；不要用 `@` 描述计划、候选人或后续可能动作。
- 首次公开分派时不要在同一轮调用 Goal 完成工具；等待被 `@` 成员在房间可见地回复后，再基于证据继续推进或验收。
- 只有涉及隐私、密钥、隐藏收集、私下提醒或用户明确要求私下协作时，才使用 Room directed message。
- 如果本轮最合适的动作是分派给其他成员，公开 `@` 后保持 Goal active；不要在等待成员回复前标记 complete。
- 被分派结果返回后，负责人基于房间可见证据继续推进或完成审计；只有完整 Room 目标已验证，且已有非负责人协作证据时，才调用 Goal 更新工具标记 `complete`。

## 工具顺序

### 查看当前 Goal

当用户问“现在目标是什么、进展如何、有没有 Goal”时：

工具：`mcp__nexus_goal__get_goal`（或裸名 `get_goal`）

```json
{}
```

调用 `mcp__nexus_goal__get_goal`（或裸名 `get_goal`）后，用工具结果里的 `goal`、`remainingTokens` 回答；没有 Goal 时直接说明当前会话未启动 Goal。

### 创建 Goal

适用：

- 用户说“启动 Goal：完成 X”
- 用户说“接下来持续帮我完成 X”
- 系统/开发者明确要求本会话进入 Goal 模式

流程：

1. 先从现有上下文判断 objective 是否已经具体到可以直接执行，不需要对会改变结果的关键信息作实质猜测。
2. 如果仍缺关键信息，向用户提问并等待回答，本轮不要调用 `create_goal`。
3. 信息足够后，把已确认的交付物、范围、对象或受众、关键约束与验收标准按实际需要合并为完整 objective。
4. 调用 `mcp__nexus_goal__get_goal`（或裸名 `get_goal`）。
5. 如果 `goal` 为 `null`，调用 `mcp__nexus_goal__create_goal`（或裸名 `create_goal`）。
6. 如果已有 Goal，说明当前目标，不创建新 Goal。

反例：“启动 Goal，写一篇 1000 字作文。”主题、用途或文体尚未明确，会直接改变交付物；应先提问，不能先创建 Goal。

可创建示例：“启动 Goal，为高二语文作业写一篇约 1000 字的议论文，主题是人工智能是否会削弱独立思考，立场明确，包含两个论据，完成后检查字数和结构。”这已经足以形成可直接执行的 objective。

示例：

工具：`mcp__nexus_goal__create_goal`（或裸名 `create_goal`）

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

### 纠正 Goal objective

仅适用于用户明确表示此前 Goal 目标说错了、要改成另一个 objective，或明确要求替换当前 active Goal 的目标。普通追问、补充细节、执行建议和模型自行判断不触发重定向。

1. 调用 `mcp__nexus_goal__get_goal`（或裸名 `get_goal`）确认当前 active Goal。
2. 调用 `mcp__nexus_goal__retarget_goal`（或裸名 `retarget_goal`），传入纠正后的完整 objective。
3. 保留同一个 Goal；不要把旧 Goal 标记 complete，也不要创建新 Goal。

```json
{
  "objective": "分析 M4 和 M5 芯片的差异，并给出适用场景"
}
```

### 完成 Goal

适用：

- 目标已完成
- 所有必要验证已做完
- 没有剩余必须继续处理的问题

完成前必须通过共享 Objective Alignment 契约做一次结构化审计。服务端提供权威 objective 与 completion criteria；模型逐项提交状态、证据或缺口。不要因为已有进展、测试看起来相关、预算接近耗尽或准备停止而标记完成。

先调用审计工具：

工具：`mcp__nexus_goal__audit_objective_alignment`（或裸名 `audit_objective_alignment`）

审计使用当前工具 schema 要求的单个 `report_json` 字符串。`aligned` 表示所有标准均有可复查证据；`not_aligned` 表示存在明确缺口，应继续工作；`inconclusive` 表示证据不足，应先补证。审计本身不改变 Goal 状态，也不能替代完成工具。只有当前 objective revision 和当前 round 保存的 `aligned` 报告可以支持紧接着完成。

审计返回 `aligned` 后，再调用 Goal MCP 更新工具，不是只回复文字：

工具：`mcp__nexus_goal__update_goal`（或裸名 `update_goal`）

```json
{
  "status": "complete"
}
```

工具成功后的下一条最终回复是用户真正的交付面，不是 Goal 状态回执。该回复必须脱离过程消息也能独立满足 objective：

- 作文、报告、方案、文案等文本本身就是交付物时，直接完整展示正文，不能只说“已经交付”或概括主题、结构和字数。
- 成果位于文件或其他产物时，给出准确链接或路径、核心结果和必要验证；实现、研究或外部操作类任务应说明真正完成了什么以及如何确认。
- 以成果本身为重点，不要把“Goal 已完成”放在开头或用简短总结替代结果。完成状态最多在结果之后作为次要说明，也可以省略，因为界面已经展示状态。
- 不要让用户回看 thinking、工具过程或更早的零散回复来拼凑最终结果。

完整交付后停止并等待用户输入，不要继续调用工具或开启新工作。不要默认复述 `completionUsageCheckpointReport` 或兼容字段 `completionBudgetReport`，也不要主动同时展示 actual/budget token、耗时或最终回复延迟结算说明；详细用量留给结构化 API 与审计界面。

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

工具：`mcp__nexus_goal__update_goal`（或裸名 `update_goal`）

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

需要定时任务时转用 `nexus_automation`；需要 Room 协作时转用 `nexus-manager` 或 Room skill。

## 回复要求

- 创建成功后，用一句话确认当前 Goal，不解释底层工具。
- 重定向成功后，用一句话确认已按用户纠正更新同一个 Goal，然后继续执行新 objective。
- 已有 Goal 时，说明已有目标并给出下一步选择。
- 完成或阻塞后，按工具结果简短说明状态。
- 不向用户展示 JSON 参数，除非用户明确要求看调用细节。
