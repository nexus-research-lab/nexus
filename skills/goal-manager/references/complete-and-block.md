# 完成与阻塞 Goal

只在判断 Goal 是否应该进入终态时读取本文件。

## 完成

只有 objective 已实际实现、所有必要验证完成且没有剩余必需工作时，才尝试完成。预算接近耗尽、准备停止或已有部分进展都不是完成证据。

先区分三种生命周期，不能把 operation 名称相近当成同一件事：

- **Goal-only**：成果确实完成后直接执行 Goal domain 的 `update_goal`；reserved Execution identity 不把它变成受管 WorkGraph。
- **WorkGraph-only**：完成全部 required Work Item 与 Acceptance 后由 Execution 自己终止；没有 Goal 审计或 Goal 终态更新。
- **Goal + WorkGraph（confirmed binding）**：先让 required Work Item 全部交付并验收。最终 accepted review 会自动把无 blocker 的 Execution 置为 terminal，并在同一物理 round 的 receipt 中返回 `next_actions[].domain=goal, operation=audit_objective_alignment`。随后切到 Goal domain，读取 exact contract，在当前 objective revision 和当前 runtime round 执行 `audit_objective_alignment`，最后按其 `nextAction` 执行 `update_goal`。

`execution/audit_execution_alignment` 只是在 **current、非终态 Execution** 上记录可选 Gate；它不完成 Execution，也不是 Goal 完成审计。Execution 已 terminal 后绝不调用它。Goal 审计在服务端仍可先于 WorkGraph readiness 留证，这是旧 MCP 保留的幂等能力；但正常收口应遵循上面的顺序，避免拿尚未验收的事实声明 aligned。

需要 Goal 审计时，先读取该 operation contract，把每条权威 completion criterion、状态、证据或缺口组织成一个 JSON 对象，再整体序列化进单个 `report_json` 字符串。

- `aligned`：全部标准有可复查证据；随后立即按 command 流程执行 `update_goal`，输入 `{"status":"complete"}`。
- `not_aligned`：存在明确缺口；继续执行。
- `inconclusive`：证据不足；先补证。

Goal 审计只记录证据，不完成 Goal。完成时后端始终校验 Goal revision、Room 责任和运行状态；只有当前 Goal 确认绑定已物化 WorkGraph 时才额外校验 Execution/WorkGraph。reserved Execution ID 不是绑定证据。若 `update_goal` 被拒绝，直接按返回的 domain-qualified `nextAction` 恢复：`goal/audit_objective_alignment` 表示当前轮缺少或已失效的 Goal 对齐证据，`execution/get_execution` 表示图仍有未完成责任；不要盲重试 `update_goal`，也不要在 terminal Execution 上改调 `audit_execution_alignment`。

### 暂停状态的恢复说明

若 `inspect`、`audit_objective_alignment` 或 `update_goal` 表明当前 Goal 为 `paused`，结束当前收尾链路并进入用户恢复步骤。用户恢复后，Nexus 会把 Goal 重新激活为 `active` 并启动新的 continuation。

向用户说明恢复位置、操作和后续行为：

> 当前 Goal 已暂停。请在当前对话输入框上方的 Goal 状态栏中，点击右侧的 ▶「继续」按钮。恢复后，系统会自动重新调度该 Goal，智能体再继续完成审计和收尾。

用户点击「继续」后会开始新的 Goal continuation；该 continuation 通过 Goal 审计和 `update_goal` 完成后续收尾。

## 完成后的最终交付

`update_goal` 返回 applied receipt 后的下一条最终回复必须脱离过程消息也能独立满足 objective：

- 文本本身是交付物时，完整展示正文；
- 文件或产物是交付物时，给出准确链接或路径、核心结果和必要验证；
- 实现、研究或外部操作类任务，说明真实成果和确认方式。

不要把“Goal 已完成”放在开头，不用状态回执或简短摘要替代成果，也不要要求用户回看 thinking 或早先零散回复。完整交付后停止，等待用户输入。

## 阻塞

只有同一个具体阻塞条件在连续 Goal 续跑中重复出现，且没有用户输入、权限或外部状态变化就无法继续时，才标记 `blocked`。当前产品阈值是至少连续三个 Goal turns；blocked Goal 被用户恢复后重新计算这一审计窗口。

不要因一次澄清、不确定、任务困难、执行缓慢或暂时缺证就阻塞。达到阈值后按 command 流程执行 `update_goal`，输入必须同时包含四个字段：`{"status":"blocked","blocker_id":"<stable-id>","reason":"<concrete blocker>","needed_input":"<exact user input, permission, or external change>"}`。相同阻塞条件才复用同一个 stable `blocker_id`；条件变化就换 ID。随后向用户说明具体缺口；不要一边持续报告阻塞，一边让 Goal 保持 active。后端校验 Goal/Room/revision 权限并持久化明确恢复路径，但连续三轮是模型必须遵守的行为策略，不能靠 status 或 blocker 字段让服务端自行推断。
