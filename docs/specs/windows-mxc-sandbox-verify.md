# Windows MXC 沙箱：验证清单（2026-10-27）

> **2026-10-27 更新**：本清单基于旧的 `wxc-exec` 方案，其中 `NEXUS_MXC_EXECUTABLE` 等环境变量已不再使用。当前的验证步骤以 [windows-sandbox-psec-plan.md](windows-sandbox-psec-plan.md) 第 6 节为准；本文第 4 节的端到端场景仍然适用。

> 风险说明和修复方案见 [windows-mxc-sandbox-risks.md](windows-mxc-sandbox-risks.md)。

## 0. 代码版本

| 仓库 | 分支 | 提交 |
|---|---|---|
| nexus | `codex/windows-mxc-sandbox` | `700995d18` 或更新 |
| nexus-agent-sdk-go | `codex/desktop-sandbox-approvals` | `e8bad7a5`（固定 pin；尚未进入 `origin/main`） |
| nexus-agent-sdk-bridge | 不用改 | nexus 沿用 main 上的 pin |

两个提交都已推送；Windows 发布流水线必须显式 checkout 并编译 e8 pin，不能用未验证的 `nxs-stable` 替代。

**本轮改动：**
- 修复 `project: read commands directory failed … Access is denied`：只读文件 helper 在 Windows 上去掉写拒绝，命令仍然不能写 `.nexus/commands`；项目元数据读到权限错误时直接报错，不再静默跳过。
- 删除 SDK 里旧的 machine-Host / NUL / AppContainer 后端和 `nxs-sandbox-system*`，MXC 是唯一的后端。
- MXC 目录拒绝只补**一个**结尾反斜杠（原来 `f4f0789c` 补了两个）。
- **提权令牌下 MXC 直接报不可用**（SDK），nexus 受限会话也会拒绝启动。**所有验证都必须在普通用户、非管理员的交互终端里做**；SSH / 管理员终端只用来验证“会被拒绝”。
- Windows 受限会话打开 MCP 强制约束（StrictConfig + MCP 网络 / helper / stdio）。

## 1. 重新编译 nxs（必须）

旧的 `nxs.exe` 没有这次的修复，必须重新编译：

```powershell
cd <SDK checkout>   # codex/desktop-sandbox-approvals @ e8bad7a5
$env:GOWORK='off'
go build -o dist\nxs\windows-amd64\nxs.exe .\cmd\nxs
```

nexus worktree 的 `.env` 里，`NEXUS_NXS_COMMAND_PATH` 指向这个新的 `nxs.exe`，`NEXUS_MXC_EXECUTABLE` 指向 `wxc-exec.exe`。

## 2. SDK 原生 MXC 测试（普通用户终端）

```powershell
cd <SDK checkout>
$env:GOWORK='off'
$env:NEXUS_REQUIRE_NATIVE_MXC='1'
$env:NEXUS_MXC_EXECUTABLE='E:\Code\nexus\.local\mxc-sdk-1.0.0\extract\bin\x64\wxc-exec.exe'
$env:NEXUS_NXS_EXECUTABLE=(Resolve-Path dist\nxs\windows-amd64\nxs.exe).Path
go test -count=1 -v -run 'TestWindowsMXC|TestMXC|TestOperationConfig' ./internal/tool/builtin/bash/sandboxexec/ ./internal/tool/builtin/file/sandboxfs/ ./internal/tool/executor/
go test -count=1 ./internal/environment/workspace/ ./internal/environment/filesystem/worker/
```

