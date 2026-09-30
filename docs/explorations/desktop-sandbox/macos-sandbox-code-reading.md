# macOS 沙箱代码解读

> 记录性质：non-normative code reading。本文按当前工作树的实现解释调用链；它不是新的 API 合同，也不把测试通过写成发布验收。
>
> 记录时间：2026-09-30。

## 1. 主调用链

```text
BuildAgentClientOptionsWithConfig
  ├─ 解析 owner / runtime / Provider / MCP
  ├─ 桌面模式强制打开 DesktopSandboxEnabled
  ├─ 校验桌面 MCP 网络准入
  ├─ 构造 runtime env、workspace、memory 和 capability
  ├─ 组装 agentclient.Options
  └─ applyDesktopSandbox
       ├─ macOS + nxs    → nxs 多能力合同
       └─ macOS + Claude → Claude 原生 sandbox settings
```

运行期随后由 `internal/runtime` 处理 process-policy、generation、policy receipt、scratch lease、关闭和恢复；macOS App 则由 `desktop/macos` 管理 sidecar、状态根和窗口宿主。

## 2. `internal/runtime/clientopts/agent_client.go`

`BuildAgentClientOptionsWithConfig` 是统一装配入口，而不是业务 handler 的临时拼接点。

### 2.1 身份与输入清理

函数先比较显式 `OwnerUserID` 与认证上下文，防止同一 runtime 混入两个 owner。随后合并 typed/legacy MCP，校验 configuration environment，并解析最终 runtime kind。

桌面模式会执行：

```go
input.DesktopSandboxEnabled = input.DesktopSandboxEnabled ||
    strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop")
```

这意味着生产桌面调用方不能通过把内部布尔值设为 false 来关闭产品默认策略。

### 2.2 环境和宿主所有权

环境构造先清理继承的 Provider、代理和秘密变量，再按 runtime/provider/background/vision/WebSearch 等宿主解析结果显式投影。`ExtraEnv` 不是最终权威：owner、memory root、Provider 和宿主调度相关变量在后面再次写入，避免任务环境重定向请求或记忆目录。

这段代码保证的是“输入所有权和作用域”，不是宣称任意后代进程都无法读取宿主进程内存或环境。

### 2.3 选项装配

函数把 CWD 固定为 workspace，追加 Skill/挂载目录，设置工具 allow/deny、permission mode、callback 和 session 选项。nxs 与 Claude 的工具/Skill 投影不同：Claude 保留其项目级 Skill 动态发现，nxs 使用宿主绑定的 Skill ID。

完成普通选项后，桌面沙箱逻辑才把具体后端合同写入 `options.Sandbox`；这避免在多个 DM/Room/background 调用点重复实现策略。

## 3. `internal/runtime/clientopts/desktop_sandbox.go`

### 3.1 平台入口

`applyDesktopSandboxForPlatform` 首先 clone 环境并删除旧的宿主 marker，避免调用方 map 被修改或残留上一代 marker。非桌面模式直接返回；Windows 当前保留既有合同；除 macOS 外的平台返回 unsupported。

macOS 桌面会写入 `NexusDesktopSandboxPolicyEnvName=1`。这只是宿主策略 marker，不能单独被当成隔离证明。

### 3.2 Claude 分支

Claude 不允许携带 nxs `SandboxResources`。Full Access 作为显式例外直接保留普通选项；受限模式则 clone 现有 Claude settings，拒绝所有 nxs-only required 字段和 `RequireClaudeRestricted`，再设置：

* `RequireClaudeNativeSandbox=true`
* `Enabled=true`
* `FailIfUnavailable=true`
* `AutoAllowBashIfSandboxed=true`
* `AllowUnsandboxedCommands=false`

Skill 目录进入 read grants，用户附加目录进入 write grants。`cloneClaudeSandboxSettings` 深拷贝切片、map 和嵌套网络设置，避免新 runtime 修改调用方持有的指针。

### 3.3 nxs 分支

nxs 分支要求 runtime kind 必须是 `RuntimeNXS`，macOS 同时启用 MCP strict config，并安装完整的 required 能力集合。`AllowUnsandboxedCommands=true` 的含义是保留独立的一次性 escape approval 入口；它不是默认裸执行。

Full Access 时清空受限文件资源，但保留 sandbox settings、handshake 和生命周期。带 host resource policy 时先执行 `Validate`，只读 scope 不能同时带 write directory grant，并强制 `AllowUnsandboxedCommands=false`。

### 3.4 为什么资源必须复制

`SandboxResources` 被按值复制后写入 options，而不是把输入指针直接交给 runtime。测试 `TestDesktopSandboxCopiesHostPreparedResources` 验证了这一点。这样 host lease 的 immutable scope 不会被上层后续修改悄悄扩大或缩小。

## 4. `internal/runtime/process_policy.go`

`managedRuntimeProcessPolicyFingerprint` 把 CLI、CWD、可执行文件、参数、settings、sandbox required flags、MCP strictness、资源策略、工具集合、受控环境和 hooks 规范化后 SHA-256 化。

它有三个工程作用：

1. 判断 warm runtime 是否仍然代表当前策略；
2. 策略变化时要求 replacement，而不是在旧进程上热更新；
3. 为 policy receipt 和恢复诊断提供不含明文凭据的稳定指纹。

