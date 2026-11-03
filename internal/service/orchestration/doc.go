// Package orchestration 实现 Goal 可选绑定的 Execution 与 WorkGraph 状态机。
// 当前产品语义以 docs/specs/execution-orchestration-spec.md 为准；Runtime Graph
// 与 Web 投影以 docs/specs/execution-graph-spec.md 为准。本头部只保留代码地图。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员地图：
//   - user_recovery.go：用户 exact 节点同 owner/reviewer 续跑、事务回执读取与出队前复核。
//   - runtime_result.go：结构化交付/审核的 durable intake、精确领域结果证明与重启对账；不从运行完成推断交付。
//   - root_recovery.go：启动 root 孤儿收口、DM 精确终态、成员恢复和非 Goal 协调恢复；后台审计从 terminal receipt 补偿队列写入失败；逐个 Execution 隔离失败，恢复目标不可用时跳过；只唤醒宿主留下在途工作的 Execution。
//   - service.go / errors.go / work_binding.go / coordination_round.go：装配、actor、
//     Work/Review/Coordination capability 与 optimistic revision fence；终态历史读取不改本轮协调权限。
//   - execution*.go / goal_retarget.go / plan_validation.go：Execution 生命周期、
//     predecessor/successor、Plan revision 与 DAG 校验。
//   - plan_document*.go / plan_proposal*.go / plan_materialization.go：严格 Plan
//     document、非权威 sealed proposal、跨 round exact binding、原子 materialization 与重启恢复。
//   - command_plan.go / command_assignment.go / command_assignment_batch.go / command_submission.go / command_review.go /
//     command_control.go：按计划、单项/原子批量分配与接管、提交、审核、阻塞与恢复及完成聚合命令；批量复用分配规则并从 durable receipt 恢复精确 self binding；
//     commands.go 保留共享授权、引用解析、失败根 Attempt 的协调者接管建议与结果投影，不改变原子命令和事务边界。
//   - runtimehook/：DM 与 Room 共用的 SDK hook 及运行观察适配；宿主提供可信 actor，适配器统一观察超时和失败日志。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go /
//     room_attempt_terminal.go：Room work、review 和 physical cancellation outbox，
//     以及 dispatched/self WorkBinding 共用的 root Attempt 终态桥。
//   - background_coordinator.go：Execution 三类 outbox、Subagent deadline 与四类
//     durable saga 共用的 startup + mutation wake + exact deadline + bounded audit 控制面。
//   - subagent_admission.go：Subagent admission、child Attempt、parent-exit deadline、
//     command subagent/spawn 的准入投影、coordinator 唤醒与重启 orphan 对账。
//   - runtime_graph*.go / execution_view.go / context.go / execution_alignment.go：
//     Runtime Graph 事实、actor context、Subagent admission 与 command-surface authority 对齐、目标对齐，以及按每个 root Attempt 与 immutable Submission/Gate 保留轮次、再按 exact Attempt/Submission/round 合并运行历史的 managed WorkGraph 当前/显式历史只读投影。
//   - goal_policy.go / promotion.go / explicit_goal.go / goal_binding.go /
//     goal_confirmation_recovery.go：Goal promotion、双向 binding 五态与 durable
//     confirmation receipt/reconciler。
//   - completion_audit_recovery.go：accepted Review 后的 blocker-aware durable
//     completion reconciler；不替代模型可见 alignment audit。
//   - invalidation.go / result.go：owner/session 只读投影失效 port、宿主消费的
//     responsibility advancement receipt，以及 final Acceptance 到 Goal completion audit
//     的跨 domain 稳定 mutation outcome/next-action envelope。
//   - repository_contract.go：生产 SQL repository 的消费侧编译期接口检查。
//   - runtime_graph_history.go：仅供版本化迁移使用的 exact Subagent 历史纯投影；普通 view 不读取 transcript。
//   - prompt.go / prompt_policy.md：DM、Room 与 Goal continuation 共用执行提示。
//
// 主要暴露接口：NewService 与 Service 的 Ensure/Get*/Read*/RuntimeContext、Plan、work、
// review、Goal binding/promotion、Room WorkGraph/Runtime Graph observation、deadline coordinator 和 recovery 方法；
// Set*Gateway/Consumer/Sink 只注入 Goal、Room、runtime 与 transport 的消费侧 port。
//
//   - runtime_graph_artifact.go：保留消息文件的 working_file/deliverable 角色与产出 Agent/round，按工具身份关联图记录。
//   - runtime_graph_command.go：消费 RuntimeCommandFact，核心不依赖 MCP；回执过滤与复制由 runtimehook 完成。
//
//   - dispatch_policy.go / scheduled_dm.go / checkpoint.go / workflow_source.go：显式依赖派发、DM 精确入队激活、进展记录与模板来源校验端口。
//
// [PROTOCOL]: 行为或 wire 变化时检查两份 execution spec、protocol L2 与 AGENTS.md。

package orchestration
