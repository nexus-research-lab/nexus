# 桌面沙箱验收矩阵

状态：开发验收清单，non-normative，更新至 2026-09-29。当前合同见 [规范](../specs/desktop-sandbox-spec.md)，开发状态见 [计划](../explorations/desktop-sandbox/development-plan.md)。

2026-09-29 [受限 prompt IO 收口](evidence/desktop-sandbox/2026-09-29-prompt-boundary-gate/README.md)：SDK `b3064c83` 在受限模式停止由宿主直接启动 `git`/`uname`，Git 快照改为省略并要求通过受限 Bash 获取；使用新 nxs（SHA-256 `71940cdc…f19db7`）的 Nexus host-integration gate 通过。该证据不扩大图形 App、后代监督、macOS 14.0、签名/公证或发布范围，`releaseAccepted=false` 继续成立。

2026-09-29 增量基线见 [计划文件沙箱收口](evidence/desktop-sandbox/2026-09-29-plan-file-sandbox/README.md)：SDK `65a028ce` 将 Enter/ExitPlanMode、恢复的 Read 状态和 compact 计划附件统一接入当前文件执行器；Nexus `2c34b0546`、Bridge `c251a8d` 的 host-and-macos-native-baseline 61 项检查、888 个必测名称通过。该证据仍是开发基线，`releaseAccepted=false`。

同日 [附件沙箱收口](evidence/desktop-sandbox/2026-09-29-attachment-sandbox/README.md) 的 SDK `b02ae892` 又将用户消息附件上传前的元数据检查接入该执行器；同一 61 项/888 个名称基线通过。

随后 [已读文件变更检测收口](evidence/desktop-sandbox/2026-09-29-changed-file-sandbox/README.md) 的 SDK `062d93d7` 将 turn 收口时的 mtime/content 检查也接入该执行器；同一基线通过。

2026-09-29 当前复核：Nexus `77c7e2340` 已把随包校验 helper、固定 `app/processes` 宿主根、
原生进程恢复和资源/策略后续恢复接入默认 macOS App 启动链；SDK `169e5c31` 的 runtime-owned
原子替换与 Nexus/Bridge 目标测试均通过，三仓统一分支且工作树干净。该复核确认默认装配已
进入产品路径，但不替代 macOS 14.0 精确终止兼容、完整图形 App 验收、签名/公证、clean-host、
Intel 或正式发布门禁，`releaseAccepted=false` 继续成立。

同日 [当前固定 host gate](evidence/desktop-sandbox/2026-09-29-current-gate/README.md) 使用
上述三仓提交重建 arm64 nxs（SHA-256 `d88e520f…190ac`）并通过 host policy、生命周期、
取消、设置恢复、App 关闭和 MCP round-trip；范围仍为 `host-integration-only`。

同日 [当前真实第三方探针](evidence/desktop-sandbox/2026-09-29-live-provider-current/README.md)
使用主工作目录 `.env` 的第三方 Anthropic-compatible Provider，nxs 与 Claude 两个后端均
通过真实 Write/Read/Bash、受保护文件与网络拒绝、sidecar 身份保护及 Interrupt/Close 清理。
该证据仍不替代图形 App、签名、公证、clean-host、Intel 或正式发布验收。

受限 prompt IO 修复后的 [nxs 实时复核](evidence/desktop-sandbox/2026-09-29-live-provider-prompt-boundary/README.md)
再次使用同一 `.env` 通过真实模型完成 nxs 的 Write/Read/Bash、越界文件/sidecar/Ruby 子进程/网络拒绝和
Interrupt/Close 清理；范围仍限 nxs runtime，不替代图形 App 或发布验收。

[随包 fork runtime smoke](evidence/desktop-sandbox/2026-09-29-bundled-fork-runtime/README.md) 随后通过：Nexus `d8eba8672`、SDK `fd9a97ac` 组装当前 nxs，workspace-write/read-only/full-access 三种随包兼容 profile 与隔离状态根 App smoke 均通过；只证明本机 ad-hoc 开发包路径。

[受限 fork executor gate](evidence/desktop-sandbox/2026-09-29-restricted-fork-gate/README.md) 随后完成：SDK `069cfa86` 将受限 fork 的 canonical transcript、独占目标发布、plan、artifact 复制与 session ID 重写统一接入文件执行器；新 nxs SHA-256 `c2e2f656…b634` 的 host-integration gate 通过。该项仍不扩大图形 App、macOS 14.0 或正式发布范围。

随后 [受限 rewind executor gate](evidence/desktop-sandbox/2026-09-29-rewind-executor-gate/README.md)
验证 SDK `ac2c41f9` 的 transcript、file-history backup 和目标文件恢复均沿受限文件端口执行；Nexus host-integration-only
门禁通过，仍不扩大图形 App、后代监督或发布验收范围。

计划恢复的 [fail-closed 回归](evidence/desktop-sandbox/2026-09-29-plan-recovery-fail-closed/README.md) 由 SDK `867e788c` 收口：仅明确确认不存在时才独占创建，权限、取消、未知结果和已有文件均不写入；基线仍通过且 `releaseAccepted=false`。

SDK `7090d9c5` 的 [rewind fail-closed 基线](evidence/desktop-sandbox/2026-09-29-rewind-fail-closed/README.md) 明确拒绝受限模式下尚未接入原子 file-executor 的 legacy 多文件回滚，避免绕过策略；Full Access 兼容路径保持不变。

2026-09-29 [真实第三方 nxs probe](evidence/desktop-sandbox/2026-09-29-live-provider-nxs/README.md) 通过，覆盖模型实际 Write/Read/Bash、受保护文件/Sidecar 身份文件、Ruby 子进程、网络拒绝及 Interrupt/Close 清理；范围仍限 nxs runtime，不等于图形 App DM/Room/自动化验收。

同日 [真实第三方 Claude probe](evidence/desktop-sandbox/2026-09-29-live-provider-claude/README.md) 也通过，使用 Claude Code `2.1.273` 覆盖同一隔离与取消清理矩阵；仍不等于图形 App 全链路或发布验收。

[当前 sidecar 崩溃恢复证据](evidence/desktop-sandbox/2026-09-29-sidecar-crash-recovery/README.md) 随后通过：原宿主退出后脱离后代确认存活，重启后 process/policy/scratch 均完成对账，`noReplay=true` 且后代已回收。该证据是默认 macOS nxs fixture 的真实启动链结果；原始临时状态根已清理，不能扩大为 macOS 14.0、完整图形 App 或正式发布验收。

[MCP helper header hardening](evidence/desktop-sandbox/2026-09-29-mcp-helper-header-hardening/README.md) 使用 SDK `3891dbdc` 构建的新 nxs 通过 `make check-desktop-sandbox` 的 `host-integration-only` 门禁；该增量只覆盖认证 header 解析的 fail-closed 规则，不扩大外部 MCP proxy、图形 App 或发布验收范围。

Nexus 持久化 MCP 配置也已在 `ad62e8d73` 收紧静态 header：非法 token 名称、CR/LF/NUL 和超过 128 个条目在连接前拒绝；`go test ./internal/runtime/clientopts -count=1` 通过。该项仍属于配置边界回归，不替代外部 MCP proxy、图形 App 或发布验收。

提交 `dc4ef2bf4` 进一步拒绝 MCP endpoint 与 OAuth metadata URL 中的 userinfo 和 fragment；同一 `clientopts` 目标测试通过。

[随包 runtime compatibility check](evidence/desktop-sandbox/2026-09-29-bundled-runtime-check/README.md) 使用现有 arm64 开发 App 通过，确认包内 sidecar 可识别 workspace-write、read-only、full-access 三种 nxs profile；该结果仍不等于签名、公证、clean-host 或发布验收。

[签名准备检查](evidence/desktop-sandbox/2026-09-29-signing-readiness/README.md) 确认当前本地包仍为 ad-hoc、无 Team ID、未 stapled notarization ticket；Developer ID 与正常 Gatekeeper 发布门禁继续保持未完成。

最新 macOS sidecar 身份回归见 [2026-09-28 信号结果与随包自检](evidence/desktop-sandbox/2026-09-28-sidecar-signal-result/README.md)：Nexus 当前工作树修正 libproc 正值错误码解释，10 个独立 Swift 回归体、固定 nxs 宿主集成基线、真实 sidecar 崩溃恢复、随包 arm64 nxs 兼容性检查和显式已发布/候选 runtime 升级四阶段均通过。该批次仍不证明 macOS 14.0 缺失的精确信号 API、图形 App 全链路、Developer ID/公证、Intel、clean-host 或发布验收。

最新 Windows 本机证据见 [2026-09-28 原生组件与恢复基线](evidence/desktop-sandbox/2026-09-28-windows-native/README.md)：53 个指定原生检查和 amd64/arm64 构建通过；完整 Windows 执行后端及发布验收仍未完成，`releaseAccepted=false`。下文按日期保留历史结果，不能把旧“无 Windows 主机”结论当作当前状态。

最新 macOS 固定基线见 [2026-09-28 后台会话记录读取](evidence/desktop-sandbox/2026-09-28-memory-transcript/README.md)：Nexus `fc888db41`、SDK `2d1fd0d6`、Bridge `4b2972f`，61 项检查及 888 个必测名称全部通过。Summary、AutoMemory 与 AutoDream 只沿当前 recorder 路径经文件执行器读取内容替换记录；拒绝、取消或不完整读取均在调用模型前停止，不再回退宿主 catalog 或 Git worktree 扫描。流式传输保留原有大日志 compact 后缀、保留段和 metadata 语义，不新增有效后缀的硬大小上限。原生 runtime 普通构建 18 个、原生流式 helper race 4 个必测名称通过，相关包 race 与 vet 通过。此前[记忆写入租约](evidence/desktop-sandbox/2026-09-28-memory-writer/README.md)、[AutoDream 读取](evidence/desktop-sandbox/2026-09-28-autodream/README.md)、[初始化与摘要](evidence/desktop-sandbox/2026-09-28-memory-persistence/README.md)和[记忆召回](evidence/desktop-sandbox/2026-09-28-memory-recall/README.md)证据保留，写入租约批次额外 runtime race 初始化超时的失败也保留原结论。普通会话录制/恢复、其余 SDK IO、任意后代监督与一般 unknown 恢复仍未闭合。

最近一次实际 App 的 [审批与切换证据](evidence/desktop-sandbox/2026-09-28-app-contracts/README.md)使用 Nexus `259ccda2d`、SDK `5a937a18`，其 48 项/689 个必测名称基线保留原来源。HTTP/WebSocket 验收覆盖双向后端切换、nxs/Claude 的 Full Access 边界恢复、Room 与后台审批、网络本次批准、取消审批后立即重连和正常退出。该批修复了 macOS workspace 别名误审批及旧请求取消误关闭新连接；完整 UI 复验因锁屏尚未完成。此前[App 正常退出与同会话重启](evidence/desktop-sandbox/2026-09-28-app-shutdown/README.md)及[重启准入](evidence/desktop-sandbox/2026-09-28-restart-admission/README.md)证据继续保留；这些不等于任意后代监督、一般 unknown 恢复或发布验收。

## 2026-09-27：真实第三方模型与两种 macOS 后端

使用主工作目录 `.env` 的第三方 Anthropic-compatible Provider，分别通过当前
Nexus clientopts、Bridge、固定 nxs 与 Claude CLI 2.1.273 发起真实模型请求。
只提取 token/base URL/model，不加载原数据库、owner、Connector 或状态目录配置。
每个后端使用新建状态根、workspace 和无敏感数据的禁止目录。

| 检查 | nxs | Claude |
| --- | --- | --- |
| 模型调用原生 Write，再由 Bash 读取并核对唯一内容 | 通过 | 通过 |
| 原生文件工具拒绝写入禁止目录，目标未创建 | 通过，受限文件执行器 | 通过，独立 Read/Edit deny 权限 |
| Ruby 子进程写入另一禁止目录，权限错误且目标未创建 | 通过，命令沙箱 | 通过，原生命令沙箱 |
| curl 请求显式禁止的 example.com，被代理/策略拒绝 | 通过 | 通过 |
| 启动真实 sleep，宿主 Interrupt + Close 后确认该测试进程退出 | 通过 | 通过 |

可重复入口为 `scripts/desktop/check-live-sandbox.mjs`；原始结果、版本、临时模块
替换的源码一致性说明与重放命令见[归档](evidence/desktop-sandbox/2026-09-27-live-provider/README.md)。
该入口必须显式执行，普通 Go 测试会跳过外部调用。脚本不把 `.env` 全量注入 runtime，
测试在启动 runtime 前清空自身临时 token 变量，并对输出脱敏。

Claude 的 `sandbox.filesystem` 不能代替原生工具的 Read/Edit 权限；首次夹具混用
两者的失败记录保留为测试修正证据。最终原生命令拒绝使用单独目录和 Ruby 文件 IO，
避免用文件工具权限或 shell 重定向前置拦截冒充 OS 命令隔离。

本轮是当前第三方网关与两种 runtime 的宿主集成冒烟结果，不是任意网关/Header、
App DM/Room/后台全链路、动态网络批准、Full Access 切换、任意脱离后代、完整凭据
隔离或签名安装包验收。`releaseAccepted=false`；官方 Claude 账号/OAuth 暂缓，Windows
另机核验。历史条目中的“真实第三方未验证”按其记录日期理解。

## 证据记录要求

每条结果记录：仓库提交、dirty 范围、runtime binary 来源与 SHA-256、OS/架构、测试命令、最终退出码、实际执行场景、skip 和剩余限制。编译、握手、依赖检测、真实隔离、安装包体验分别记录。

## 必须验收的场景

最终产品要求：受控运行环境内建于 SDK 执行链，所选后端直接落实默认资源策略，无独立沙箱开关；完全访问必须来自用户显式选择，且只改变资源/审批范围，不取消执行生命周期和领域授权。依赖和能力可以单独配置，缺少必需能力时任务必须失败关闭。以下原生文件与辅助 IO 全覆盖断言属于 nxs 自主引擎；Claude Code 单列原生沙箱与工具权限验收，不能借用 nxs 的 OS 覆盖证据，也不能把工具 helper 的隔离视为整个 SDK 主进程的隔离。

| 类别 | 场景 | 通过条件 |
| --- | --- | --- |
| 准入 | 旧关闭环境变量、server、macOS/Windows、旧 nxs、Claude、缺能力 | 旧变量不能关闭桌面合同；需要但无法提供的边界在命令开始前拒绝 |
| 默认权限与后端 | 新任务、升级后默认值、nxs/Claude 切换、缺依赖/不支持平台、显式 Full Access | 默认请求批准/自动审核均自动受限；旧实例收口、新实例确认后才能发任务；不支持不静默裸执行 |
| Settings receipt | applying/reconcile_required、数据库重开/独立进程、旧 revision、密钥损坏、review、人工 reconcile、重复/过期 revision | unknown 持久化且不自动重放；同一快照跨重启可比，旧计划仍失效；旧格式明确不可比较；缺失/损坏密钥拒绝；人工收口必须带当前 revision，Agent 不能代替真人确认 |
| Claude 原生设置合同 | Bridge typed `RequireClaudeNativeSandbox`/`CapabilityClaudeNativeSandbox`、唯一 host-owned `--settings`、`enabled`/`failIfUnavailable`/`allowUnsandboxedCommands`、Full Access、模式切换 | settings 缺失/重复/覆盖/尾随 JSON、bypass 或 unsandboxed command 在首条任务前拒绝；保留 Bash/构建能力；只计 Bridge 配置合同，不能替代 Claude 实际命令沙箱 |
| Claude 原生命令沙箱 | 独立原生 sandbox 配置、settings 来源/合并、缺依赖和未受限回退、Bash/构建命令、文件权限、网络批准、取消 | 必須保留正常命令能力，并证明允许操作成功、越界确实拒绝；`--restricted` 移除代码执行工具不能满足本行；不伪造 nxs 协议或复用文件 helper 覆盖 |
| 策略 | default/auto/Full Access、未来只读 profile、附加目录、deny 优先 | 审批方式不改变资源；强制策略不被用户设置/env/hook 覆盖 |
| shell | 文件/子进程、构建、Git、包管理、PTY、后台进程 | 边界内正常工作，子孙继承；普通工具 allow 不能授予越界 |
| 文件 | Read/Write/Edit/Glob/Grep、Notebook/附件、流式与大文件 | 真实工具通过同一受限数据面；拒绝未授权读写且不破坏原语义 |
| 辅助 IO | PDF/Git、指令导入、Skill、设置、记忆维护、mtime | 没有从宿主绕回未限制 IO；允许内容仍正常读取 |
| 路径 | symlink/reparse、改名/替换、硬链接、worktree、UNC/大小写 | 不因别名或竞态扩大资源；不能保证的情况拒绝并解释 |
| 控制对象 | 宿主/runner 进程、线程、token、句柄与凭据 | 命令无法读取/更改控制权，也不能利用继承句柄绕过 |
| 网络 | IPv4/IPv6、TCP/UDP、DNS/DoT、loopback/private、直连/代理 | 未批准目标确实不可达；不得仅验证代理环境或规则存在 |
| 越界审批 | 一次批准、输入/cwd 偷换、取消、并发、迟到与永久规则伪造 | 精确身份与原输入固定；过期/越权失败关闭 |
| 网络审批 | 连接允许/拒绝、后台连接、proxy 关闭、端口重用 | 批准恢复同一待连接；命令不重放；旧 epoch 不再授予连接 |
| 自动审核 | allow/deny、超时、损坏结果、人类专属、重复拒绝、人工覆盖 | 失败不扩权；保留原因和精确范围，不绕过人类专属要求 |
| 模式切换 | DM/Room、两方向、后台子孙、pending approval、清理失败 | 旧实例完成退出或显式未清理；新工作不沿用旧权限 |
| 恢复 | 创建/恢复线程/写入后崩溃、ACK 丢失、断管道、重连、重启 | unknown 先对账；同 request 不换输入，不重复副作用 |
| Windows 设置 | 安装、拒绝 UAC、不同管理员凭据、策略拒绝、修复 | 正确绑定原 owner；缺边界不启动；错误分类明确 |
| Windows 升级 | 签名失败、版本不兼容、更新中断、卸载、遗留 Job | 受保护文件和状态完整；撤销资源不影响其他实例 |
| 包与兼容 | macOS、Windows 支持版本/架构、Linux owner、Claude 支持版本/环境、旧数据根 | nxs 全链路及 Claude 原生接入分别有真实证据；WSL2 通过不能表示原生 Windows 通过 |
| 产品路径 | 设置、Composer、审批卡、DM/Room/自动化、重载 | 展示实际边界；一个清晰下一步；不泄漏内部标识，不自动重发 |

### 2026-09-21：资源-backed nxs 启动合同与 lease 范围收口

复审发现原 Nexus clientopts 在带有 host scratch resource lease 时仍投影
`allowUnsandboxedCommands=true`。固定 Bridge 的资源合同会在 transport 前拒绝这一组合，
因此只做普通能力握手不能证明 DM/Room 的真实 scratch-backed 启动可用。本批次让资源-backed
nxs 强制 `allowUnsandboxedCommands=false`，拒绝 read-only scope 携带显式写目录，并拒绝
Claude（包括 Full Access）接收 nxs resource contract；同一 owner/session 的活动 lease
不能改变写入范围。

