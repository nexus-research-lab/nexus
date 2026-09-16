# 桌面沙箱验收矩阵

状态：开发验收清单，non-normative，2026-09-15。当前合同见 [规范](../specs/desktop-sandbox-spec.md)，开发状态见 [计划](../explorations/desktop-sandbox/development-plan.md)。

## 证据记录要求

每条结果记录：仓库提交、dirty 范围、runtime binary 来源与 SHA-256、OS/架构、测试命令、最终退出码、实际执行场景、skip 和剩余限制。编译、握手、依赖检测、真实隔离、安装包体验分别记录。

## 必须验收的场景

最终产品要求：受控运行环境内建于 SDK 执行链，所选后端直接落实默认资源策略，无独立沙箱开关；完全访问必须来自用户显式选择，改变资源/审批策略而不取消执行生命周期和领域授权。以下原生文件与辅助 IO 全覆盖断言属于 nxs 自主引擎；Claude Code 单列原生沙箱与工具权限验收，不能借用 nxs 的 OS 覆盖证据，也不能把工具 helper 的隔离视为整个 SDK 主进程的隔离。

| 类别 | 场景 | 通过条件 |
| --- | --- | --- |
| 准入 | 开关关闭、server、macOS/Windows、旧 nxs、Claude、缺能力 | 兼容路径保持；需要但无法提供的边界在命令开始前拒绝 |
| 默认权限与后端 | 新任务、升级后默认值、nxs/Claude 切换、缺依赖/不支持平台、显式 Full Access | 默认请求批准/自动审核均自动受限；旧实例收口、新实例确认后才能发任务；不支持不静默裸执行 |
| Claude 原生接入 | 固定版本、settings 来源/合并、Bash 子进程、文件权限、网络批准、取消、模式切换 | 配置确实生效，缺依赖拒绝；原生接口保证不了的边界明确拒绝；不伪造 nxs 协议或文件 helper 覆盖 |
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
