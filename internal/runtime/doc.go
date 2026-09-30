// Package runtime 驱动 bridge runtime 的 round 执行与会话生命周期。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - sandbox_process_recovery_scan.go：持锁分页处理原 pending 进程，失败保留记录、游标继续，取消保留未处理项；不重放任务或清除策略/资源 unknown。
//   - autodream.go：一次性 AutoDream control 经共享启动事务、后台取消、精确 scratch 交接及终态退休；不拥有记忆领域规则。
//   - client.go：Client 接口、Factory 与 agentClient（宿主管理 Agent runtime 的能力边界），
//     并统一收口并发连接失败、永久撤销失去 Manager 所有权的 client、取消换代中的
//     connect/config RPC、识别关闭态控制错误及隔离未收口的 SDK 会话；清理失败不被
//     普通断管分类吞掉，也不释放重连或旧配置启动重试的栅栏。
//   - session.go / round.go / idle*.go / owner.go / interrupt.go / streaming_input.go / task.go /
//     goal_accounting.go：Manager 管理 session_key → SDK client、owner、运行中 round、
//     key 级启动与关闭栅栏、client 换代、lease 条件关闭、round keyed state、
//     兼容性在 GetOrCreate 前已失败时对既有 warm client 的 owner-fenced 退休、
//     idle 消息消费者租约，以及不占用全局锁的 owner epoch/reap flight；Goal accounting、
//     scope-aware Goal create guard、ClearGoalAccountingRounds 部分 activation 回滚与
//     objective revision adoption 均随 round state 统一清理；interrupt.go 额外区分唯一运行
//     round 的 provider interrupt 与 exact local context cancellation，并在 provider interrupt
//     窗口阻止 successor admission，共享 session 不回退为可能误伤 successor 的 interrupt；
//     Goal pause 使用 exact Goal/revision→round accounting identity 逐轮取消，不误伤同 session 其他工作。
//   - guidance.go / contextual_input.go / command_context.go / responsibility_authority.go / work_binding_state.go / goal_authority.go / goal_continuation_authority.go / subagent_hook.go：轮内引导、按 priority/name/content/metadata 确定性排序并绑定 user、由 nxs 保留在 live model history 而不落 transcript 的隐藏上下文与输入选项、runtime lease 与 Connector 选择、Goal/Execution/Work/Review 共用且由宿主 mutation receipt 原子推进的动态 responsibility snapshot（WorkBinding exact fail-close）、DM Goal continuation 的 host-only exact owner/Agent/session/Goal/revision/Execution/round binding、provider init/fork 后动态更新且 command 调用时读取的 SDK Session identity、Goal steering 与 mutation fence 分离，以及按 parent round/tool_use_id 冻结 lifecycle callback 的 Agent tool 强准入、迟到事件、固定 grace deadline 持久化、无上限退避 fallback 与重启时 process-cutoff orphan 对账。
//   - subagent_control.go：绑定 exact parent round 的 bridge 子智能体 capability 回连。
//   - diagnostics_env.go / cache_surface.go：诊断开关、stderr 归一化，以及不持久化
//     prompt/tool schema 明文、也不冒充 provider cache key 的宿主 tool surface 脱敏归因；
//     同一指纹也为不支持会话内动态工具更新的 runtime 提供保守 resume/reset 栅栏。
//   - goal_usage.go / task.go / context_usage.go：Goal actual/budget token
//     口径换算（含矛盾 provider 零 total 的 breakdown 回退）、跨 round 的 nxs child task 累计量去重，以及 runtime 权威上下文快照
//     的归一化与按 Session/Agent 热缓存；跨进程恢复由 Session 服务负责。
//   - lifecycle.go：session 关闭栅栏、保留失败结果与跨 core/exec 共用的 round 中断宽限。
//   - shutdown.go：宿主退出时永久关闭 Manager 准入，取消 round/后台任务，先等待在途启动和回执写入，再并行关闭全部 Session；重复调用等待同一结果，调用者超时不关闭仍有写入的数据库。
//   - process_policy.go：进程策略指纹，显式纳入不进入普通 settings JSON 的文件/搜索/本地媒体/远程图片网络/Skill/设置写入能力与资源要求。
//   - sandbox_policy.go：桌面托管沙箱跨 Full Access 边界时要求退休旧进程，不通过权限热更新伪装生效。
//   - sandbox_resources.go：宿主持有 owner/session 作用域的 scratch 租约，向 DM、Room 与后台 runtime 提供版本化资源策略；经 internal/infra/confinedfs 固定目录句柄完成创建、marker 读写、扫描与回收，Bridge 关闭成功后才回收，失败保留会话栅栏并把 cleanup_unknown 状态持久化；Windows marker 核验进程创建时间与内核存活信号，避免 PID 重用误回收或残留句柄把已退出进程误判为存活，查询失败保持未知。
//   - sandbox_receipt_process.go：Connect 策略按 client 原始进程代次绑定 exact runtime launch；warm 复用保持原进程身份，缺失或不匹配拒绝连接。
//   - sandbox_receipt.go：Connect 后核对当前桌面 runtime 实际确认的 Bridge 能力、策略摘要和 host lease 身份，并通过可选 SandboxPolicyReceiptStore 持久化 owner/session/generation 生命周期回执；回执只表达本次 runtime generation 的生效输入，不替代 OS/全 SDK 隔离证据。
//   - sandbox_process_recovery.go：显式持锁回调覆盖整个恢复、核对同 app 根及目录 inode，在宿主已取得独占实例所有权的前提下，以会话 gate 和 exact key 恢复原进程登记；活动 client 拒绝恢复，不重放任务，不自动清除 policy/lease 栅栏。
//   - sandbox_lifecycle_recovery_darwin.go：原生扫描后的独立有界资源/策略扫描，发现已终止进程的未完成后续步骤；失败保留并报告，不把分页结束当成恢复成功。
//   - sandbox_scratch_cleanup_darwin.go：监督 lease 最后一次正常 Release 复用持久隔离删除流程，绑定原 supervisor 与资源身份，提交响应丢失可重试。
//   - sandbox_scratch_recovery_darwin.go / sandbox_policy_recovery.go：持锁且无活动 client 时隔离原资源、持久删除阶段、仅收口 exact process 关联策略；pending 资源记录阻断新启动，macOS App 启动扫描已装配。
//   - sandbox_scratch_identity_darwin.go / sandbox_scratch_identity_other.go：macOS 从 lease 创建时的 parent/leaf 身份生成持久资源证明，每次监督启动重新核验；其他平台不伪造证明。
//   - sandbox_process_supervisor.go：宿主显式监督配置、factory 前的 exact 代次/已取得 scratch lease 绑定及各 probe/runtime 的独立 Host；普通热更新保留原 client 的监督身份，macOS App 默认装配已接入。
//   - sandbox_process_host.go：原宿主目录内的长路径 socket、Bridge 显式监督启动的宿主数据库/受限目录适配，绑定 exact owner/session/generation 与 lease；放行一次，丢失响应按原记录收口，文件清理失败保留栅栏。显式 transport 已经由 Manager 配置接入。
//   - sandbox_process_startup.go：消费宿主数据库启动事实；prepared/registered/released 即使没有策略回执也阻断新 factory，终态与策略回执共用代次下界，不读取用户可写的进程登记。
//   - sandbox_startup.go：创建新 client 前读取 exact owner/session 的最新持久回执并延续代次；confirmed/retiring/unknown 阻断重建。scratch 新建前以固定父目录句柄检查同 scope 的 cleanup_unknown，包括旧 stale 目录，防止重启换目录绕过失败栅栏。
//
// 子包：exec/（轮次执行内核，ExecuteRound 主链）、trace/（SDK 消息调试字段与摘要）。
// 系统消息到产品事件的投影统一由 internal/message 负责，runtime 不保留第二套展示语义。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
// 桌面策略另要求 RequireContextFiles，并将其纳入进程身份；指令能力不代表全局配置和后台 IO。
// 桌面策略另要求 RequireNotebookFiles；Notebook 能力不代表 Notebook 执行或远程网络已受限。
// 桌面策略独立要求 RequireProjectFiles，项目发现读取失败不能被旧 nxs 能力掩盖。
// 托管策略完整性通过 RequireManagedPolicy 独立要求；它同样进入进程指纹。
// 普通配置读取和完整快照通过 RequireSettingsFiles 独立要求并参与进程替换。
// 受控配置写入通过 RequireSettingsWrites 独立要求；成功更新后旧 runtime 不再发起 provider 请求。
// clientopts 在最终环境中固定 nxs Provider/后台唤醒的宿主所有权，任务覆盖不能撤销。
// Provider 所有权、凭据清理和后台唤醒标记参与进程指纹；变化先替换进程，不伪装成环境热更新。
// macOS nxs 的 MCP 专用网络要求与显式配置参与进程指纹和有效策略回执；不授予命令网络。
// MCP helper 要求进入进程策略和有效回执，不能用远端网络能力代替命令执行合同。
// macOS stdio MCP 通过 RequireMCPStdio 独立要求受限进程，纳入替换指纹与有效策略回执。
package runtime
