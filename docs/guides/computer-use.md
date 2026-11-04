# Nexus Computer Use（macOS 接入）

Nexus 通过 pinned 的 `nexus-cua` SDK/sidecar 使用原生桌面能力。模型只看到
`nexus.command` 的 `computer` domain；endpoint、token、socket、native handle 和
截图文件路径都由 Nexus 私有管理。分发、启用与权限引导的规范见
`docs/specs/computer-use-spec.md`。

## 1. 常规启用（设置页）

在 macOS 桌面端打开 设置 → 电脑操控（CUA）开关。未安装时，Nexus 会从
`nexus-research-lab/nexus-cua` 下载与内置 SDK 同版本的 GitHub Release 安装包，完成
源码固定 SHA256、Release checksum 与 manifest 校验后安装到状态根的 `computer-use/versions/` 下，再启用并
自动检查系统权限。启用但 Screen Recording / Accessibility / Input Control 任一未授予时，
按设置页引导前往 系统设置 → 隐私与安全性 完成授权，然后「重新检查权限」。

安装与启用状态持久化在本地；重启 Nexus 后保持。该入口只在 macOS 桌面宿主显示。

## 2. 环境变量覆盖（开发/内置打包）

需要使用自建或随应用打包的 sidecar 时，在 Nexus 服务环境中设置绝对路径：

```env
NEXUS_COMPUTER_USE_ENABLED=true
NEXUS_COMPUTER_USE_BINARY=/absolute/path/to/nexuscua
```

显式二进制路径优先于已安装版本。测试发布通道可用 `NEXUS_CUA_RELEASE_API_URL`
覆盖 Release API；`NEXUS_CUA_ALLOW_UNACCEPTED_RELEASE=1` 仅限开发环境放行未验收候选。
本地构建 sidecar：

```bash
cargo build --release --locked --package nexus-cua --bin nexuscua
cargo run --locked --package nexus-cua -- doctor
```

macOS 需要由运行 sidecar 的稳定应用身份取得 `Screen Recording`、`Accessibility`
和前台输入对应的系统授权。`doctor` 返回的三类 permission state 是当前事实；
没有授权时不要通过其他工具绕过。设置页会分别展示三项权限，用户点击某一项的「开启」
按钮后进入对应系统设置页面；返回 Nexus 后页面会自动复查。

默认关闭。Nexus 启动时不会启动 sidecar；本轮第一次调用 `computer` domain 时，
它才会检查普通可执行文件、创建私有 endpoint/token/artifact root 并等待协议就绪。
不要把 endpoint、token 或 native handle 放进 Agent prompt、Skill 参数或日志。

## 3. Agent 使用顺序

内置 `skills/computer-use/SKILL.md` 是模型侧规则。典型顺序是：

1. `computer` `inspect` 读取 capabilities 和 permission state；
2. `list_applications` 后用精确名称或 application id `select_target`；
3. 多窗口应用先调用 `list_windows`，再把返回的 `window_ref` 或精确标题传给 `observe`；单窗口应用可直接 `observe`；
4. 每次动作前 `observe`，只使用最新的 window/element/observation refs；
5. `perform_action` 每次等待当前 runtime session 的一次性真人批准；
6. 动作后重新 `observe`，必要时 `verify_state`；
7. 结束时 `close_target`。

动作结果未知时复用同一个顶层 `request_id` 对账，不能换 ID 重放。sidecar 崩溃或
owner 切换会使当前 round 的 session、observation、element ref 和 pending mutation
全部失效，必须重新发现、选择和观察目标。runtime round 正常结束时，Nexus 会自动
关闭对应 CUA session；`close_target` 仍可用于任务中途主动释放。

## 当前边界

这条开发链路证明 Nexus 与 CUA 的代码、协议和本地测试已经接通；签名 payload、
clean-host 安装、真实维护硬件权限撤销以及真实 Codex acceptance 仍属于
`nexus-cua` 的独立验收门槛，不能用本地 Go/Rust 测试替代。
