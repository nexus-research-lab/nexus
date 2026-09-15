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

## 自动基线入口

[check-sandbox-baseline.mjs](../../scripts/desktop/check-sandbox-baseline.mjs) 强制 GOWORK=off 和只读 module 解析，拒绝 Bridge replace，记录模块版本、checksum、源码提交、binary SHA-256、OS/架构、每条命令和最终退出码。具名必测用例 skip 或未匹配均失败，不以包级 PASS 替代。

原生入口还逐项要求伪造 PATH、空 PATH、资源禁止与后台网络子场景的成功证据，不能只凭父测试 PASS；当前使用 SDK d9687a3e 或包含这些用例的后续提交。历史记录使用各自对应的 Nexus 版本入口复核。

```sh
# 已有 nxs：宿主集成基线，包含真实握手/诊断；不宣称原生隔离已验收。
NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox

# macOS：导出固定 SDK 提交、构建 nxs，再执行宿主与原生隔离基线。
# SDK dirty 改动只记录清单，git archive 不包含这些改动。
node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref d9687a3ed9d3a7da86b84be02a49ddbbed6dfa91
```

原生模式需允许运行临时回环服务器与 Seatbelt；不会发送模型请求、设置账号、防火墙或开启产品开关。控制台打印独立证据目录，其中 report.json 和各检查日志保留本次结果；releaseAccepted 始终为 false，UI/其他工具覆盖/安装包门禁仍需另行完成。
