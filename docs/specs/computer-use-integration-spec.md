# Computer Use 集成规范

本文记录 Nexus Product 对独立 Nexus Computer Use Runtime（仓库标识
`nexus-cua`）的当前集成合同。

## 架构边界

```text
Nexus Agent
  -> computer-use Skill
  -> round-scoped `nexus computer` CLI
  -> Nexus loopback command broker
  -> Nexus Computer Use host service
  -> version-pinned nexus-cua sidecar
  -> macOS / Windows desktop
```

- Nexus Agent 负责推理、选择目标、决定下一步与解释结果。
- Nexus Product 持有用户开关、包版本、下载校验、sidecar 生命周期、动作策略和
  round identity。
- `nexus-agent-sdk-bridge` 只负责把宿主签发的环境和 Skill 选择传给 Agent runtime；
  它不持有 CUA token、session 或 Nexus 用户状态。
- `nexus-cua` 是模型中立的桌面执行 runtime，不包含 Agent loop、Provider、Nexus
  Session、Room、偏好或审批语义。
- Browser 继续使用现有 MCP + Chromium 扩展链路。Computer Use 不依赖 Browser；
  用户可以只开启其中一个，也可以同时开启。
- Alpha 不挂载 CUA MCP。未来 MCP adapter 必须是独立生态包，不能进入 runtime
  core，也不能替代 Nexus 的 round-scoped broker。

## 用户开关

- `computer_use_enabled` 是 owner 级持久偏好，默认关闭。
- 关闭时，运行时不选择 `computer-use` Skill；`nexus computer` broker 除只读
  `get_computer` 状态外必须拒绝所有 Computer Use 操作。已安装包或仍在空闲的
  sidecar 不构成授权。
- 开启只授予使用 Nexus Computer Use host policy 的资格，不把 sidecar transport
  token、endpoint 或任意 native authority 交给 Agent。
- Browser 开关与 Computer Use 开关相互独立。Computer Use 不得因 Browser 未开启
  而降级、自动开启 Browser 或改走 CDP。

## 包与版本

- Nexus Go host client 固定到 `nexus-cua` commit
  `764430824aa26ee52540845dca6be56a38f5e1e0`（Go module pseudo-version
  `v0.0.0-20260826095132-764430824aa2`）。升级必须显式更新依赖、兼容性证据与本文，
  不跟随 `main` 漂移。

- Nexus 只运行明确固定版本的 `nexus-cua`。开发环境可以通过
  `NEXUS_CUA_COMMAND_PATH` 指向已经由开发者验证的二进制；正式安装使用宿主配置
  中固定的 target version、manifest URL 和 manifest SHA-256。
- manifest 是闭合 JSON，声明 schema version、CUA version、protocol version 和
  各平台 artifact。平台选择只接受维护矩阵中的 OS/architecture，不从 artifact
  文件名猜测。
- 下载同时校验 manifest digest、archive digest、解包后 binary digest、CLI version
  和 `doctor --compact` 返回的 protocol version。任何一项不匹配都不能切换 current
  version。
- 安装先写同一 package root 下的临时目录，校验完成后原子切换 current marker。
  update 与 rollback 都按版本目录并存；运行中的 sidecar 绝不在线替换 executable。
- remove 只删除 Nexus 管理的 Computer Use package root；环境覆盖指向的外部
  executable 不属于 Nexus，不能被删除。
- 当前 alpha 的正式签名、SBOM、checksum 发布资产仍由 `nexus-cua` M4 gate 管理；
  缺少发布资产时 Nexus 只能报告不可安装，不能下载 unsigned fallback。

## Sidecar 生命周期

- Nexus 为每个 sidecar epoch 创建新的 owner-private token、Unix socket 或 Windows
  named pipe、artifact generation 和日志尾缓冲。
- 启动必须在有界 deadline 内通过 typed `get_capabilities`，并核对
  `nexus.cua.v1`、已安装版本和平台；仅看到进程存活不表示 ready。
- readiness 前退出、超时、协议不匹配和权限缺失使用稳定、可诊断的产品错误分类。
- unexpected exit 使该 epoch 的 session、observation、element 和 mutation request
  全部失效。Nexus 可以启动 fresh epoch，但不能把旧 mutation 重放到新进程。
