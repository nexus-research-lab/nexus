# Nexus Computer Use（macOS 开发接入）

Nexus 通过 pinned 的 `nexus-cua` SDK/sidecar 使用原生桌面能力。模型只看到
`nexus.command` 的 `computer` domain；endpoint、token、socket、native handle 和
截图文件路径都由 Nexus 私有管理。

## 1. 准备 macOS runtime

在 `nexus-cua` 工作区构建本机 sidecar：

```bash
cargo build --release --locked --package nexus-cua --bin nexuscua
```

构建后先单独确认驱动和系统权限：

```bash
cargo run --locked --package nexus-cua -- doctor
```

macOS 需要由运行 sidecar 的稳定应用身份取得 `Screen Recording`、
`Accessibility` 和前台输入对应的系统授权。`doctor` 返回的三类 permission state
是当前事实；没有授权时不要通过其他工具绕过。

## 2. 启用 Nexus

在 Nexus 服务使用的环境中设置绝对路径：

```env
NEXUS_COMPUTER_USE_ENABLED=true
NEXUS_COMPUTER_USE_BINARY=/absolute/path/to/nexuscua
```

默认关闭。Nexus 启动时不会启动 sidecar；本轮第一次调用 `computer` domain 时，
它才会检查普通可执行文件、创建私有 endpoint/token/artifact root 并等待协议就绪。
不要把 endpoint、token 或 native handle 放进 Agent prompt、Skill 参数或日志。

## 3. Agent 使用顺序

内置 `skills/computer-use/SKILL.md` 是模型侧规则。典型顺序是：

1. `computer` `inspect` 读取 capabilities 和 permission state；
2. `list_applications` 后用精确名称或 application id `select_target`；
3. 每次动作前 `observe`，只使用最新的 window/element/observation refs；
4. `perform_action` 每次等待当前 runtime session 的一次性真人批准；
5. 动作后重新 `observe`，必要时 `verify_state`；
6. 结束时 `close_target`。

动作结果未知时复用同一个顶层 `request_id` 对账，不能换 ID 重放。sidecar 崩溃或
owner 切换会使当前 round 的 session、observation、element ref 和 pending mutation
全部失效，必须重新发现、选择和观察目标。runtime round 正常结束时，Nexus 会自动
关闭对应 CUA session；`close_target` 仍可用于任务中途主动释放。

## 当前边界

这条开发链路证明 Nexus 与 CUA 的代码、协议和本地测试已经接通；签名 payload、
clean-host 安装、真实维护硬件权限撤销以及真实 Codex acceptance 仍属于
`nexus-cua` 的独立验收门槛，不能用本地 Go/Rust 测试替代。
