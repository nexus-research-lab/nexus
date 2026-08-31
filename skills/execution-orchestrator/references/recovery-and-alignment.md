# 恢复、对齐与 Goal bridge

只在调用 `block_work`、`resume_work`、`audit_execution_alignment`、`promote_execution_to_goal`，或处理 Execution/Goal 收口时读取本文件。字段与枚举以 fresh exact contract 为准。

## block_work 与 resume_work

- block 只用于缺少具体外部输入或 authority；Plan dependency 由图自动管理，不是 blocker。由 current Assignment owner 或 coordinator 提交具体 `reason` 与 `needed_input`。存在未审核 Submission 时先 review，不能用 block 催审。
- resume 只针对 `waiting_input` Work Item，提交 blocker 已解决的 `resolution` 和至少一项真实 `evidence`。Work 已 open 时的 no-op 不是新 Attempt；resume 不创建 Assignment，也不复活旧 Attempt。
- exact WorkBinding 可按 contract 供应 locator；unbound DM round 必须显式定位 Work Item。Room conversational round 的显式 locator 不授予 mutation authority：verified coordinator 先调用 execution `action=inspect`（`get_execution`）进入 coordination，其他成员必须持有 exact WorkBinding。显式 locator 必须相互一致并匹配当前图。

## audit_execution_alignment

- 只用于 current、非 terminal Execution 的可选 Gate。逐条精确复制当前 completion criteria，按 contract 提交三态 decision、每项 status、证据或具体 gap，以及 aggregate summary。
- satisfied 必须有可复查的 `{ref,claim}`；unsatisfied/inconclusive 写清缺失结果或证据。
- 该 operation 不完成 Execution，也不是 Goal 完成证据。terminal Execution 拒绝此调用；不要因名称相近改用它替代 Goal audit。

## promote_execution_to_goal

- 只对 current active/waiting transient Execution，且最新 `allowed_actions` 开放该动作时使用。`execution_id` 只确认 current，`objective_proposal` 不能授予 authority。
- 用户或系统明确要求 durable Goal 时使用 `activation_reason=persistence_requested`；其他 reason 只表达真实 observed boundary，不能拿复杂度、Room 人数或模型偏好伪造持久化意图。
- 若 receipt 表示 binding commit 部分完成，使用相同语义输入和 request ID 恢复；不要创建第二个 Goal 或第二张 Execution。

## Goal + WorkGraph 收口

1. 让全部 required Work Item 交付并获得最终 Acceptance；最后一次 accepted review 会在无 blocker 时自动终止 Execution。
2. 只有 confirmed Goal binding 才继续读取 receipt 中 domain-qualified Goal action；切换到 `goal-manager`，在同一 physical round 读取 Goal exact contract 并执行 `audit_objective_alignment`。
3. aligned Goal audit 之后才执行 Goal `update_goal status=complete`。WorkGraph-only 到 Execution terminal 即结束；Goal-only 不走本流程。
4. Goal completion 被拒绝时按返回的 `domain + operation` 恢复：缺 Goal 证据回 Goal audit，未完成责任回 Execution inspect。不要靠相似 operation 名称猜测恢复路径。