| 验证 | 结果与边界 |
| --- | --- |
| clientopts 单测 | `GOWORK=off GOPROXY=off go test ./internal/runtime/clientopts -count=1` 通过；覆盖 nxs/Claude 分离、资源与 Full Access、只读写目录拒绝 |
| lease 与竞态 | `GOWORK=off GOPROXY=off go test ./internal/runtime -run 'SandboxResource|AgentClientCleanup' -count=1` 与 `go test -race ./internal/runtime -count=1` 通过；覆盖独立持有句柄、旧 runtime 精确回收、cleanup fence、marker、显式 stale sweep |
| 固定 nxs 真实握手 | `NEXUS_SANDBOX_TEST_BINARY=/private/tmp/nxs-desktop-9956-bridge374 GOWORK=off GOPROXY=off go test ./internal/runtime/clientopts -run 'TestDesktopSandboxRealRuntimeWithHostResources' -count=1` 通过；Bridge `37434c2d38b1`、无模型请求，确认资源合同被真实 nxs 能力确认 |
| 桌面门禁、架构、增量 Go/vet | `make check-desktop-sandbox`、`make check-architecture`、`make check-go`、目标 `go vet` 均 exit 0；门禁报告为 `host-integration-only` |
| scratch 路径竞态 | `internal/runtime` 回归覆盖固定目录句柄创建、marker/扫描的 no-symlink 读取，以及父目录替换为 symlink/普通文件后回收失败关闭并保留原 lease |
| 当前边界 | 仍不证明 nxs/Claude 的真实命令与全 SDK IO、Provider/秘密文件/句柄/网络出口、完整后代清理、Windows/macOS clean-host、签名安装包或生产发布；`releaseAccepted=false` |

本批次只证明 Nexus/Bridge/固定 nxs 的输入合同和 resource-backed 初始化闭合，证据目录见
[2026-09-21 Bridge probe cleanup](./evidence/desktop-sandbox/2026-09-21-bridge-probe-cleanup/)。

### 2026-09-22：lease 转移与跨平台 marker 边界加固

本批次复审发现三个可在 Nexus 层闭合的生命周期问题：启动绑定失败时不能把
未接管的 scratch 句柄报告为已转移；同一句柄的并发 `Release` 不能重复减少共享引用；
尚未安装 Bridge session 的 unclean discard 也必须排空已绑定的 lease。现已让
`ClientStartup.BindSandboxLease` 在失败时返回未消费，串行化每个 lease 句柄的释放，并让
discard 路径启动同一 cleanup fence；Windows 的 marker 读取增加句柄级硬链接检查，无法
取得文件身份时失败关闭；`agentClient` 还按 runtime kind 拒绝把 Nexus lease 交给 Claude。

| 验证 | 结果与边界 |
| --- | --- |
| 目标 race/vet | runtime、confinedfs、clientopts 与 DM、Room realtime、AutoDream race 通过；目标 `go vet` 通过 |
| 跨平台编译 | Windows amd64 与 macOS arm64 的 runtime/confinedfs 测试二进制交叉编译通过；不等于原生运行验收 |
| 架构与桌面门禁 | `GOWORK=off make check-architecture`、固定 nxs `make check-desktop-sandbox` 通过；报告为 `host-integration-only` |
| 固定资源握手 | SDK `9956def130da33af47accf799a9c27c16a551104`、Bridge `37434c2d38b1`、nxs SHA-256 `0f91b17f…014270`；无模型请求 |
| 证据 | [2026-09-22 lease hardening](./evidence/desktop-sandbox/2026-09-22-lease-hardening/) |
| 当前边界 | `releaseAccepted=false`；原生 Windows/macOS clean-host、签名安装包、真实 Claude 认证命令、完整 nxs IO/网络/秘密/句柄隔离、后代监督、跨重启恢复和有效策略回执仍未验收 |

本批次完成的是 Nexus/Bridge 输入与本地 scratch 生命周期加固；它不能把 host
integration 结果扩大为桌面发布承诺。

### 2026-09-23：启动换代的 exact lease 回收

复审发现配置换代或 Disconnect 恰好落在 Bridge session 启动完成窗口时，旧代
cleanup 不能拿当前代 lease；尚未安装 session 的 in-flight Connect 也不能由
nil-session cleanup 提前释放 scratch。现已让 cleanup 按启动快照持有 exact handle，
在 session close 完成前保持 fence；生命周期失效期间的延迟回收在 cleanup 完成前不
释放资源，错误回执使用同步访问。

| 验证 | 结果与边界 |
| --- | --- |
| lease race 回归 | `TestAgentClientReceiptRetryRetainsLeaseDuringReconfigure`、`TestAgentClientStartupReconfigureRetainsLeaseDuringStaleCleanup`、`TestAgentClientDiscardDuringConnectClosesLateSessionBeforeReleasingLease` 通过；新增场景 `go test -race ./internal/runtime -run 'TestAgentClient(DiscardDuringConnectClosesLateSessionBeforeReleasingLease|ReceiptRetryRetainsLeaseDuringReconfigure|StartupReconfigureRetainsLeaseDuringStaleCleanup)' -count=50` exit 0 |
| 变更范围门禁 | 目标 runtime/clientopts/DM/Room/AutoDream/confinedfs 测试、runtime race、vet、`make check-go-fresh`、架构检查和固定 nxs `make check-desktop-sandbox` 均 exit 0 |
| owner-scoped diagnostics | `GET /settings/runtime/sandbox/receipt?session_key=...` 优先返回当前 owner 的 connected generation receipt；重启后按 owner/session 回退到最新 durable receipt；缺失、跨 owner 或无 durable row 返回 404；receipt clone/owner 栅栏测试通过（本行属于 2026-09-23 历史基线） |
| 固定真实 nxs | SDK `9956def130da33af47accf799a9c27c16a551104`、Bridge `37434c2d38b1`、nxs SHA-256 `0f91b17fc0ed6976e01a76363f466640a1cddfa63bc32338cb7647153e014270`；macOS arm64，未发送模型请求 |
| 当前边界 | 仍是 host integration 与 macOS 开发基线；原生 Windows/macOS clean-host、签名安装包、真实 Claude 认证命令、全 SDK IO/网络/秘密/句柄/后代隔离、跨重启自动恢复和生产发布仍未验收，`releaseAccepted=false` |

本批次只收口 Nexus runtime 的启动换代资源生命周期；它不能把 macOS 开发基线扩大为发布验收。

### 2026-09-24：cleanup fence 转移与 receipt 不可变

本轮复审发现两个恢复边界需要继续收紧：如果首先标记 `cleanup_unknown` 的精确
lease handle 在 sibling 仍存活时先释放，清理栅栏不能随已释放句柄丢失；同一
generation 的重复 receipt 回调也不能改写已经确认的 payload 或 `confirmed_at`。
现已让共享 resource 保留每个活动 handle，并在 uncertain owner 释放时把栅栏转移到
仍存活的 sibling；同一 generation 的 receipt payload 与 `confirmed_at` 保持首次写入值，
重复回调只刷新观察时间，终态 receipt 不接受迟到回调覆盖。

| 验证 | 结果与边界 |
| --- | --- |
| lease transfer 回归 | `GOWORK=off go test ./internal/runtime -count=1` 通过；新增 uncertain owner 先释放、sibling 完成 reconcile 的场景 |
| runtime race | `GOWORK=off go test -race ./internal/runtime -run 'Sandbox|Lease|Receipt|Close|Owner|Process' -count=1` 通过；覆盖精确 handle、cleanup fence、进程身份与 receipt 生命周期 |
| receipt store | `GOWORK=off go test -race ./internal/storage/sandbox -count=1` 通过；同 generation 冲突 retry 不得修改 payload/`confirmed_at` |
| Windows 交叉门禁 | `make check-desktop-sandbox-windows` 通过；installer contract、runtime/clientopts/confinedfs 测试编译以及 `nexus-server`/`nexusctl`/`nexuscfg` 的 amd64/arm64 构建均 exit 0；Windows 原生身份测试固定使用主机 Node 架构，报告 `releaseAccepted=false` |
| 当前边界 | 仍无 Windows 11 amd64/arm64 实机的 `GetProcessTimes`、P3 兼容/隔离组合、ACL/网络/账号 provisioning、取消与后代终态、签名安装/升级/clean-host 证据；SDK 原生 Windows `PrepareExecution`/`InspectBackend` 继续 fail closed |

本批次收口的是 Nexus 本地恢复语义与 Windows 双架构构建门禁；它不把共享
resource 的生命周期证据扩大为原生 Windows 沙箱或发布验收。

### 2026-09-20：配置 revision 跨重启恢复

先以 `TestConfigurationRevisionSurvivesDatabaseReopen` 在旧实现复现：配置未变化，
数据库关闭重开后 `revision_relation` 却变成 `different`，测试 exit 1。新实现将
持久 revision 与进程内 plan digest 分开；migration 142 保留旧 receipt，旧格式返回
`incomparable`，不改写旧历史或自动决定写入结果。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 数据库重开、旧计划拒绝、人工 reconcile | 通过，exit 0 | 未变化的 Preferences 与 recorded-after 可比；旧 plan digest 被拒绝且不创建执行 receipt；人工对账不推进配置版本 |
| 两个独立进程、并发初始化、升级 | 通过，exit 0 | SQLite 单例密钥 CAS；独立宿主读取同一持久值；升级与重复 migration 保留既有回执和密钥 |
| 缺行、损坏、空密钥、未知版本、非法初始状态 | 五个具名子场景通过 | 现有/新 Store 均拒绝，不缓存旧值、不重建密钥、不回显秘密 |
| 配置目标包、定向 race/vet、migration/CLI/app runtime/server | 通过，exit 0 | 本机服务、存储与应用装配；不是 PostgreSQL 原生或安装包验收 |
| 固定 Bridge + 原有真实 nxs 桌面 gate | 通过，exit 0，必测项无 skip | 新增 `host-settings-recovery` 必测组，保留 host-policy/host-lifecycle；SDK/Bridge 提交和 binary 未变 |
| 架构、脚本语法、diff 格式 | 通过，exit 0 | 生产依赖方向与门禁入口 |

证据见 [2026-09-20-settings-revision](evidence/desktop-sandbox/2026-09-20-settings-revision/)。
本批次是 Nexus 配置控制面恢复证据，不证明 SDK 多文件掉电事务、所有领域写入与人工
reconcile 的跨进程原子性、完整凭据/网络隔离或 Claude/原生平台/安装包验收。
`releaseAccepted=false`；三个仓库均仅本地，未推送。

### 2026-09-20：Provider 与辅助请求继承环境清理

Nexus 提交 `a1eaa106f` 扩展 `scrubInheritedRuntimeEnv`，在 Bridge 继承宿主环境前
清理 SDK bootstrap 描述符与 fallback、Provider 凭据/自定义 headers、WebSearch 与
WebFetch helper 密钥、TLS client key/passphrase、OTEL headers、SSH agent/命令入口和
Connector client secret。空宿主变量不会制造显式空覆盖；后续宿主解析的当前 Provider
值仍会在环境合并末端重新投影。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| Nexus 目标包 | `GOWORK=off GOPROXY=off go test ./internal/runtime/clientopts -count=1`，exit 0 | 清理清单、Provider ownership 和显式 credential projection 回归 |
| Nexus race | `GOWORK=off GOPROXY=off go test -race ./internal/runtime/clientopts -count=1`，exit 0 | 目标包并发安全 |
| 关联 runtime/vet | `GOWORK=off GOPROXY=off go test ./internal/runtime -count=1`、`GOWORK=off GOPROXY=off go vet ./internal/runtime/clientopts ./internal/runtime`，exit 0 | 关联运行时回归与静态检查 |
| 当前边界 | 未通过发布门禁 | 该批次是环境来源清理，不证明任意秘密文件、继承句柄、外部 MCP/helper、后代进程、真实网络出口或原生平台隔离 |

证据见[2026-09-20-runtime-env-scrub](evidence/desktop-sandbox/2026-09-20-runtime-env-scrub/)。
三个仓库仍仅本地提交，`releaseAccepted=false`。

### 2026-09-20：固定 SDK + macOS 无模型基线重验

在 Nexus `29ba7eef9`、SDK `9d60e166` 和 Bridge
`02fbc0e5f6a699fad7106e202d119c272ef4e170` 上运行：

```text
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox \
  --sdk-ref 9d60e166
```

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 固定版本与构建 | exit 0；Bridge 精确模块无 replace；SDK clean archive 构建 nxs | nxs SHA-256 `45525bf29672249dc2a99eb4cb88ccd7e8fa6b9d5fb7623005546c34d34d1ce8` |
| host 与 settings | 13 个 host policy、22 个 host lifecycle、18 个 settings recovery、35 个 Provider environment 用例通过 | 本地无模型请求、macOS arm64 |
| macOS 能力路径 | 文件、资源、搜索、媒体、Skill、上下文、项目、托管策略、settings-writes 和原生路径全部通过 | 38 个检查均 exit 0，必测项无 skip |
| 发布结论 | `releaseAccepted=false` | 不替代签名安装包、clean-host、Windows/Linux、Claude 认证会话或生产验收 |

完整报告、选定原始日志和 `manifest.sha256` 见[2026-09-20-macos-baseline](evidence/desktop-sandbox/2026-09-20-macos-baseline/)。

### 2026-09-20：跨物理根 settings journal 基线重验

SDK `9956def130da33af47accf799a9c27c16a551104` 将 settings journal 恢复提升为跨根事务
判断：恢复前同时锁定用户与项目物理根，按 transaction ID 统一核对；一根旧一根新时保留
全部 journal 并失败关闭，不能清理其中一根后继续。门禁脚本新增
`TestSettingsJournalCrossRootMixedStateFailsClosed` 必测用例。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 固定版本与构建 | exit 0；Bridge 精确模块无 replace；SDK clean archive 构建 nxs | SDK `9956def1`；nxs SHA-256 `e004c631ec555df466c14e13fc091e53f29a0ca4e113e1cd81031c0a2bab0f84` |
| macOS 开发基线 | 38 个检查全部 exit 0，无必测 skip | host policy/lifecycle、Provider environment、settings-writes 与 macOS 全能力路径；无模型请求 |
| settings writers | 47 个通过事件，包含跨根混合失败关闭 | 全旧/全新收口、混合/损坏失败关闭、正文脱敏、目录身份和写入准入 |
| 发布结论 | `releaseAccepted=false` | 不替代跨根掉电 all-or-nothing、领域 receipt/reconcile、Provider 秘密文件/句柄/网络、Claude OS 沙箱、Windows/Linux、clean-host 或签名包验收 |

完整报告、日志和 `manifest.sha256` 见[2026-09-20-settings-journal-cross-root](evidence/desktop-sandbox/2026-09-20-settings-journal-cross-root/)。

### 2026-09-20：SDK durable settings transaction journal

SDK `431966dd8862429f80a0bb555aef048d02dedf23` 在每个物理 settings 根增加
`.nexus-settings-transaction.json`。journal 只保存旧/新 canonical 内容摘要，不复制
Provider 或其他设置正文；写入前持久化，重启时核对全部文档。全旧/全新状态安全清除，
混合、损坏或无法证明的状态保留 journal 并失败关闭，不能自动重放。Bridge 继续固定
`v0.1.34-0.20260920021254-02fbc0e5f6a6`（`h1:4sQoNTQUHwidSCAP6DawenSZcrKIhwf23Gm542GI2XU=`）。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| SDK settings target/race/vet | `go test ./internal/config/settings`、`go test -race ./internal/config/settings`、`go vet ./internal/config/settings` 均 exit 0 | journal 写入/清理、全旧/全新恢复、混合失败关闭、正文脱敏 |
| 固定 macOS 基线 | `check-sandbox-baseline.mjs --sdk-ref 431966dd...` exit 0；38 个检查无必测 skip | host policy/lifecycle/settings recovery/Provider 与 macOS 全能力路径；无模型请求 |
| settings writers | 46 个指定通过事件 | 新增 5 个 journal 场景已进入脚本必测清单 |
| 固定 runtime | nxs SHA-256 `96da0c6022a7eda42ffe5a3fb3a2df59a1dea80c0d38a67be6a6dbf98b36f4cd` | SDK archive 构建；Bridge 无 replace |
| 当前边界 | `releaseAccepted=false` | journal 不等同于跨根掉电 all-or-nothing、exact 领域 receipt/reconcile、Provider 秘密文件/句柄/网络出口、Windows/Linux/Claude/签名安装包验收 |

完整报告、选定日志和 `manifest.sha256` 见[2026-09-20-settings-journal-baseline](evidence/desktop-sandbox/2026-09-20-settings-journal-baseline/)。

### 2026-09-20：Bridge 最终进程入口的继承凭据过滤

Bridge `436346420c2905907375cc63b8fee9b88bc07287` 已固定到 Nexus
`v0.1.34-0.20260920062457-436346420c29`（模块 checksum
`h1:kwNq8HuqV0NihWeE96YulddxdlX6Gbq+XiAJEkGs8nM=`）。Bridge 在合并 typed
`Options.Env` 前过滤继承环境中的常见 Provider/API key、bearer token、secret/password、
private key、cookie、SSH agent 与 proxy-auth 名称；显式宿主投影仍可提供当前会话凭据。
Windows 按不区分大小写执行同一规则，普通 PATH、HOME 与 runtime identity 保留。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| Bridge target/race/vet | `go test ./client ./internal/transport`、`go test -race ./internal/transport`、`go vet ./client ./internal/transport` exit 0 | 继承秘密清理、显式 Provider override、Unix/Windows 大小写行为 |
| Nexus 接入 | `GOWORK=off GOPROXY=file:///private/tmp/nexus-bridge-4363464-proxy go test -mod=readonly ./internal/runtime/clientopts ./internal/runtime` 与 vet exit 0 | 固定 module pin 下的宿主 Provider ownership、runtime replacement 与进程选项 |
| 跨平台编译 | Bridge client/transport Windows amd64、Linux amd64；SDK settings package Windows/Linux amd64 均通过 `go test -c` | 仅编译证据，不替代原生运行与安装包验收 |
| 当前边界 | `releaseAccepted=false` | 不证明秘密文件、继承句柄、外部 MCP helper、Provider 网络出口、Claude OS 沙箱、后代清理或签名包 |

完整日志、固定版本和 manifest 见[2026-09-20-provider-env-boundary](evidence/desktop-sandbox/2026-09-20-provider-env-boundary/)。

## 2026-09-20：Bridge 原生 Claude settings 探测与最新固定基线

Nexus 工作树将 Bridge 精确 pin 更新为
`v0.1.34-0.20260920071621-8e90ff5e35e3`（Bridge 本地提交
`8e90ff5e35e3`，模块 checksum
`h1:Craz/NDn5xxVYC9uTP29wm4FEUdPZiOHopbS7kY+qTM=`）。
`check-claude-restricted.mjs` 现在除版本、`--restricted` 和 bypass 拒绝外，
还用生成的 `sandbox.enabled=true`、`sandbox.failIfUnavailable=true`、
`sandbox.allowUnsandboxedCommands=false` 调用 `--settings <json> --help`，确认
正式 CLI 接受该 settings 入口；探测仍不发送 prompt 或模型请求。

本机 `/Users/berhand/.local/bin/claude` `2.1.273` 的 settings 探测 exit 0，
帮助文本声明 `--settings`；两个 restricted bypass 组合仍 exit 1。固定 SDK
`9956def130da33af47accf799a9c27c16a551104` 的 macOS arm64 归档基线共 38 项
检查 exit 0，settings writers 47 个事件通过，nxs SHA-256 为
`9896f72b69797b180d24274f861ed5fca77f153f9c54e6a0796d700251d894ac`。

证据见 [最新固定基线](evidence/desktop-sandbox/2026-09-20-macos-baseline-latest/)
和 [Claude settings 探测](evidence/desktop-sandbox/2026-09-20-claude-native-probe.json)。
这些结果只证明 CLI 解析入口、Bridge 配置合同和 macOS 本地开发基线，不能证明
已认证 Claude 命令的文件/网络/Provider 隔离、取消与后代清理，或 Windows/Linux、
clean-host、签名包和生产发布；`releaseAccepted=false` 继续成立。

