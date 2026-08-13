# 完成与阻塞 Goal

只在判断 Goal 是否应该进入终态时读取本文件。

## 完成

只有 objective 已实际实现、所有必要验证完成且没有剩余必需工作时，才尝试完成。预算接近耗尽、准备停止或已有部分进展都不是完成证据。

确认绑定 managed WorkGraph 的 Goal 必须在当前 objective revision 和当前 runtime round 先调用 `audit_objective_alignment`。Goal-only 在成果确实完成后可直接调用 `update_goal`；reserved Execution identity 不把它变成受管 WorkGraph。需要审计时，按照工具 schema，把每条权威 completion criterion、状态、证据或缺口组织成一个 JSON 对象，再整体序列化进单个 `report_json` 字符串。

- `aligned`：全部标准有可复查证据；随后立即调用 `update_goal({"status":"complete"})`。
- `not_aligned`：存在明确缺口；继续执行。
- `inconclusive`：证据不足；先补证。

审计只记录证据，不完成 Goal，也不选择工作路线。完成时后端始终校验 Goal revision、Room 责任和运行状态；只有当前 Goal 确认绑定已物化 WorkGraph 时才额外校验 Execution/WorkGraph。reserved Execution ID 不是绑定证据。

## 完成后的最终交付

`update_goal` 成功后的下一条最终回复必须脱离过程消息也能独立满足 objective：

- 文本本身是交付物时，完整展示正文；
- 文件或产物是交付物时，给出准确链接或路径、核心结果和必要验证；
- 实现、研究或外部操作类任务，说明真实成果和确认方式。

不要把“Goal 已完成”放在开头，不用状态回执或简短摘要替代成果，也不要要求用户回看 thinking 或早先零散回复。完整交付后停止，等待用户输入。

## 阻塞

只有同一个具体阻塞条件在连续 Goal 续跑中重复出现，且没有用户输入、权限或外部状态变化就无法继续时，才标记 `blocked`。当前产品阈值是至少连续三个 Goal turns；blocked Goal 被用户恢复后重新计算这一审计窗口。

不要因一次澄清、不确定、任务困难、执行缓慢或暂时缺证就阻塞。达到阈值后调用 `update_goal({"status":"blocked"})`，并向用户说明具体缺口；不要一边持续报告阻塞，一边让 Goal 保持 active。当前 `update_goal` 只接收终态并由后端校验 Goal/Room/revision 权限，连续三轮是模型必须遵守的行为策略，不是该 status-only 调用能够自行推断的服务端审计。
