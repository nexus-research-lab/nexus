---
name: goal-manager
description: 在当前 Nexus round 中创建、读取、明确纠正、审计、完成或阻塞 durable Goal。只处理用户或系统明确的跨轮目标生命周期；普通任务、提醒和执行图责任分别使用直接执行、Automation 或 Execution。
---

# Goal Manager

Goal 是跨物理 round 持续追求的服务端目标，不是普通聊天、Task、WorkGraph 或 Automation 的别名。Objective 说明最终要实现什么；Execution 另行记录责任如何交付。

## 入口与工具协议

1. 先通过 `nexus.goal_read` 读取当前 Goal：

   ```json
   {}
   ```

   不要探测命令路径、临时文件或环境变量，不要使用 `nexusctl`、其他管理入口或 `/goal` 文本代替工具。
2. 读取最新 Goal/objective revision 与 completion criteria 后，从 `nexus.goal_write` 当前 schema 选择 operation 并直接提交字段：

   ```json
   {"operation":"<operation>","<field>":"<value>"}
   ```

   工具 schema 就是当前合同，不再调用 contract 工具。
3. 输入是 `additionalProperties=false` 的 closed object，只写当前 operation schema 暴露的业务字段。不要提交 Goal/owner/Agent/Session/Room/round identity、authority、request ID、revision、receipt 或其他隐藏字段；request identity 由宿主从真实 tool use 生成。
4. 只有 `is_error=false` 且 `data.outcome=applied` 表示状态已改变；`nextAction` 只给 domain-qualified 恢复方向，不携带新 authority，继续前仍读取目标 domain 的 current state。

## 按当前动作读取参考

- `nexus.goal_read` 结果为空且有显式 Goal 意图，或用户明确替换 current objective：[references/create-and-retarget.md](references/create-and-retarget.md)
- 判断 Goal complete/blocked，提交 Goal Objective Alignment，或处理完成拒绝：[references/complete-and-block.md](references/complete-and-block.md)
- shared Room Goal 的 Lead、协作与关闭条件：[references/room-goals.md](references/room-goals.md)

只读取当前动作需要的参考；operation 字段、枚举和长度始终以当前工具 schema 为准。

## 不变量

- 创建前 objective 必须完整、具体、execution-ready；仍缺少会实质改变结果的信息时先取得最少必要输入，不创建占位 Goal。
- current Goal 存在时不创建第二个。用户明确纠正 objective 时 retarget 同一 Goal，保留 identity 与累计用量。
- `token_budget` 只在用户明确给出正数预算时设置；暂停、恢复、预算和用量限制属于用户或系统控制面。
- Plan Mode 中 Goal mutation 只做 validation，不改变 Goal、Execution、Plan 或 cancellation state；需要持久化时离开 Plan Mode，重新读取 current state 后再调用 `nexus.goal_write`。
- `update_goal` 只标记 `complete|blocked`，不创建、retarget、暂停或改预算。complete 只在 objective 真实满足且没有 required work 时使用；blocked 必须满足连续三次同一 blocker 的模型行为门槛。
- 提醒、定时与周期任务使用 Automation；WorkGraph 责任使用 `execution-orchestrator`。
- `goal/audit_objective_alignment` 是 confirmed Goal+WorkGraph 的完成审计；`execution/audit_execution_alignment` 只是非终态 Execution Gate，不能互换。
