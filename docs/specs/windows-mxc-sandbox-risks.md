# Windows MXC 沙箱：已知风险与修复方案

> **2026-10-27 更新**：Windows 沙箱已改为直接调用系统进程安全环境（不再使用 `wxc-exec`），R2（ACL/进程残留）、R3（目录拒绝后缀）、R4（重复探测）、R5（命令行长度）已在新方案中按设计处理，但仍需在 Windows 上验证。开发计划与后续任务见 [windows-sandbox-psec-plan.md](windows-sandbox-psec-plan.md)。

本文记录 2026-10-27 静态复核 Windows 桌面沙箱代码后发现的风险，以及每项风险建议的验证方法和修复方式。代码版本：

| 仓库 | 分支 | 提交 |
|---|---|---|
| nexus | `codex/windows-mxc-sandbox` | `024f6f472` |
| nexus-agent-sdk-go | `codex/desktop-sandbox-approvals` | `e8bad7a5` |

结论：功能代码已经齐全（命令、文件 helper、网络、MCP、提权检测、打包），旧的 machine-Host 后端也已清理；但 Windows 原生验收、端到端场景、取消和崩溃后的清理都还没验证。开发基本完成，验证还没开始。能否交付，主要取决于 R1–R3 的验证结果。

复核中确认没问题的部分：

- Windows 命令路径只走 MXC；sidecar 缺失、探测失败或提权令牌时 fail closed，不会退回无沙箱执行。
- 网络：nexus 下发显式空域名清单，出站默认拒绝，只能经 Nexus 代理；继承的代理环境变量会被清掉，SOCKS/MITM 直接拒绝。
- token、secret 和 `NEXUS_*` 变量不会进入 `wxc-exec` 命令行。
- 工作区（`<state>\users\...`）和受保护的 app 根（`<state>\app`）不重叠，宿主路径保护不会把工作区一起拒掉；授权以外的路径默认拒绝，其他用户和 Agent 的工作区访问不到。
- 安装包里 MXC 的位置（`Resources\bin\mxc\bin\x64`）、C# sidecar 注入的 `NEXUS_MXC_EXECUTABLE`、SDK 的查找顺序三者一致。
- Windows 上会正常获取宿主 scratch 租约。

---

## R1 安全边界还没在普通用户下验证（阻塞项）

**现象**：交接时在 SSH 管理员（High 完整性）令牌下，`TestWindowsMXCFilesystemDenialIsEnforced`、`TestWindowsMXCOutsideGrantsAreDefaultDenied`、`TestWindowsMXCDeniedSingleFileKeepsSiblingWritable` 都失败了，被拒绝的路径和授权以外的路径都能写进去。现在 SDK 和 nexus 在提权令牌下都会拒绝，但普通用户（Medium 完整性）下 MXC 是否真的拦得住，还没有实测证据。

**验证**：在普通用户的交互终端里跑 SDK 原生测试（命令见 [windows-mxc-sandbox-verify.md](windows-mxc-sandbox-verify.md) 第 2 节），上面三项必须通过。

**如果普通用户下仍然失败**：

1. 用 `wxc-exec.exe --config-base64 <配置> -- cmd /c echo X> <被拒路径>` 单独复现，排除 SDK 适配层的问题。
2. 检查 `--probe` 输出：`tier` 是否为 `base-container`，`needsDaclAugmentation` 是否为 false。若需要 DACL augmentation，SDK 现在会直接拒绝，不会进入执行。
3. 检查目标路径是否在某个 `readwritePaths` 授权之内。MXC 对"授权内的拒绝"是否生效，是 `TestWindowsMXCDeniedSingleFileKeepsSiblingWritable` 要回答的问题。如果 MXC 不支持在授权内部再拒绝，`mxcFilesystemDenyPolicy` 必须改为 fail closed：授权内部存在受保护路径时，拒绝启动，而不是只下发 `deniedPaths`。
4. 如果确认是 MXC 本身的缺陷，Windows 受限会话必须保持不可用，并上报微软。不能在 nexus 侧用路径检查代替操作系统边界。