- shutdown 先停止新 admission，关闭仍可关闭的 CUA session，再终止进程；超时后
  才强制结束。Nexus server `Close` 必须收口 sidecar。
- stdout/stderr 只保留有界尾部用于 doctor；不得记录截图、可访问性 value、输入
  文本、token、完整 request payload 或 native handle。

## Round-scoped command

Agent-facing domain 名称固定为 `computer`。它复用 Goal/Execution 的单进程 CLI、
fresh contract、0600 input staging 和 stable request ID 规则。

公开 operation 分为：

- `get_computer`：读取启用、安装、运行、协议、权限和当前 round target 状态。
- `list_applications`：可信宿主 discovery 的脱敏投影。
- `select_target`：按唯一应用和窗口选择器打开宿主生成的 bounded session，并产生
  第一份 observation；Agent 不能提交原始 capability manifest。
- `observe`：刷新当前 target 的 screenshot/semantic observation。
- `perform_action`：只接受 closed action union，并使用最新 observation。
- `verify_state`：用 closed predicate 验证当前 target。
- `close_target`：显式关闭本轮 target；physical round 结束时仍会兜底清理。

宿主为每个 physical round 保存独立 target state。跨 round 不复用 CUA session、
window ref、observation ref 或 element ref。截图从 sidecar 私有 artifact root 复制到
当前 owner 的 round 临时目录，返回给 Agent 的路径随 round 清理。

## Mutation reconciliation

- `perform_action` 把 Nexus command `request_id` 与一个不可变 CUA `ActionRequest`
  绑定。相同 request ID + 相同 closed input 才能继续等待或返回缓存结果。
- 同 ID 不同输入失败关闭。等待中断或 CUA 返回 indeterminate 时，只能对同一个
  `ActionRequest` 延长等待，不能生成新 native mutation。
- sidecar epoch 变化后返回稳定 indeterminate/epoch-changed 错误；允许 fresh observe，
  但绝不自动重放旧动作。
- 成功动作后立即刷新 observation，并把 action dispatch 与 post-action observation
  分开报告；刷新失败不能把已经 dispatch 的动作伪装成未执行。

## 权限与错误

- `screen_capture`、`accessibility` 和 `input_control` 分别展示；read-only observation
  不要求尚未使用的 mutation permission。
- `permission_required`、`protocol_mismatch`、`stale_discovery`、
  `stale_observation`、`target_unavailable`、`target_unresponsive`、`busy`、
  `deadline_exceeded` 和 mutation indeterminate 必须保持可区分。
- Agent 只能使用 host policy 允许的 application 和 action。Alpha 的 policy 不提供
  arbitrary desktop、system surface、进程启动/终止、clipboard 或 raw native API。
- typed text 和 set-value 是敏感输入；不进入日志、receipt、错误正文或诊断输出。

## 验证门禁

- package manager：平台选择、manifest/archive/binary 校验、原子切换、remove 边界。
- supervisor：ready、timeout、early exit、restart epoch、shutdown 和 bounded logs。
- command broker：默认关闭、Skill 选择、strict schemas、target 隔离、stale refs、
  same-request reconciliation 和 round cleanup。
- 端到端：Agent CLI → loopback broker → typed host client → fake/live sidecar →
  observation/action/verify。portable CI 使用 deterministic fake；macOS/Windows 真机
  gate 复用 `nexus-cua` fixture，不在 Nexus 仓库复制 native fixture。

当前交接状态不能解释为正式发布：`nexus-cua` M0/M1 complete；M2 engineering
implemented 但仍等待最终 SHA 下的 maintained macOS Apple Silicon 与 Windows x64
accepted evidence、1h/8h soak 和性能基线；M3 client engineering implemented，但完整
live native acceptance 依赖 M2；M4 的签名、notarization/AuthentiCode、SBOM、checksum、
provenance、clean-machine smoke、registry/tag/release 仍 pending。

[PROTOCOL]: 变更 package manifest、用户开关、sidecar lifecycle、command operation、
错误分类或 Skill 绑定时，必须同步更新本文、Computer Use 包 L2、Skill、API 类型、
测试和 CHANGELOG。
