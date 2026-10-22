# Windows MXC 沙箱：交接说明（2026-10-21）

目标：Windows 上本地 `make dev` / `make dev-nxs` 能跑起来，打开桌面沙箱后命令真正经由 SDK 的 MXC ProcessContainer 执行，且不影响 macOS 和其他功能。**暂不发版**；nexus / SDK / Bridge 下次一起发，所以现在可以继续指向开发分支或用 go.work 联调。

> 本文件用于交接当前开发状态；它不代表 Windows MXC 已完成发布验收。

---

## 1. 分支现状

### nexus（主仓库）

| 分支 | 位置 | 状态 |
|---|---|---|
| **`codex/windows-mxc-sandbox`**（新，干净） | Mac: `~/program/Work/Nexus/worktrees/nexus/windows-mxc`；Windows: `E:\Code\nexus\worktrees\windows-mxc\nexus`（通过 git bundle 传过来） | 基于 `origin/main` `32f23b0c6`，原有功能提交 2 个；本交接修订提交后共 3 个，**未推送到 origin** |
| `codex/desktop-sandbox-approvals`（旧） | Mac: `worktrees/nexus/desktop-sandbox`；Windows: `E:\Code\nexus\worktrees\desktop-sandbox\nexus` | **不要再往上加东西，也不要合并**，只当参考 |
| `codex/windows-development` | — | 已合进旧分支并删除（本地+远端） |

新分支的 2 个提交：

1. `a5cc32096` :sparkles: Add native Windows development commands —— 从旧的 `24d324ee6` cherry-pick：`scripts/dev-windows.ps1`、`dev-windows-job.cs`、`test-dev-windows.ps1`、makefile 的 Windows 分支、README。
2. `955b6bc27` :sparkles: Enable the Windows desktop sandbox through the SDK MXC backend —— `internal/runtime/clientopts/`：
   - 删掉 `desktopSandboxDeferredOn`，`applyDesktopSandboxForPlatform` 接受 `windows`。
   - Windows + nxs + Full Access：不下发任何 Sandbox 设置，带 resources 报错。
   - Windows + nxs 受限：必须有宿主 scratch lease（`SandboxResources`），走与 macOS 相同的 nxs 合同，但 MCP network/helpers/stdio 三个 Require 保持 macOS-only。
   - Windows 受限 Claude：报错（沿用原逻辑）。
   - 网络准入：仅 Windows Full Access 提前返回；macOS 行为不变（有回归测试 `TestDesktopSandboxNetworkAdmissionStillAppliesToDarwinFullAccess`）。
   - spec（`docs/specs/desktop-sandbox-spec.md`）和 CHANGELOG 已更新。

**为什么放弃旧分支**：旧分支合入过重写前的旧 main，与 `origin/main` 的 diff 有 963 个文件，其中 771 个是旧版本，合并会回滚 main 上 296 个提交（含 pgx/x/text 安全修复退回）；另外它带了约 2600 行 Go 的旧 machine-Host supervisor（`windows_sandbox_*`、`storage/sandbox/windows_*`、migrations 00159–00161、WiX 包），MXC 不需要。新分支**一个都没带**。

### SDK（nexus-agent-sdk-go）

- **以远端 `codex/desktop-sandbox-approvals` @ `633e5a6e` 为准**。Windows 检出：`E:\Code\nexus\worktrees\desktop-sandbox\nexus-agent-sdk-go`。
- nxs 已用它构建：`E:\Code\nexus\worktrees\desktop-sandbox\nexus-agent-sdk-go\dist\nxs\windows-amd64\nxs.exe`。
- `E:\Code\nexus\nexus-agent-sdk-go` 本地 `main` 上有一个**未推送**的 `d74924d9`（"Complete Windows MXC sandbox integration"）。它比 `633e5a6e` **少** 1203 行（缺 `windows_mxc_policy.go`、违规上报、相关测试），已过时，不要用；确认无用后可以丢弃。
- SDK 分支里仍有大量旧 machine-Host/NUL/journal 代码（`NEXUS_WINDOWS_SANDBOX_BACKEND=legacy` 才走），发版前应清理，但不影响本地 dev。

### Bridge（nexus-agent-sdk-bridge）