---

## R2 取消或崩溃后可能残留进程和 ACL

> 当前 PSEC 处理：新方案不修改宿主 ACL；包装进程持有 kill-on-close Job。已有 ACL 不变、取消和前台退出后后代清理的原生测试，尚待 Windows 执行。 以下保留旧方案风险分析（历史，non-normative）。

**现象**：

- 取消命令时，`internaltool.CancelProcessTree` 用 `taskkill /PID <wxc-exec> /T /F` 按父进程树强杀。ProcessContainer 里的工作进程不一定是 `wxc-exec` 的子进程，可能杀不到。
- `TestWindowsMXCCancellationStopsSidecar` 只检查了 `wxc-exec` 是否在 5 秒内返回，没有确认容器里的 `ping` 是否真的退出了。
- MXC schema 里有 `filesystem.clearPolicyOnExit`（默认"执行结束后移除文件访问策略"），说明 MXC 执行期间会修改真实的文件 ACL。用 `/F` 强杀 `wxc-exec`，或 nxs、整个 App 崩溃时，这些 ACL 改动可能来不及恢复，残留在工作区和授权目录上。交接文档里"没有 MXC 租约需要跨重启恢复"的说法，没有考虑这种情况。

**验证**：

1. 记下工作区、scratch、`AllowWrite` 目录的 `icacls` 输出，启动 `ping -n 60 127.0.0.1`，然后在 UI 里取消。
2. 用 `Get-Process ping`（或按命令行筛选的 `Get-CimInstance Win32_Process`）确认没有残留进程。
3. 再跑一次 `icacls`，和取消前对比，不能多出 AppContainer/容器 SID 的 ACE。
4. 用任务管理器结束 `nexus-server.exe`（或 nxs），重复第 2、3 步，验证崩溃场景。

**修复方案**（按验证结果选用）：

- **SDK 测试**：把 `TestWindowsMXCCancellationStopsSidecar` 改成：工作进程写一个带唯一标记的 PID 文件，取消后断言该 PID 已退出；同时在执行前后各取一次授权目录的 DACL（`windows.GetNamedSecurityInfo`），断言没有新增 ACE。
- **优雅终止**：不再对 `wxc-exec` 直接 `taskkill /F`，改为先通知它退出（Ctrl-Break：`CREATE_NEW_PROCESS_GROUP` 加 `GenerateConsoleCtrlEvent`；或者看 MXC 是否提供 stdin 关闭或取消协议），等待 2–3 秒，超时再强杀。改动位置：`sandboxexec` 给 MXC 执行计划设置专门的 `Cancel`，不再复用 `internaltool.CancelProcessTree`。
- **兜底收口**：如果容器进程不在 `wxc-exec` 的进程树里，就让 `wxc-exec` 运行在 SDK 创建的 Job Object 里（`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`），确认 MXC 创建的容器进程也会进入这个 Job。bridge 已经把 nxs 放进 kill-on-close Job，需要确认它对容器进程有没有效。
- **ACL 残留恢复**：如果验证发现强杀会残留 ACE，可以在 App 启动时扫描 scratch、工作区和 `AllowWrite` 目录，删除 MXC 容器 SID（确认 SID 的前缀规则后再定）留下的 ACE；或者改用 MXC 提供的清理入口（如果 `wxc-host-prep.exe` 或 FFI 提供的话）。实现前必须先确认残留 ACE 的特征，不能按猜测删除 ACL。

---

## R3 目录拒绝的结尾反斜杠语义没有依据

> 当前 PSEC 处理：新策略直接下发 deniedPaths，不再添加目录后缀；目录子树拒绝由原生测试验收，尚未执行。 以下保留旧方案风险分析（历史，non-normative）。

