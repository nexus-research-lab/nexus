# Windows PSEC 沙箱验证清单

这是原生验收步骤，不代表已通过。普通用户、非管理员，Windows 11 24H2 build 26100.9278 / 25H2 build 26200.9278 或更新受支持版本。

## 0. 代码版本

Nexus 使用 `codex/windows-mxc-sandbox`；SDK 固定到 `622ec03c205ab38dac1b6b9be3503447d784601d`；bridge 保持 go.mod 的远程 pin。SDK checkout 必须干净，三处 Nexus pin 必须一致。

## 1. 重新编译 nxs（必须）

旧的 `nxs.exe` 没有这次的修复，必须重新编译：

```powershell
cd <SDK checkout>   # codex/desktop-sandbox-approvals @ 622ec03c
$env:GOWORK='off'
go build -o dist\nxs\windows-amd64\nxs.exe .\cmd\nxs
```

nexus worktree 的 `.env` 里，`NEXUS_NXS_COMMAND_PATH` 指向这个新的 `nxs.exe`。

## 2. SDK 原生 MXC 测试（普通用户终端）

```powershell
cd <SDK checkout>
$env:GOWORK='off'
$env:NEXUS_REQUIRE_NATIVE_MXC='1'
$env:NEXUS_NXS_EXECUTABLE=(Resolve-Path dist\nxs\windows-amd64\nxs.exe).Path
go test -count=1 -v -run 'TestWindowsMXC|TestMXC|TestEncodePSEC' ./internal/tool/builtin/bash/sandboxexec/ ./internal/tool/builtin/file/sandboxfs/ ./internal/tool/executor/
go test -count=1 ./internal/environment/workspace/ ./internal/environment/filesystem/worker/
```

重点看下面几项，上次在 SSH 管理员令牌下都失败了：
- `TestWindowsMXCFilesystemDenialIsEnforced`、`TestWindowsMXCOutsideGrantsAreReadOnly`、`TestWindowsMXCDeniedSingleFileKeepsSiblingWritable`：普通用户下必须通过。**如果普通用户下仍然能写入被拒绝的路径，就是安全边界失效，最高优先级。**
- `TestWindowsMXCCreatedProtectedFileIsReportedAsViolation`：路径拼两次的问题应该已经修好。
- `TestWindowsMXCPowerShellCommandUsesEncodedArguments`：交互会话里不应再出现 `0xC0000142`。
- `TestWindowsMXCDeniedDirectoryCoversItsSubtree`、`TestWindowsMXCDoesNotMutateHostACLs`、`TestWindowsMXCForegroundExitEndsDescendants`：目录拒绝、ACL 不变、前台退出后后代清理必须通过。
- 再在**管理员**终端跑一次：MXC 测试应该统一报 `MXC sandbox cannot run with an elevated administrator token`，不应出现任何“写入成功”。

## 3. nexus 门禁

```powershell
cd <nexus worktree>
$env:GOWORK='off'
$env:NEXUS_SANDBOX_SDK_SOURCE='<SDK checkout>'
$env:NEXUS_ALLOW_LOCAL_NATIVE_EVIDENCE='1'
node scripts/desktop/check-windows-sandbox.mjs --native
go test ./internal/runtime/clientopts/ ./internal/app/...
nexus-server check-desktop-runtime <新的 nxs.exe 绝对路径>
```

`check-desktop-runtime` 的三个 profile 都要握手成功。在 Windows 上它们现在也会要求 MCP 相关能力。

## 4. 端到端（`make dev-nxs`，普通用户终端）

设置 › 沙箱：打开桌面沙箱 → 点“检查沙箱支持”，应该显示可用。然后新建 DM 会话：

1. **原报错**：工作区放一个 `.nexus\commands\hello.md`，新建会话不应再报 `read commands directory failed`，`/hello` 能补全出来。再把 `.nexus\commands` 删掉，新建会话也必须正常。
2. 让 agent 在工作区里写文件：应成功。
3. 让 agent 往 `.nexus\commands\x.md` 写：应被拒绝，或者报违规；已有元数据用 `cat`/`type` 应仍可读。
4. 往工作区外写文件：应被拒绝并报违规。
5. 读 `C:\Users\zigen\.nexus-dev\app\data`：应被拒绝。
6. 访问未授权的域名：应走审批或被拒绝。
7. PowerShell 命令：能正常运行。
8. 取消一个长命令（`ping -n 60 127.0.0.1`）：进程树会被清掉。
9. 配一个 stdio MCP 和一个 HTTP MCP：受限会话能连上，文件 / 网络访问受限；headers helper 不会被直接拒绝。
10. Full Access 下重复 2–5：都不受限制。
11. 至少开一个 Room 试一次命令。
12. **以管理员身份运行 Nexus**：受限会话应该报 “Windows 桌面沙箱不能在管理员权限下运行”，“检查沙箱支持”显示不可用；Full Access 不受影响。

## 5. 打包与性能

构建安装包并检查无 MXC 目录，安装后重复第 4 节。记录系统 build、架构、令牌、nxs SHA256、初始化总耗时和项目发现文件操作次数。T7 的长驻只读 worker 仅在项目发现超过 10 秒时另行实施；当前没有原生测量结果。

## 6. 未完成验收

- Windows 普通用户安全边界、实际端到端启动、安装包和 ARM64 验收尚未执行。
- 准备期不存在的受保护目标执行后复核不等于事前阻止副作用。
- CI 缺 API、required 测试 skip 或未执行均不得算通过。
