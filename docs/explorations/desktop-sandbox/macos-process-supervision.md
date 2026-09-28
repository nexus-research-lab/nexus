# macOS 进程监督接入候选

状态：**non-normative / 原型通过，尚未接入生产，2026-09-28**。本文细化[剩余开发计划](development-plan.md#macos-当前剩余工作2026-09-28)中的进程监督，不修改[当前合同](../../specs/desktop-sandbox-spec.md)。

## 当前决策依据

原进程组/session 扫描无法覆盖 `setsid` 后代；kqueue `NOTE_TRACK` 不支持，本机 Endpoint Security 后代接口缺 entitlement。新路线让用户级 launchd 为每个执行边界创建独立 resource coalition，由内核保留跨 fork、exec、改组与父进程退出的成员关系。普通用户的三个本机原生实验通过，见[原型证据](../../testing/evidence/desktop-sandbox/2026-09-28-process-coalition/README.md)。

Apple 的[固定 XNU 文档](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/doc/observability/coalitions.md)说明：成员身份在创建后不能改变；集合 ID 在一次系统启动内递增且不复用；创建、终止和回收由 launchd 管理。原型不直接调用 privileged coalition 创建接口，也不要求 Endpoint Security entitlement。

本机可通过 task name port 读取候选进程的 audit token，以 PID version 防止向复用 PID 发信号；读取 coalition 前后重新取得并核对 audit token。发信号使用 `proc_signal_with_audittoken`，不能将“先查身份再裸 kill(pid)”视为等价。进程枚举可以漏掉新生后代，所以空列表只代表本次观察，不能成为退出证明。

job 缺失、根进程退出和资源计数相等都不是终态。只有已经登记、同一 boot identity 下的原 coalition 经受支持的内核接口确认被回收，才能候选为该执行范围退出的证明。无权读取、接口缺失、结构不符、截断、未知错误及超时都保留 unknown；任意外部输入的不存在 ID 不能构造“已退出”。跨 boot 恢复必须先核对持久登记的真实启动身份，不能依赖墙钟或 PID 消失。

## 内部组件进展

Bridge 已落地尚未接线的 `internal/processscope`：严格登记/恢复、audit-token 精确终止、有界重扫及内核回收证明；缺原生 API 明确返回 unavailable。本机原生、竞态与无 cgo 用例通过，见[组件证据](../../testing/evidence/desktop-sandbox/2026-09-28-processscope-component/README.md)。补充实验也确认 `bootout` 返回成功仍可能保留脱离后代。后续 `CapturePeer` 已把连接的内核 audit token 与可信 launcher 指定进程核对，原生夹具通过“登记后才经连接放行”。但生产 job/可执行文件认证、双向认证、宿主持久登记、任务与 fd 传输、transport 和恢复接入仍按下列步骤推进。

## 完整接入顺序

1. **固定平台能力与支持范围**：提供独立、限界的原生观察/信号接口，确认内核布局、错误分类与可用符号；验证支持系统版本和 arm64/Intel。当前 14.0 的精确信号接口缺口尚未解决，不提高产品最低版本，不把静态源码存在当成动态兼容证明。
2. **先持久登记、后执行**：宿主先记录 exact owner/session/generation、boot identity 和唯一 job intent，再启动只运行可信引导代码的 helper。helper 通过受保护、已认证的控制通道报告自身精确身份及 coalition。宿主核验并持久确认后才发送可执行请求；未确认时不得启动模型 runtime、用户 hook 或其他任务代码。Provider 凭据与任务正文不能写进 plist、argv 或恢复记录。
3. **接入真实传输**：可信引导与 nxs/Claude 业务内核分离，保留 stdin/stdout/stderr、EOF、退出状态、环境过滤和 fd 关闭语义。nxs 和 Claude 仍使用各自沙箱，监督层只拥有进程生命周期，不混用后端能力。Bridge Close、启动失败和宿主退出都必须等待精确集合收口。
4. **明确取消范围**：一个 runtime 集合首先证明其全部关闭；单个 round/tool 的取消不能自动借用整个 runtime 的终态。实现时需要明确子执行边界，或由宿主正式退休该 runtime 并恢复会话；不能误伤其他 Agent、并发请求或已独立授权后台任务，也不能重放原工具调用。
5. **持久恢复与资源回收**：观察失败或宿主崩溃保留执行登记和 scratch 栅栏。新宿主按 exact generation、boot 与 coalition 重新检查；只有取得终态证据才能原子更新回执并清理相应资源。旧记录没有上述证明时继续保留，不将旧 unknown 批量解锁。
6. **真实故障验收**：连续 fork/exec、双重脱离、根进程先退出、错误身份、权限/观察拒绝、控制连接丢失、helper/宿主崩溃、同 scope 迟到关闭、无 API 系统、Full Access 与双向后端切换都必须经过真实产品路径。之后再验证签名包、最低支持系统、Intel 和干净安装。

## 尚未解决的兼容与信任边界

当前 App 最低版本是 macOS 14.0。XNU `xnu-10002.1.13` 没有精确 audit-token 信号入口，较新的 `xnu-10002.61.3` 有；早期系统仍需可部署的精确终止方案。`proc_info_extended_id` 在旧实现中只把 identity 参数传给查询，`PROC_INFO_CALL_TERMINATE` 仍直接调用 `proc_terminate(pid)`，不能作为无竞态替代。

coalition 观察结构及 usage wrapper 涉及私有 ABI，必须明确验证及失败关闭策略；进程自行委托其他系统服务产生的副作用不等于直接 fork 后代，仍属于各服务/网络/IPC 的独立权限边界。原型不提供抵御任意未受限同 UID 程序伪造本地控制状态的保证。正式登记与 IPC 鉴权完成前，不将实验函数接入现有自动回收入口。

本候选不缩减完整交付目标。其他 SDK IO、凭据、MCP、产品 UI、老用户升级与正式分发继续按统一开发计划验收。

## 引导接收端进展

Bridge 已新增 `cmd/nexus-runtime-bootstrap` 和内部启动协议：按固定内核身份认证宿主，接收限长启动输入与三条方向固定的标准管道，然后原地 exec。真实 launchd fixture 验证了身份拒绝、放行前断连、登记后执行、PID/标准流/显式环境/控制 fd 与退出码；异常 fd 传输泄漏也已复现并修复。见[引导组件证据](../../testing/evidence/desktop-sandbox/2026-09-28-process-bootstrap/README.md)。

该批次只完成接收端。生产 launcher 的 job/可执行文件认证、持久放行、退出观察、默认 transport、unknown 恢复和正式打包继续待实现；没有宣称 macOS 14.0 或产品全链路已通过。

## 根进程退出观察进展

内部 `Scope.WatchRoot` 在 helper 放行前核验连接身份并注册 kqueue `NOTE_EXIT | NOTE_EXITSTATUS`，已实测跨 exec 保留退出码，取消等待或停止观察不会构造退出事实。另一原生场景证明主进程退出后脱离后代仍然存活，必须独立 Reap。见[退出观察证据](../../testing/evidence/desktop-sandbox/2026-09-28-root-exit-observer/README.md)。此组件不恢复历史退出码；生产 launcher、持久放行、默认 transport 和 unknown 恢复仍待接入。

## 宿主持久登记进展

Nexus 已新增可信数据库启动事实和产品 factory 前读栅栏，绑定现有 owner/session/generation；原集合登记后仅允许一次放行领取，未收口记录阻断跨后端重建。真实 SQLite 并发领取、重开及迟到/跨 scope/错误证据拒绝通过，runtime 包与架构检查通过。见[登记证据](../../testing/evidence/desktop-sandbox/2026-09-28-process-registry/README.md)。生产 launcher 还未调用写入链；下一步连接 job 核验、持久放行、退出观察与撤销/回收，不将此数据库批次当作实际崩溃窗口已经关闭。

## 显式启动器进展

Bridge `supervision` 已把固定 helper 校验、Host 持久阶段、job 启动、原生登记/退出观察、一次性放行和回收结果连接起来。真实 launchd 竞态测试覆盖脱离后代输出、取消关闭等待者、各持久阶段失败及丢失放行响应；[证据](../../testing/evidence/desktop-sandbox/2026-09-28-supervised-launch/README.md)。生产 Host 数据库适配、默认 client transport、重启恢复和打包仍待接线；Unix socket 的 103 字节路径限制需要在受保护宿主目录内解决，不能借用任务可写路径规避。

### 2026-09-28：Nexus Host 数据库适配

`internal/runtime/sandbox_process_host.go` 已把 Bridge 持久回调连接到现有进程仓储，并经 `confinedfs` 发布及回收专属 job 目录。真实 helper + launchd + SQLite 集成验证了执行后 exact 回收终态；提交后丢失响应、错误集合、重复放行和符号链接反例由目标竞态测试覆盖。目录仅由宿主装配提供，短临时路径只属于测试夹具，不能作为生产任务隔离证明。默认 transport、生产受保护根装配、崩溃恢复、长路径和 App 打包仍未接入。

### 2026-09-28：Bridge 显式监督传输

`Options.ProcessSupervision` 已接入普通 process transport，每个 runtime、版本探测及 Claude 两种准入探测向宿主工厂请求独立 Host；失败不回退普通 exec。中断使用既有 runtime control，强制关闭使用原集合身份；JSON、退出码、脱离后代 EOF、关闭、清理错误、全部探测和探测取消的八个原生必测名称通过。见[证据](../../testing/evidence/desktop-sandbox/2026-09-28-supervised-transport/README.md)。Nexus 已固定此模块，但 Manager 启动前绑定 generation、生产可信目录、独立 probe 身份、崩溃恢复与 App helper 打包仍待接入。

### 2026-09-28：Manager 启动代次与 probe 登记

`SetSandboxProcessSupervisor` 只在 Manager 使用前接受宿主配置，调用方必须提供受保护根和可信 helper 摘要；不读取任务环境。factory 收到的工厂冻结 exact owner/session/generation，warm reconfigure 不能换掉它。仓储在同代次增加固定 launch_order（Claude sandbox、restricted、version、runtime），允许跳过不适用的探测，不允许倒退、重放或越过未清理的前项。迁移保留旧 intent JSON，并将旧无用途记录定位到 runtime；回退若会丢失多启动证据则事务失败。该批次没有自动启用 App，也尚未把 scratch lease 绑定到进程登记。

### 2026-09-28：原登记恢复入口

Bridge `Recover` 先验证原意图、登记和当前观察者，再撤销同 boot 原 job 并回收原集合；boot 改变不动新 boot 同名 job。真实测试宿主在任务仍存活时直接退出而不 Close，新宿主仅凭持久登记回收成功，重复恢复未重放任务。Nexus `RecoverSandboxProcess` 在宿主已持有跨进程独占实例锁的前提下，通过会话 gate 读取 exact 记录并调用该入口；活动 client 拒绝，失败保留状态。该入口不自动清除 policy/scratch 栅栏，也尚未由 App 启动自动调用。

### 2026-09-28：原宿主目录中的长 socket 路径

上述历史记录中的 103 字节完整路径缺口已在 Bridge `c994b19` 解决：专用原生线程使用父目录句柄及 basename 绑定/连接，退出时销毁线程 cwd，不修改进程 cwd。真实长路径启动/回收和 Nexus 固定 nxs 双次 AutoDream 生命周期通过，见[证据](../../testing/evidence/desktop-sandbox/2026-09-28-long-control-paths/README.md)。宿主仍须提供任务不可写的目录；当前只在本机 arm64 验证，不声明 macOS 14.0 或 Intel 支持已验收。
