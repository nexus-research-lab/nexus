# Windows 桌面沙箱：改用进程安全环境（PSEC）的开发计划

> Historical / non-normative: the Go PSEC implementation described below was replaced on 2026-10-29 by a bundled Rust adapter using Microsoft mxc-sdk. Current contract: [desktop-sandbox-spec.md](desktop-sandbox-spec.md); current build and acceptance: [windows-mxc-sandbox-verify.md](windows-mxc-sandbox-verify.md). Do not use the obsolete file lists or commands below for release.

本文为 **non-normative 开发计划与历史交接**；当前合同以 [desktop-sandbox-spec.md](desktop-sandbox-spec.md) 和 SDK 文档为准。

本文是 Windows 桌面沙箱从 `wxc-exec.exe` 迁移到"进程内调用系统 PSEC API"的开发计划和交接说明。阶段 1（SDK 核心）已经完成，阶段 2 及之后的任务按本文继续。

| 仓库 | 分支 | 阶段 1 完成时的提交 |
|---|---|---|
| nexus-agent-sdk-go | `codex/desktop-sandbox-approvals` | `22ccd6c2` |
| nexus | `codex/windows-mxc-sandbox` | 本文所在提交 |
| nexus-agent-sdk-bridge | 不改 | nexus 沿用 main 上的 pin |

相关文档：[windows-mxc-sandbox-risks.md](windows-mxc-sandbox-risks.md)（旧方案的风险复核，R2/R3/R4/R5 已在本方案中按设计处理，仍需验证）、[windows-mxc-sandbox-verify.md](windows-mxc-sandbox-verify.md)（旧方案的验证清单，环境变量已过时，以本文第 6 节为准）、[desktop-sandbox-spec.md](desktop-sandbox-spec.md)。

---

## 1. 为什么要改

**现象**：打包后的 Windows 桌面端无法对话，会话初始化报 `context deadline exceeded`。

**根因**（代码推算，尚无 Windows 日志佐证）：受限模式下 nxs 的项目发现（commands、agents、settings、约 18 个 skill、AGENTS.md 与 rules）全部经沙箱文件 helper 读取，初始化期间约 70–100 次文件操作。旧实现中每次操作都要：启动 `wxc-exec --probe` 两次 → 启动 `wxc-exec` 创建一次 ProcessContainer → 启动 nxs helper。单次超过约 0.6 秒，总耗时就会超过 bridge 默认 1 分钟的初始化超时。