### 历史 Claude --restricted 合同（已由原生 sandbox settings 取代）

以下保留旧工具裁剪合同的验证记录；当前桌面接线使用 `RequireClaudeNativeSandbox`。
Bridge 的合同和 Claude 自身的实际隔离必须分开记证据：

已记录的 CLI `2.1.273` 将 `--restricted` 描述为移除代码执行工具并限制文件工作目录。
该参数测试只能证明当前工具受限启动路径；正常 Bash/构建命令在 OS 沙箱内执行仍须
按上表独立验收，不能以禁用代码执行作为功能完整的通过证据。

- Bridge 单测/静态检查证明 `RequireClaudeRestricted` 只接受 `RuntimeClaude`，拒绝
  nxs、Full Access/bypass、危险 bypass 以及通过 `ExtraArgs` 或其他普通输入伪造
  `--restricted`；受限参数必须恰好出现一次，且策略变化触发进程替换。
- Bridge 真实进程证据至少记录固定 Claude CLI 版本、`claude --help` 中的
  `--restricted` 可用性、最终 argv 和取消/清理结果。Bridge 的 capability 只表示
  本次启动参数合同已安装，不能单独证明 Claude 已经实施 OS 文件/网络/Provider
  隔离，也不能借用 nxs 的 capability 或原生证据。
- Nexus 接线证据必须分别覆盖：Claude 受限设置 `RequireClaudeRestricted=true`
  且不设置 nxs `RequireSandbox`；Claude Full Access 不设置该要求并保留用户明确的
  例外语义；缺少 Bridge 合同或能力不明时首条任务前失败关闭。
- Windows 原生 Claude 不因 WSL2 或交叉编译而通过；WSL2、macOS CLI 和 Linux
  owner 证据分别记录，不能互相替代。

在上述三层证据齐全前，本矩阵中的 Claude 仍为“未闭合”，`releaseAccepted=false`。

### 2026-09-18：settings receipt review/reconcile

`internal/service/configuration` 新增按 owner/scope 重新授权的 `ReviewChange` 和
`ReconcileChange`；`nexuscfg review` 返回当前脱敏快照、receipt 和 revision 关系，
`nexuscfg reconcile` 只接受带当前 `observed_revision` 的人工 `applied`/
`not_applied` 决定。round-scoped Agent 可以 review 但不能提交人工 reconcile；收口只
把 receipt 更新为 `reconciled`，不会再次调用原始领域写入。

验证：

```sh
GOWORK=off go test ./internal/service/configuration -run 'TestConfigurationReceiptReviewAndHumanReconcileDoesNotReplay' -count=1
GOWORK=off go test ./internal/cli ./internal/app/runtime
```

结果：集成测试证明人工收口不会再次推进 Preferences 版本，备注正文不落审计；CLI 和
loopback broker 编译/目标测试通过。原始日志、报告与校验和见[证据目录](evidence/desktop-sandbox/2026-09-18-settings-receipt/)。
使用 SDK `9d60e166` 构建的真实 nxs 和当前精确 Bridge 运行 host integration gate 也通过；
gate report 明确为 `releaseAccepted=false` 的无模型 host integration-only 范围。
此批次仍不证明 revision 在跨进程密钥生命周期外
可比较，也不替代设置页 UI、掉电事务、Provider/网络/辅助进程、Claude 真实会话或
Windows/macOS/Linux/安装包验收；`releaseAccepted=false` 继续成立。

### 2026-09-18：Claude CLI 受限参数探测（macOS 本机）

使用 `scripts/desktop/check-claude-restricted.mjs` 对本机安装的 Claude Code
执行只读的 CLI 探测。探测进程使用临时 `HOME`、`CLAUDE_CONFIG_DIR` 和
`XDG_CONFIG_HOME`，不继承 Provider 凭据；只调用 `--version`、`--help` 以及
两个在参数预检阶段失败的 bypass 组合，不提供 prompt，也不发起模型请求。

```sh
node scripts/desktop/check-claude-restricted.mjs \
  --binary /Users/berhand/.local/bin/claude \
  --expected-version 2.1.273 \
  --report /tmp/claude-restricted-probe.json
```

本机结果：macOS 27.0 / arm64，Claude Code `2.1.273`；`--help` 明确列出
`--restricted`，并说明会移除代码执行工具、限制文件工具工作目录和拒绝
`bypassPermissions`。`--restricted --dangerously-skip-permissions` 与
`--restricted --permission-mode bypassPermissions` 均以 exit 1 结束，并返回
`bypassPermissions not supported in restricted mode`。报告中的 `scope` 明确把
取消、清理、Provider、网络、文件和 OS 隔离标为未测试。

这条记录证明当前机器的 CLI 版本和参数语义可被复现，不能证明 Bridge 已经
启动过一个已认证的 Claude 会话，也不能证明真实工具、网络或子进程被隔离；
取消/清理及 Windows、Linux、安装包验收仍未闭合。

### 2026-09-18（历史）：Bridge Claude 受限准入预检

本节记录的 `--restricted` 启动参数合同已由 2026-09-20 的原生 sandbox settings
合同取代，仅保留作为参数语义和回归基线，不能作为当前桌面实现状态。

Bridge 已在本地提交 `6ea77309fdd4ed4f8177e9d9731253cd1f03879c` 接入正式进程
启动前的 CLI 预检。`RequireClaudeRestricted=true` 时，Bridge 使用和正式会话
相同的已解析可执行程序（Windows 使用安全 PowerShell shim）运行有界的
`--restricted --help`；非零退出、超时或帮助文本未声明该参数均失败关闭。预检
环境剥离常见 Provider token/key/secret/password、Cookie、代理变量，且输出有上限。
Full Access 不触发该预检。

本批次 Nexus 精确固定 Bridge 模块
`v0.1.34-0.20260918074231-6ea7730`，checksum 为
`h1:xRd4iHVLEDL08Ey0FqYk66D087/SkkQsGrl8B0LfxVQ=`；go.mod checksum 为
`h1:vrO/rqDQJM2orurZpB49MfPX4LjSNlb6DQZAmELJw1Y=`。证据目录为
[2026-09-18-claude-bridge-probe](evidence/desktop-sandbox/2026-09-18-claude-bridge-probe/)，
包含 Bridge target/race/vet、Nexus clientopts、`make check-desktop-sandbox`、
模块清单和 macOS Claude CLI 只读探测结果。

结果：Bridge 目标包、竞态和 vet 通过；Nexus Claude 接线测试通过；桌面 gate
exit 0；本机 Claude Code `2.1.273` 的 `--help` 声明 `--restricted`，两个 bypass
组合均 exit 1。该历史证据只闭合 Bridge 的启动参数准入和 Nexus 传递，不证明已认证
Claude 会话、取消/清理、Provider/网络/文件/子进程 OS 隔离，也不替代 Windows、
Linux 或安装包验收；因此 `releaseAccepted=false` 继续成立。

### 2026-09-20：Claude 原生命令 sandbox settings 接线

Bridge 本地提交 `02fbc0e5f6a699fad7106e202d119c272ef4e170` 增加
`RequireClaudeNativeSandbox` 与 `CapabilityClaudeNativeSandbox`。正式 Claude
进程启动前，Bridge 对唯一 host-owned `--settings` 做严格 JSON 解析，要求
`sandbox.enabled=true`、`sandbox.failIfUnavailable=true` 和
`sandbox.allowUnsandboxedCommands=false`；非法、重复、覆盖或尾随 JSON 均失败关闭。
Bridge 不再用 `--restricted` 移除 Bash/构建工具来冒充命令沙箱。

Nexus 本地提交 `67cb67e85` 固定 Bridge 模块
`v0.1.34-0.20260920021254-02fbc0e5f6a6`，checksum 为
`h1:4sQoNTQUHwidSCAP6DawenSZcrKIhwf23Gm542GI2XU=`，go.mod checksum 为
`h1:vrO/rqDQJM2orurZpB49MfPX4LjSNlb6DQZAmELJw1Y=`。受限 Claude 当前只在 macOS
安装该合同；原生 Windows 受限路径拒绝，Full Access 不安装该合同。

| 验证 | 结果与边界 |
| --- | --- |
| Bridge | `go test ./...`、`go test -race ./client ./internal/transport` 通过；覆盖 Bash 参数保留、settings 校验、bypass/覆盖/混用拒绝和尾随 JSON |
| Nexus | `GOWORK=off GOPROXY=off go test ./internal/runtime/clientopts ./internal/runtime`、对应 race、`go vet` 通过；确认 Claude settings 合同、网络域名准入和进程策略 fingerprint |
| 三仓 pin | SDK `9d60e166`、Bridge `02fbc0e5f6a6...`、Nexus `67cb67e85` 均为本地提交，未推送；Nexus 无 replace，使用本地 module cache/proxy 精确校验 |
| 未闭合项 | Claude 真实命令允许/拒绝、网络/Provider 凭据/辅助 IO、取消/后代清理、macOS/Windows/Linux clean-host、签名安装包与 P7 发布验收仍未完成 |

本批次将 Claude 从“仅 `--restricted` 启动参数”推进到“原生 settings 配置已接线”，
但不把配置准入当作 OS 生效回执；`releaseAccepted=false`，Goal 仍 active。

## 本次重新审计的执行记录

日期：2026-09-15。Nexus 基线 928c7e803；Bridge module 为 3da56a2；SDK 使用 c8245262 的独立导出，排除工作区未提交变更。

本节是该版本组合的历史记录；复现时使用对应 Nexus 版本的脚本和依赖。当前入口及最新 SDK 参考见文末，不能用新增能力门禁验证不具备该能力的旧版本。

实际平台：macOS 27.0（26A428），arm64；Go 1.26.2。本机独立源码/binary 目录为 /private/tmp/nexus-desktop-sandbox-audit.4mhcm0。该路径只用于复核本次结果；后续应使用下述自动入口生成新的源码导出和证据目录。

| 检查 | 当前结果 | 证据范围 |
| --- | --- | --- |
| Nexus clientopts / permission / nxsruntime 定向测试，GOWORK=off | 通过，exit 0 | 固定 Bridge module 的宿主装配、审批与诊断逻辑 |
| Windows run 34918708741 状态复核 | completed / failure | GitHub 现存 run 元数据；未重新执行 Windows |
| SDK 固定提交编译真实 nxs | 通过，exit 0 | 仅证明该 macOS binary 构建成功 |
| macOS 真实命令、文件与指令读取 | 通过，exit 0；7 个顶层测试、2 个后台子场景 | Read/Write/Edit、导入、大文件、子进程写边界、直连拒绝、批准连接及旧 epoch 失效 |
| Nexus → 固定 Bridge → 真实 nxs 握手/诊断 | 通过，exit 0；2 个测试 | 只读真实进程链；不发送模型请求；依赖齐全不代表会话沙箱开启 |
| DM/Room 沙箱模式替换定向测试 | 通过，exit 0 | runtime / room/realtime 中的 Sandbox 测试 |
| runtime 设置页与 controller 组件测试 | 通过，exit 0；23 个测试 | DOM 行为测试，不是安装包视觉验收 |
| 新自动基线：固定 SDK 导出/构建＋宿主＋macOS 原生 | 通过，exit 0；14 个宿主＋7 个原生顶层用例，必测项无 skip | [报告](./evidence/desktop-sandbox/2026-09-15-macos-arm64/report.json)；基线覆盖，不是全工具或发布验收 |
| 门禁证据解析测试与架构检查 | 通过，exit 0；6 个证据用例 | 拒绝缺失/跳过/失败/损坏证据；生产依赖方向检查 |
| 签名安装包、Windows 本机、Linux owner 实机 | 未执行 | 发布门禁仍保留 |

首次离线测试因禁用 checksum database 导致 Go toolchain 校验失败；保留正常校验后，用 GOPROXY=off 重新运行成功。环境启动失败不计入业务测试通过数。

macOS 测试第一次在外层禁止 localhost listen 的环境中失败；在获准运行临时回环服务器的环境中复测通过。没有为通过测试更改 SDK 安全断言或扩大产品策略。

本次 nxs SHA-256：fbdb0b5b677ec8f274697f0e9f3e27aa65f6978292183ecf14700a0345477386。构建源为 SDK c8245262 的 git archive；binary 位于本机临时审计目录，不是发布安装包。

随后通过新自动入口重新导出和构建同一提交，binary SHA-256 为 221aadea5f635672dc3f5ca57f9bcf1005677c8a872496ca85c7420fcbb60776。构建路径不同，两份 binary 分别记录；不宣称本次构建具备字节级可重复性。自动入口的原始报告与命令日志已归档到 [证据目录](./evidence/desktop-sandbox/2026-09-15-macos-arm64/)，其中包括 [宿主策略日志](./evidence/desktop-sandbox/2026-09-15-macos-arm64/host-policy.stdout.log)、[生命周期日志](./evidence/desktop-sandbox/2026-09-15-macos-arm64/host-lifecycle.stdout.log) 和 [原生日志](./evidence/desktop-sandbox/2026-09-15-macos-arm64/macos-native.stdout.log)。

复现命令（先从上述 SDK 提交构建 nxs，并设置绝对 binary 路径）：

```sh
# Nexus 仓库；必须脱离本地 go.work。
GOWORK=off go test ./internal/runtime/clientopts ./internal/runtime/permission ./internal/service/nxsruntime
NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs GOWORK=off go test ./internal/runtime/clientopts ./internal/service/nxsruntime -run 'TestDesktopSandboxRealRuntimeNegotiation|TestSandboxDiagnosisRealNXS' -count=1 -v
GOWORK=off go test ./internal/runtime ./internal/service/room/realtime -run 'Test.*Sandbox' -count=1

# SDK 固定提交；原生测试需要允许临时文件、本地监听与 Seatbelt 子进程。
NEXUS_FILE_HELPER_TEST_BINARY=/absolute/nxs NEXUS_SANDBOX_INTEGRATION=1 GOWORK=off go test ./internal/tool/executor -run '^TestDarwin(RequiredSandbox|SandboxFileToolsRuntime|FileInstructionsShareSandbox|LargeFileReadRetainsFullState)' -count=1 -v

# Nexus web 目录。
corepack pnpm exec vitest run src/features/settings/runtime/settings-runtime-section.test.tsx src/features/settings/runtime/use-runtime-settings-controller.test.tsx
```

## 后续记录

完成对应测试后用结果替换“执行中”，不要追加互相矛盾的成功/失败结论。原始日志可作为附件，文档保留可复现命令与必要摘要；不得写入凭据或完整进程环境。

### P1 分项文件能力，本地集成（2026-09-15）

SDK `c8580cf49342a7d946c849ecb04c1db01c30eb4c`；Bridge `a4fef0aec5bc4049cf8c4e20047fcb2f8bedd45c`；Nexus 在 `5a781cece` 上验证本批次未提交改动，准确清单保留在报告。用户要求只保留本地提交，三个仓库均未推送。

Bridge 固定为 `v0.1.34-0.20260915080938-a4fef0aec5bc`，由固定 Git 提交生成标准 module archive，再从本机 `file://` proxy 取得；Go 计算的 checksum 为 `h1:GSjnxL0+p1yQtRf4bvoUeKQS3jv7qpKdoYL3efA9XEE=`。本轮脱离 go.work、没有 replace，但**模块尚未发布，只证明本地固定依赖集成**。新机器在线取得依赖仍属于 P7。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| SDK initialize/分项能力 | 5 个顶层测试通过，exit 0 | 拒绝缺基础能力、错误类型和非法初始化；不修改既有状态 |
| Bridge client/protocol/transport | 3 个包通过，172 个顶层 pass，exit 0 | 具名 skip 保留在报告；未提供 binary 的进程测试在下一行独立执行 |
| 固定 Bridge 模块 → 新旧真实 nxs | current、legacy、基础合同通过，exit 0，无 skip | 旧版本明确因缺文件能力拒绝；不发送模型请求 |
| 固定 SDK 提交导出＋本地固定模块基线 | 14 个宿主、7 个 macOS 顶层用例通过，exit 0，无必测 skip | 生命周期、审批、命令/文件隔离及网络；并非全部 IO 覆盖 |
| 架构检查、Windows 测试程序交叉编译 | 通过，exit 0 | Windows 未原生运行；Claude、Linux owner、安装包未验收 |

固定 SDK 导出所构建 nxs 的 SHA-256 为 `2826515dc193bd41a5e05cde5cfe799c7361f0bf027d9223b39a38d2655e720f`；旧 binary 继续使用本页 P0 记录。详见 [汇总与局限](./evidence/desktop-sandbox/2026-09-15-file-capability/report.json)、[固定版本基线](./evidence/desktop-sandbox/2026-09-15-file-capability/baseline-report.json) 与 [真实新旧进程日志](./evidence/desktop-sandbox/2026-09-15-file-capability/bridge-pinned-real-process.jsonl)。本机 module proxy 位于 `/private/tmp/nexus-sandbox-capabilities.60SUhq/local-proxy`，生成器位于同目录的 `module-builder`，均为本地复核材料。

```sh
# 本节使用 Nexus 18c9964a2 的入口；最新入口还要求 P2 新增原生用例。
# 本机复核：先载入固定本地模块，保留其他模块与 toolchain 的常规校验。
GOWORK=off GOPROXY=file:///private/tmp/nexus-sandbox-capabilities.60SUhq/local-proxy GONOPROXY=none GONOSUMDB=github.com/nexus-research-lab/nexus-agent-sdk-bridge go mod download github.com/nexus-research-lab/nexus-agent-sdk-bridge@v0.1.34-0.20260915080938-a4fef0aec5bc
GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref c8580cf49342a7d946c849ecb04c1db01c30eb4c

# NEXUS_SANDBOX_TEST_BINARY 指向上述固定提交构建结果；旧 binary 取 P0 来源。
GOWORK=off GOPROXY=off NEXUS_SANDBOX_TEST_BINARY=/absolute/new/nxs NEXUS_SANDBOX_LEGACY_TEST_BINARY=/absolute/old/nxs go test -mod=readonly github.com/nexus-research-lab/nexus-agent-sdk-bridge/client -run 'Test(FileSandboxRealProcess|RequiredSandboxRealProcess)$' -count=1 -timeout=2m -json
```

### P2 系统 Seatbelt 路径（2026-09-15）

SDK `12ea63b8fa7664c8abe5dcde625ec2905ddfa5b8`，Bridge 继续使用上述本地固定模块。Nexus 基线为 `18c9964a2`，本次入口变更及其他 dirty 清单记录在报告；SDK 由干净提交导出构建。

修复前的真实测试复现两项问题：任务 PATH 能选中伪造的 `sandbox-exec`；PATH 不含系统目录会误报系统后端不可用。修复后固定 `/usr/bin/sandbox-exec`，在同一测试中证明允许写仍能完成、子进程的禁止写被拒绝、伪造程序未执行。参考为 Codex `4e6450bb` 的 `codex-rs/sandboxing/src/seatbelt.rs:63`，只是该路径选择规则的参考。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 修复前反例 | 预期失败，exit 1 | 仅新增测试、尚未修改生产代码；保留伪造后端与空 PATH 的失败事实 |
| 固定提交系统路径回归 | 2 个顶层、2 个执行子场景通过，exit 0，无 skip | 空 PATH、同名可执行文件、真实允许/拒绝及子进程写入 |
| sandboxexec 包回归 | 109 个顶层测试通过，exit 0 | Linux helper 与未启用的原生执行测试有 skip；原生场景由上一行独立验证 |
| 固定版本完整开发基线 | 14 个宿主＋7 个原生隔离＋上述 2 个路径顶层用例通过，exit 0 | 命令/文件、审批、生命周期；无必测 skip，releaseAccepted=false |
| Windows 变更包交叉编译、证据解析与入口语法 | 通过，exit 0；6 个证据解析用例 | 不代表 Windows 原生运行或安装包验收 |

