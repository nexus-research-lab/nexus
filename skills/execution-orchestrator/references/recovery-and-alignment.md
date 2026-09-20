# 恢复、对齐与 Goal bridge

只在调用 `block_work`、`resume_work`、`audit_execution_alignment`、`promote_execution_to_goal`，或处理入口错误、Execution/Goal 收口时读取本文件。字段与枚举以 fresh exact contract 为准。

## 固定 inspect 入口与调用纠正

- 工具 contract 的 `inspect_operation` 标识固定入口：`execution/get_execution` 调用 `{"domain":"execution","action":"inspect"}`，`goal/get_goal` 调用 `{"domain":"goal","action":"inspect"}`；省略 `operation`，无需 `request_id`。`next_actions` 中的这两个语义名称也按此映射，不能机械复制成 invoke。
- 误用 invoke，或 inspect 仍携带 operation 时，按错误返回的 JSON 修正调用；只保留该操作 schema 允许的 input（如历史图的 `execution_id`）。这是入口错误，补 request_id、重复原调用或等待工具注册都不能解决，也不因此阻塞工作或要求用户重新开始。
- `action=contract, operation=get_execution|get_goal` 仍可读取精确 schema。其他 operation 按 contract 调用；名字含 inspect/get 并不意味着使用固定 inspect 入口。隐藏草图编辑会话只使用其已绑定的 revise/select 目录。
- 入口纠正不授予权限。仍服从最新 lane/binding；`round_refresh_required` 继续结束旧轮，不把入口纠正当作重新取得 authority 的办法。

## DM Goal continuation 的 exact binding

DM 自动续跑必须由宿主在 durable continuation plan 完成 `validate` 和 `claim` 后签发
host-only authority，并逐字段绑定：`owner`、`Agent`、DM `session`、`goal_id`、
`objective_revision`、可选 `execution_id` 和 `root_round_id`。同一轮的 Goal authority
与 Responsibility authority 必须同时匹配；只看到一个 Execution ID、Goal 卡片或
`agent_internal` source label 都不构成执行权限。

这套 binding 只修复 DM 的 continuation 入口，不把所有内部 round 放行。queue、echo、
automation、外部 IM 和普通 internal round 仍然按各自 source policy 处理。Room 续跑
继续使用 verified Room authority；它与 DM 使用同一组 Goal/Execution service，但保留
独立的 `room_id/conversation_id` 边界。

因此，DM continuation 的正常调用顺序仍是：

1. `execution` `action=inspect` 读取当前 Execution context。
2. 读取当前 operation 的 fresh contract；schema 中的 locator 只能来自 inspect 或 typed receipt。
3. 按 contract 调用 `assign_work`、`submit_work` 或 `review_work`。

模型不应把 owner、Agent、session、Goal、revision、Execution 或 round identity 填进
业务 input 来“补授权”。任何一项宿主绑定不一致都必须 fail closed；如果服务端返回
`round_refresh_required`，结束当前 physical round，等待新的宿主 continuation。

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
