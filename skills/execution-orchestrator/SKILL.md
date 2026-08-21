---
name: execution-orchestrator
description: 为 substantial task 选择最小充分执行结构，并在当前 round 的 Nexus Execution/WorkGraph authority 下规划、分派、交付、审核、恢复或保存可复用命名工作图。简单直接任务不因复杂度标签机械进入托管图。
---

# Execution Orchestrator

Execution 管理“当前责任如何交付”；Goal 管理“什么目标需要跨轮持续追求”。两者可以独立存在，也可以显式绑定，不以任务长度、Room 人数或是否调用 Subagent 相互推断。

## 入口与命令协议

1. substantial execution 前先判断直接执行、局部 Task、Subagent、WorkGraph、Room Assignment、Gate/Loop 或 Goal 中哪些结构真的降低风险；简单原子任务直接完成。
2. 只有需要读取或改变受管责任图时才调用宿主注入的 `NEXUS_COMMAND_PATH`。先独立执行：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution inspect
   ```

   明确读取同一可信 scope 的历史图时才加 `--execution-id '<execution-id>'`。PowerShell 使用 `& "${env:NEXUS_COMMAND_PATH}" ...`，不要混用 shell 变量语法。
3. 只从最新 `data.execution_context.allowed_actions` 选择动作。mutation 前读取 fresh exact contract：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract --operation '<operation>'
   ```

   完整遵守返回的顶层 `command_usage`、`contract` 和 `input_staging`；不要凭 Skill、记忆或旧 round 重建命令、字段、路径、identity、authority 或 revision。受管命令必须是无管道、重定向和后处理的单进程调用。
4. 输入是 `additionalProperties=false` 的 closed JSON object。只写 fresh `input_schema.properties` 中属于当前意图的字段；opaque locator 来自 inspect/receipt，不从标题或正文猜。相同语义重试复用 request ID，operation、目标或输入变化时换新 ID。
5. 只有顶层 `is_error=false` 且 `data.outcome=applied` 表示 mutation 已应用；`prepare_plan_execution` 成功是 `data.outcome=prepared`。`next_actions` 是建议，不授权，始终服从同一结果里的最新 lane、binding 和 `allowed_actions`。

`context_status=refresh_required` 时在本轮重新 inspect；`round_refresh_required` 表示旧 round authority 已失效，立即结束本轮，不再 inspect、改 Plan 或重试 mutation，等待宿主 successor round。

## 按当前动作读取参考

- 选择直接执行、Task、Subagent、Room 或 WorkGraph：[references/structure-selection.md](references/structure-selection.md)
- 创建、replan、replace、abandon 或提交 sealed Plan：[references/graph-control.md](references/graph-control.md)
- assign、submit、review 或 takeover：[references/responsibility-and-delivery.md](references/responsibility-and-delivery.md)
- block、resume、Execution audit、Goal promotion 与跨域收口：[references/recovery-and-alignment.md](references/recovery-and-alignment.md)
- 保存或复用命名 WorkGraph Slash：[references/workgraph-distillation.md](references/workgraph-distillation.md)
- Room/父子 Agent 的内容传递、并行与连续执行：[references/communication-and-continuity.md](references/communication-and-continuity.md)

只完整读取当前决策需要的参考；不要为调用一个 operation 加载全部说明。

## 不变量

- 图 materialize 前不能 assign；持久责任先建 Work Item，再由最新 context 建 Assignment。裸 `@` 只用于讨论或一次性帮助。
- observation 只授予读取共享图的可见性，不授予 coordination、WorkBinding、ReviewBinding、Submission 或 Plan mutation。
- Goal+WorkGraph 创建必须串行：先由 `goal-manager` 创建 Goal 并取得 applied receipt，再用 `goal_binding=current` prepare Plan。已有 transient WorkGraph 后出现明确 Goal 意图时使用 `promote_execution_to_goal`，不创建第二张图。
- required Work Item 的最终 accepted review 可以自动终止无 blocker Execution；确认绑定 Goal 时，再按 receipt 切到 Goal domain 执行 `audit_objective_alignment` 与 `update_goal`。`execution/audit_execution_alignment` 只是非终态 Execution 的可选 Gate，不能替代 Goal 审计。
- `/workgraph` 只启用当前 WorkGraph 协作；只有宿主在用户确认草图后启动 `HiddenFromUser` 的内部 `workgraph_distillation` round，并提供 exact `preview_id` 时才调用 `distill_workgraph`。该调用只原样保存已确认预览，不得重新 inspect、选节点、命名或抽象；不得要求或制造一条可见聊天消息，预览失效时请用户重新生成，不能绕过。
- mutation、分派和交付推进到真实结果、明确外部 blocker 或终态；不因 handoff 要求用户发送“继续”。
