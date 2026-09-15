# 桌面沙箱验收矩阵

状态：开发验收清单，non-normative，2026-09-15。当前合同见 [规范](../specs/desktop-sandbox-spec.md)，开发状态见 [计划](../explorations/desktop-sandbox/development-plan.md)。

## 证据记录要求

每条结果记录：仓库提交、dirty 范围、runtime binary 来源与 SHA-256、OS/架构、测试命令、最终退出码、实际执行场景、skip 和剩余限制。编译、握手、依赖检测、真实隔离、安装包体验分别记录。

## 必须验收的场景

| 类别 | 场景 | 通过条件 |
| --- | --- | --- |
| 准入 | 开关关闭、server、macOS/Windows、旧 nxs、Claude、缺能力 | 兼容路径保持；需要但无法提供的边界在命令开始前拒绝 |
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
| 包与兼容 | macOS、Windows 支持版本/架构、Linux owner、Claude、旧数据根 | 各有真实证据；没有用某平台通过推断另一平台 |
| 产品路径 | 设置、Composer、审批卡、DM/Room/自动化、重载 | 展示实际边界；一个清晰下一步；不泄漏内部标识，不自动重发 |

## 本次重新审计的执行记录

日期：2026-09-15。Nexus 基线 928c7e803；Bridge module 为 3da56a2；SDK 使用 c8245262 的独立导出，排除工作区未提交变更。

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

## 自动基线入口

[check-sandbox-baseline.mjs](../../scripts/desktop/check-sandbox-baseline.mjs) 强制 GOWORK=off 和只读 module 解析，拒绝 Bridge replace，记录模块版本、checksum、源码提交、binary SHA-256、OS/架构、每条命令和最终退出码。具名必测用例 skip 或未匹配均失败，不以包级 PASS 替代。

```sh
# 已有 nxs：宿主集成基线，包含真实握手/诊断；不宣称原生隔离已验收。
NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox

# macOS：导出固定 SDK 提交、构建 nxs，再执行宿主与原生隔离基线。
# SDK dirty 改动只记录清单，git archive 不包含这些改动。
node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /absolute/nexus-agent-sdk-go --sdk-ref c824526230860fbfc2a28736980b5fa720dbffe4
```

原生模式需允许运行临时回环服务器与 Seatbelt；不会发送模型请求、设置账号、防火墙或开启产品开关。控制台打印独立证据目录，其中 report.json 和各检查日志保留本次结果；releaseAccepted 始终为 false，UI/其他工具覆盖/安装包门禁仍需另行完成。