重点看下面几项，上次在 SSH 管理员令牌下都失败了：
- `TestWindowsMXCFilesystemDenialIsEnforced`、`TestWindowsMXCOutsideGrantsAreDefaultDenied`、`TestWindowsMXCDeniedSingleFileKeepsSiblingWritable`：普通用户下必须通过。**如果普通用户下仍然能写入被拒绝的路径，就是安全边界失效，最高优先级。**
- `TestWindowsMXCCreatedProtectedFileIsReportedAsViolation`：路径拼两次的问题应该已经修好。
- `TestWindowsMXCPowerShellCommandUsesEncodedArguments`：交互会话里不应再出现 `0xC0000142`。
- `TestMXCRecursiveDenyPathAddsOneSeparatorOnWindows`、`TestMXCDenyPolicyExistingDirectoryIsDeniedItself`：目录拒绝后面只有一个 `\`。
- 再在**管理员**终端跑一次：MXC 测试应该统一报 `MXC sandbox cannot run with an elevated administrator token`，不应出现任何“写入成功”。

## 3. nexus 门禁

```powershell
cd <nexus worktree>
$env:GOWORK='off'
$env:NEXUS_SANDBOX_SDK_SOURCE='<SDK checkout>'
$env:NEXUS_ALLOW_LOCAL_NATIVE_EVIDENCE='1'
$env:NEXUS_MXC_EXECUTABLE='E:\Code\nexus\.local\mxc-sdk-1.0.0\extract\bin\x64\wxc-exec.exe'
node scripts/desktop/check-windows-sandbox.mjs --native
go test ./internal/runtime/clientopts/ ./internal/app/...
nexus-server check-desktop-runtime <新的 nxs.exe 绝对路径>
```

`check-desktop-runtime` 的三个 profile 都要握手成功。在 Windows 上它们现在也会要求 MCP 相关能力。

## 4. 端到端（`make dev-nxs`，普通用户终端）

设置 › 沙箱：打开桌面沙箱 → 点“检查沙箱支持”，应该显示可用。然后新建 DM 会话：

1. **原报错**：工作区放一个 `.nexus\commands\hello.md`，新建会话不应再报 `read commands directory failed`，`/hello` 能补全出来。再把 `.nexus\commands` 删掉，新建会话也必须正常。
2. 让 agent 在工作区里写文件：应成功。
3. 让 agent 往 `.nexus\commands\x.md` 写：应被拒绝，或者报违规；写完 `cat`/`type` 读不出来也是预期的（MXC 读写同拒）。
4. 往工作区外写文件：应被拒绝并报违规。
5. 读 `C:\Users\zigen\.nexus-dev\app\data`：应被拒绝。
6. 访问未授权的域名：应走审批或被拒绝。
7. PowerShell 命令：能正常运行。
8. 取消一个长命令（`ping -n 60 127.0.0.1`）：进程树会被清掉。
9. 配一个 stdio MCP 和一个 HTTP MCP：受限会话能连上，文件 / 网络访问受限；headers helper 不会被直接拒绝。
10. Full Access 下重复 2–5：都不受限制。
11. 至少开一个 Room 试一次命令。
12. **以管理员身份运行 Nexus**：受限会话应该报 “Windows 桌面沙箱不能在管理员权限下运行”，“检查沙箱支持”显示不可用；Full Access 不受影响。

## 5. Windows 机器上的清理

- 删除旧的计划任务：`Unregister-ScheduledTask -TaskName NexusMXCInteractiveTest -Confirm:$false`
- 可以删掉：`E:\Code\nexus\.local\windows-mxc.bundle`、`nexus-server-mxc.exe`、`mxc-interactive.*`
- `E:\Code\nexus\nexus-agent-sdk-go` 本地 `main` 上未推送的 `d74924d9` 已经过时，可以丢弃。

## 6. 已知限制（不在本轮范围内）

- MXC 的 deniedPaths 读写同拒：沙箱里的命令读不了 `.nexus/commands` 这类禁写目录。
- 准备期还不存在的受保护路径，只能在命令结束后复核并报违规，不能事前拦截。
- SDK 的 `.github/workflows/desktop-sandbox.yml` 改成了用临时普通账户跑 MXC 测试，还没在 GitHub 上实际跑过。
- 系统兼容性（哪些 Win10/Win11 版本支持 base-container）、MXC 再分发许可和签名、ARM64：发版前确认。
