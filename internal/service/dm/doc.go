// Package dm 编排 DM（单 Agent 私聊）会话的写入、运行时轮次与队列/中断/续跑。
// 持有宿主版本化配对回信目标的本地 DM 可以回传外部渠道，普通本地会话不自动外发。
// 已确认的 round 完成状态先于上下文占用查询广播；指标仍在本轮内顺序保存，避免旧轮次覆盖新值。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - recovery.go：启动队列扫描（跳过主窗口，改为唤醒 inbox 定时器）、非 Goal 自动/用户节点恢复输入（主窗口写入 inbox）、持久受理查询与精确 DM root Attempt 收口。
//   - dispatch_lock.go：按会话的派发边界锁（引用计数回收、可随 ctx 取消）；不同会话的起轮、投递与引导 hook 互不阻塞，锁不可重入。
//   - wiring.go：RequireWiring 在启动时校验生产必需依赖，业务方法不再为缺失依赖降级。
//   - Service 嵌入 runtimehost.Host：Provider/admission/队列信任/用量/额度/执行上下文/MCP 工厂等共用依赖、注入方法与共用阶段只在 runtimehost 实现一次。
//   - im_delivery_reply.go：IM 反馈持久幂等入队（主窗口进 inbox，其他 DM 进 JSONL 队列）、派发前复核与一次领取；反馈新轮次不继承旧轮次权限。
//   - service.go / request.go / guidance_input.go / round*.go：写请求阶段状态、直接或 queue/guide 物化的首条 Room DM 用户消息消费 conversation draft，与运行时轮次编排；同步准备响应请求与准入取消，持久受理由 round 持有；round 登记覆盖 runtime 准备，失败完成收尾，避免交接期间被空闲回收；完整回复落盘后写入独立摘要，编辑重发先失效旧摘要；/plan 在 runtime 准备阶段覆盖本轮权限。
//   - input_queue.go / running_input.go / guidance_input.go / interrupt.go：durable 幂等受理、admission 暂时失败时保留并允许原请求重试恢复、先 ACK 后异步启动与下一轮队列、hook applied ACK 后消费引导、错过 hook 的接力与中断。
//   - main_inbox.go：主智能体主窗口的 inbox 路径——受理/撤回（用户开口即取消待定 Echo 并恢复暂停闸门）、空闲时按种类规格取批开跑、运行中投递（running_input_v1：input_consumed 回执后 consumed；未协商时 PostToolUse 安全点注入、applied 后 consumed）、轮末释放与续跑、启动恢复；不经过 input_queue.go 的队列与 admission。
//   - main_inbox_kinds.go：inbox 事件种类规格表（是否宿主事件、能否交给运行中的轮次、提示词与分隔条元数据、轮次 ID 是否由事件派生、是否受暂停闸门约束、起轮入口），派发、合并与注入只查表不分支。
//   - main_inbox_host.go：定时任务与跟进的宿主 run 起轮、带来源包装的模型输入与时间线触发分隔条元数据；宿主事件独占一轮、不进入待发送列表。
//     空闲独占的宿主 run 是 Announce + KeepTranscript 的延迟提交轮：只提交最终回复，沉默（HostNoReplyMarker）/失败/中断记为 host_run_outcome；宿主 run（含 Echo 轮）期间不接收运行中输入。
//   - main_inbox_timer.go：启动退回遗留 delivering，按 inbox 最近 available_at 唤醒到点的未来事件（duework）。
//   - main_inbox_follow_up.go：agent 自设跟进的设定/列出/取消（5 分钟–7 天、至多 10 条）、到点输入与分隔条元数据，以及待触发跟进的上下文告知。
//   - main_inbox_recovery.go：主窗口的宿主恢复（host_recovery）与 IM 回执（im_reply）——暂停闸门的停止/恢复代数、幂等入队与同类合并、续跑代数校验、起轮前陈旧与重复检查、IM 回执先 consumed 后起轮。
//   - main_inbox_echo.go：主窗口 Echo——可见用户轮成功后写入 echo 事件，到点认领后锁外跑 EchoPolicy 判断，CAS 确认后起隐藏无工具延迟提交轮，提交前遇无回复标记或待处理用户输入即丢弃。
//   - deferred_assistant.go：延迟提交轮（DeferredAssistant）的终态收口、丢弃与广播。
//   - goal_command.go：UI set_goal 与 `/goal` 共用的 Goal 写入、完成态控制 marker、会话 started/title 基础事实；不创建普通模型 round。
//   - goal_continuation.go / goal_runtime.go / goal_completion_receipt.go：含旧显式 Goal 确定性 reservation 恢复的 exact Goal/revision/Execution 续跑启动 claim、上下文、消费后 revision adoption、live scope create guard、parent terminal ledger、child lifecycle evidence、fenced 结算与最终回复完成收据。
//   - history.go / fork.go / transient_fork.go / transient_session.go / rewrite.go / title.go / transport_context.go / execution_context.go / context_usage.go：历史、同 Agent 延迟物化的 conversation fork、仍需继承上下文的嵌入场景所用且跳过 overlay-only/局部投影的隐藏 fork、不继承源 transcript 且供 WorkGraph 编辑/保存使用的目录隐藏专用 Session、provider-specific 边界兼容、先提交 Room SQL 再投影 workspace 的 SDK session/fingerprint 同步、区分 transcript rewrite 与启动失败 overlay-only rerun 的重写、标题、Goal/Automation/上一轮失败恢复上下文、active-paired IM 的无标识只读 transport 上下文、每轮权威 WorkGraph 上下文与 compact 持久证据，以及每轮终态上下文占用快照持久化。
//   - attachments.go / broadcast.go / external_reply.go / command_input.go / connector_context.go：附件、广播、外部回复、Connector 配置、当前 Session 选择与 GitHub 等挂载别名的可信动态上下文，以及把 DM 查看权限与仅 WebSocket 可用的 host Slash 权限分离的无副作用校验。
//   - quota.go / subagent_task.go / runtime_client.go / runtime_settings_preparation.go：账号额度门禁与 Goal 限制投影、子任务、带 Execution-aware Agent hook 装配，以及 exact 临时 Session 的受限 system prompt / tool policy，
//     Connector 选择提交后按 Session latest-wins 预备工具面 fork，真实输入仍同步兜底；工具面变化时从旧 transcript 幂等 fork 新物理 Session，并签发 nexuscfg 与 Agent-facing nexus command 的 physical-round capability；active-paired
//     外部私聊复用同 Agent Skill，provider init/fork 后的 SDK Session identity 动态写回同轮 command context，受控 Automation 执行可覆盖创建时工具快照且 CLI 只读绑定当前 job/run。
//     Goal continuation 只有在 durable claim 后携带 host-only exact continuation authority，普通 internal round 不得借用该能力。
//
// 运行开始、消息、命令回执、附件、结束及 compact 证据共用 orchestration/runtimehook.Observer；Goal 快照转换共用 goal/runtimeusage，子任务证据合并与确认由 Goal 观察值负责。本包保留可信身份、锁、结算屏障和会话编排。
//
//   - runtime_wake.go：idle drain 收到 runtime 自行开启的顶层 turn（system/init）时退出 drain，以不发 query 的内部 round 接管同一消息流，拥有完整 hook/权限/终态生命周期；启动失败回退旧 drain。
//
//   - workgraph_dispatch.go：主窗口 inbox 或 JSONL 持久队列承接精确工作/审核绑定，遵守暂停闸门与出队授权；round.go 对支持消息策略的内部轮次关闭自动记忆抽取。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）

package dm