nxs SHA-256：`7482d5d18a6d00f510eaa78a5c172a71e9a618a8af21eca28e78a6ed6126b97e`。详见 [范围与限制](./evidence/desktop-sandbox/2026-09-15-system-seatbelt/report.json)、[固定提交基线](./evidence/desktop-sandbox/2026-09-15-system-seatbelt/baseline-report.json)、[修复前日志](./evidence/desktop-sandbox/2026-09-15-system-seatbelt/seatbelt-path-before.jsonl) 和 [修复后原生日志](./evidence/desktop-sandbox/2026-09-15-system-seatbelt/macos-backend-path.stdout.log)。所有提交与产物按用户要求仅保留本地；helper 版本准入、完整 IO、资源策略、生效回执和发布仍未完成。

### P1/P2 强制资源禁止优先（2026-09-15）

SDK `d9687a3ed9d3a7da86b84be02a49ddbbed6dfa91`，Bridge 沿用本地固定模块 `a4fef0a`；Nexus 基线 `5747c2744` 加本次门禁变更，精确 dirty 清单保留在报告。所有版本与产物继续仅在本机，未发布。

修复前，同级、父级和子级 `allowRead` 均让真实 Bash/Read 读出 `denyRead` 保护的测试内容，6 个子场景失败、exit 1。目录移动的原有保护已经通过，并非本次新发现的绕过。修复将必需模式的读写和移动禁止项最终施加在普通授权之后，保留非必需模式的兼容 read carve-out。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 重叠读根与符号链接的 Bash/Read | 8 个子场景通过，exit 0 | 允许文件仍可读；禁止内容不返回；两类工具共享实际 OS 边界 |
| 受保护祖先目录移动 | read/write 两个子场景通过，exit 0 | 保留源目录、目标未创建，命令正常启动后被拒绝 |
| 固定提交开发基线 | 25 个顶层用例通过，exit 0，必测项无 skip | 14 个宿主＋7 个原生隔离＋2 个后端路径＋2 个资源禁止；新增逐项核验 10 个资源子场景及 2 个后台网络子场景 |
| sandboxexec 包、Windows 交叉编译 | 109 个顶层测试通过；编译 exit 0 | Linux helper 及未启用的原生路径测试有 skip；当前 macOS 场景由固定提交基线独立执行；未运行 Windows 原生 |
| 门禁解析与入口语法 | 6 个解析测试通过；语法检查 exit 0 | 证据检查，不替代完整产品验收 |

nxs SHA-256：`801c33a9448a517d1c6c6cb5eee5f797b0ccd405279e42ec04daffecdf87fadb`。详见 [结果与限制](./evidence/desktop-sandbox/2026-09-15-mandatory-deny/report.json)、[固定版本基线](./evidence/desktop-sandbox/2026-09-15-mandatory-deny/baseline-report.json)、[修复前反例](./evidence/desktop-sandbox/2026-09-15-mandatory-deny/mandatory-deny-before.jsonl) 和 [真实资源回归](./evidence/desktop-sandbox/2026-09-15-mandatory-deny/macos-resource-denials.stdout.log)。`denyRead` 不自动变成 `denyWrite`；本批次未接入只读 profile、实际策略回执、其他平台或全部 SDK IO。

### P1/P2 写入范围与私有 scratch 合同（2026-09-15）

SDK `67975b901ce6700d88ac4b64da80ae2254b4c3cf`、Bridge `1fc3dca78dd8bc23e45a7c7d4c6f6754ad0b11f8`，Nexus 固定本地模块 `v0.1.34-0.20260915091919-1fc3dca78dd8`。模块来自精确提交的本机 file proxy，尚未发布。固定 SDK 归档构建的 nxs SHA-256：`9168d5187010f9a1409e917d6372a04229115d795a3ecd80b93dfad121fc2c71`。

| 检查 | 结果 | 证据范围 |
| --- | --- | --- |
| 原生 read-only/workspace-write | 两个具名子场景通过 | 真实 Read/Write/Edit/Bash；工作区、明确写根、scratch；HOME/TMPDIR、硬链接、符号链接和显式旁路不能扩大只读写范围 |
| SDK 资源合同与文件拒绝 | 9 个合同顶层用例＋1 个文件拒绝用例通过 | 无效版本、字段、目录和冲突授权在初始化/执行准备前拒绝；普通 settings 不替代宿主权威；Windows PowerShell 准入逻辑在 macOS 上通过错误请求拒绝测试 |
| SDK 目标包 | 285 个顶层用例成功，exit 0 | 包含有条件跳过的 PowerShell/原生/平台用例，完整 skip 在报告；当前 macOS 必测行为由固定基线独立验证 |
| Bridge 目标包、race 与新旧真实进程 | 目标包 exit 0；race 定向 exit 0；资源真实进程组 4 个顶层用例、current/legacy 均成功且无 skip | 只有命令和文件能力的旧 nxs 在任务前被拒绝；资源变化需替换进程；连接前拒绝的关闭回归先超时失败、修复后通过 |
| 固定提交开发基线 | 26 个顶层用例与 16 个指定子场景通过，必测项无 skip | 14 个宿主＋7 个原生隔离＋2 个后端路径＋2 个资源禁止＋1 个资源范围顶层用例 |
| Windows 交叉编译、门禁及架构 | executor/nxs 测试二进制编译 exit 0；6 个解析测试、入口语法及架构检查通过 | 没有运行 Windows 原生或安装包验收 |

详见 [结果与限制](./evidence/desktop-sandbox/2026-09-15-resource-scopes/report.json)、[固定版本基线](./evidence/desktop-sandbox/2026-09-15-resource-scopes/baseline-report.json)、[原生写入范围](./evidence/desktop-sandbox/2026-09-15-resource-scopes/macos-resource-scopes.jsonl) 和 [新旧实际进程](./evidence/desktop-sandbox/2026-09-15-resource-scopes/bridge-pinned-real.jsonl)。Nexus 当前只固定新 Bridge，尚未提供 Resources；SDK 不证明 scratch 独占租约或回收。限定读取、全 SDK IO、实际策略回执、默认产品策略、Claude、其他平台及发布验收仍未完成。

### P1/P2 清理失败与会话栅栏（2026-09-15）

Bridge `034c449fb4cf1d402213a07450e0d02d7578bf9d`，Nexus 固定本地模块 `v0.1.34-0.20260915095108-034c449fb4cf`，校验和 `h1:8ksEXEKjVOEMXmzH4upmOHoQAI7hD5u04pQTE6sHze4=`。SDK 仍为 `67975b901ce6700d88ac4b64da80ae2254b4c3cf`，本次重新从该提交归档构建的基线 nxs SHA-256 为 `66c9c61a7de6203ed13e46e8d8bb53b70273913a946e410def14b0c5c6dae0eb`。所有版本仅保留本地。

| 验证 | 结果 | 证据边界 |
| --- | --- | --- |
| 修复前回归 | Bridge 1 个、Nexus 2 个顶层用例按预期失败 | 主进程 exit 0 掩盖清理失败；失败后仍重连或重试旧配置启动 |
| Bridge 竞态与原生清理 | client/transport 目标包通过 | 含主动 TERM、重复 Close、最终观察失败/残留、宿主信号失败与真实同 session 后代；默认跳过项逐一记录 |
| 实际新旧 nxs | 3 个顶层与 4 个子场景全部通过，无 skip | 当前版本准入、旧文件/资源能力拒绝；没有模型请求；不等同于完整进程树验收 |
| Nexus runtime 子包与竞态 | 目标包通过，20 个定向竞态顶层用例无 skip | 同步/超时后失败、重连、替换、owner/Agent/idle 关闭以及宿主资源指纹；目标包的平台跳过项见报告 |
| 固定 SDK/Bridge 基线 | 32 个必测顶层与 21 个指定子场景通过，无缺失或 skip | GOWORK=off、无本地 replace；宿主与 macOS 开发基线，非发布验收 |
| Linux/Windows | client 与 transport 均交叉编译通过 | amd64、CGO 关闭；没有原生运行、owner 隔离或安装包证据 |
| 架构与证据校验器 | 架构门禁和 6 个证据解析测试通过 | 依赖方向、具名用例缺失/跳过/失败检测 |

详见 [结果与限制](./evidence/desktop-sandbox/2026-09-15-cleanup-fences/report.json)、[固定版本基线](./evidence/desktop-sandbox/2026-09-15-cleanup-fences/baseline-report.json)、[宿主竞态](./evidence/desktop-sandbox/2026-09-15-cleanup-fences/cleanup-host-race.jsonl)、[Bridge 竞态](./evidence/desktop-sandbox/2026-09-15-cleanup-fences/cleanup-bridge-race.jsonl) 与 [文件校验和](./evidence/desktop-sandbox/2026-09-15-cleanup-fences/manifest.json)。Unix session 观察不覆盖另建 session 的后代；成功信号回调不代表 Bridge 独立确认退出。Nexus 栅栏只在当前进程内保留，尚无持久清理回执或自动对账恢复。本批没有准备、租约或回收 scratch，也没有启用默认产品资源策略；完整 IO、后代监督、Claude、其他平台及发布门禁仍未完成。

### P1/P2 搜索工具隔离与独立准入（2026-09-16 归档）

SDK `19c80fb21bb6a00d1132fdea3e63251dac4c015d`、Bridge `f6e456da2ecbfd3c6b9edfbd62d43d7e25e6b103` 已本地提交，Nexus 固定 `v0.1.34-0.20260915103347-f6e456da2ecb`，模块校验和 `h1:TOcpWlLdRaKpBe1rz5EldtyuwRR48WypXsj9C331gMo=`。固定 SDK 归档构建的 nxs SHA-256：`597cae76273dd59c71f3d911f7c203f5022e80b68287ee9e5784bd10d0ae742c`。运行记录主要产生于 9 月 15 日，9 月 16 日完成归档和文档/架构检查；所有提交和模块仍未发布。

修复前，Glob 和 Grep 的三种输出模式可经直接目录或符号链接泄露被禁止文件的名称/内容；自定义 rg 可继承测试环境变量、连接测试回环服务并写入只读工作区。修复将路径检查、建议、rg 与结果元数据绑定到同一受限文件环境；独立 `sandbox_search_tools_v1` 要求让仅具备旧文件能力的 SDK 在任务前被拒绝。

| 验证 | 结果 | 证据边界 |
| --- | --- | --- |
| 修复前反例 | 2 个顶层场景失败、exit 1 | 8 个直接/链接读取子场景与自定义 rg 越权事实；测试标记均为夹具数据 |
| 原生搜索 | 4 个顶层与 18 个指定子场景通过 | 目录/绝对模式/单文件/计数/无匹配/路径建议，强制 deny、辅助环境/网络/写限制与准备失败；固定提交基线无必测 skip |
| 分项搜索合同 | 2 个顶层与 7 个指定子场景通过 | 未协商、缺命令/文件依赖、类型错误、平台及兼容分支；普通 settings 不替代宿主要求 |
| SDK 目标包 | 165 个顶层、76 个子场景通过 | 平台/显式原生测试及无测试文件包的 skip 在报告列出；受限搜索竞态另有原始记录 |
| Bridge 与宿主竞态 | Bridge 154 个顶层/74 个子场景，Nexus 157 个顶层/39 个子场景通过 | 指定 client/protocol 和 runtime/clientopts 包；真实进程 opt-in 在独立组验证 |
| 新旧真实 SDK | 4 个顶层与 6 个子场景通过、无 skip | GOWORK=off 的固定 Bridge 模块；当前版本及分别缺文件/资源/搜索能力的三个旧二进制；不发模型请求 |
| 固定版本开发基线 | 38 个顶层、46 个指定子场景通过 | GOWORK=off、无 replace、固定源码构建、全部必测实际运行，releaseAccepted=false |
| Windows/Linux | SDK 与 Bridge 变化包 amd64 交叉编译通过 | CGO 关闭，只证明编译，不证明原生限制、Linux owner 隔离或安装包可用 |
| 其他门禁 | 架构检查、6 个证据解析用例、入口语法通过 | 依赖方向与验收记录完整性 |

原始记录见[结果和限制](./evidence/desktop-sandbox/2026-09-16-search-tools/report.json)、[固定提交基线](./evidence/desktop-sandbox/2026-09-16-search-tools/baseline-report.json)、[修复前反例](./evidence/desktop-sandbox/2026-09-16-search-tools/search-before.jsonl)、[原生搜索](./evidence/desktop-sandbox/2026-09-16-search-tools/macos-search.stdout.log)、[新旧真实进程](./evidence/desktop-sandbox/2026-09-16-search-tools/bridge-pinned-real.jsonl)和[校验和](./evidence/desktop-sandbox/2026-09-16-search-tools/manifest.json)。受限搜索取消、超时或达到输出限额时返回失败，不报告部分成功；自定义 argv0 尚不支持。Notebook、启动/Skill/配置/后台 IO、实际生效回执、默认产品策略、Claude 与安装包继续待验收。

### macOS 后代监督实验

9 月 15 日的两个原生探测只操作自身测试进程，不改系统授权、订阅系统事件或操作其他应用：

| 实验 | 结果 | 下一步约束 |
| --- | --- | --- |
| 父进程退出，子进程另建 session | 原组已不存在，子进程仍存活；夹具最后显式清理自己的子进程并确认退出 | 原组/session 扫描不能作为全部后代终态或 scratch 回收证明 |
| kqueue `NOTE_TRACK` | 返回 `ENOTSUP` | 不能依靠该接口跟踪 macOS 完整后代 |
| macOS 27 `es_new_descendants_client` | API 存在，但返回 `ERR_NOT_ENTITLED`；没有订阅事件 | Endpoint Security entitlement、系统支持范围、精确身份、事件缺失及崩溃恢复均未完成验收；未改变产品最低系统版本 |