- **不需要改**。nexus 新分支沿用 main 的 Bridge pin。Bridge 的 `codex/desktop-sandbox-approvals` 多出的 5 个提交全是旧 machine-Host supervisor（`windowssandbox/` 包），MXC 用不到。

### nexus-control

- Windows 上检出在 `codex/windows-development` @ `64bd20c`（Windows SQLite/权限修复），已推送；`make dev` 依赖它。

---

## 2. Windows 环境要点

- `E:\Code\nexus\go.work` 会把模块解析劫持到 `E:\Code\nexus\{nexus,…}`，在 worktree 里跑 go 命令要 `GOWORK=off`（或改 go.work 指向 worktree）。
- 私有模块：`GOPRIVATE=github.com/nexus-research-lab/*`；网络走 FlClash：`HTTPS_PROXY=http://127.0.0.1:7890`。
- MXC sidecar：`E:\Code\nexus\.local\mxc-sdk-1.0.0\extract\bin\x64\wxc-exec.exe`（同目录有 `mxc_ffi.dll`、`winhttp-proxy-shim.exe`）。nxs 通过 `NEXUS_MXC_EXECUTABLE` 找它；nexus 不会从 runtime 环境里清掉这个变量。`wxc-exec.exe --probe` 已确认 `tier=base-container`、支持 deny/enumerate paths。
- `make dev-nxs` 建议的 `.env`（worktree 根目录，已被 gitignore）：
  ```
  NEXUS_STATE_ROOT=C:/Users/zigen/.nexus-dev
  NEXUS_AGENT_RUNTIME_KIND=nxs
  NEXUS_NXS_COMMAND_PATH=E:/Code/nexus/worktrees/desktop-sandbox/nexus-agent-sdk-go/dist/nxs/windows-amd64/nxs.exe
  NEXUS_MXC_EXECUTABLE=E:/Code/nexus/.local/mxc-sdk-1.0.0/extract/bin/x64/wxc-exec.exe
  ```
  worktree 不在 `E:\Code\nexus\nexus` 下，`../nexus-control` 找不到，需要 `make dev NEXUS_CONTROL_ROOT=E:/Code/nexus/nexus-control`（或在 .env 里设）。

---

## 3. 已验证

- Mac：新分支 `go build ./...`、`go vet ./...`、`GOOS=windows go vet ./...`、`go test ./...` 全部通过。
- Windows（SSH，session 0 / High 完整性）：
  - `scripts/test-dev-windows.ps1` 通过。
  - `internal/app/...` 通过。
  - SDK MXC 原生测试只有部分通过：网络放行/拒绝、取消、文件 helper、MCP stdio 等路径有通过证据；文件隔离、路径复核和 PowerShell 测试在该 SSH 管理员会话下失败，不能作为 Windows 隔离验收。

---

## 4. 待办（按优先级）

整体评估：用 MXC 是正确方向（隔离交给操作系统，不需要跨重启的恢复状态，和 macOS 共用同一套 clientopts 合同）。SDK 侧已有命令/文件/网络策略实现，Nexus 已完成初步 wiring；但 Windows Nexus↔Bridge↔nxs 的真机验收尚未完成。要算“比较完整”，还差：执行边界的普通用户会话验证（第 2 项）、提权检测、Windows 宿主路径保护、Windows MCP 强制要求（P0.5），以及 CI MXC 门禁、系统兼容性和分发许可。

### P0：让 dev 端到端能用

1. **`check-desktop-runtime` 只允许 darwin**：`internal/app/runtimecheck/check.go:35` 写死 `runtime.GOOS != "darwin"` 就报 "native macOS runtime compatibility check required"。改成允许 windows，然后跑：
   `nexus-server check-desktop-runtime --nxs <nxs.exe>`，分别在**没设**和**设了** `NEXUS_MXC_EXECUTABLE` 时跑：前者应 fail-closed，后者三个 profile（workspace-write / read-only / full-access）都应初始化成功。这是 nexus↔Bridge↔nxs(MXC) 真实握手的最小验证。
