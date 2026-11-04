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

`list_applications` 和 `list_launchable_applications` 接受可选 `query`（最多 256 字符），
按名称或稳定 application id 做不区分大小写的子串筛选；省略仍返回完整发现结果。
筛选不选择应用、不授予权限，也不把 Runtime 的 `complete=false` 改成 true。
启动目录仅保留本次返回的 launch_ref，后续仍校验精确安装身份并审批。
已知目标时优先筛选，避免无关应用目录触发模型侧大文本分页。

`select_target` 先确认精确应用、动作集合、前台输入开关与时长，默认不允许前台输入。常规操作使用本次授权，敏感操作审批规则见 permission-runtime-spec §3.4。`get_computer` 返回 Runtime `action_limits` 及由其生成的 `action_schema`；Runtime 始终执行最终参数校验。

新增 `restore_window`、`set_window_bounds`、`set_range_value`、`press_point` 和单独审批的 `terminate_application`。仅在明确的 `state=minimized` 时使用恢复动作；此前先 `observe` 且 `include_screenshot=false, accessibility="disabled"`，恢复后再截图。可启动应用通过 `list_launchable_applications` 发现，`launch_application` 仅启动并对未知结果按原 ID 对账，随后仍需 `select_target` 授权。

`session_active` 只表示已选应用的授权会话存在。尚未成功观察时，动作返回缺少 observation，
不误报未选目标。macOS 后台/台前调度窗口返回 `target_unavailable` 并指示 `focus_window` 时，
先对同一窗口取得 `include_screenshot=false, accessibility="disabled"` 的元数据观察，
再在已有权限范围内执行一次明确的 `focus_window`，然后正常观察。一次恢复仍失败就停止并报告；
不能通过反复选择应用或重新授权来代替窗口恢复，此流程也不解除用户接管状态。

`observe` 省略 `window_selector` 时沿用仍在授权窗口列表中的精确 window_ref；
原窗口消失时仅在唯一候选窗口下自动选择，多窗口不能按旧标题猜测。默认包含截图，
显式 `include_screenshot=false` 保留元数据恢复路径。按键在 MCP 输入边界忽略大小写，
Return/Esc 分别规范为 enter/escape；Runtime 保留最终支持范围校验。
显式观察的 `window_status` 保留观察前 `list_windows` 的 state、foreground、visible，
并标记来源；它是 best-effort 发现状态，不与截图原子同步，也不复用到动作后观察。
`unknown/offscreen` 和缩略图尺寸均不能证明最小化；台前调度恢复使用 `focus_window`。
原生窗口查找在派发前失败时返回 `not_dispatched`，不会创建虚假的待对账动作。
`indeterminate` 只允许原 request_id 做一次无重放对账；仍未知就停止并报告，不能
换 ID、重开 target 或启动应用来绕过待对账状态。
动作失败可能已消耗 observation，`stale_observation` 不表示 session 到期。
未派发的 stale 错误向模型明确保留 target、重新观察并重新决策；持续输入导致再次
失效时等待用户空闲，不能循环重试或重新授权。未派发的 `discover_applications`
恢复指引要求查询已运行应用并选择精确身份，不再重复启动。这些提示不覆盖
indeterminate 的原 ID 对账规则。
错误的模型可见 Content 与宿主 StructuredContent 同时保留 error_code、mutation_status、
retryable 和 recovery_action。后台截图失败额外明确元数据恢复必须同时传
`include_screenshot=false` 与 `accessibility="disabled"`；只禁用 AX 不会禁用截图。
`get_computer.actions` 是授权类别，`action_schema` 是模型动作参数；`action_aliases`
明确 `restore_window` 使用 `set_window_minimized` 授权，不能把两者误判为能力缺失。


## 动作后观察与接管

Windows 桌面宿主通过私有、当前用户限定的 named pipe 向自家 CUA 子进程传递
`AllowSetForegroundWindow` 资格。请求只能来自宿主拥有的后端，目标必须是其仍存活的
直接子进程；PID 不进入模型工具或 Web 请求。单次请求最多等待 500 ms，不使用
`ASFW_ANY`、模拟输入或线程输入队列绑定。OS 拒绝传递时仍由 Runtime 的原生聚焦与
实际前台窗口回读决定结果；此通道不授予目标、动作或会话权限，也不解除用户接管。

