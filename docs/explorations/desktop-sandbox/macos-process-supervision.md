# macOS 进程监督接入候选

状态：**non-normative / 原型通过，尚未接入生产，2026-09-28**。本文细化[剩余开发计划](development-plan.md#macos-当前剩余工作2026-09-28)中的进程监督，不修改[当前合同](../../specs/desktop-sandbox-spec.md)。

## 当前决策依据

原进程组/session 扫描无法覆盖 `setsid` 后代；kqueue `NOTE_TRACK` 不支持，本机 Endpoint Security 后代接口缺 entitlement。新路线让用户级 launchd 为每个执行边界创建独立 resource coalition，由内核保留跨 fork、exec、改组与父进程退出的成员关系。普通用户的三个本机原生实验通过，见[原型证据](../../testing/evidence/desktop-sandbox/2026-09-28-process-coalition/README.md)。

Apple 的[固定 XNU 文档](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/doc/observability/coalitions.md)说明：成员身份在创建后不能改变；集合 ID 在一次系统启动内递增且不复用；创建、终止和回收由 launchd 管理。原型不直接调用 privileged coalition 创建接口，也不要求 Endpoint Security entitlement。

本机可通过 task name port 读取候选进程的 audit token，以 PID version 防止向复用 PID 发信号；读取 coalition 前后重新取得并核对 audit token。发信号使用 `proc_signal_with_audittoken`，不能将“先查身份再裸 kill(pid)”视为等价。进程枚举可以漏掉新生后代，所以空列表只代表本次观察，不能成为退出证明。

job 缺失、根进程退出和资源计数相等都不是终态。只有已经登记、同一 boot identity 下的原 coalition 经受支持的内核接口确认被回收，才能候选为该执行范围退出的证明。无权读取、接口缺失、结构不符、截断、未知错误及超时都保留 unknown；任意外部输入的不存在 ID 不能构造“已退出”。跨 boot 恢复必须先核对持久登记的真实启动身份，不能依赖墙钟或 PID 消失。

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
