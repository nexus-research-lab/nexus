---
name: execution-orchestrator
description: 选择执行结构、委派子智能体、管理 Nexus 工作图与可复用模板；遵守本轮权限，简单任务直接完成。
---

# Execution Orchestrator

Execution 管理当前责任交付；Goal 管理跨轮目标。按需独立选择或显式绑定，不按任务长度或人数推断。

## 入口与命令协议

1. substantial execution 前先判断直接执行、局部 Task、Subagent、WorkGraph、Room Assignment、Gate/Loop 或 Goal 中哪些结构真的降低风险；简单原子任务直接完成。
   子任务使用 `nexus.command/subagent`，按下方参考操作；不要求先创建工作图。
2. 执行当前任务的责任图时，先调用宿主提供的 `nexus.command`：

   ```json
   {"domain":"execution","action":"inspect"}
   ```

   明确读取同一可信 scope 的历史图时才传 `input.execution_id`。
   inspect 不回放成功工具历史；恢复优先使用交付、验收与产物证据，避免重做。
   查询、提取、编辑、选择或保存模板草图则直接走 WorkGraph authoring 参考；这些操作不要求 active Execution，也不先运行 execution inspect。
3. 执行图从 `data.execution_context` XML 的 `<allowed_actions>`、lane 和 binding 判断权限；模板草图使用 authoring 目录。mutation 前读取 fresh exact contract：

   ```json
   {"domain":"execution","action":"contract","operation":"<operation>"}
   ```

   字段、identity、authority 和 revision 以本轮 contract 为准。
4. 业务输入是 `additionalProperties=false` 的 closed object，直接放在工具的 `input` 字段中。只提交 fresh `input_schema.properties` 中属于当前意图的字段；opaque locator 来自 inspect/receipt，不从标题或正文猜。相同语义重试复用 `request_id`，operation、目标或输入变化时换新 ID。
5. 先检查 `is_error`；Execution 操作再按 operation 解读 `data.outcome`：`applied` 表示已应用；`no_op` 无新增变更，结合 message 区分重放与 Plan Mode 校验；prepare 成功为 `prepared`，草图提取为 `ready`，版本选择为 `selected`。`rejected`/`superseded` 不能当成功。`next_actions` 是建议，不授权，始终服从同一结果里的最新 lane、binding 和 `allowed_actions`。

`get_execution` 只用 `action=inspect`，省略 `operation/request_id`，不走 invoke。Room 当前负责人自行 inspect 恢复协调权限，成员只获得观察。lane/background 不是界面模式；不要让用户切模式或再发“开始”。

`context_status=refresh_required` 时在本轮重新 inspect；`round_refresh_required` 表示旧 round authority 已失效，立即结束本轮，不再 inspect、改 Plan 或重试 mutation，等待宿主 successor round。

## 按当前动作读取参考

- 选择直接执行、Task、Subagent、Room 或 WorkGraph：[references/structure-selection.md](references/structure-selection.md)
- 派生、读取、等待、续聊或停止子智能体：[references/subagents.md](references/subagents.md)
- Plan 创建、替换、放弃或提交：[references/graph-control.md](references/graph-control.md)
- assign、submit、review 或 takeover：[references/responsibility-and-delivery.md](references/responsibility-and-delivery.md)
- 恢复、审计与 Goal 跨域收口：[references/recovery-and-alignment.md](references/recovery-and-alignment.md)
- 工作图查询、草图编辑与模板复用：[references/workgraph-distillation.md](references/workgraph-distillation.md)
- Room/父子 Agent 的内容传递、并行与连续执行：[references/communication-and-continuity.md](references/communication-and-continuity.md)

只完整读取当前决策需要的参考；不要为调用一个 operation 加载全部说明。

## 不变量

- 图 materialize 前不能 assign；持久责任先建 Work Item，再由最新 context 建 Assignment。裸 `@` 只用于讨论或一次性帮助。
- observation 只授予读取共享图的可见性，不授予 coordination、WorkBinding、ReviewBinding、Submission 或 Plan mutation。
- Goal+WorkGraph 创建必须串行：先由 `goal-manager` 创建 Goal 并取得 applied receipt，再用 `goal_binding=current` prepare Plan。已有 transient WorkGraph 后出现明确 Goal 意图时使用 `promote_execution_to_goal`，不创建第二张图。
- required Work Item 的最终 accepted review 可以自动终止无 blocker Execution；确认绑定 Goal 时，再按 receipt 切到 Goal domain 执行 `audit_objective_alignment` 与 `update_goal`。`execution/audit_execution_alignment` 只是非终态 Execution 的可选 Gate，不能替代 Goal 审计。
- `/workgraph` 只启用当前协作。模板查询、提取、编辑、版本选择与保存走 authoring 参考，已有 Draft 直接复用。修改后展示草图，不机械增加确认；保存须展示后的明确确认。UI 保存直接提交宿主事务，编辑结果不回传来源聊天。
- mutation、分派和交付推进到真实结果、明确外部 blocker 或终态；不因 handoff 要求用户发送“继续”。
