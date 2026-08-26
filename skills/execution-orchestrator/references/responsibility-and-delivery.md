# Responsibility 与交付

只在调用 `assign_work`、`submit_work`、`review_work` 或 `take_over_work` 时读取本文件。字段、枚举、长度和数组上限只认当前工具 schema；这里保留 binding、locator 与状态转换规则。

## Locator 与 authority

- `execution_id`、`work_item_id`、`logical_key`、`assignment_id`、`submission_id` 都是 opaque locator。多个 locator 同时出现时必须指向同一对象；显式 locator 只确认目标，不授予 authority。
- 只有 exact trusted WorkBinding 可以为 submit 供应 Work Item/Assignment；只有 exact ReviewBinding，或服务端允许的 self-review WorkBinding，可以为 review 供应目标。`assigned_work`、`current_actor`、coordinator 身份和 observation 都不能替代 binding。
- 在 DM coordination 或任何 unbound round，按当前 schema 显式提供所需 Work Item/Submission locator；不要靠省略字段试探服务端默认。

## assign_work

- 只由最新 context 的 coordinator 调用，定位 active Plan 中 Ready、没有 current Assignment 的 Work Item。`target_agent_id` 必填；显式选择 `strategy=self|room_member`。
- `self` 的 target 必须是当前 actor，不能携带 Room dispatch。Room coordinator 的 self Assignment applied 会在同一 physical round 安装 exact WorkBinding，并把 lane 切到 work。
- `room_member` 只用于 Room Execution，target 不能是当前 actor；`dispatch_kind` 只能按 schema 选择 directed/public，`return_to_agent_id` 决定 selected reviewer。`instruction` 只补充 handoff，不替换 immutable deliverable 与 acceptance criteria。
- coordinator 先串行分派当前 Ready 的其他成员责任，最后再 self assign。每次 mutation 后消费最新 context；进入 work lane 后不能继续分派兄弟节点或修改 Plan。

## submit_work

- 只由 current Assignment owner 调用。`result_summary` 描述具体交付；`result_refs` 与 `evidence` 只写真实、可复查的引用。
- exact WorkBinding 可按 schema 省略 locator；unbound round 必须提供 Work Item locator，`assignment_id` 不能单独替代它。所有显式值必须匹配 current Assignment/binding。
- `waiting_input` 必须先 resume；已有未审核 Submission 时不重复提交。Submission 建立 immutable Gate，不能通过重交绕过审核。

## review_work

- Room 中只由 Assignment 的 selected `return_to_agent_id` 审核；self-review 也必须有允许它的 exact binding。
- `decision` 只按 schema 选择。accepted 必须逐条精确复制 immutable acceptance criterion，在 `criteria_results` 中全部给出 `passed=true`，并附可复查证据；漏项、重复、文本不一致或任一未通过都不能 accepted。
- rejected/changes_requested 不伪造通过结果，用 `feedback` 给出拒绝原因或具体改动。Acceptance 只作用于当前 immutable Submission Gate。

## take_over_work

- 只由 coordinator 对已有 current Assignment、且没有未审核 Submission 的 Work Item 调用。提交 replacement `target_agent_id` 和具体 `reason`；strategy、dispatch、reviewer 与 assign 规则相同。
- 该 operation 原子释放旧 Assignment 并创建 replacement。不要自行拆成两次状态变化，也不要在 work/review lane 依靠 coordinator 身份越权调用。