**调研结论**：Codex 同样使用 MXC，但接法不同（[`codex-rs/mxc-sandbox`](https://github.com/openai/codex/blob/5ef96ab2785f3ecddf279639eff7b1b42c906fa4/codex-rs/mxc-sandbox/README.md)）。MXC 的核心是系统 API：`System32\processmodel.dll` 的 `CreateProcessSecurityEnvironment`（接收 "PSEC" FlatBuffer 策略），再通过 `PROC_THREAD_ATTRIBUTE_SECURITY_ENVIRONMENT` 挂到 `CreateProcessW` 上（[microsoft/mxc](https://github.com/microsoft/mxc) 的 `secenv.rs`、`base_container_runner.rs`）。旧实现和 Codex 的关键差异：

| 方面 | 旧实现 | 新实现（对齐 Codex） |
|---|---|---|
| 调用方式 | 每次启动 `wxc-exec.exe` | 宿主进程内算策略，nxs 自身作包装进程调用系统 API |
| 可用性探测 | 每次操作 2 次 `--probe` 子进程 | 进程内创建/关闭一次 PSEC，`sync.Once` 缓存 |
| 只禁写路径 | 映射为 `deniedPaths`（读写同拒），导致 `.nexus\commands` Access denied | 读写授权内部的只读 carveout，可读不可写 |
| 读权限 | 授权外全部拒绝（连 Program Files 都读不到） | 卷根及一级子目录、系统目录只读，与 macOS `(allow file-read*)` 一致 |
| DACL | 未设 `allowDaclMutation`，MXC 默认允许改 ACL | 走 BaseContainer，从不修改宿主 ACL |
| 取消 | `taskkill /T` 按父进程树杀，容器内进程可能残留 | 包装进程持有 kill-on-close Job，被杀时整棵树结束 |
| 配置传递 | base64 塞进命令行（32K 上限） | 4 KB 环境变量块，上限 1 MB |
| 分发 | 需要打包 `wxc-exec.exe`、`mxc_ffi.dll`、`winhttp-proxy-shim.exe` | 不需要任何 MXC 二进制，许可/签名问题消失 |

---

## 2. 新架构

```
nxs（宿主进程，普通用户令牌）
 ├─ psecAvailable()        进程内创建/关闭一次 PSEC，缓存结果；每次检查提权令牌
 ├─ mxcBuildFilesystemPolicy / mxcNetworkPolicy / mxcUIRestrictions   纯策略
 ├─ encodePSECSpec()        PSEC FlatBuffer
 └─ PrepareExecution → ShellExecution{Shell: nxs.exe, Args: [--sandbox-mxc-launch],
                                      Env: 最小启动变量 + NEXUS_MXC_LAUNCH_* 请求块}
        │
        ▼  exec.Cmd 启动（stdio 是普通管道）
nxs --sandbox-mxc-launch（包装进程，未受限）
 ├─ 解码请求（命令行、工作目录、子进程环境、PSEC 字节）
 ├─ CreateProcessSecurityEnvironment(spec)
 ├─ CreateProcessW(CREATE_SUSPENDED | EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW,
 │                 属性: SECURITY_ENVIRONMENT + HANDLE_LIST(自身 stdio))
 ├─ AssignProcessToJobObject(kill-on-close) → ResumeThread
 ├─ 等待前台进程 → TerminateJobObject（清理残留后代）→ 关闭环境
 └─ 以子进程退出码退出
        │
        ▼
受限子进程（cmd / bash / powershell / nxs 文件 helper / MCP stdio …）
```

策略规则（`sandboxexec/windows_mxc_policy.go`）：

- **读写**：工作区（workspace-write 时）、scratch、`AllowWrite`。
- **只读**：`AllowRead`、只读模式下的工作区、所有本地卷根及其当前一级子目录（BaseContainer 的卷根授权不递归）、`C:\Windows`、Program Files、Program Files (x86)、ProgramData；网络盘只在工作目录位于其上时纳入。
- **只禁写（denyWrite）**：位于读写授权内 → 只读 carveout；位于授权外 → 丢弃（本来就不可写）；与读写授权同一路径 → 该授权降为只读；包住某个读写授权 → 拒绝启动。
- **禁读（denyRead）**：进入 `deniedPaths`，按 BaseContainer"更具体路径覆盖祖先"的规则作用于整棵子树，不再补结尾反斜杠。
- **准备期不存在的受保护路径**（位于读写授权内）：照常下发，同时进入 `MXCUnprotectedPaths`，命令结束后复核并按违规上报。
- **网络**：配置了域名 allowlist（包括显式空列表）→ 出站默认拒绝，只放行 `127.0.0.0/8` 与 `::1`，入站拒绝，宿主回环放行（peer `MXC-Loopback`），流量经 Nexus HTTP 代理；没有 allowlist → 出站放行，入站拒绝，`AllowLocalBinding` 时放开宿主回环。
- **capability**：总带 `registryRead`；有出站放行时 `internetClient`；宿主回环放行时 `networkLoopback`。
- **UI**：不禁用 Win32k（PowerShell 启动需要），禁止剪贴板读写、输入注入、IME 切换、桌面切换与注销关机、系统参数与显示设置修改（掩码 `0x3DE`，按系统 build 去掉不支持的位）。

---

## 3. 阶段 1：SDK 核心（已完成，`22ccd6c2`）

| 文件 | 内容 |
|---|---|
| `sandboxexec/windows_psec_spec.go` | PSEC FlatBuffer 编码器；字段槽位与 MXC 生成代码的 `VT_*` 常量逐一核对过 |
| `sandboxexec/windows_mxc_policy.go` | 纯策略层（文件、网络、UI、capability），无构建标签 |
| `sandboxexec/windows_mxc_transport.go` | 请求的环境变量块编码/解码（4 KB 块、1 MB 上限、缺块拒绝） |
| `sandboxexec/windows_mxc_windows.go` | Windows 适配器：后端选择、包装进程定位、提权检测、隐式只读根枚举、执行计划 |
| `sandboxexec/windows_psec_windows.go` | `processmodel.dll` 封装、缓存探测、包装进程启动流程、Job 管理 |
| `cmd/nxs/main.go` | 新增 `--sandbox-mxc-launch` 入口 |
| `sandboxfs/operation_policy.go` | 删除：新映射不再拒绝读，绕行不再需要 |

包装进程的定位：`NEXUS_MXC_LAUNCHER`（只读宿主进程环境，测试用）优先；否则要求当前进程是 nxs（`NEXUS_RUNTIME_KIND=nxs`），使用 `os.Executable()`。

**已验证**（Mac）：`go build ./...`；`GOOS=windows`（amd64/arm64）与 `GOOS=linux` 的 `go vet ./...`（含测试编译）；`go test ./...` 全部通过。新增单测：PSEC 槽位解码、策略映射（含 `.nexus\commands` 回归）、网络/UI/capability、传输往返与上限。

**未验证**：任何 Windows 原生行为。原生验收测试已改写（见第 6 节），需要在 Windows 上跑。

---

## 2026-10-27 接续实施状态

- T1：SDK 文档已更新，提交 `67ea1838`。
- T2：SDK CI/发布已去掉 MXC 二进制，提交 `622ec03c`，已推送。原生 CI 未在本轮执行，不代表 runner 支持 PSEC。
- T3：Nexus 构建、打包、签名列表、C# 注入和诊断均移除旧二进制依赖；安装器静态合同通过，Windows 打包待验。
- T4：三处 SDK pin 统一到 `622ec03c205ab38dac1b6b9be3503447d784601d`，测试名单按当前 SDK 更新；Windows amd64/arm64 交叉构建门禁通过，`releaseAccepted=false`。
- T5：Windows 受限会话默认初始化预算三分钟，保留显式值；诊断页与启动错误共用已知原因的中文提示。Go 定向测试通过。
- T6：规范、风险说明、验证清单、CHANGELOG 已同步；不宣称原超时已被实测修复。
- 本轮本机验证：SDK 策略/编码定向测试与 Windows amd64 定向 vet、Nexus `make check-go`/架构门禁、Windows 双架构交叉构建、安装器静态合同、两仓 workflow YAML 解析和前端 `npm run check` 均通过。未执行 Windows 原生测试、C#/PowerShell 原生构建或 CI runner 验收。
- T7：待 Windows 实测项目发现操作次数和耗时；超过十秒才决定长驻 worker，当前未实施。
- T8：保留旧 bridge 分支，不在本轮删除；ARM64 原生验收待设备。

阶段 2 的代码接续已实施；下方任务列表保留原计划依据，所有原生验收标准仍未完成。

---

## 4. 阶段 2：原始开发任务与验收标准

按顺序做；每项都写了改动位置和验收标准。

### T1 SDK 文档与元数据

- `docs/sdk/windows-mxc.md`：整篇按第 2 节重写（PSEC 模型、策略规则、包装进程、`NEXUS_MXC_LAUNCHER`、原生测试的运行方式）；删除 `wxc-exec`、`NEXUS_MXC_EXECUTABLE`、sidecar 打包的内容。
- `docs/sdk/sandbox.md` 的"Windows 后端实施状态"一节、`README.md` 第 7 行、`AGENTS.md` 第 27 行：去掉"发布包携带 wxc-exec.exe…"，改为"直接调用 processmodel.dll 的进程安全环境，不打包 MXC 二进制"。
- `CHANGELOG.md` Unreleased：把"Add Microsoft MXC ProcessContainer…Package the pinned wxc-exec.exe sidecar…"改写为新方案的描述；"Windows project discovery reads … read-only file helpers that drop write-only denials"改为"write-only protections become read-only carveouts"。
- 验收：`git grep -i "wxc-exec\|NEXUS_MXC_EXECUTABLE\|mxc_ffi\|winhttp-proxy-shim"` 在 SDK 仓库只剩 `windows_mxc_windows.go` 文件头的历史说明（或一并删掉）。

### T2 SDK CI

- `.github/workflows/desktop-sandbox.yml`：删除"Install MXC ProcessContainer sidecar"一步与 `setup-node`；"Build MXC acceptance fixture"不再复制 `mxc` 目录；"Run MXC acceptance as a standard user"里去掉 `NEXUS_MXC_EXECUTABLE`，只传 `NEXUS_REQUIRE_NATIVE_MXC=1`、`NEXUS_NXS_EXECUTABLE`（同时作为包装进程）、`NEXUS_MXC_TEST_ROOT`；测试过滤改为 `^(TestWindowsMXC|TestMXC|TestEncodePSEC)`，sandboxfs 增加 `TestWindowsMXCFileHelperReadsWriteProtectedProjectMetadata`。
- 注意：GitHub 托管 runner 的系统 build 可能低于 26100，PSEC API 不存在时测试会以"Windows build does not provide the process security environment API"失败。若确认 runner 不支持，原生验收只能放到自托管 Windows 11 24H2+ runner，或作为本地证据；不要改成 skip 后当作通过。
- `.github/workflows/release-nxs.yml`：删除 `install_mxc` 函数、`setup-node` 步骤、zip 中的 `mxc` 目录和 manifest 里的 `mxc/bin/x64/*` sidecar 条目。
- 验收：workflow 在支持 PSEC 的 runner 上跑通，且没有任何 required 测试被 skip。

### T3 nexus 删除 MXC 二进制打包

| 文件 | 改动 |
|---|---|
| `desktop/windows/Nexus.Desktop/Sidecar/SidecarSupervisor.cs` | 删除 `ApplyBundledMXCRuntime` 及调用（第 181、302–340 行附近），不再设置 `NEXUS_MXC_EXECUTABLE` |
| `desktop/windows/Nexus.Desktop/Diagnostics/DesktopDiagnosticsReport.cs` | 删除 `bundled_mxc_*` 三个字段（第 190–192 行） |
| `scripts/desktop/build-windows-app.ps1` | 删除 `BundleMXCRuntime`/`MXCRuntimePath` 参数、`Resolve-MXCExecutable` 与复制三件套的逻辑 |
| `scripts/desktop/package-windows-app.ps1` | 删除对应参数、`mxcRuntimeRoot` 检查、签名列表与 manifest 中的 MXC 条目 |
| `scripts/desktop/run-windows-app.ps1` | 删除对应参数透传 |
| `scripts/desktop/install-windows-mxc.ps1` | 整个文件删除 |
| `makefile` | 删除 `install-windows-mxc` 目标（第 297 行附近）及 `NEXUS_MXC_VERSION`、`NEXUS_DESKTOP_MXC_*` 变量 |
| `scripts/desktop/verify-windows-installer-contract.mjs` | 删除第 71–78 行对 MXC 文件的断言 |
| `desktop/windows/README.md` | 删除第 40–51 行 MXC 三件套与安装脚本说明，改为"沙箱直接使用系统 API，需要 Windows 11 24H2（build 26100.9278）或更新版本，以普通用户运行" |
| `internal/app/runtimecheck/check.go` | 第 112 行起：不再透传 `NEXUS_MXC_EXECUTABLE`；`NEXUS_REQUIRE_NATIVE_MXC` 可保留或删除 |

- 验收：`git grep -i "wxc-exec\|NEXUS_MXC_EXECUTABLE\|mxc_ffi\|winhttp-proxy-shim\|BundleMXC\|MXCRuntime"` 在 nexus 仓库（除 `docs/specs/windows-mxc-sandbox-*.md` 历史文档）无结果；`make app-win-build` 和打包脚本在 Windows 上跑通；安装包里没有 `mxc` 目录。

### T4 nexus 门禁与 pin

- `scripts/desktop/check-windows-sandbox.mjs`、`.github/workflows/windows-desktop-build.yml`、`.github/workflows/publish.yml`（`Checkout pinned Windows sandbox SDK`）：SDK pin 从 `e8bad7a5…` 改为阶段 2 完成后的 SDK 提交（三处保持一致）。
- `check-windows-sandbox.mjs` 的测试清单：
  - `WINDOWS_NATIVE_TESTS`（CI，提权环境，不需要 PSEC）：用新的纯策略测试替换已删除/改名的测试——删去 `TestMXCDenyPolicy*`、`TestMXCRecursiveDenyPathAddsOneSeparatorOnWindows`、`TestMXCDenyPolicyKeepsProjectCommandsWriteDenial`，加入 `TestMXCDenyWriteInsideWorkspaceBecomesReadOnlyCarveout`、`TestMXCDenyWriteOutsideGrantsIsDropped`、`TestMXCDenyWriteEqualToGrantRemovesTheGrant`、`TestMXCDenyWriteCoveringGrantFailsClosed`、`TestMXCDenyReadBecomesDeniedPathAndOverridesReads`、`TestMXCReadOnlyWorkspaceAndExplicitReadGrants`、`TestMXCMandatoryProjectCommandsStayReadable`、`TestMXCNetworkPolicyProxyModeAllowsOnlyLoopback`、`TestMXCNetworkPolicyWithoutAllowlistAllowsEgress`、`TestMXCUIRestrictionsFollowBuildSupport`、`TestMXCPSECSpecAlwaysCarriesRegistryRead`、`TestMXCLaunchRequestRoundTripsThroughEnvironmentChunks`、`TestMXCLaunchRequestRejectsOversizedPayloads`、`TestEncodePSECSpecMatchesSchemaSlots`、`TestEncodePSECSpecOmitsEmptyOptionalFields`；保留 `TestMXCPathContainmentBoundaries` 与 junction 测试。
  - `WINDOWS_NATIVE_FILE_TESTS`：`TestOperationConfigDropsWriteDenialsOnlyForWindowsReads` 已随绕行删除，从清单移除（或整组删除 `native-sdk-file-policy` 一步）。
  - `WINDOWS_LOCAL_MXC_TESTS`（本地普通用户）：`TestWindowsMXCConfigPassesNativeDryRun` → `TestWindowsMXCProcessSecurityEnvironmentProbe`；`TestWindowsMXCOutsideGrantsAreDefaultDenied` → `TestWindowsMXCOutsideGrantsAreReadOnly`；`TestWindowsMXCCancellationStopsSidecar` → `TestWindowsMXCCancellationStopsTheSandboxedTree`；新增 `TestWindowsMXCForegroundExitEndsDescendants`、`TestWindowsMXCDoesNotMutateHostACLs`、`TestWindowsMXCDeniedDirectoryCoversItsSubtree`；文件 helper 一组新增 `TestWindowsMXCFileHelperReadsWriteProtectedProjectMetadata`。
  - 本地证据分支（第 279–291 行）：不再要求 `NEXUS_MXC_EXECUTABLE`；构建出的 `nxs-mxc-file-helper.exe` 同时作为 `NEXUS_NXS_EXECUTABLE` 和包装进程。
- 验收：`node scripts/desktop/check-windows-sandbox.mjs --native` 在 CI 通过；带 `NEXUS_ALLOW_LOCAL_NATIVE_EVIDENCE=1` 时在普通用户的 Windows 11 24H2+ 终端通过。

### T5 nexus 初始化超时与错误提示

- `internal/runtime/clientopts/desktop_sandbox.go` 的 Windows 受限分支：`options.Runtime.InitializeTimeout` 为 0 时设为 3 分钟，作为保险（新方案预计远低于 1 分钟，若实测仍接近上限，说明需要做 T7）。补单测。
- 设置 › 沙箱"检查沙箱支持"的不可用原因目前是 SDK 返回的英文。按关键字映射为中文：
  - `elevated administrator token` → "Nexus 正以管理员权限运行，桌面沙箱不可用。请以普通用户身份重新启动 Nexus。"
  - `does not provide the process security environment API` → "当前 Windows 版本不支持桌面沙箱，需要 Windows 11 24H2（build 26100.9278）或更新版本。"
- 验收：管理员身份启动和旧版本系统上，设置页给出上述中文提示；受限会话启动失败时错误信息同样可理解。

### T6 nexus 文档

- `docs/specs/desktop-sandbox-spec.md` 第 12 行：把"SDK drops write-only denials for its read-only file helper"改为"write-only protections are read-only carveouts"，并说明 PSEC 模型、系统版本要求和读权限语义。
- `windows-mxc-sandbox-risks.md`：在 R2/R3/R4/R5 下记录"新方案的处理方式 + 验证结果"。
- `windows-mxc-sandbox-verify.md`：按第 6 节更新命令，删除 `NEXUS_MXC_EXECUTABLE`。
- CHANGELOG（Unreleased）：说明 Windows 沙箱改为直接调用系统进程安全环境、不再打包 MXC 二进制、修复初始化超时（待验证后写入）。

### T7 性能后续（按实测决定是否做）

阶段 1 后每次文件 helper 操作仍是"nxs 包装进程 + nxs helper"两次进程启动加一次 PSEC 创建，但没有探测子进程。先测：

- 在 nxs 初始化前后打点，记录项目发现期间的文件操作次数和总耗时。
- 若总耗时超过 10 秒：实现只读长驻 worker——包装进程把自身 stdio 直接交给受限子进程，stdin 可以实时转发（旧方案中 `wxc-exec` 是否实时转发无法确认，这个限制已经不存在）。设计：`worker.ServeReadLoop`（4 字节长度前缀帧，复用 `decode` 校验，只接受 read/stat/lstat/read_dir/eval_symlinks）＋ nxs `--sandbox-file-read-loop` ＋ sandboxfs 侧每个 `FileSystem` 一个懒启动 worker，空闲 5 秒退出，请求取消即杀进程，文件超过 32 MiB 回退流式路径。

### T8 可选清理

- `nexus-agent-sdk-bridge` 的 `codex/desktop-sandbox-approvals` 分支只包含旧 `windowssandbox` supervisor，确认无用后删除。
- ARM64：新方案不依赖 MXC 二进制，`GOARCH=arm64` 已能编译；需要在 ARM64 设备上跑一次原生验收。

---

## 5. 已知限制（设计取舍，不在本计划范围）

- 读权限按 macOS 语义放开：受限命令可以读取用户目录等位置（Nexus 私有目录除外，它们在禁读清单里）。
- 硬链接的不同名字各自适用路径权限；卷根授权只覆盖策略构建时已存在的一级子目录（Codex 同样接受这两点）。
- 前台进程退出后，后台启动的后代会被结束（与 MXC 一致），因此"命令里启动的常驻服务"不会保留。
- 只支持 Windows 11 24H2（build 26100.9278）/ 25H2（26200.9278）及以上；不支持时 capability 关闭，受限会话无法启动。
- 提权（管理员或关闭 UAC）时受限会话不可用。
- 受限 Claude 会话在 Windows 上仍不支持。

---

## 6. Windows 验证步骤（阶段 1 起适用）

在**普通用户、非管理员**的交互终端里进行。

1. 编译 nxs（SDK `codex/desktop-sandbox-approvals` 最新提交）：
   ```powershell
   $env:GOWORK='off'
   go build -o dist\nxs\windows-amd64\nxs.exe .\cmd\nxs
   ```
2. SDK 原生测试：
   ```powershell
   $env:GOWORK='off'
   $env:NEXUS_REQUIRE_NATIVE_MXC='1'
   $env:NEXUS_NXS_EXECUTABLE=(Resolve-Path dist\nxs\windows-amd64\nxs.exe).Path
   go test -count=1 -v -run 'TestWindowsMXC|TestMXC|TestEncodePSEC' ./internal/tool/builtin/bash/sandboxexec/ ./internal/tool/builtin/file/sandboxfs/ ./internal/tool/executor/
   ```
   重点：`TestWindowsMXCProcessSecurityEnvironmentProbe`、`TestWindowsMXCFilesystemDenialIsEnforced`、`TestWindowsMXCOutsideGrantsAreReadOnly`、`TestWindowsMXCDeniedSingleFileKeepsSiblingWritable`、`TestWindowsMXCDeniedDirectoryCoversItsSubtree`、`TestWindowsMXCCancellationStopsTheSandboxedTree`、`TestWindowsMXCForegroundExitEndsDescendants`、`TestWindowsMXCDoesNotMutateHostACLs`、`TestWindowsMXCFileHelperReadsWriteProtectedProjectMetadata`。任何一项"写入被拒绝的路径成功"都是阻塞问题。
3. 再在**管理员**终端跑一次第 2 步：应统一报 `cannot run with an elevated administrator token`，不能出现任何写入成功。
4. 端到端（`make dev-nxs`，`.env` 里 `NEXUS_NXS_COMMAND_PATH` 指向第 1 步的 nxs.exe，不再需要 `NEXUS_MXC_EXECUTABLE`）：打开桌面沙箱 → 新建 DM，确认能正常对话、不再出现 `context deadline exceeded` 和 `read commands directory failed`；记录从发送第一条消息到开始回复的耗时。然后按 [windows-mxc-sandbox-verify.md](windows-mxc-sandbox-verify.md) 第 4 节的场景逐项验证（工作区内写入、`.nexus\commands` 写入被拒但可读、工作区外写入被拒、`app\data` 读取被拒、未授权域名、PowerShell、取消长命令、MCP、Full Access、Room、管理员身份）。
5. 打包（T3 完成后）：安装包里没有 `mxc` 目录，安装后行为与第 4 步一致。

---

## 7. 交接约定

- SDK 与 nexus 的 Windows 沙箱改动分别提交在上表的两个分支上；SDK 先推送，nexus 再更新 pin（T4）后推送。
- 每完成一个任务（T1–T7）单独提交，提交信息说明对应的任务编号。
- 阶段 2 完成的标准：T1–T6 合入；第 6 节第 2–5 步在 Windows 11 24H2+ 普通用户下全部通过；T7 视第 6 节第 4 步的实测耗时决定。