固定的 `v0.2.0-rc.4` SDK/Runtime 支持 `observe_after`。Nexus 在目标授权时读取能力，
每次动作沿用最近 `observe` 的截图、AX 和 `application_scene` 选项，让一次动作请求
返回新的观察；新的 guard 替换已消耗的旧 guard。场景观察仅包含锚点显示器上的同一应用。
Runtime 有界稳定等待不证明业务完成，必要时仍执行 `verify_state`。

`observe` 与 `perform_action` 的截图通过原生 MCP image content block 交付，
不把 Base64 图像放进 JSON 文本或 StructuredContent，避免触发大文本分页。
宿主将 SDK 单条 JSON 消息上限固定为 32 MiB，容纳 16 MiB PNG 编码与观察元数据。
SDK 读流提前关闭（包括超限）时，先关闭该子进程再等待其退出，不能在无人继续读管道时
等待仍在写入的进程而使工具永久保持运行。
持久化消息投影从两种操作的结果中移除图片内容和 AX 元素树，仅保留动作证据、
opaque refs、元素数量等摘要；此清理不改变提供给模型的即时结果。

后续观察或截图读取失败时，动作的 dispatched/effect evidence 必须保留，错误另列在
`observation_error`，不能自动重做动作。已完成请求的本地缓存仅保留动作结果，不保留
截图或 AX 树；重复 request_id 不派发动作，返回 `fresh_observation_required` 时另行观察。

macOS 两次动作间的输入保留原授权，等待连续空闲 2 秒（单次最多 5 秒），按键或按钮
仍按住时不算空闲。输入之前的观察失效，重新观察后决定后续动作；持续活动返回
`busy/not_dispatched` 与 `wait_for_idle_then_observe`，模型最多额外尝试一次。
等待不延长 session/round 到期时间。原生动作期间收到 `user_active` 后，本回合保留用户接管状态并拒绝后续新动作；fresh observe 不解除
此状态。只有新的 `select_target` 宿主授权可以恢复；未知结果仍按原 ID 对账。
Input Control 同时需要事件派发和 Input Monitoring，桌面引导补充输入监控请求及面板。

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

本地 CUA 预览构建可设置独立 `NEXUS_DESKTOP_BUNDLE_IDENTIFIER` 和
`NEXUS_DESKTOP_DEFAULT_STATE_ROOT`，把预览默认数据根写入 App，避免从 Finder
再次启动时回落到正式版数据。预览可用 `NEXUS_DESKTOP_REGISTER_URL_SCHEME=0`
避免接管正式版深链；此类构建不验收 OAuth 回跳。跨进程显示/退出通知按 bundle ID
隔离。`NEXUS_DESKTOP_PRESERVE_CUA_SIGNATURE=1` 验证并保留已测试 Runtime 字节，
随包带入它的 manifest、SBOM 和许可证；默认正式构建仍执行宿主签名流程。

## 启用生命周期

- 设置开关是唯一常规启用入口：未安装时先安装，再启用，随后自动读取一次系统权限状态。
- 启用状态持久化在 `state.json`；服务启动时以「环境变量启用 或 持久化启用」播种，
  且仅在存在可用二进制（已安装或 `NEXUS_COMPUTER_USE_BINARY`）时生效。
- `NEXUS_COMPUTER_USE_BINARY` 显式指定的绝对路径优先于已安装版本，用于打包内置与开发覆盖。
- 关闭开关会结束当前所有 CUA round、关闭 session 并停止 sidecar；重新启用从新 epoch 开始。
- round 关闭先禁止后续准入并向 Runtime 撤销现有 session，再等待动作互斥锁清理；不能等长按结束才发送撤销。关闭期间返回的新 session 必须立即关闭。撤销不能撤回已派发事件，原生停止能力以 Runtime 版本为准。
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