**现象**：`f4f0789c` 认为目录路径后面加 `\` 才能让拒绝覆盖整棵子树。`da4c85b9` 把原来误加的两个反斜杠改成了一个，但 MXC 的 schema（`deniedPaths?: string[]`）和 README 都没有提到这条规则。如果 MXC 并不按结尾反斜杠区分，"带 `\`"和"不带 `\`"可能都只拒绝了目录本身，子文件仍然可读写。

**验证**：在普通用户下准备 `work\protected\secret.txt`，`readwritePaths=[work]`，再分别用 `deniedPaths=[work\protected]` 和 `[work\protected\]` 执行 `type work\protected\secret.txt` 和 `echo X> work\protected\new.txt`。两种写法都要记录结果。

**修复方案**：

- 两种写法都能拒绝子树：保持现状。
- 只有带 `\` 的写法能拒绝子树：保持现状，并在 `windows_mxc_test.go` 里加一个"拒绝目录后子文件不可读写"的原生测试固定下来。
- 两种都只拒绝目录本身：`mxcFilesystemDenyPolicy` 在拒绝已存在的目录时，必须把目录下的已有条目逐个展开成具体路径，并设上限（例如 4096 个，超出就 fail closed）。准备期之后新建的子项，加入 `MXCUnprotectedPaths` 做事后复核。

---

## R4 每次文件操作重复探测，性能差

> 当前 PSEC 处理：进程内 PSEC 探测通过 sync.Once 缓存；删除逐次探测子进程。策略单测通过，但初始化耗时和操作次数仍待 Windows 采样，尚不能证明超时已解决。 以下保留旧方案风险分析（历史，non-normative）。

**现象**：一次文件 helper 操作（stat、read、read_dir 都算）会启动：`PrepareExecution` 里 `mxcUnavailableReasonWithEnvironment` 的 `wxc-exec --probe`、`prepareWindowsMXCExecution` 里再一次 `--probe`、`wxc-exec` 本体、`nxs --sandbox-file-helper`；构造 `sandboxfs.New` 时还会再探测一次。项目发现（commands、agents、settings、rules、instruction files）要做几十次文件操作，会话启动可能要好几秒，甚至撞上初始化超时。

**修复方案**（SDK `internal/tool/builtin/bash/sandboxexec/windows_mxc_windows.go`）：

- 加一个进程级的探测缓存，key 用 `wxc-exec.exe` 的绝对路径加文件的修改时间和大小；只缓存成功结果，失败结果不缓存，或只缓存很短时间（例如 5 秒）。
- `mxcUnavailableReasonWithEnvironment` 和 `prepareWindowsMXCExecution` 共用这个缓存，同一次准备只探测一次。
- 提权检测 `IsElevated()` 很便宜，每次都实时检查，不进缓存。
- 补一个单测：用一个计数用的 fake prober，确认连续多次准备只探测一次，`wxc-exec` 被替换后会重新探测。
- 之后可以进一步做：项目发现期间复用同一个长驻的 file helper worker（macOS 已有类似的 memory writer 长驻进程），不再每次操作都起一个 `wxc-exec`。

---

## R5 MXC 配置经命令行传递，有长度上限

> 当前 PSEC 处理：启动配置通过 4 KiB 环境变量块传递，上限 1 MiB；编码往返和超限单测通过，原生大请求仍待验收。 以下保留旧方案风险分析（历史，non-normative）。

**现象**：`prepareWindowsMXCExecution` 把完整的 MXC 配置（包括整份环境变量和全部授权/拒绝路径）做 base64 后，经 `--config-base64` 放进命令行。Windows 命令行上限是 32767 个字符，base64 还会把配置放大约 4/3。PATH 很长、环境变量多、授权目录多时，`CreateProcess` 会直接失败。

**验证**：构造一个约 20 KB 的 PATH 加上数十个 `AllowWrite` 目录，执行一次命令，看是否报错。

**修复方案**：

- 首选：确认 `wxc-exec` 是否支持从文件或 stdin 读取配置（看 `wxc-exec --help`，或问 MXC 团队）。支持的话，把配置写到 scratch 目录下一个只有当前用户可读的临时文件，执行结束后删除。
- 不支持的话：在 SDK 里计算最终命令行长度，超过约 30000 字符就返回明确的错误（例如"MXC 配置过大，请减少环境变量或授权目录"），不要让 `CreateProcess` 报一个看不懂的错误；同时进一步收窄传给 MXC 的环境变量（只保留 `SYSTEMROOT`、`WINDIR`、`COMSPEC`、`PATH`、`TEMP`、`TMP`、`USERPROFILE`、`LOCALAPPDATA`、`APPDATA` 和命令真正需要的变量）。

---

## R6 产品层面的限制

| 限制 | 影响 | 处理建议 |
|---|---|---|
| 提权令牌下受限会话不可用 | 以管理员身份运行 Nexus、或关闭了 UAC 的机器，完全用不了桌面沙箱 | 在 设置 › 沙箱 里把提权原因翻译成明确的中文提示，并说明怎么用普通权限启动；SDK 现在返回的是英文原因，nexus 侧按关键字映射即可 |
| MXC 拒绝是读写一起拒 | 沙箱里的命令读不了 `.nexus\commands`、`.git\hooks` 等禁写目录 | 写进用户文档；如果以后 MXC 支持只禁写，再去掉 `sandboxfs/operation_policy.go` 这层绕行 |
| 准备期不存在的受保护路径只能事后复核 | `.bashrc`、`.git\hooks\*` 这类文件可能被创建出来，结束后才报违规，比 macOS Seatbelt 弱 | 维持现状并写进文档；或者准备期先创建一个占位目录再拒绝它（会改动用户工作区，需要产品决策） |
| nxs 进程本身不在容器里 | 只隔离 nxs 发起的命令和文件 helper，和 macOS 模型一致 | 维持现状 |
| 受限 Claude 会话不支持 | Windows 上只能用 nxs | 维持现状 |

---

## R7 发版前必须确认的事项

当前不分发第三方 MXC 二进制，旧方案的再分发与重签名项不再适用；系统兼容性、普通用户隔离及 ARM64 仍是原生验收项。

| 事项 | 现状 | 需要做什么 |
|---|---|---|
| SDK CI 工作流 | `desktop-sandbox.yml` 已改成用临时普通账户跑 MXC 测试，但还没在 GitHub 上实际跑过 | 触发一次 workflow，确认普通账户能读取 fixture 和 MXC 目录，测试真的执行了、没有全部 skip |
| 系统兼容性 | 只在 build 26300 上测过 | 确认 base-container 支持的最低 Windows 版本；不支持时 设置 › 沙箱 要显示"此系统不支持"，并给出可理解的错误 |
| 再分发许可与签名 | 未确认 | 确认 `@microsoft/mxc-sdk` 的 LICENSE 是否允许把 `wxc-exec.exe`、`mxc_ffi.dll`、`winhttp-proxy-shim.exe` 打进安装包，以及是否需要重新签名 |
| ARM64 | MXC 包里其实带了原生 `bin/arm64`，但 `build-windows-app.ps1` 和 SDK `release-nxs.yml` 只打包 x64；SDK 的查找顺序已经会优先找 `mxc/bin/arm64` | ARM64 安装包改为打包 `bin/arm64`（或两个都打），并在 ARM64 机器上跑一次原生测试 |
| bridge 旧分支 | `nexus-agent-sdk-bridge` 的 `codex/desktop-sandbox-approvals` 只多出旧 `windowssandbox` supervisor，用它的 SDK 旧后端已经删掉 | 确认没有需要的内容后删除该分支，不要合进 main |

---

## 建议的处理顺序

1. Windows 按 [windows-mxc-sandbox-verify.md](windows-mxc-sandbox-verify.md) 验证，重点是 R1、R2、R3。
2. R1 不通过：停止推进，先排查 MXC 本身。
3. R1 通过后，修 R4（探测缓存）和 R2 的测试补强；R2 验证有残留的话，再做优雅终止和 ACL 恢复。
4. 根据 R3 的结果决定是否展开目录拒绝。
5. 发版前处理 R5 和 R7。
