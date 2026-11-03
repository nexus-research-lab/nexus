# Computer Use 能力规范

Nexus Computer Use 的宿主侧分发、启用与权限引导合同。模型侧命令面、会话及敏感动作审批和失败对账分别在
`execution-orchestration-spec.md` §5.1 与 `permission-runtime-spec.md` §3.4 中维护，本规范不重复定义。

## 架构边界

```text
settings computer-use
  -> web/src/features/settings/computer-use
  -> internal/handler/computeruse
  -> internal/service/computeruse (installer + supervisor)
  -> nexus-cua sidecar (外部仓库，pinned SDK)
```

- `internal/service/computeruse` 是安装、启用、sidecar 监管与 round 状态的唯一真相源；
  endpoint、token、native handle 与 artifact root 不进入 Agent input、审批事件或普通日志。
- `internal/handler/computeruse` 只投影浏览器安全的状态与动作，不暴露私有路径。
- `web/src/features/settings/computer-use` 只负责安装开关、状态展示与权限引导，不承载会话或动作。
- 设置入口只在 macOS 桌面宿主显示（宿主注入 `platform=macos`）；Web 与 Windows 隐藏入口，
  直接访问分区时重定向回常规设置。Windows 支持为未实现的 non-normative 计划。

## Round 会话边界

首次使用 Computer Use 为本物理回合建立最多 30 分钟的授权上限。选定目标的 session TTL 取请求时长、该回合剩余上限与 Runtime `action_limits.max_session_ttl_seconds` 的最小值；审批等待也扣除，不静默续期。round 结束或禁用时立即关闭；session 过期需重新 `select_target`，回合上限到达则需新回合。

`select_target` 先确认精确应用、动作集合、前台输入开关与时长，默认不允许前台输入。常规操作使用本次授权，敏感操作审批规则见 permission-runtime-spec §3.4。`get_computer` 返回 Runtime `action_limits` 及由其生成的 `action_schema`；Runtime 始终执行最终参数校验。

新增 `restore_window`、`set_window_bounds`、`set_range_value`、`press_point` 和单独审批的 `terminate_application`。恢复最小化窗口前可 `observe` 且 `include_screenshot=false`，恢复后再截图。可启动应用通过 `list_launchable_applications` 发现，`launch_application` 仅启动并对未知结果按原 ID 对账，随后仍需 `select_target` 授权。


## 动作坐标

点击、移动、拖拽和滚动点统一使用 `0..1000` 归一化坐标，映射由 Runtime 完成。滚动必须显式传目标点，delta 为逻辑像素意图且受 ±100000 限制。Windows 滚轮以系统行/字符设置换算，应用实际滚动距离仍需观察确认。

## 分发与安装

- 安装源固定为 `github.com/nexus-research-lab/nexus-cua` 的 latest Release；
  `NEXUS_CUA_RELEASE_API_URL` 仅用于测试覆盖。
- Release 必须为正式版（非 draft、非 prerelease），tag 为合法 semver；
  归档内 manifest 合同为 `nexus.cua.release-candidate.v1`，且 `release_state=release_accepted`。
  `NEXUS_CUA_ALLOW_UNACCEPTED_RELEASE=1` 只允许开发环境放行未验收候选。
- 安装包必须附带 `<archive>.sha256`；归档整体 SHA256 与 manifest 内 runtime 文件 SHA256
  双重校验通过后才落盘。归档拒绝符号链接、路径穿越、超限成员与多重 manifest/runtime。
- 安装产物原子落位到 `<CUA 状态根>/versions/<version>-<architecture>/nexuscua`，
  状态根为 SQLite 数据库同目录下的 `computer-use/`（0700）；安装、启用与二进制路径记录在
  同目录 `state.json`。安装本身不启用能力。
- 安装器只接受 macOS 归档，按当前 CPU 架构优先选择 arm64/x86_64、其次 universal。

## 启用生命周期

- 设置开关是唯一常规启用入口：未安装时先安装，再启用，随后自动读取一次系统权限状态。
- 启用状态持久化在 `state.json`；服务启动时以「环境变量启用 或 持久化启用」播种，
  且仅在存在可用二进制（已安装或 `NEXUS_COMPUTER_USE_BINARY`）时生效。
- `NEXUS_COMPUTER_USE_BINARY` 显式指定的绝对路径优先于已安装版本，用于打包内置与开发覆盖。
- 关闭开关会结束当前所有 CUA round、关闭 session 并停止 sidecar；重新启用从新 epoch 开始。
- 安装新版本会停止当前 sidecar 并切换后续 epoch 使用的二进制，旧 round 的 opaque ref 全部作废。

## 宿主 HTTP API

路由固定为 `/settings/runtime/computer-use` 前缀，响应均为 no-store：

| 端点 | 语义 |
| --- | --- |
| `GET /` | 安装/启用状态、版本、来源；不含权限探测 |
| `POST /install` | 同步下载、校验并安装 latest Release |
| `POST /enable` `POST /disable` | 显式启用/关闭；未安装时启用返回 409 |
| `POST /permissions/check` | 必要时拉起 sidecar 并返回 Screen Capture / Accessibility / Input Control 状态 |

## 权限引导

- 宿主只读取并展示系统权限事实，永不代为授予或绕过；授权必须由用户在 macOS
  系统设置中完成。
- 设置页在启用后显示 Screen Recording、Accessibility、Input Control 三项权限状态；
  「重新检查权限」只读取三项当前事实，不自动打开系统设置。每个未授予项目各自提供
  「开启」按钮，按钮只调用对应的 macOS 权限请求 API 并打开对应隐私面板。用户返回 Nexus
  后页面自动复查，三项全部通过后明确显示“权限已全部开启”。权限状态同时经 `get_computer`
  提供给模型，未授权时模型应停止并说明。

窗口发现返回 `state` 封闭枚举：`normal`、`minimized`、`offscreen`、`unknown`，
不再返回 `minimized` 布尔值；未知状态不等同于可交互（C1）。

C8：SDK 协议支持有限小数归一化坐标，Nexus 模型侧输入仍为整数；构造动作时
显式转换为浮点值，裁剪/zoom 精度由 Runtime 与 SDK 保留。

## SDK 独立构建（C3）

生产 SDK 由 go.mod/go.sum 固定具体提交；本地 go.work 只用于联调，禁止提交或充当可复现构建依据。`make check-cua-sdk` 显式使用 GOWORK=off，验证 CUA 接入包并构建命令入口。后端 CI 同样禁用 workspace 替换。