2. **在交互桌面会话（普通 Medium 权限，不要管理员终端）重跑 SDK MXC 原生测试**：
   ```
   cd E:\Code\nexus\worktrees\desktop-sandbox\nexus-agent-sdk-go
   $env:GOWORK='off'; $env:NEXUS_REQUIRE_NATIVE_MXC='1'
   $env:NEXUS_MXC_EXECUTABLE='E:\Code\nexus\.local\mxc-sdk-1.0.0\extract\bin\x64\wxc-exec.exe'
   $env:NEXUS_NXS_EXECUTABLE=(Resolve-Path dist\nxs\windows-amd64\nxs.exe).Path
   go test -count=1 -v -run TestWindowsMXC ./internal/tool/builtin/bash/sandboxexec/ ./internal/tool/builtin/file/sandboxfs/ ./internal/tool/executor/
   ```
   在 SSH（session 0、High 完整性管理员令牌）下这 5 个失败：
   - `TestWindowsMXCFilesystemDenialIsEnforced`、`TestWindowsMXCOutsideGrantsAreDefaultDenied`、`TestWindowsMXCDeniedSingleFileKeepsSiblingWritable`：**拒绝/授权外的写入都成功了** —— 怀疑是提权令牌绕过，必须在 Medium 会话确认；若仍失败就是 SDK 的真 bug（安全边界失效），最高优先。
   - `TestWindowsMXCCreatedProtectedFileIsReportedAsViolation`：路径被拼了两次（`…\.nexus-mxc-test-N\.nexus-mxc-test-N\.bashrc`），像是相对路径解析 bug，与会话无关。
   - `TestWindowsMXCPowerShellCommandUsesEncodedArguments`：`STATUS_DLL_INIT_FAILED (0xC0000142)`，Win32k 初始化失败，可能是 session 0 没有桌面导致；交互会话里再看。