该摘要包含 `SandboxResources` 和多项 required capability，因此单独改变媒体、搜索、MCP 或 settings 写入要求也会触发代次替换。

## 5. `internal/runtime/sandbox_policy.go` 与运行时恢复

`desktopSandboxModeTransition` 只关心桌面 marker 存在且 permission mode 是否跨越 Full Access 边界。跨越时返回 `ErrDesktopSandboxPolicyChanged`，调用方必须退休旧 runtime 后重新构造。

其余 `internal/runtime/sandbox_*.go` 文件形成持久生命周期：

* `sandbox_resources.go` 持有 owner/session 作用域的 scratch lease；
* `sandbox_process_host.go` 和 `sandbox_process_recovery.go` 记录、回收并核验 supervised process；
* `sandbox_receipt*.go` 把 policy receipt 绑定到 exact runtime launch；
* `sandbox_scratch_recovery_darwin.go` 按固定目录句柄恢复删除阶段；
* `idle.go` / `owner.go` 在 reaper 不确定时把精确 generation 降级为 unknown。

恢复代码有意保守：旧 PID 死亡只代表“该 PID 当前不可见”，不代表后代、句柄、scratch 和策略都已收口。`cleanup_unknown` 会继续阻断新 acquisition，直到独立 reconcile 证明完整边界。

## 6. `internal/infra/confinedfs`

`confinedfs` 是宿主访问状态和恢复资料的文件边界。它通过固定目录句柄打开 root、拒绝 symlink entry，并在支持的平台检查 hard-link marker。macOS scratch、lease marker、process record 和 runtime bootstrap 都依赖它，避免“先检查绝对路径、随后被替换”的 TOCTOU 旁路。

注意：这是宿主文件访问保护，不等同于给 SDK 主进程安装 macOS Seatbelt。Seatbelt/file executor 由 nxs/Claude 原生后端负责，二者必须分别验证。

## 7. `internal/infra/runtimebootstrap`

`runtimebootstrap.Load` 只接受 `Contents/MacOS/nexus-server` 旁边的固定 App bundle。它通过 `confinedfs` 打开 `Resources/runtime-bootstrap.json` 和 `bin/nexus-runtime-bootstrap`，检查：

* manifest version、相对路径、Bridge version、架构和小写 SHA-256；
* helper 可执行权限；
* Go build info 的 module、版本、`CGO_ENABLED=1`、`GOOS=darwin`、架构；
* helper 内容摘要；
* 摘要计算前后 inode、size 和 mtime 未改变。

加载成功只证明 helper 与随包构建身份匹配。代码注释明确说它不替代代码签名或原生接口可用性证明，因此不能用它单独宣布 DMG 发布验收通过。

## 8. `desktop/macos` 宿主层

macOS Swift 代码分成状态根、sidecar、生命周期和 WebView 等职责：

* `DesktopStateRootStore` / `DesktopStateRootMigration` 负责 canonical `NEXUS_STATE_ROOT` 和完整迁移；
* `SingleInstanceGuard` 与 `internal/infra/desktopinstance` 持有状态根独占锁；
* `SidecarSupervisor` 启动并监督随包 `nexus-server`，将 process identity 和关闭阶段交给恢复链；
* `SidecarProcessIdentity` 保存 boot session、audit token 等精确身份；
* `SidecarOrphanReaper` 只回收可证明属于本次 App/boot 的旧实例，未知或旧格式记录保留；
* `runtimebootstrap` 在 sidecar 启动前校验随包 helper；
* `WebViewConfigurationFactory`、URL policy 和 bridge handler 负责 UI/网页边界，不能代替 runtime tool sandbox。

宿主关闭顺序是“停止 runtime 准入 → 等待进程、policy、scratch 终态落盘 → 关闭数据库和 App 资源”。任何阶段失败都应保留 recovery record，而不是把正常退出状态写死。

## 9. 如何读测试

推荐按以下顺序理解证据：

1. `internal/runtime/clientopts/desktop_sandbox_test.go`：构造、分流、深拷贝和拒绝条件。
2. `desktop_sandbox_integration_test.go`：真实 nxs capability handshake 和 host resources 组合。
3. `internal/runtime/process_policy_test.go`、`sandbox_*_test.go`：代次、receipt、scratch、回收和 unknown 栅栏。
4. `desktop/macos/Tests/NexusDesktopTests/`：状态根、sidecar、URL/WebView 与宿主恢复。
5. `scripts/desktop/check-sandbox-baseline.mjs`、`docs/testing/desktop-sandbox-acceptance.md`：跨仓固定 binary 和原生门禁。

任何一层通过，都只能支持该层的结论。特别是“Go 测试通过”不能替代 macOS 原生、签名、公证、clean-host、真实安装升级或发布验收。

## 10. 与产品规范的关系

本文是代码阅读辅助材料。需要修改能力、状态、身份或恢复语义时，应先更新 [`docs/specs/desktop-sandbox-spec.md`](../../specs/desktop-sandbox-spec.md)，再同步 L2 `doc.go` 和代码顶部契约；需要补实现证据时，再更新 [`docs/testing/desktop-sandbox-acceptance.md`](../../testing/desktop-sandbox-acceptance.md)。