见[原生结果](./evidence/desktop-sandbox/2026-09-16-search-tools/native.json)、[探测程序](./evidence/desktop-sandbox/2026-09-16-search-tools/probe.py)、[Endpoint Security 结果](./evidence/desktop-sandbox/2026-09-16-search-tools/endpoint.json)及[程序](./evidence/desktop-sandbox/2026-09-16-search-tools/endpoint_probe.c)。`NOTE_TRACK` 拒绝与 [Apple XNU 实现](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/kern/kern_event.c)一致；新后代 API 的可用版本和 entitlement 来自本机 macOS SDK 的 `EndpointSecurity/ESClient.h`，公开入口见 [Apple 文档](<https://developer.apple.com/documentation/endpointsecurity/es_new_descendants_client(_:_:)>)。

Codex 对照仍固定 `4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd`，此次读取 [spawn.rs](https://github.com/openai/codex/blob/4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd/codex-rs/core/src/spawn.rs) 和 [exec.rs](https://github.com/openai/codex/blob/4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd/codex-rs/core/src/exec.rs) 的进程启动/退出片段；局部进程组清理代码不能证明任意脱离后代已被监督。完整后代清理、持久失败回执、scratch 租约和自动回收仍未实现。

2026-09-28 补充：[用户级 launchd coalition 原型](evidence/desktop-sandbox/2026-09-28-process-coalition/README.md)的三个本机场景通过：单次脱离、双 fork/exec、Seatbelt 内双 fork/exec。父进程退出后集合仍跟踪后代，外部观察取得精确 audit identity；错误 PID version 的信号被拒绝，正确身份终止后原集合被内核回收，独立对照仍存活。测试 job 最终均不存在，对照子进程均已回收。`LaunchOnlyOnce` 的 job 先消失而后代仍存活的反例明确保留，因此 job 缺失和资源计数不能代替终态证明。

本次只验证候选机制，没有修改生产启动/取消/恢复链、最低系统版本或旧 unknown 回执。XNU `xnu-10002.1.13` 缺少精确 audit-token 信号入口，较新的 `xnu-10002.61.3` 有；产品最低 macOS 14.0、私有观察 ABI、Intel、持久登记及真实崩溃恢复仍待验证或实现。后续合同见[非规范接入方案](../explorations/desktop-sandbox/macos-process-supervision.md)，`releaseAccepted=false`。

## 媒体文件入口子批次（2026-09-16）

SDK `143e987c2e45adece435bda1621aef81bcac4040`、Bridge `eeaff7df69f324bf5d1e346692e30f39235ce0c5`；Nexus 在 `86ec36fca` 上验证本批次工作树，准确变更列表保存在报告。Bridge 固定为 `v0.1.34-0.20260916013218-eeaff7df69f3`，本机 exact-commit module archive 的 checksum 为 `h1:txRuwSPedbxkbZEPNilyj5BxoT2/de6t6ADnhxNI0X4=`。全部只保留本地提交，模块未发布，新机器取得依赖尚未验收。

| 检查 | 结果 | 覆盖与限制 |
| --- | --- | --- |
| 修复前原生反例 | 2 个顶层、6 个子场景失败 | SDK 19c80fb2 的来源解析加测试和行为不变的入口整理；绝对路径、file URL、链接、附件引用以及主模型用户/工具图片可读取被禁止的测试 PNG |
| 正常路径回归 | 初次发现 2 个子场景失败，修复后通过 | 普通本地路径曾原样传给模型；现在同 file URL 一起受限读取并物化 |
| 原生媒体读取 | 4 个顶层、19 个指定子场景通过 | 允许/拒绝、用户/嵌套工具、惰性引用、内联图片及准备失败；使用真实 Seatbelt/helper 和进程内假模型 |
| 独立媒体合同 | 2 个顶层、7 个指定子场景通过 | 类型、依赖、旧文件合同、协商与平台；不能冒用 Claude 的能力 |
| SDK/Bridge/宿主目标包竞态 | 分别 266/158/157 个顶层用例通过 | 分别 104/77/39 个子场景；平台及 opt-in skip 在报告列明，原生媒体与真实进程由单独门禁强制执行 |
| 新旧真实 SDK | 5 个顶层、8 个子场景通过，无 skip | GOWORK=off 固定 Bridge；当前 SDK 及分别缺文件、资源、搜索、媒体能力的旧二进制，不发送模型请求 |
| 固定版本开发基线 | 44 个顶层、72 个指定子场景通过 | 固定 SDK git archive 构建、无 Bridge replace；缺失或 skip 必测均失败，releaseAccepted=false |
| 其他门禁 | 架构、6 个证据解析器测试、入口语法通过 | SDK/Bridge Windows/Linux amd64 CGO=0 仅交叉编译，不计原生验收 |

本次 nxs SHA-256：`a61a7f632062d9f209755e957b64c446e3c43bdd6d763776f5f8cc676d4490f6`。原始记录见[报告与限制](./evidence/desktop-sandbox/2026-09-16-media-files/report.json)、[固定版本基线](./evidence/desktop-sandbox/2026-09-16-media-files/baseline-report.json)、[修复前反例](./evidence/desktop-sandbox/2026-09-16-media-files/media-before.jsonl)、[普通路径问题](./evidence/desktop-sandbox/2026-09-16-media-files/media-race.jsonl)、[原生媒体读取](./evidence/desktop-sandbox/2026-09-16-media-files/macos-media-files.stdout.log)、[新旧真实进程](./evidence/desktop-sandbox/2026-09-16-media-files/bridge-pinned-real.jsonl)和[校验和](./evidence/desktop-sandbox/2026-09-16-media-files/manifest.json)。

本地读取先于辅助分析缓存，失败不回退；每次读取端口独立传递，缓存拒绝及并发请求隔离另有竞态回归。该媒体能力不覆盖远程 HTTP 下载和图片 URL 直传的网络策略。启动/Skill/配置/后台 IO、后代监督、持久清理回执、scratch、默认产品策略、其他平台/Claude 与安装包继续待验收。

## P1/P2 Skill 文件与发现链路（2026-09-16）

SDK `3d7938cdb4524a603c40401375195d7e162b474e`、Bridge `f903386e6cfaaf1fddc3615b56af1362b0c03bf2`；Nexus 在独立分支 `codex/desktop-sandbox-isolated` 的 `69451849e` 上验证本批次变更。Bridge 固定为本地模块 `v0.1.34-0.20260916021122-f903386e6cfa`，checksum 为 `h1:flKp3ML5SGPuivpzsVYNzOimxxqYKFrEAL9WPPh+kwc=`。模块来自精确 Git 提交归档，未发布，不能作为新机器可在线取得依赖的证明。

先在 SDK `143e987c` 复现四类来源 × 五个入口共 20 个 Skill 越界读取、记忆设置读取及 Git 辅助进程的文件/环境/网络旁路。修复将初始目录、DiscoverSkills、Skill、用户 Slash、Read 触发的动态发现及 remember 设置读取绑定到相同文件端口；Git 使用相同资源范围和最小环境。另以正常配置测试复现并修复相对 settings 路径未以当前 workspace 解析的问题。Git 测试增加可写 scratch 中的执行回执，独立证明命令确实运行、环境清理及文件/网络拒绝，不能仅凭未出现越界文件推断成功。

| 验证 | 结果与范围 |
| --- | --- |
| 固定 SDK 导出构建、固定 Bridge 与 macOS 开发基线 | exit 0；52 个顶层、128 个指定子场景通过；所有必测项无 skip |
| Skill 原生 race | exit 0；6 个顶层、49 个子场景；允许/拒绝入口、链接、动态/条件发现、Git 忽略、非仓库、配置和辅助进程 |
| SDK 目标包 race 与 vet | exit 0；306 个顶层、125 个子场景通过；18 个 opt-in/platform skip 单列，不能替代原生门禁 |
| Bridge 目标包 race | exit 0；192 个顶层、103 个子场景通过；3 个 opt-in skip 单列 |
| Nexus runtime 目标包 race | exit 0；180 个顶层、51 个子场景通过；1 个显式二进制 opt-in 由固定基线单独执行 |
| Nexus 实际固定 Bridge → 真实新旧 nxs | exit 0；6 个顶层、10 个子场景，当前与五个历史二进制均执行，无 skip；不发送模型请求 |
| SDK/Bridge Windows 与 Linux amd64 | CGO_ENABLED=0 编译通过；没有原生运行、owner 或发布证明 |
| 架构与证据解析 | exit 0；生产依赖方向通过，6 个门禁解析用例通过 |

本次固定构建 nxs SHA-256 为 `ac7229b542218770a695be59104919b6fd1005bfb627785e3bc733f74999b6cb`。原始记录见[报告与限制](./evidence/desktop-sandbox/2026-09-16-skill-files/report.json)、[固定基线](./evidence/desktop-sandbox/2026-09-16-skill-files/baseline-report.json)、[修复前反例](./evidence/desktop-sandbox/2026-09-16-skill-files/skills-before.jsonl)、[原生 Skill](./evidence/desktop-sandbox/2026-09-16-skill-files/macos-skill-files.stdout.log)、[真实新旧准入](./evidence/desktop-sandbox/2026-09-16-skill-files/skills-bridge-pinned-real.jsonl)及[校验和](./evidence/desktop-sandbox/2026-09-16-skill-files/manifest.json)。本机私有仓库配置曾绕过本地代理，成功取得模块使用任务级 `GONOPROXY=none` 和本地 file proxy；架构检查使用可写的任务 GOCACHE。

P1/P2 仍进行中。全局启动/compact 重载设置与指令、Skill hook、后台 IO、远程网络、生效回执、完整后代监督与 scratch 租约尚未闭合；默认产品策略、原生 Windows/Linux owner、Claude 与签名包/升级继续保留 P3–P7 门禁。所有提交仅本地，`releaseAccepted=false`。


## P1/P2 指令与压缩上下文文件（2026-09-16）

SDK `d3695455cb0323a2710440f5d57ab8864d3d5201`、Bridge `318c79166b2061369b1a65c4bd4609d746be1c36`；Nexus 在独立分支 `codex/desktop-sandbox-isolated` 的 `8a21bd633` 上验证本批次变更，报告保存对应源码文件的哈希。Bridge 固定为本地精确提交模块 `v0.1.34-0.20260916025049-318c79166b20`，checksum 为 `h1:pe2muTUpmkbuDkA0VfOt0nqoC7fRAo/kxTJc+TZHyYY=`，未发布。

先在 SDK `3d7938cd` 上复现七类启动指令读取与两种 compact 文件恢复的越界内容，共九个失败子场景。修复后，启动和重载通过工具文件执行器读取指令及排除设置；compact 的近期文件恢复重新检查当前内容，历史 read-state 不再通向宿主读取。读取/解析排除配置失败会拒绝启动或重载；重载失败清除旧指令，并在读取恢复前阻止下一次模型请求。主任务、手动压缩及子任务传递当前取消上下文。

| 验证 | 结果与边界 |
| --- | --- |
| 固定 SDK 原生及宿主基线 | exit 0；58 个顶层、171 个指定子场景；必测项无 skip |
| 上下文原生测试 | exit 0；4 个顶层、36 个子场景；允许/拒绝来源、include、链接、排除配置、取消与重载恢复 |
| SDK 目标包 race / vet | exit 0；357 个顶层、134 个子场景；所有 opt-in/platform skip 在报告具名保存，原生用例由固定基线单独执行 |
| Bridge 目标包 race / vet | exit 0；195 个顶层、106 个子场景；显式二进制用例在下一行独立执行 |
| 实际固定模块的新旧 nxs | exit 0；7 个顶层、12 个子场景；全部执行、无模型请求、无 skip |
| Nexus runtime 子包 race | exit 0；219 个顶层、191 个子场景；二进制 opt-in 由基线覆盖，Windows/Linux 原生 skip 不视为通过 |
| SDK/Bridge Windows/Linux amd64 | CGO_ENABLED=0 编译通过；不代表原生或 owner 验收 |
| 架构与门禁解析 | exit 0；依赖方向通过，6 个解析用例通过 |

原生 runtime 测试通过测试入口运行生产文件 worker，并真实施加 Seatbelt；固定归档另构建真实 nxs 用于宿主和 Bridge 进程验收。原生上下文测试本身没有开启 race，受影响包另跑 race。真实进程首轮漏传旧资源二进制而 skip 一项，保留原记录；补齐后最终整组重跑无 skip。

固定构建 nxs SHA-256 为 `35b5286f59c377f93f97ed5159d675dc1d994fd4fb901a85e5948f2a2a63b608`。完整记录见[报告与范围](./evidence/desktop-sandbox/2026-09-16-context-files/report.json)、[固定基线](./evidence/desktop-sandbox/2026-09-16-context-files/baseline-report.json)、[修复前反例](./evidence/desktop-sandbox/2026-09-16-context-files/before.jsonl)、[原生上下文](./evidence/desktop-sandbox/2026-09-16-context-files/native-context.jsonl)、[真实进程](./evidence/desktop-sandbox/2026-09-16-context-files/bridge-pinned-real.jsonl)与[校验和](./evidence/desktop-sandbox/2026-09-16-context-files/manifest.json)。

P1/P2 仍进行中。全局权限/Provider 配置、项目 Agent/命令定义、hook 和后台 IO、远程网络、有效策略回执、后代监督、scratch、默认产品策略、原生其他平台/Claude 及安装包仍未闭合。本批次只增加独立 `sandbox_context_files_v1` 能力，不代表整个 SDK 进程已被统一 OS 沙箱包住。所有提交仅本地，`releaseAccepted=false`。

## 自动基线入口

[check-sandbox-baseline.mjs](../../scripts/desktop/check-sandbox-baseline.mjs) 强制 GOWORK=off 和只读 module 解析，拒绝 Bridge replace，记录模块版本、checksum、源码提交、binary SHA-256、OS/架构、每条命令和最终退出码。具名必测用例 skip 或未匹配均失败，不以包级 PASS 替代。

原生入口还逐项要求伪造 PATH、空 PATH、资源禁止、写入范围、搜索/媒体/Skill 合同、正常/拒绝搜索、图片与 Skill 读取、后台网络子场景的成功证据，不能只凭父测试 PASS；指令启动/重载及配置排除用例也逐项要求正反向证据。当前使用 SDK d3695455 或包含这些用例的后续提交。历史记录使用各自对应的 Nexus 版本入口复核。

```sh
# 已有 nxs：宿主集成基线，包含真实握手/诊断；不宣称原生隔离已验收。
NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox

# macOS：导出固定 SDK 提交、构建 nxs，再执行宿主与原生隔离基线。
# SDK dirty 改动只记录清单，git archive 不包含这些改动。
node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref d3695455cb0323a2710440f5d57ab8864d3d5201
```

原生模式需允许运行临时回环服务器与 Seatbelt；不会发送模型请求、设置账号、防火墙或开启产品开关。控制台打印独立证据目录，其中 report.json 和各检查日志保留本次结果；releaseAccepted 始终为 false，UI/其他工具覆盖/安装包门禁仍需另行完成。


## 2026-09-16：项目定义文件子批次

在独立 worktree 完成项目 Agent/命令/Skill 定义与所选 hook 设置读取改造。先用固定旧 SDK 导出复现 Agent 定义被禁止读取却仍载入，再由宿主执行输入建立只读文件边界后进行发现。目录、元数据、链接和正文均经过 worker；未知设置不能作为空对象继续。刷新失败清空目录并阻止普通 query 与手动 compact；Agent/hook 绑定发生变化必须重建 runtime，Slash 正文仍可通过成功的受限刷新更新。

版本固定：SDK `f9bddb5f8e2114d3a592b4f57d8e4b8bb3c736bd`，Bridge `aa46520ea55f6bcddcfba2054e4844b7f0805eb5`，Nexus 使用本地 module `v0.1.34-0.20260916032704-aa46520ea55f`，checksum `h1:OFneOwdNIvJlGq8BS+uJoqAuVy8XxwHnfHR4Fqy5bro=`。固定 SDK archive 构建的 nxs SHA256 为 `58d7cbfa27d941fe6be69543b20af63ea7bc288fa5edfa8c7d5b0261a9ebe5d0`。

| 检查 | 结果与边界 |
| --- | --- |
| 修复前固定导出 | 拒绝 Agent 定义的子用例失败，允许读取的对照通过；原始日志与测试文件保留 |
| 固定开发基线 | 62 个顶层、201 个指定子场景通过，无必测 skip；项目文件 18 个允许/拒绝场景及 5 个刷新场景纳入门禁 |
| Bridge 固定模块真实进程 | 当前 nxs 与 7 个历史 binary，8 个顶层、14 个子场景全部通过，无 skip；不发送模型请求 |
| SDK 相关包 | 8 包竞态首轮通过；刷新绑定及 compact 守卫追加后，3 包及 runtime 最终竞态补验通过，vet 通过 |
| Bridge / Nexus | Bridge 3 包竞态 199 个顶层/109 个子场景；Nexus runtime 219 个顶层/191 个子场景，vet、架构及 6 个证据解析测试通过 |
| 平台构建 | SDK 与 Bridge 的 Windows/Linux amd64 交叉编译通过，只是构建证据 |

[证据报告](evidence/desktop-sandbox/2026-09-16-project-files/report.json)、[固定基线](evidence/desktop-sandbox/2026-09-16-project-files/baseline-report.json)和 [SHA256 清单](evidence/desktop-sandbox/2026-09-16-project-files/manifest.json)记录完整范围、跳过项及局限。runtime 原生用例以测试可执行文件转发生产 worker 协议；真实 nxs 另用于固定宿主基线和新旧握手。普通竞态套件中的平台/live/显式原生开关 skip 均保留在报告，不能替代上述必测项。

P1/P2 继续进行中：全局权限/Provider/managed 设置读取、权限持久化、hook 执行、后台 IO、网络、生效回执、完整后代监督与 scratch 尚未闭合。默认产品策略、原生 Windows、Linux owner、Claude 和安装包继续按 P3–P7 验收；本次独立 `sandbox_project_files_v1` 不代表整个 SDK 进程已统一隔离。全部仅本地，`releaseAccepted=false`。


## 2026-09-16：托管策略完整性子批次

在独立 worktree 先复现旧 SDK 的三条失败路径：managed 配置损坏、删除或变宽后，仍能准备新的文件执行范围。现在会话在任务 settings 环境投影前固定来源及不可变快照；query、手动 compact、工具派发、文件上下文和权限更新前核对同一来源。错误不能删除强制规则，也不能通过普通 settings 环境重定向到另一份 policy。恢复原有效内容可继续使用，应用新策略须重建 runtime。必需执行在读取前过滤普通设置；托管文件校验包括 drop-in、已知权限/沙箱字段类型、普通文件与大小上限，FIFO 不阻塞加载。

SDK `8031091325a882d384ac40902e3905bd46b021db`、Bridge `796ab55c1b7d1481c7ee6946f49f6eda3e1c37f3`；Nexus 以 `e4a637552e975e5283539783b604b91006ca167c` 为本批次基线，报告记录最终变更源码哈希。固定 Bridge 为本地精确模块 `v0.1.34-0.20260916040534-796ab55c1b7d`，checksum `h1:RdDB4o0YV9S0pYUt6zrqI8ABilbEIe2VxVexYSxOU5I=`；固定 SDK archive 所构建 nxs SHA256 为 `6c92e19887668a53845b8e10f623d377db4ff590e532ad8725374b04f2b56b2a`。没有 replace 或 go.work 参与验收，模块尚未发布。

| 验证 | 结果 |
| --- | --- |
| 旧 SDK 原生复现 | malformed/deleted/relaxed 三个子场景均失败，证明旧行为缺口 |
| 固定 SDK 开发基线 | 72 个顶层、225 个指定子场景通过，必测项无 skip |
| 新增原生完整性 | 三种策略变化均阻断文件范围、query、compact、Bash 与权限写入；恢复后强制 deny 保留 |
| 新旧真实进程 | 9 个顶层、16 个子场景通过，无 skip；只缺托管能力的旧 nxs 被明确拒绝 |
| 目标包 | SDK 12 包、Bridge 3 包、Nexus runtime 子包的 race 检查及 vet 通过；最终命名后的 config 子场景另行通过 |
| 其他门禁 | 架构、证据解析器、SDK/Bridge 的 Windows amd64 与 Linux amd64 交叉编译通过 |

证据：[汇总](./evidence/desktop-sandbox/2026-09-16-managed-policy/report.json)、[固定基线](./evidence/desktop-sandbox/2026-09-16-managed-policy/baseline-report.json)、[真实进程](./evidence/desktop-sandbox/2026-09-16-managed-policy/bridge-pinned-real-process.jsonl)、[哈希清单](./evidence/desktop-sandbox/2026-09-16-managed-policy/manifest.json)。宽范围 race 中按需原生/真实进程用例的 skip 均单列在汇总中，不能替代这里的显式原生门禁。

本次只增加独立 `sandbox_managed_policy_v1` 保证，P1/P2 整体仍进行中。普通 settings/Provider 凭据、权限持久化、hook/后台 IO、网络、生效回执、完整后代监督与 scratch 尚未闭合；默认产品策略、原生 Windows、Linux owner、Claude 和安装包按 P3–P7 保留。全部仅本地，`releaseAccepted=false`，原目录现场未改动。

```sh
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref 8031091325a882d384ac40902e3905bd46b021db
```

此复核命令要求新 Bridge 精确模块已在本机缓存，并在原生 macOS 上运行；本地 file proxy 与历史 nxs 的位置记录在报告和真实进程命令中，不能当作新机器可取得的发布版本。


## 2026-09-16：普通配置输入与快照子批次

先在 SDK `80310913` 复现 user/project/flag/inline 无效配置被忽略、配置 env 重定向根目录，以及 malformed/deleted/relaxed 后权限规则被丢弃。现在客户端在 profile 投影前固定根目录与来源，必需执行经文件 worker 完整读取；runtime 消费绑定快照，外部变化阻断 query、compact、工具、文件上下文和配置更新。动态控制拒绝尚不能生效的静态参数，stdio `get_settings` 读取同一快照，不再直接重读 flag 文件。仅覆盖运行配置输入和这条控制查询链；Config 工具自己的文件操作与权限持久化继续单列。

SDK `c90c7f7c12f4f8ed9441d88d0a453ef9d07f3f03`、Bridge `0f906d102f818e89425179d7f9d44e3b0c20f6ff`；Nexus 基线 `0a45a8b9ca6ab0a6b5b0ebf46e9f2b332136b17a`。固定本地 Bridge 模块 `v0.1.34-0.20260916045904-0f906d102f81`，checksum `h1:ntkgUvnXF07qC4CarommTI1irtFt6AZdFG72EVIqTsI=`；固定 SDK archive 构建 nxs SHA256 `067114598b2deed78bfcd2677e6634e9a8aaa6bb475a9a168b4b23e1676c4cb2`。验收使用 `GOWORK=off`，没有 replace；模块未发布。

| 验证 | 结果 |
| --- | --- |
| 旧 SDK 复现 | 四种无效来源、根目录覆盖及三种运行期文件变化均触发预期失败 |
| 固定 SDK 开发基线 | 85 个顶层、256 个指定子场景通过，必测项无 skip |
| 新增原生来源 | user/project/local/flag、链接和禁用来源共 12 个允许/拒绝场景通过 |
| 新增配置恢复 | malformed/deleted/relaxed 阻断 query、compact、Bash 与权限更新；恢复原内容后仍保留 deny |
| 控制与更新 | 取消、快照冲突、竞争更新、未知写后结果、成功更新、静态配置拒绝和 get_settings 快照投影通过 |
| 真实进程 | 10 个顶层、18 个子场景通过，无 skip；只缺新配置能力的旧 nxs 被明确拒绝 |
| 其他检查 | SDK/Bridge/Nexus 目标包 race、vet、架构与证据解析器通过；SDK/Bridge Windows amd64 和 Linux amd64 交叉编译通过 |

证据：[汇总](./evidence/desktop-sandbox/2026-09-16-settings-snapshot/report.json)、[固定基线](./evidence/desktop-sandbox/2026-09-16-settings-snapshot/baseline-report.json)、[真实进程](./evidence/desktop-sandbox/2026-09-16-settings-snapshot/bridge-pinned-real-process.jsonl)、[哈希清单](./evidence/desktop-sandbox/2026-09-16-settings-snapshot/manifest.json)。宽范围 race 中可选原生/真实进程 skip 另列于报告，不替代显式必测门禁。

本批次确认 `sandbox_settings_files_v1` 的配置输入边界，不证明权限写入原子性、跨进程版本/批准、持久回执或 unknown 对账。`Config` 工具仍有自身宿主文件读写路径，需随配置持久化改造；Provider 凭据与任务环境分离也未完成。P1/P2 整体及 P3–P7 继续保留，`releaseAccepted=false`，全部仅本地，原目录现场未改动。

```sh
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref c90c7f7c12f4f8ed9441d88d0a453ef9d07f3f03
```

此命令要求精确 Bridge 模块已在本机缓存，并在原生 macOS 上运行；临时 file proxy 和历史 nxs 位置记录在报告，不是公开发布渠道。


## 2026-09-16：main 同步与 Bridge 兼容

Nexus 在独立 worktree 以合并提交 `050079978` 纳入 main `0e18e6aa7`。首次固定旧沙箱 Bridge 时，main 的 `TestSDKMCPPreservesRuntimeToolUseIdentity` 复现空 `tool_use_id`；Bridge `6325d2a` 合并 `c7ecea2` 的真实 MCP 元数据传递后，Nexus 固定 `v0.1.34-0.20260916063139-6325d2acc450`（`h1:odDCya2lEfIPGJvxp70RS1Le77nkyksPTUC1AFxR/Rk=`）。最终验收使用 `GOWORK=off`、`-mod=readonly` 和实际固定模块；中途临时 modfile 检查仅作诊断，不代替固定版本验证。SDK 保持 `c90c7f7c`，本批次未修改 SDK 生产代码。

| 验证 | 结果与边界 |
| --- | --- |
| Nexus 受影响包竞态 | 最终 33 个测试包、1198 个顶层及 433 个子场景通过。首轮两包失败记录保留；修正夹具后单独重跑 channels/automation 的 323 个测试事件通过，其他未变包沿用首轮通过结果，并非声称首轮整体通过 |
| 审批夹具 | 通道测试改用生产数据库连接配置；Automation 等待持久投递成功后才断言消息。修正测试中的数据库竞争和执行/投递阶段混淆，没有修改产品审批逻辑 |
| Bridge 竞态与静态检查 | tools/client/transport/nxs 四包通过；Nexus 与 Bridge 目标包 vet 通过。普通套件中的可选真实进程和其他平台 skip 逐项保留 |
| 固定 Bridge → 真实 nxs | 当前十项能力的 10 个顶层/9 个子场景通过；SDK `80310913` 缺普通配置能力的拒绝场景另行通过，无必测 skip。其他八个历史缺能力 binary 未在本批次重跑 |
| Nexus → 固定 Bridge → 真实 nxs | 宿主装配/握手和诊断两个用例通过，无 skip，不发送模型请求 |
| 前端与架构 | 设置页 19 个组件测试、TypeScript 类型检查和架构检查通过；不替代安装包视觉验收 |
| 后续权限持久化反例 | 固定 SDK 配合 test-only overlay，`file_symlink`、`directory_symlink`、`directory_replaced`、`hardlink` 四个子场景如预期失败：内容相等仍可写入替换路径或共同 inode，尚未修复 |

[报告](./evidence/desktop-sandbox/2026-09-16-main-sync/report.json)、[SHA256 清单](./evidence/desktop-sandbox/2026-09-16-main-sync/manifest.json)保存版本、源码/二进制哈希、命令、最终退出码、skip 与压缩原始日志。该目录的 README 解释合并结果与后续红色反例。SDK/Bridge 已从缺失的临时目录恢复到固定的独立 worktree，具体位置见开发计划；原 Nexus main 的未提交文件保持原状。

仅本地提交及本地模块缓存，模块尚未发布；本批次没有重新声明全 SDK 原生隔离、跨平台、默认策略、完整进程树、持久回执或安装包验收。完整 P1–P7 与 Goal 继续。

## 2026-09-17：配置受控写入与请求准入

SDK `65e65b86` 完成配置 writer、绑定快照与逐次请求栅栏，`00b72d22` 补齐 WebFetch 环境端点和宿主摘要 adapter 的准入；Bridge `a2316d7` 增加独立 `sandbox_settings_writes_v1`。Nexus 固定 `v0.1.34-0.20260917020413-a2316d747b09`（`h1:nOd4a9+Fw0jdd37FgYFGIweRYb8WURNBu/56K1RF+hM=`），受限桌面启动要求该能力并将其纳入进程指纹。验证全部脱离 `go.work`，Nexus 使用真实固定模块。

| 验证 | 结果与边界 |
| --- | --- |
| 文件写入 | 固定物理根和目录身份；叶子/祖先链接、目录换代、特殊/只读文件拒绝；同目录替换保留外部硬链接目标；多个 clone 共用部分提交 unknown 和重建栅栏 |
| 原生任务写保护 | 字面/物理别名、临时名字、正则特殊字符与新建硬链接拒绝；required macOS 对预先存在的多硬链接配置拒绝启动 |
| 配置语义与请求 | canonical Config 字段、显式 Options/env 优先和 no-op 保留；有效写入后主模型、摘要/权限审核/compact 请求停止。页面抓取期间配置变化时，环境摘要和宿主 adapter 两条反例先失败，修复后均阻断 |
| 初始化顺序 | initialize 入队时保留准入状态；普通消息最多缓冲 32 条，成功后才继续；失败与控制入口不能绕过创建 base-config Session |
| 固定 SDK 开发基线 | 128 个顶层、285 个指定子场景通过，无必测 skip；`macos-settings-writes` 为真实 Seatbelt 检查 |
| 真实进程 | 当前十一项能力 11 个顶层/10 个子场景通过；旧 SDK 缺写能力的顶层/legacy 场景通过，无 skip；不发送模型请求 |
| SDK 回归 | 全包测试 1786 个通过事件、226 个可选 skip；vet、配置/Provider/初始化目标竞态通过；Windows/Linux 仅编译通过 |
| Nexus/Bridge 回归 | runtime、nxsruntime、Room realtime 目标包 718 个通过事件、8 个可选 skip；目标竞态、vet、架构与证据解析器通过。实际 Nexus MCP 包的工具身份竞态回归通过 |

[汇总报告](./evidence/desktop-sandbox/2026-09-17-settings-writes/report.json)、[固定基线](./evidence/desktop-sandbox/2026-09-17-settings-writes/baseline-report.json)、[证据说明](./evidence/desktop-sandbox/2026-09-17-settings-writes/README.md)和[SHA256 清单](./evidence/desktop-sandbox/2026-09-17-settings-writes/manifest.json)保存最终结果及修复前失败。Bridge 模块按 Go canonical zip 生成；此前普通 Git zip 的校验和未进入本次最终依赖。

本批次只完成进程内配置边界，不证明持久 request/approval/revision、跨进程 CAS、多文件原子事务、父目录 fsync 或重启 unknown 对账。Provider 凭据隔离、其余 SDK IO、完整后代监督、scratch、默认策略、Claude、其他平台原生及安装包继续按 P1–P7 推进。`releaseAccepted=false`；所有提交与模块仅本地，原目录现场未修改。

## 2026-09-17：Provider 环境所有权与插值隔离

SDK 本地提交 `101f34fa`、`460c0f1c` 收口 settings 到主/辅助请求、命令/hook 环境及 HTTP hook/MCP 变量插值的凭据流。Nexus 在最后的环境合并边界固定宿主管理标记；标记、scrub 或后台唤醒所有权变化时替换旧进程，普通 Provider 凭据更新仍可热更新。Bridge 保持 `a2316d7` 的固定模块；没有发布或推送。

| 验证 | 结果与边界 |
| --- | --- |
| 修复前反例 | SDK 旧提交加测试复现 26 个失败事件（含父子项）；请求正文替换、Nexus 启动覆盖、HTTP hook/MCP 插值与旧进程复用另有独立失败记录 |
| 固定 SDK 基线 | `460c0f1c` 导出构建；143 个顶层、309 个指定子场景通过，必测 skip 为 0；含真实 nxs 握手与 macOS 文件/命令测试 |
| 最终宿主增量 | 基线启动后补入进程标记指纹；最终 runtime/clientopts 的 205 个通过事件、1 个可选 skip，race 通过；新增 2 个顶层/4 个指定子场景另经严格证据解析确认通过。不是声称初次基线已经包含这 6 项 |
| SDK 目标包 | 首批受影响包 1171 个通过事件、167 个可选 skip，下游调用方 247 个通过事件、1 个可选 skip；定向 race 31 个事件通过。插值补充后相关包 race 358 个通过事件、90 个可选 skip |
| 兼容性 | 独立 SDK/CLI settings、独立 MCP 插值、专用 hook/MCP 认证变量与普通 Provider 凭据热更新保持；未访问真实 Provider |
| 静态/跨平台 | 目标包 vet、架构和脚本检查通过；Windows/Linux amd64 仅交叉编译通过，不代表原生环境或安装包验收 |

[汇总报告](./evidence/desktop-sandbox/2026-09-17-provider-environment/report.json)、[基线报告](./evidence/desktop-sandbox/2026-09-17-provider-environment/baseline-report.json)、[复核说明](./evidence/desktop-sandbox/2026-09-17-provider-environment/README.md)与[哈希清单](./evidence/desktop-sandbox/2026-09-17-provider-environment/manifest.json)保留精确版本、源文件差异、修复前后日志和 skip。最终自动入口已要求总计 145 个顶层/313 个指定子场景；本批证据由固定 SDK 基线与随后实际执行的宿主增量组合证明。

这批仅证明已知环境凭据不会经上述输入通路借出，不证明任意秘密文件、其他进程或继承句柄不可读，也不证明外部 MCP/认证 helper 已进入 OS 边界。后台 IO、网络、持久恢复、完整后代监督、scratch、默认权限体验、Claude、其他平台原生及安装包仍按 P1–P7 继续；`releaseAccepted=false`，Goal active。

## 2026-09-18：MCP helper 与后台记忆根所有权

SDK `81104dd9c4a2a0c2a22c6cb7b021da4065a51351`、Nexus `064ccb7fa02e68199c0bb5beae8092c1c423bdcb` 和 Bridge `a2316d7` 均为独立 worktree 的本地提交。SDK 的 MCP registry 将 runtime-owned 环境传给 `headersHelper`，更新环境时替换 helper 环境；托管 helper 看不到已知 Provider 凭据、`NEXUS_MEMORY_DIR` 或远端 memory 覆盖。SDK 的 Summary、AutoMemory、AutoDream 统一从宿主 workspace 解析记忆根。Nexus 在所有 ConfigurationEnv/capability 合并完成后再次固定 nxs 的 workspace memory 根，并把 memory 所有权纳入进程复用指纹。

| 验证 | 结果与边界 |
| --- | --- |
| 固定跨仓基线 | `check-sandbox-baseline.mjs` 使用 SDK `81104dd9` 运行通过；提交干净，Nexus 无未提交代码改动（证据归档另列）；`releaseAccepted=false`，未推送 |
| SDK 目标包 | `client`、`internal/config/env`、`internal/mcp/client`、`internal/agent/runtime` 定向测试、vet 通过；MCP/runtime race 通过 |
| MCP helper 反例 | helper 只读到普通 task 值；已知 Provider 凭据和 memory 根为空；registry 环境更新后新 helper 使用新 runtime-owned 环境 |
| 后台记忆根反例 | 托管 profile 清除任务/远端 memory 根覆盖，memory store 解析为 workspace；standalone profile 保留既有配置语义 |
| Nexus 宿主边界 | `ConfigurationEnv` 无法重定向 managed memory；memory ownership key 参与 process-policy replacement；`make check-go` 与 `make check-architecture` 通过 |
| 跨平台证据 | SDK client/env/MCP/runtime 的 Windows/Linux amd64 `go test -c` 交叉编译通过；这只是编译证据，不代表原生权限、DACL、Seatbelt 或安装包验收 |

[证据说明与日志归档](./evidence/desktop-sandbox/2026-09-18-mcp-memory/README.md)、[基线报告](./evidence/desktop-sandbox/2026-09-18-mcp-memory/baseline-report.json)、[哈希清单](./evidence/desktop-sandbox/2026-09-18-mcp-memory/manifest.json)。

本批次只证明 MCP authentication helper 的环境来源和 nxs 后台记忆根所有权。外部 MCP server 的 OS 进程/句柄/秘密文件/网络边界、完整后台 summary/transcript IO、持久回执与重启恢复、后代监督、scratch、默认策略、Claude、原生 Windows/Linux/macOS 安装包和生产发布仍未闭合；全部仅本地，Goal active。

## 2026-09-18：未受信 MCP authentication helper 失败关闭

Nexus 在 `internal/runtime/clientopts` 的桌面 nxs 准入层拒绝持久 MCP 配置中的任意 `headersHelper` 路径。该检查在默认受限与 Full Access 两种权限模式都执行；Full Access 仍保留 nxs runtime/lifecycle 边界，不能把未证明的外部认证进程当作安全例外。Claude、非桌面调用和 stdio MCP 不复用这条 nxs-only 规则。

| 验证 | 结果与边界 |
| --- | --- |
| 目标测试 | `GOWORK=off go test ./internal/runtime/clientopts` 通过；远程 MCP 受限拒绝、Full Access 兼容行为与 helper 两种模式拒绝均覆盖 |
| 交叉编译 | `GOOS=windows GOARCH=amd64` 与 `GOOS=linux GOARCH=amd64` 的 clientopts 测试二进制均构建成功；这只是编译证据，不代表原生 helper 或 OS 权限验收 |
| 安全结论 | 任意 helper 路径不能进入受限桌面 nxs 会话；错误包含 server 名称和 helper 准入原因 |
| 未闭合项 | 宿主 attested helper 能力、helper 的 OS 进程/文件/句柄/网络边界、持久批准与跨平台原生验收仍未完成 |

本批次仅是 Nexus 准入保护，未改变 SDK/Bridge 版本或声称外部 MCP 已完成沙箱隔离；`releaseAccepted=false`，提交仅本地。

## 2026-09-18：桌面默认受限策略

配置与 client-options builder 现在把 `NEXUS_APP_MODE=desktop` 直接映射为沙箱合同；`NEXUS_DESKTOP_SANDBOX_ENABLED=false` 不再关闭该路径。服务端模式仍使用独立的 runtime isolation 入口。选定后端或平台无法证明所需能力时，builder 在发送任务前失败关闭。

| 验证 | 结果与边界 |
| --- | --- |
| 配置回归 | desktop 模式即使设置旧环境变量为 `false` 仍得到 `DesktopSandboxEnabled=true` |
| builder 回归 | `AppMode=desktop` 且未显式设置标记时进入桌面策略；server 模式不受影响 |
| 目标门禁 | `GOWORK=off go test ./internal/config ./internal/runtime/clientopts`、`GOWORK=off make check-go` 通过 |
| 安全边界 | 旧开关不能把桌面会话变成裸 runtime；能力/平台不满足时仍拒绝启动或任务发送 |

该批次只完成产品默认入口收口，不证明 Claude 原生沙箱、Windows/Linux/macOS 实机、有效策略回执、完整进程树、持久恢复、scratch 或安装包发布验收；`releaseAccepted=false`。

## 2026-09-18：宿主资源合同入口与 Windows Bridge 进程边界

Nexus `5af222fbb` 将宿主准备的 `SandboxResourcePolicy` 作为独立输入复制并校验；资源合同只能在桌面受限模式进入 SDK options，Full Access 携带受限资源合同会在任务前失败关闭。Bridge `6bb7b495`、`162cc79` 为 Windows runtime 绑定 Job Object，并在宿主信号回调失败时仍继续本地后代收口，保留合并清理错误。

| 验证 | 结果与边界 |
| --- | --- |
| Nexus 目标测试 | `GOWORK=off go test ./internal/runtime/clientopts` 通过；覆盖资源策略复制、别名隔离、非法路径与 Full Access 冲突 |
| Bridge 目标测试 | `go test ./internal/transport ./client` 通过；Windows amd64/arm64 transport 交叉编译通过 |
| 安全结论 | 资源策略不是普通 task settings；未携带有效宿主合同时不会获得资源能力，Full Access 仍保留 nxs runtime/lifecycle 边界 |
| 未闭合项 | Nexus 尚未在 DM/Room/后台 runtime 创建和回收 scratch 租约；Bridge 提交尚未发布；无 Windows 实机/clean-host、持久回执、完整后代监督、网络、Claude、签名安装包证据 |

本批只收口输入合同和 Windows Bridge 的进程树边界，不能据此宣称桌面沙箱或 P0–P7 完成；`releaseAccepted=false`，提交仅本地。

## 2026-09-18：scratch lease 与 settings unknown 恢复增量

Nexus `3fb64260c`、`53a6be247`、`1e04e87ad` 在桌面 nxs 的 DM、Room 和 AutoDream 启动前创建 owner/runtime-scoped scratch lease，并把独立的 `SandboxResourcePolicy` 传给 Bridge；默认未显式指定的桌面 runtime 也进入 nxs scratch 路径。Bridge session 成功关闭后才释放 scratch；关闭失败时保留 scratch 和 runtime close fence。Nexus `46229c723` 增加 durable settings receipt 恢复：超过 5 分钟仍处于 `applying` 的 receipt 通过条件更新收口为 `reconcile_required`，结果持久化为 `applied: "unknown"`，等待新的 inspect/reconcile 请求。

| 验证 | 结果与边界 |
| --- | --- |
| scratch helper | `GOWORK=off go test ./internal/runtime -run 'Test(Acquire|ReleasePath)'` 通过；路径身份、私有目录、scope 重用、取消与幂等释放覆盖 |
| runtime lifecycle | `GOWORK=off go test ./internal/runtime -run 'TestAgentClientCleanup|TestManagerCleanup'` 通过；Bridge 失败不释放 scratch，成功关闭后释放 |
| DM/Room/background | `GOWORK=off go test ./internal/service/dm ./internal/service/room/realtime ./internal/service/memorymaintenance` 通过；三类入口均在 Build/initialize 前注入资源合同 |
| durable unknown | `GOWORK=off go test ./internal/service/configuration` 通过；关闭数据库后重新打开，旧 `applying` receipt 恢复为 `reconcile_required` |
| dependency pin | Nexus `go.mod` 使用 Bridge `v0.1.34-0.20260918033416-162cc7951ae1`，checksum `h1:nmfmKMRBJKzpA+A8j0v8cYixnv9x+9ljUxrUcPTRtQI=`；固定提交由本机 file proxy 提供，未发布 |
| 当前边界 | 内存 registry 尚不能在宿主崩溃后自动 sweep stale scratch；恢复 primitive 尚未接入 scheduler/startup 或实际 inspect/reconcile UI；全后代、句柄、秘密文件、网络、Claude、原生平台和安装包仍未验收 |

本批次改变了真实启动和恢复路径，但仍不能宣称 P0–P7 或发布完成；`releaseAccepted=false`，本地提交未推送。

## 2026-09-18：settings unknown 启动与周期恢复接入

`RecoverStaleApplyingChangesForAllOwners` 现在由 server lifecycle 在其他后台调度器前执行一次，并以每分钟周期任务继续扫描。入口按 owner 发现 stale `applying` receipt，再委托原有 owner/request 条件更新；全局批次有界，未知结果持久为 `reconcile_required` / `applied: "unknown"`，不会自动 inspect 或重放。

| 验证 | 结果与边界 |
| --- | --- |
| owner-scoped recovery | `GOWORK=off go test ./internal/service/configuration -run 'TestRecoverStaleApplyingChanges'` 通过；覆盖跨 owner 的全局批次限制和后续扫描收口 |
| target package | `GOWORK=off go test ./internal/service/configuration ./internal/app/server` 通过 |
| startup/scheduler wiring | `startBackgroundServices` 首先执行配置恢复首扫，随后启动每分钟 ticker；停止函数等待恢复 goroutine 退出 |
| fail-closed boundary | 首次数据库扫描失败会阻止 server 启动；周期扫描失败只记录告警并保留 durable receipt，等待下一次扫描 |
| 未闭合项 | 尚未连接设置页 inspect/reconcile 操作；跨进程 all-or-nothing/CAS/fsync、scratch 崩溃后 stale sweep、Provider/辅助进程/网络、Claude、原生 Windows/macOS/Linux 与安装包验收仍未完成 |

该批次只证明 durable unknown recovery 已进入明确的 server 启动与周期调度，不构成 P0–P7 或发布通过；`releaseAccepted=false`，本地提交未推送。

## 2026-09-18：scratch durable marker 与显式 stale recovery

`internal/runtime` 为每个 owner/runtime/session scratch 目录持久化版本化 `.nexus-sandbox-lease.json` marker。marker 通过独占创建和文件 `Sync` 写入，包含 lease、owner/session/round、canonical runtime root、创建 PID 与 UTC 时间；它只用于发现和人工恢复，不能让新进程采用旧 lease。每次 Acquire 都返回独立持有句柄，DM/Room runtime 将 exact handle 交给 Bridge cleanup；正常 Bridge close 后 marker 随 scratch 删除，close 失败仍保留 scratch、marker、runtime close fence，并拒绝同 scope 的新 Acquire。

| 验证 | 结果与边界 |
| --- | --- |
| marker 生命周期 | `GOWORK=off go test ./internal/runtime -run 'Test(AcquirePersistsDurableMarker|SweepStaleSandboxResources)' -count=1` 通过；覆盖 durable marker、正常释放删除、模拟崩溃 dead-PID dry-run/apply、owner/root 身份和 malformed marker |
| 显式恢复 API | 只读 `DiscoverSandboxResources` 列举有效 marker；`SweepStaleSandboxResources` 要求 owner 与正 `OlderThan`，默认 `Apply=false`，只有显式 `Apply=true` 才删除过期 dead-PID candidate |
| fail-closed 条件 | 当前 registry lease、`cleanup_unknown` marker、Unix 可证明存活的 PID、平台无法证明存活、年龄不足、损坏/不匹配 marker 均保留；旧 PID 已退出不能单独证明后代/句柄已收口；不在启动或 scheduler 中自动 sweep，不自动采用未知 runtime |
| 交叉编译 | `GOOS=windows GOARCH=amd64`、`GOOS=linux GOARCH=amd64` 的 `internal/runtime` 测试二进制构建通过；Windows 进程存活探测仍未知并故意保留 marker，这不是 Windows 原生清理验收 |
| 当前边界 | 该 primitive 不证明任意后代、句柄、秘密文件或网络已停止/清理；未连接设置页、启动调度、native cleanup、Provider/辅助进程、Claude、原生平台和安装包验收，`releaseAccepted=false` |

本批次只把 scratch 从易失内存 registry 扩展为可发现、可审计、经用户明确批准才可执行的恢复边界；所有提交仍仅本地、未推送。

## 2026-09-18：桌面 nxs 网络域名与 Provider 凭据准入

新增 `DesktopSandboxNetworkAdmission` 作为宿主准备的网络 grant：精确 bare DNS
域名、HTTPS、默认 443 端口才可通过；nil/空 grant 保持 Bridge 显式
`allowedDomains=[]` 的 deny-all 语义。受限桌面 nxs 的持久 HTTP/SSE MCP 只有在该
grant 精确匹配 URL 主机时才会进入 runtime；没有 grant 仍 fail closed，未受信的
`headersHelper` 仍被拒绝。桌面 WebSearch 的 `allow_private_network` 也在启动时拒绝，
不能借用户偏好绕过宿主网络策略。

| 验证 | 结果与边界 |
| --- | --- |
| Nexus 目标测试 | `GOWORK=off go test ./internal/runtime/clientopts` 通过；覆盖 nil/空 deny-all、域名规范化/重复消除、非 HTTPS/非 443/IP/userinfo 拒绝、HTTP/SSE MCP 未批准与已批准两条路径、sandbox network 配置复制以及 WebSearch private-network fail closed |
| Provider 环境 | 覆盖 desktop nxs 只从解析 `RuntimeConfig` 投影 Provider credential；`ExtraEnv` 不能覆盖 `OPENAI_API_KEY`。该断言只证明输入来源和 merge 顺序 |
| 安全结论 | 桌面受限 runtime 没有隐式外网 allowlist；HTTP/SSE MCP 与 private WebSearch 必须取得宿主准入，未声明或无效 grant 不会创建远程连接配置 |
| 未闭合项 | 真实 Provider、DNS/代理、IPv4/IPv6、loopback/private、TCP/UDP、辅助进程/句柄/秘密文件、Bridge/native 网络执行、持久批准/epoch、Windows/macOS/Linux clean-host、Claude 与安装包尚未验收；env scrub 不能替代 OS 隔离 |

本批次只完成 Nexus 进程内网络和 Provider 输入准入，未改变 SDK/Bridge 版本，也不构成完整网络沙箱或 P0–P7 发布通过；`releaseAccepted=false`，提交仅本地、未推送。

## 2026-09-18：Claude Bridge 原生受限启动合同

Bridge `35fbf72bfd062b5f7ca3965e37428347598b473a` 增加独立的
`RequireClaudeRestricted` 与 `CapabilityClaudeRestricted` typed contract。仅当
runtime 是 Claude 且权限模式不是 Full Access 时，Bridge 才注入恰好一个原生
`--restricted`；nxs、bypass/dangerous bypass、ExtraArgs/ExtraBoolArgs 冒用和
受限策略变化均在 transport/进程替换边界拒绝或触发重建。Nexus 已固定本地模块
`v0.1.34-0.20260918045243-35fbf72bfd06`，checksum 为
`h1:54adZydLrjKDvkfv2bRocdwPuUdfKhV+j5G+Lj4trUU=`，Claude 受限只设置该合同，
不再设置 nxs `RequireSandbox`；Claude Full Access 不设置合同或参数。

| 验证 | 结果与边界 |
| --- | --- |
| Bridge typed contract | `go test ./...` 与 `go test -race ./client` 通过；`TestClaudeRestricted` 覆盖参数唯一性、runtime/bypass/伪造拒绝、Connect 失败关闭、无 transport 写入、快照/重启指纹 |
| Nexus 接线 | `GOWORK=off GOPROXY=file:///private/tmp/nexus-bridge-35fbf72-proxy ... go test ./internal/runtime/clientopts -run 'TestDesktopSandbox(UsesClaudeNativeRestrictedContract|ClaudeFullAccessDoesNotInstallRestrictedContract|ClaudeRestrictedPreservesOrdinarySettingsButRejectsNXSContract|PolicySeparatesResourcesAndFullAccess|DoesNotAlterServerIsolationOrDisabledFeature)$' -count=1` 通过 |
| 合同边界 | capability 只证明 Bridge 已安装 argv 合同，不是 Claude wire 能力或 OS 文件/网络/Provider 隔离回执；没有复用 nxs capability/原生证据 |
| 未闭合项 | 固定 Claude CLI 版本、`claude --help`/真实 `--restricted` 行为、取消/清理、macOS/Windows/Linux clean-host 和安装包验收仍未完成；`releaseAccepted=false` |

本批次提交和模块只保留在本地 worktree，未推送；真实 Claude CLI 与平台证据必须另行归档后，才能把 Claude 从未闭合状态移出 P1/P6/P7 门禁。

## 2026-09-18：Notebook 文件能力独立准入

SDK `7bc597ea3c9db479b561d6203fb2a8d03698c982` 增加
`sandbox_notebook_files_v1`，Bridge `8a4576ba97ece60e0485f2bfbb0bce53e5b89502`
发送并验证 `required_sandbox_notebook_files`，Nexus 桌面 nxs 将该能力纳入默认
受限合同和进程策略指纹。Nexus 固定本地 Bridge 模块
`v0.1.34-0.20260918053632-8a4576ba97ec`，checksum 为
`h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`；固定 nxs SHA-256 为
`b1aebef92731ab9a1397136b2e1656407d8c2b59818b71d9ca4c71025f0c52b5`。

| 验证 | 结果与边界 |
| --- | --- |
| SDK 目标包 | `GOWORK=off GOPROXY=off go test ./cmd/nxs ./protocol` 通过；初始化前校验 Notebook 要求依赖必需沙箱和原生文件能力，缺能力不改变会话状态 |
| Bridge 目标与竞态 | `GOWORK=off GOPROXY=off go test ./client ./protocol` 与 `go test -race ./client` 通过；覆盖能力顺序、旧/部分能力拒绝、nxs/Claude 后端区分、进程替换指纹和无 transport 写入失败 |
| Bridge → 真实 nxs | `NEXUS_SANDBOX_TEST_BINARY=/private/tmp/nexus-notebook-evidence/nxs` 的 `TestNotebookFileSandbox` 通过，无模型请求；nxs 回报 `sandbox_notebook_files_v1` |
| Nexus 宿主 | 固定 Bridge 模块下 `go test ./internal/runtime/clientopts ./internal/runtime` 通过；nxs 默认合同要求 Notebook 能力，Claude 不冒用该 nxs 能力 |
| 固定版本 | SDK `7bc597ea`、Bridge `8a4576ba`、模块 checksum、binary SHA-256、命令和日志见 [Notebook 证据目录](./evidence/desktop-sandbox/2026-09-18-notebook-files/README.md)；提交仅本地、未推送 |

本批次只证明 macOS nxs 本地 Notebook 内容与 cell output 读取的能力准入，读取复用受限文件执行器。Notebook 执行、远程网络、完整 SDK IO、Provider/秘密文件/句柄、崩溃恢复、Windows/Linux 原生、Claude 和签名安装包仍未验收；`releaseAccepted=false`。

## 2026-09-18：settings-writes 跨进程锁与真实 nxs 集成

SDK `ce136cfe` 为每个物理 settings 根增加稳定的 `.nexus-settings.lock`，按根路径排序获取多根锁，等待支持 context 取消；获得锁后重新核验预期快照，原子替换后同步已打开的父目录，并在锁关闭或写入已发生但结果无法证明时进入既有 unknown 栅栏。Bridge 继续固定为
`8a4576ba97ece60e0485f2bfbb0bce53e5b89502`，Nexus 为 `223dd495a915842a7676ec7b5f95e852670203da`。

固定模块为 `v0.1.34-0.20260918053632-8a4576ba97ec`，checksum 为
`h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`。从 SDK `ce136cfe` 构建的
nxs SHA-256 为 `3ec4aeb09208733c74a135f923f04fc0e89269f94b49530f1dc85d0779671afb`。
命令、版本、退出码和原始日志见 [settings-lock 证据目录](./evidence/desktop-sandbox/2026-09-18-settings-lock/README.md)。

| 验证 | 结果与边界 |
| --- | --- |
| SDK target | `GOWORK=off GOPROXY=off go test ./cmd/nxs ./internal/config/settings ./internal/agent/runtime ./internal/tool/builtin/config` 通过 |
| SDK race | `GOWORK=off GOPROXY=off go test -race ./internal/config/settings ./internal/agent/runtime` 通过 |
| nxs build | `GOWORK=off GOPROXY=off go build -o /tmp/nxs-settings-lock ./cmd/nxs` 通过；SHA-256 已固定 |
| Nexus desktop gate | `NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-settings-lock make check-desktop-sandbox` 通过；证据解析、Bridge module、host policy、host lifecycle 和真实 Nexus→Bridge→nxs negotiation 均 exit 0，无模型请求 |
| 当前边界 | 仍未证明多文件断电 all-or-nothing、持久 SDK request/approval/revision receipt、设置页 inspect/reconcile、Provider 秘密文件/继承句柄、外部 MCP/网络出口、完整后代清理、Windows/Linux 原生、Claude 认证会话或安装包发布；`releaseAccepted=false` |

本批次只完成 settings-writes 的跨进程串行化和最新固定 nxs 的桌面集成证据；所有提交仍仅本地、未推送。

## 2026-09-18：settings-writes 可证明失败回滚

SDK `9d60e166` 在同一物理根的跨进程锁窗口内记录每个已提交文档；当后续文档写入、写后读取或计划核对失败时，按逆序恢复已有文档，删除本次创建的文档。回滚前后都重新校验绑定目录、文件身份和当前内容；外部改动、删除失败或回滚结果不明仍进入 unknown，不会把未证明的状态当作成功。

固定 Bridge 仍为 `v0.1.34-0.20260918053632-8a4576ba97ec`
（`h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`），Nexus 为
`223dd495a915842a7676ec7b5f95e852670203da`；新 SDK 构建的 nxs SHA-256 为
`374a022e84a1dd081c2c9e2b56474dcc61dbfd4868b70f9fbeaf05de8ff49330`。命令和压缩日志见 [settings-rollback 证据目录](./evidence/desktop-sandbox/2026-09-18-settings-rollback/README.md)。

| 验证 | 结果与边界 |
| --- | --- |
| SDK target/race | `go test ./internal/config/settings ./cmd/nxs ./internal/agent/runtime ./internal/tool/builtin/config` 与 `go test -race ./internal/config/settings ./internal/agent/runtime`（均 `GOWORK=off GOPROXY=off`）通过；新建/已有文档回滚测试通过 |
| Nexus desktop gate | `NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-settings-rollback make check-desktop-sandbox` 通过；固定 Bridge、host policy/lifecycle 和真实 Nexus→Bridge→nxs negotiation 均通过，无模型请求 |
| 当前边界 | 只闭合可证明的运行期失败；掉电跨文件 all-or-nothing、持久 SDK request/approval/revision receipt、设置页 inspect/reconcile、Provider/辅助进程/网络/后代隔离、Windows/Linux 原生、Claude 认证会话和安装包仍未验收，`releaseAccepted=false` |

本批次仍只在本地 worktree 提交，未推送。

## 2026-09-23：cleanup_unknown 持久状态与 runtime 生效回执

scratch marker 现在在 Bridge close 或目录回收失败时通过固定 lease 目录句柄原子写回
`cleanup_unknown`、限长错误摘要和更新时间；旧 marker 没有该字段时按 `active` 兼容读取。
`DiscoverSandboxResources` 会保留并返回该状态，未知状态仍不会被启动流程自动删除。
Connect 在安装桌面 runtime generation 前独立核对 Bridge 已确认的 required sandbox
capabilities，并保存当前 generation 的 policy digest、session ID、精确 lease/round
identity 和确认时间回执；它不替代平台或全 SDK 隔离证据。

| 验证 | 结果与边界 |
| --- | --- |
| durable cleanup state | `GOWORK=off go test ./internal/runtime -run 'Test(CleanupUnknownStateIsPersistedAndDiscovered|SweepRetainsCleanupUnknownEvenWhenOwnerPIDIsDead|ReleaseRetainsLeaseWhenScratchParentIsReplaced)' -count=1` 通过；覆盖 marker 原子更新、父目录替换后仍用固定句柄写回、重启式 discovery 读取、死 PID 仍保留 unknown 和原有 lease fence |
| runtime receipt | `GOWORK=off go test ./internal/runtime -count=1` 通过；Bridge required/acknowledged capability 校验、policy digest、资源/lease identity 进入连接代次回执，缺能力时连接失败关闭 |
| recovery HTTP surface | `GET /settings/runtime/sandbox/resources` 仅列举当前认证 owner 的 marker；`POST /settings/runtime/sandbox/reconcile` 要求正 `older_than_seconds`，默认 dry-run，只有显式 `apply=true` 才请求回收；handler 测试覆盖 owner 隔离、首次空目录、活动 lease 保留和非法年龄 |
| 当前边界 | marker 恢复与 receipt 已可跨进程读取，但不证明 OS 命令/网络/秘密/句柄/后代隔离；设置页已接入 inspect/预览/显式回收，但 Windows unknown 进程存活、自动恢复调度、原生 Windows/macOS clean-host、签名安装包、真实 Claude 认证会话和生产发布仍未验收，`releaseAccepted=false` |

本批次增加可重启读取的 cleanup 状态、Connect 后的有效策略核对和设置页恢复入口；提交仍仅本地、未推送。

## 2026-09-24：effective-policy receipt 持久化与崩溃恢复 harness

每个桌面 runtime generation 的 effective-policy receipt 现在以
`owner_user_id + session_key + generation` 持久化到 `sandbox_policy_receipts`。回执只
保存 Bridge 能力、policy digest、资源策略、lease/round identity 和生命周期时间，关闭
按 `confirmed → retiring → retired|unknown` 收口；未知阶段带有原因，不能因 PID 已退出
或宿主重启被自动改猜为 reconciled。迟到的旧代关闭回调不能把 terminal 阶段重新打开。
当前没有把 receipt unknown 自动改写为 reconciled 的入口；必须保留 unknown，等待未来能
证明完整 runtime 边界的人工控制面。HTTP receipt 读取优先使用当前 connected generation，
没有连接时按认证 owner/session 返回最新 durable 审计投影。

跨进程 harness 由子进程取得 scratch lease 后直接退出模拟 host crash；重启侧可发现
marker，dry-run 不删除，Unix 只有显式 apply 才能清理可证明过期的普通 marker，
`cleanup_unknown` 即使记录 PID 已退出仍保留。这个 harness 只证明 marker 可发现和显式
回收栅栏，不证明后代、句柄、秘密、网络或完整 SDK IO 已收口。

Bridge 已报告干净关闭后，owner 级进程回收仍是独立的收口条件；如果 reaper 返回错误，
宿主会把该 generation 的 `retired` 保守降级为 `unknown`，并保留限长原因，不把干净的
Bridge ACK 当作后代已收口的证明。

| 验证 | 结果与边界 |
| --- | --- |
| receipt persistence | `GOWORK=off go test ./internal/storage/sandbox ./internal/runtime ./internal/handler/core -count=1` 通过；覆盖 migration、重开读取、owner 栅栏、durable fallback 和关闭阶段 |
| owner reaper failure | `GOWORK=off go test ./internal/runtime -run 'TestManagerOwnerReaperFailureDowngradesRetiredReceiptToUnknown' -count=1` 通过；覆盖 Bridge 已关闭但 owner reaper 失败时的 exact generation `retired → unknown` 保守修正 |
| crash recovery | `GOWORK=off go test -race ./internal/runtime -run 'TestSandbox(Crash|CleanupUnknown)' -count=1` 通过；确认重启 discovery、dry-run、显式 apply 与 unknown 保留 |
| 第三方 Anthropic-compatible Provider | 现有本地 mock SSE 证据覆盖 Claude CLI 的 `ANTHROPIC_AUTH_TOKEN` Bearer 路径；新增 nxs 路径投影到 `ANTHROPIC_API_KEY`，固定 SDK 对兼容 endpoint 发送 `x-api-key` 并保留 Bearer fallback。nxs `Sandbox.Network` 只约束命令/工具网络，不能当作 Provider 进程的 OS 出口防火墙；这是 host-integration-only 证据，不代表真实外部 Provider、任意自定义 header、签名安装包或生产验收 |
| 当前边界 | `releaseAccepted=false`；Windows 原生/clean-host、macOS 签名/公证安装包、全 SDK IO/网络/秘密/句柄/后代监督与生产发布仍未验收。官方 Claude 账号/OAuth 不作为本阶段门禁；第三方非 Bearer header 仍未实现 |

本批次只增加持久审计事实与可重复的崩溃恢复验证；没有接入启动自动 sweep，也没有扩大
现有 host integration 证据的发布含义。

## 2026-09-24：nxs 第三方 Anthropic-compatible API-key 投影

Nexus 的 nxs runtime 现在把第三方 Anthropic-compatible Provider 的宿主
`AuthToken` 投影到 `ANTHROPIC_API_KEY`，由固定 SDK 生成 `x-api-key`，并在兼容
endpoint 上保留 SDK 的 Bearer fallback。Claude runtime 仍使用
`ANTHROPIC_AUTH_TOKEN`，不把这项 nxs 兼容路径冒用成 Claude 原生认证证明。

| 验证 | 结果与边界 |
| --- | --- |
| Nexus credential routing | `GOWORK=off go test ./internal/runtime/clientopts -run TestAnthropicRuntimeEnvRoutesCredentialsByBaseURL -count=1` 通过；覆盖 Claude-compatible Bearer 与 nxs-compatible API-key 两条投影 |
| 固定 nxs SDK 语义 | SDK `9956def1` 的 Provider 客户端对 `ANTHROPIC_API_KEY` 发送 `x-api-key`，兼容 endpoint 在无显式 Authorization 时生成 Bearer fallback；该事实来自固定源码/目标包测试 |
| 当前边界 | 自定义 header、真实外部 Provider、Provider 进程网络出口、Claude 账号/OAuth、签名安装包和生产发布仍未验收；`releaseAccepted=false` |

这项改动只扩大 nxs 的已声明第三方 token 兼容性，不改变沙箱 Network 的工具侧范围，也不把 Provider transport 当成已完成的 OS 网络隔离。

## 2026-09-24：macOS arm64 App、捆绑 nxs 与 DMG 本机验收

在 macOS 27.0 arm64 开发机上，使用固定 nxs 输入
`0f91b17fc0ed6976e01a76363f466640a1cddfa63bc32338cb7647153e014270` 和本机
arm64 `rg` 构建捆绑 runtime 的 App，并从 DMG 内的只读挂载路径再次启动。完整记录、
metadata、SHA-256 和 UI harness report 见
[2026-09-24 macOS App acceptance](./evidence/desktop-sandbox/2026-09-24-macos-app-acceptance/README.md)。

| 验证 | 结果与边界 |
| --- | --- |
| App build + smoke | `GOWORK=off ... make app-check` exit 0；Web、Swift shell、Go sidecar、nexusctl/nexuscfg、bundled nxs/rg、主窗口/Launcher 路由、退出与 sidecar 清理通过 |
| DMG | `scripts/desktop/package-macos-app.sh` exit 0；arm64、`bundled: true`、ad-hoc 签名、metadata 与 SHA-256 一致 |
| DMG 内 App | 只读挂载后 `codesign --verify --deep --strict` exit 0，arm64 nxs/rg 存在；从挂载 DMG 直接 smoke exit 0 并正常卸载 |
| Native UI | `GOWORK=off make app-check-ui-app` exit 0；12 个 app-shell 场景通过，fixture 无拒绝业务请求，原生输入/缩放/调整大小/恢复通过 |
| 本机安装演练 | 捆绑 App 复制到临时 Applications、全新状态根启动、替换一次再回退一次，3 次 smoke 均 exit 0；仅证明脱离构建目录的本机可运行 |
| 当前边界 | 这是 dirty-tree 的本机 arm64、ad-hoc 开发证据；Developer ID、公证、clean-host/quarantine、Intel、升级/回退、真实外部 Provider 和生产发布仍未验收，`releaseAccepted=false` |

本批次同时修正了 UI harness 对 macOS 原生控件偏移的旧 64px 硬编码，并为
`/auth/v1/status` 增加隔离只读 fixture；这两项只影响验收 harness，不改变产品运行时授权边界。

## 2026-09-23：固定 SDK 的 macOS 原生基线

使用 SDK `9956def130da33af47accf799a9c27c16a551104` 的干净归档构建 nxs，固定
Bridge `37434c2d38b129b6bbde67ac81afee673f39816d`，执行完整 macOS arm64
基线（host policy/lifecycle、资源与文件能力、搜索/媒体/Skill/context/project、managed
policy、settings writes 与 native 入口）。

| 验证 | 结果与边界 |
| --- | --- |
| 完整基线 | `GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox --sdk-ref 9956def130da33af47accf799a9c27c16a551104` 通过；报告 scope=`host-and-macos-native-baseline`、`passed=true`、`releaseAccepted=false` |
| nxs provenance | 归档构建的 nxs SHA-256 为 `0f91b17fc0ed6976e01a76363f466640a1cddfa63bc32338cb7647153e014270`；无模型请求 |
| 当前边界 | 这是当前开发机的 macOS arm64 基线，不是签名/公证安装包或 clean-host 验收；Windows 原生 ACL/UAC/Job Object、安装升级回退、Claude 真实认证/取消/清理、完整后代/句柄/秘密/网络隔离及生产发布仍未完成 |

本批次把固定 SDK/Bridge 的 macOS 原生开发基线跑通，但 `releaseAccepted=false` 保持不变。

### 2026-09-24：Windows 进程身份与双架构宿主门禁

Nexus scratch marker 现在在可用的平台记录进程创建时间。Windows recovery 使用
`OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` + `GetProcessTimes` 核验 PID 和创建时间；
不匹配代表 PID 已重用，可以把普通 active marker 作为候选，权限错误或无法查询仍保持
unknown，`cleanup_unknown` 永远优先保留。旧 marker 和无安全创建时间接口的平台不改变原有
保守路径。

交叉构建入口为 `make check-desktop-sandbox-windows`，实现见
[`check-windows-sandbox.mjs`](../../scripts/desktop/check-windows-sandbox.mjs)。它验证
Windows installer 单实例/per-user 合同，交叉编译 `internal/runtime`、
`internal/runtime/clientopts`、`internal/infra/confinedfs` 测试程序，以及
`nexus-server`、`nexusctl`、`nexuscfg` 的 Windows amd64/arm64 产物；在 Windows 主机上
追加两个进程身份回归。当前 macOS 本机结果已保存为
[2026-09-24 Windows cross-build evidence](./evidence/desktop-sandbox/2026-09-24-windows-cross-build/README.md)。

新增原生组件入口 `make check-desktop-sandbox-windows-native`。它只能在 Windows 主机
运行，要求 `NEXUS_SANDBOX_SDK_SOURCE` 指向干净的 SDK checkout，并精确匹配
`9956def130da33af47accf799a9c27c16a551104`；随后执行 SDK
`internal/tool/builtin/bash/sandboxexec` 的进程参数、private desktop、token、pipe、Job、
runner 生命周期、临时目录和 fail-closed 必测项。门禁会解析 Go JSON 事件，任何必测项
skip、缺失或失败都会失败关闭。Windows workflow 使用该入口；本机 macOS 只验证它在
非 Windows 主机上拒绝执行，不能伪造原生结果。workflow 同时上传门禁生成的 report
和原始 stdout/stderr，便于复核失败原因。

| 验证 | 结果与边界 |
| --- | --- |
| 本机 Windows gate | exit 0；installer contract、三个 runtime 目标包和三个命令均完成 amd64/arm64 构建；报告 `scope=windows-cross-build`、`releaseAccepted=false` |
| Windows native component gate | 已接入独立入口和 Windows workflow；要求固定 SDK checkout 与 25 个指定原生组件测试全通过；当前工作机没有 Windows 运行结果，报告仍不构成完整后端或发布验收 |
| Windows native marker | Windows 专用 `GetProcessTimes` 测试已编译；当前没有 Windows 主机运行日志，不能宣称 PID 重用/访问拒绝实机通过 |
| 原生执行后端 | SDK 当前原生 Windows `PrepareExecution` 仍 fail closed；现有 token/Job/private desktop/pipe 组件 CI 只证明组件，不证明完整 Nexus→Bridge→nxs 受限命令链 |
| 发布边界 | Windows 11 amd64/arm64 实机、P3 兼容/隔离组合矩阵、ACL/网络/账号设置、取消/后代收口、签名安装/升级/clean-host 和 `releaseAccepted` 仍待完成 |

因此，这一批次只把 Windows 的身份判断和构建门禁收口；在真实 Windows 证据到齐前，
Windows 受限 nxs 执行继续失败关闭，不能用交叉编译或组件 CI 代替原生验收。


## 2026-09-28：图片网络与 macOS 升级配套检查

固定 SDK `593b1fa6`、规范远程 Bridge `b0402649d44b` 的 40 项原生基线、527 个必测名称通过。
新增随包 sidecar/nxs 三种权限配置的发布自检；Nexus/nxs 配套发布，原有 runtime 选择逻辑保持。
旧开发会话升级/回退与原配置保留通过；已发布版本升级和正式包仍待单列证据；新固定依赖的真实第三方 Provider 10 项基础检查通过。
详见 [证据](evidence/desktop-sandbox/2026-09-28-media-network-compatibility/README.md)。


## 2026-09-28：已发布内核的历史数据与干净整包验证

已发布 nxs `v0.1.34` 创建的持久会话，在当前内核及当前 App 捆绑内核上均完成升级续用和回退续用；
配置、历史、记忆与工作文件保留。Nexus 干净提交 `223849cb8` 的 arm64 ad-hoc App/DMG 通过本机 smoke。
这是配套版本的数据兼容证据；原 runtime 选择逻辑保持，没有新增用户升级步骤。
见 [报告](evidence/desktop-sandbox/2026-09-28-released-upgrade/README.md)，完整 App 数据库升级和正式分发仍独立验收。

## 2026-09-28：macOS 远端 MCP 配置兼容与独立网络

正常发布按 Nexus/nxs 整包验收，旧用户兼容聚焦数据、配置和会话。
已修复默认沙箱下已有 HTTP/SSE MCP 的整 Agent 启动拒绝；独立端点授权、跨 origin/代理拒绝、权限取消和配置撤销已接通。
固定 SDK `b487ef24`、canonical Bridge `c2b5eaf` 与 Nexus `626f11d5c` 的基线 42 项检查、569 个指定测试名全部通过；真实 nxs 的持久 HTTP/Connector SSE 工具往返及三种权限模式自检通过。
扩展回归中另有两项已在改动前复现的记忆测试失败，原生 worker 的 race 插桩尝试也未通过，未计入通过声明。
见[完整证据与边界](evidence/desktop-sandbox/2026-09-28-mcp-network/README.md)。App UI、正式签名/公证、clean-host、helper/stdio/Provider 与完整后代监督仍独立验收，`releaseAccepted=false`。

## 2026-09-28：macOS MCP 认证 helper 与辅助输出限额

Nexus `2a0fdb895`、SDK `5dc1eb8b` 与远程 Bridge `32d41b7` 的固定来源基线通过 44 项检查、595 个必测名称；完整四条 HTTP/SSE 与 helper 工具往返、原生受限执行/取消/关闭、文件辅助输出限额及配套自检见[证据](evidence/desktop-sandbox/2026-09-28-mcp-helpers/README.md)。首次启动指令组累计超时的失败报告和对照结果保留；整组预算按用例数分配，生产操作时限、断言和必测 skip/缺失拒绝规则不变。

认证 helper 由 `sandbox_mcp_helpers_v1` 独立协商，失败不回退静态或旧凭据；端点网络授权不授予 helper 网络。stdio、独立脱离后代、完整 App UI 和正式安装升级仍未闭合；本批未运行 Windows，`releaseAccepted=false`。

## 2026-09-28：macOS stdio MCP

Nexus `1b1bb88ca`、SDK `b84b7b6c` 与规范 Bridge `4b2972f` 的固定基线通过 45 项检查、640 个必测名称。持久配置和 Connector 两条 stdio 入口通过真实 nxs 完成工具往返；原生文件、网络、环境、取消/替换/关闭、并发 wire ID 与聚合输出限额见[证据](evidence/desktop-sandbox/2026-09-28-mcp-stdio/README.md)。本项不代替脱离后代、实际外部 MCP、App UI 或签名安装验收；未运行 Windows，`releaseAccepted=false`。

## 签名包专用验收入口

`.github/workflows/macos-sandbox-acceptance.yml` 是仅用于统一沙箱分支的可复用工作流。
通过现有 `macos-desktop-build.yml` 的手动输入 `sandbox_acceptance=true` 调用；普通
PR/main 构建保持原行为。固定 SDK 提交与当次 Nexus SHA，在 Apple Silicon/Intel 两种
原生 macOS runner 构建，使用既有 Developer ID 与公证配置，仅上传验收产物；不建 tag、
不发布 Release，也不运行 Windows。

SDK 是私有仓库且明确禁止 Deploy Key（GitHub API 422）。临时 key 配置失败，未向 CI 写入
新密钥，生成的本地私钥已清理。执行前需要 `NEXUS_SANDBOX_SDK_READ_TOKEN`：仅授权该 SDK
仓库 Contents read 的短期 CI token，或用户确认的同等既有授权；不得借用个人全权限账号 token。
此凭据不能写入源码、产物或聊天。入口及脚本已通过 actionlint v1.7.12、bash 语法检查，以及
脏来源/未签名/未公证三项拒绝夹具；远程签名运行尚未开始。

```sh
gh workflow run macos-desktop-build.yml \
  --ref codex/desktop-sandbox-approvals -f sandbox_acceptance=true
```

`check-macos-signed-install.sh` 先校验包摘要及签名元数据，再从只读 DMG 复制到新的临时目录、
写入 quarantine，验证 codesign/stapler/Gatekeeper 并执行实际 bundled-runtime 自检。
原始日志、安装自检与三仓 provenance 随包归档。构建 smoke 允许 runner 的无图形界面回退，
故本入口不声明完整 App UI、用户数据库升级/回退或真实用户干净机器验收；工作流文件通过
静态检查也不代表远程运行已经通过。

## 2026-09-28：当前 stdio 内核的 App、DMG 与真实第三方模型

Nexus `d92fdb7e9` 与 SDK `b84b7b6c` 从干净来源生成 arm64 ad-hoc App/DMG；独立状态根下 App smoke、
只读 DMG 内 App smoke、三种资源配置的实际内核握手，以及包内 nxs 的五项真实第三方模型检查通过。
临时验证器首次卸载收尾错误保留，修正后整条挂载/启动/自检/卸载流程通过，详见
[本批证据](evidence/desktop-sandbox/2026-09-28-stdio-app/README.md)。不代表完整 DM/Room/后台 UI、
正式签名、公证或用户数据库升级；签名 CI 仍等待私有 SDK 只读授权。

## 2026-09-28：独立 macOS processscope 组件

[组件证据](evidence/desktop-sandbox/2026-09-28-processscope-component/README.md)记录原生脱离后代清理、错误身份拒绝、登记恢复、对照保留、竞态与无 cgo 拒绝路径。组件未接入默认 transport 或产品恢复，原生通过不改变 `releaseAccepted=false`，完整进程监督仍未闭合。

## 2026-09-28：macOS 引导接收端

[真实 helper 证据](evidence/desktop-sandbox/2026-09-28-process-bootstrap/README.md)覆盖启动身份、放行前断连、登记后原地 exec、标准流、显式环境及任务存活时控制 fd 关闭。异常输入/fd 传输与无 cgo 检查通过，描述符截断泄漏的复现和修复证据一并保留。产品宿主持久登记、默认传输、恢复和发布包尚未接通，`productionIntegrated=false`、`releaseAccepted=false`。

## 2026-09-28：内核根退出观察

[原生退出观察证据](evidence/desktop-sandbox/2026-09-28-root-exit-observer/README.md)确认 helper 放行前注册后可跨 exec 获取真实退出码，取消等待/停止观察不伪造退出，主进程退出后的脱离后代仍须独立回收。默认 transport、持久宿主绑定、unknown 恢复和正式发布仍未接通，`releaseAccepted=false`。

## 2026-09-28：宿主持久启动事实

[登记与准入证据](evidence/desktop-sandbox/2026-09-28-process-registry/README.md)覆盖 SQLite 重开、并发一次性放行、迟到及跨 scope 拒绝、原集合证据绑定和无策略回执时阻断新 factory。runtime 包、仓储竞态与架构门禁通过；生产 launcher 尚未调用写入链，不能据此声称实际启动/崩溃恢复已完成，`releaseAccepted=false`。

## 2026-09-28：显式监督启动器

[原生竞态证据](evidence/desktop-sandbox/2026-09-28-supervised-launch/README.md)验证持久阶段顺序、脱离输出后代、共享关闭、丢失放行响应不重放及回收写入失败保留。Host 仍为独立文件夹具，不等同 Nexus 数据库/默认 transport 接入；`releaseAccepted=false`，真实产品接线和恢复继续待办。

### 2026-09-28：监督启动 Host 数据库适配

[真实 helper/launchd/SQLite 证据](evidence/desktop-sandbox/2026-09-28-process-host/README.md)已通过，目标竞态、runtime 包回归、无 cgo 与架构检查通过。固定 Bridge `95b9616`；实际默认 transport、App 受保护目录装配、崩溃恢复及完整发布验收仍未完成。

### 2026-09-28：显式监督 transport

[Bridge 原生传输证据](evidence/desktop-sandbox/2026-09-28-supervised-transport/README.md)八个必测名称全部通过；JSON、退出码、脱离后代、强制关闭、保留清理错误、全部 probe 及取消已覆盖。Nexus 固定 Bridge `e8787a4`，但 Manager 默认启用及完整崩溃恢复仍未完成，不能按模块更新宣称 App 已启用。

### 2026-09-28：Manager 监督身份与探测代次

[绑定与迁移证据](evidence/desktop-sandbox/2026-09-28-supervisor-binding/README.md)覆盖 factory 前身份、同代次四种进程目的、重启阻断、热更新和替换代次；真实 helper/launchd/SQLite 四阶段回收通过。迁移 145 原样保留旧意图，存在多个启动的代次拒绝有损回退。当前仍是显式 Manager 装配，App 默认启用、scratch lease 关联和崩溃恢复未完成；PostgreSQL 仅静态审查，无运行验收。

### 2026-09-28：原进程登记恢复

[恢复证据](evidence/desktop-sandbox/2026-09-28-process-recovery/README.md)包含真实宿主未 Close 即退出、原任务仍存活后按原登记回收的 Bridge race 测试，以及 Nexus 真实 SQLite/注入原生结果的 exact scope 回归。进程收口不清除 policy unknown；活动 client、跨 owner 和原生失败不会成功收口。App 独占实例锁装配、自动扫描、lease/policy 对账和实际重启验收仍未完成。

### 2026-09-28：macOS sidecar 状态根锁

[实例锁证据](evidence/desktop-sandbox/2026-09-28-sidecar-instance/README.md)验证迁移前独占、真实持有者退出释放、CLOEXEC、链接/替换拒绝及旧数据库/会话迁移保留。此锁已进入 macOS 桌面服务入口；仅协调采用协议的 sidecar，旧版未持锁宿主与完整 App 自动恢复仍须单独接线验收。

### 2026-09-28：command hook 沙箱与关闭

[Hook 证据](evidence/desktop-sandbox/2026-09-28-command-hooks/README.md)覆盖原生 shell/argv 文件边界、两种异步形式在权限变化/关闭时撤销、输出限制、准备/清理失败和 SessionEnd 关闭顺序。新版 nxs/Claude 第三方模型联调通过。原生 MCP race 初始化超时未算通过；App 默认监督恢复及发布验收仍未完成，`releaseAccepted=false`。

### 2026-09-28：默认监督装配

[默认 sidecar 证据](evidence/desktop-sandbox/2026-09-28-default-supervisor/README.md)包含默认启动校验、两阶段恢复分页、关闭资源顺序、真实 nxs AutoDream、随包 sidecar 健康和重复/篡改拒绝。开发 sidecar 与 Swift 壳构建通过；本机 XCTest 框架缺失，独立 Swift 断言通过不能替代 XCTest。图形 App、带活动后代的默认崩溃重启、旧 sidecar、macOS 14.0 和正式发布仍未验收。

### 2026-09-28：真实默认 sidecar 崩溃恢复

[真实入口证据](evidence/desktop-sandbox/2026-09-28-sidecar-crash/README.md)两次通过：HTTP 配置、WebSocket DM、第三方模型、真实 nxs Bash 启动 setsid 后代，SIGKILL sidecar 后确认后代仍活；默认重启自动回收原记录、scratch 和策略，无新 launch、无命令重放。此前 smoke 的 `/health` 被纠正为 `/nexus/v1/health` 加 JSON 状态断言，原 200 不作为健康证据。此结果不替代图形 App、Claude crash、旧 sidecar、macOS 14/Intel 和发布验收。

### 2026-09-28：原生 sidecar 精确身份

[身份清理证据](evidence/desktop-sandbox/2026-09-28-sidecar-identity/README.md)覆盖 boot/audit-token 登记、正常与孤儿精确信号、PID 复用不误杀、旧格式存活/未知记录保留、损坏/替换/链接拒绝和真实孤儿清理。九个 Swift 测试主体以独立断言通过；Swift 构建、clientopts 回归和两后端第三方模型文件保护通过。macOS 14.0 接口缺口、图形升级/退出重开、签名与干净机器仍未验收。