3. **宿主路径保护在 Windows 上没生效**（具体做法见下面 P0.5-B）：`internal/runtime/clientopts/desktop_host_paths.go:21` 的 `applyDesktopHostPaths` 只对 `darwin` 生效。Windows 受限会话目前不会给 nexus 状态目录（`NEXUS_STATE_ROOT`、`app/data` 数据库、`NexusSidecar.pid.json`）加 DenyRead/DenyWrite。MXC 是 grant 以外默认拒绝，所以状态目录不在授权里时理论上已拒绝，但若工作区/授权目录与状态目录重叠就会漏。需要决定：Windows 也套用同样的拒绝规则（注意 Windows 路径格式，Claude 的 `Edit(/…)` 规则对 `C:\` 路径不适用，但 Windows 上 Claude 受限本来就被拒绝）。
4. **`TestDesktopHostPathsPreserveAliasesAndFullAccess` 在 Windows 上失败**（`desktop_host_paths_test.go:44` "Claude record write rule missing"）。测试显式传 `"darwin"` 但用 Windows 路径，`"Edit(/"+record+")"` 拼出来是 `Edit(/C:\…)`。先在 `E:\Code\nexus\nexus`（origin/main）上跑同一测试确认是否 main 原本就挂；大概率是 main 已有的平台问题，修测试即可（Windows 上跳过或按平台构造路径）。
5. **`make dev-nxs` 实机走一遍**（交互桌面、普通终端）：设置 › 沙箱 打开桌面沙箱 → 新建 DM 会话 → 让 agent 执行：在工作区内写文件（应成功）、在工作区外写文件（应被拒并报违规）、读 `C:\Users\zigen\.nexus-dev\app\data`（应被拒）、访问未授权域名（应走审批/拒绝）、PowerShell 命令（应能运行）、取消长命令（进程树应被清掉）。Full Access 下同样操作应不受限。Room 和后台记忆维护也走同一个 builder，至少开一个 Room 试一次命令。

### P0.5：与 macOS 防护层对齐（做完才算"比较完整"）

这三项做完，Windows 与 macOS 在防护层面基本对齐。

A. **提权检测**：在 SSH 管理员（High 完整性）令牌下，MXC 的写拒绝与授权外默认拒绝都失效了（见第 2 项）。即使普通 Medium 会话下正常，也说明用户"以管理员身份运行 Nexus"时沙箱可能形同虚设。
   - 在 nexus 侧（推荐在 `internal/runtime/clientopts/desktop_sandbox.go` 的 Windows 受限分支，或 App 启动时一次性检测后注入到配置）检测当前进程令牌是否提权（`windows.GetCurrentProcessToken().IsElevated()`，或查 `TokenElevation`/完整性级别 ≥ High）。
   - 提权时：受限会话**拒绝启动**并返回明确错误（如"桌面沙箱不能在管理员权限下运行，请以普通权限启动 Nexus"），不得静默降级；Full Access 不受影响。
   - 设置 › 沙箱 的诊断里也展示这一状态（可以复用 nxs `SandboxBackendStatus` 的 unsupported/missing_dependencies 投影，或新增一个 nexus 侧原因）。
   - SDK 侧也建议在 MXC `--probe`/状态诊断里报告提权，双保险。
   - 测试：用可注入的 "isElevated" 函数做单测；真机在管理员终端和普通终端各跑一次 `check-desktop-runtime`。
   - 若第 2 项在 Medium 会话下也失败，这一项就不是"加固"而是阻塞 bug，先修 SDK。

B. **宿主路径保护扩展到 Windows**（即上面第 3 项，提升为必做）：
   - `internal/runtime/clientopts/desktop_host_paths.go` 的 `applyDesktopHostPaths` 去掉 darwin-only 限制，对 Windows 受限 nxs 会话同样给 `NEXUS_STATE_ROOT`、App 数据目录（数据库）、`NexusSidecar.pid.json` 等加 DenyRead/DenyWrite。
   - 注意 Windows 路径：大小写不敏感、`\\?\` 前缀、8.3 短名、junction/符号链接别名都要归一（macOS 版本已处理 alias，参考其做法并用 `filepath.EvalSymlinks` + `GetFinalPathNameByHandle`）。
   - MXC 的 `deniedPaths` 对"已存在目标"有效；状态目录一定已存在，所以这层拒绝是事前拦截。
   - Claude 的 `Edit(/…)`/`Read(/…)` 规则只用于 macOS（Windows 受限 Claude 本来被拒绝），不要为 Windows 生成。
   - 修 `TestDesktopHostPathsPreserveAliasesAndFullAccess` 在 Windows 上的失败（第 4 项），并新增 Windows 平台的用例：挂载目录与状态目录重叠时，状态目录仍被拒绝。

C. **MCP 强制要求在 Windows 上打开**：
   - 现在 `desktop_sandbox.go` 里 `RequireMCPNetwork / RequireMCPHelpers / RequireMCPStdio` 和 `options.MCP.StrictConfig` 只在 `platform == "darwin"` 时为 true。Windows 上用户配置的 HTTP/SSE MCP、helper 进程不受强制约束。
   - SDK 已有 `TestWindowsMXCMCPStdioUsesTheCommandPolicy`（MCP stdio 走 MXC 命令策略，已通过）。先确认 SDK `633e5a6e` 在 Windows 上是否**声明**了对应能力（`sandbox_mcp_network_v1` / helpers / stdio 一类，看 SDK `docs/sdk/sandbox.md` 的 per-area capabilities 和 nxs initialize 返回）。
   - 能力齐备的那几项：在 nexus 对 Windows 也设为 Require，并打开 `MCP.StrictConfig`；同时 `RejectDesktopSandboxTypedMCPServersWithNetworkAdmission` 的平台判断一并放开。
   - SDK 尚未支持的项：在 SDK 补实现（MCP HTTP/SSE 走 Nexus 网络代理、helper 进程进 MXC），不要在 nexus 侧假装已满足。
   - 测试：`check-desktop-runtime` 三个 profile 都能完成握手；配置一个 stdio MCP 和一个 HTTP MCP，在受限会话里验证其文件/网络访问受限。

### P1：CI 与验收门禁

A. **更新 Windows CI 的 MXC 门禁**：当前 `.github/workflows/windows-desktop-build.yml` 和 `scripts/desktop/check-windows-sandbox.mjs` 仍固定旧 SDK commit `38221838946f...`，并验证旧 machine-host / settings-journal 路径；它们没有验证 `wxc-exec.exe --probe`、MXC filesystem/network policy 或 Windows nxs 三 profile handshake。将 CI 改为使用包含 MXC 的 SDK commit，加入 sidecar 获取/探针、`NEXUS_MXC_EXECUTABLE`、受限/Full Access handshake 和原生 MXC 测试；在此完成前，CI 通过不能证明 Windows MXC 已验收。

### P1：测试与清理

B. Windows 上 main 原本就会挂的测试（**不是回归**，别被误导）：`internal/runtime` 的 10 个 `TestSandboxProcess*/TestSupervisedReceipt*/TestProcessRecovery*`、`internal/service/dm` 的 `TestConversationForkPreparesAndMaterializesIndependentHistory`、`internal/service/nxsruntime` 编译失败、`internal/service/session` 和 `internal/service/room/realtime` 一批（文件锁 / 0666 权限）。可以顺手修，但和 MXC 无关，建议单独提交。
C. 清理：Windows 上我注册过一个计划任务 `NexusMXCInteractiveTest`（无触发器，只手动运行，用来在交互会话里跑上面的 MXC 测试；没跑成），确认不要了就删：`Unregister-ScheduledTask -TaskName NexusMXCInteractiveTest -Confirm:$false`。另外 `E:\Code\nexus\.local\windows-mxc.bundle`、`nexus-server-mxc.exe`、`mxc-interactive.*` 都可以删。

### P2：下次三仓一起发版前

8. SDK：把 MXC 合进 SDK main，`release-nxs` 打包 `mxc/bin/x64/{wxc-exec.exe,mxc_ffi.dll,winhttp-proxy-shim.exe}`；删除 SDK 里的 legacy machine-Host/NUL/journal 代码（或至少确认默认路径不可达）。
9. nexus：确认 Windows 桌面安装包能在 nxs 旁边找到 `mxc/bin/x64/wxc-exec.exe`（SDK 解析顺序：`NEXUS_MXC_EXECUTABLE` → nxs 同目录 `mxc/bin/x64/`）；`check-windows-sandbox.mjs` 的 release 模式接上 MXC 验收。
10. 推送 `codex/windows-mxc-sandbox` 并开 PR；Bridge/SDK 用正式 tag。
11. 旧分支：nexus 和 Bridge 的 `codex/desktop-sandbox-approvals` 在确认没有需要的东西后删除（nexus 旧 worktree 里还有一个未提交的 `desktop_sandbox_integration_test.go` +6 行，先看一眼）。**SDK 的同名分支是 MXC 的来源，合进 SDK main 之前不能删。**
12. **系统兼容性**：确认 MXC base-container 在哪些 Windows 版本/版次上可用（测试机是 build 26300），主流 Win10/Win11 不支持时：设置 › 沙箱 明确显示"此系统不支持"，并且在偏好已开的情况下给出可理解的错误，而不是会话神秘地启动失败。
13. **分发与许可**：确认微软 MXC SDK 的再分发许可、`wxc-exec.exe` / `mxc_ffi.dll` / `winhttp-proxy-shim.exe` 的签名要求、安装包体积；Windows 打包流程（`.github/workflows/windows-desktop-build.yml`）把它们放到 nxs 旁的 `mxc/bin/x64/`。
14. **ARM64**：现在回退到 x64 sidecar 走模拟，没测过；至少在 ARM64 机器上跑一次 `check-desktop-runtime` 和 MXC 原生测试。

---

## 4.1 已知限制（写进 spec，不要求本轮解决）

- **尚不存在的受保护路径只有条件地可事前拒绝**：如果最近存在祖先严格位于授权范围内且不会吞掉其他授权，SDK 会把该祖先加入 MXC deny；无法安全表达为 concrete deny path 时，才记录到执行计划并在命令结束后 `Lstat` 复核，文件可能已经被创建（如 `.bashrc`、`.git/hooks/*`）。比 macOS Seatbelt 弱。
- **write-only 拒绝被加强为读写都拒绝**（MXC v1 只有一个 deniedPaths 集合），可能比预期更严。
- **nxs 进程本身不在容器里**：MXC 只隔离 nxs 发起的命令/文件 helper；nxs 进程靠 Bridge 的 kill-on-close Job 兜底。没有 macOS 那套 sidecar 所有权、进程监管和 scratch 崩溃恢复；崩溃残留的 scratch 靠通用的过期清理。
- **只支持 nxs**：Windows 上受限 Claude 会话直接拒绝。
- **命令里的 SOCKS / MITM 代理设置不支持**（MXC 下 fail-closed）。
- **PowerShell 需要放开 Win32k 控制台子系统**才能启动（剪贴板和输入注入仍禁止）。

---

## 5. 同步代码

新分支只在 Mac 本地和 Windows worktree 有（Windows 的是 bundle 导入的同一提交 `955b6bc27`）。Windows 上直接在 `E:\Code\nexus\worktrees\windows-mxc\nexus` 继续提交即可；要回传 Mac 时推到 origin 的同名分支。
