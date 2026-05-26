# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Goal 长程任务对齐 Codex 语义：新增预算/用量限制状态、模型侧 `get_goal`/`create_goal`/`update_goal` 工具契约、续跑上下文和 `/goal` 控制命令。
- Goal 运行时新增 durable resume 后台恢复与 WebSocket 状态事件广播，服务重启后可继续推进 active Goal，前端 Goal 面板也能随状态事件刷新。
- Goal 新增 Codex app-server 风格 `thread/goal/set|get|clear` 兼容 HTTP 入口，返回 camelCase `ThreadGoal` 投影。
- Goal WebSocket 新增 Codex app-server 轻量 JSON-RPC 兼容入口，支持 `thread/goal/set|get|clear` 和 `thread/goal/updated|cleared` 通知。

### Changed
- Goal token 预算统计改为 Codex 口径：仅累计非缓存输入 token 与输出 token，缓存和 reasoning token 不再触发预算耗尽。
- Goal 工具完成结果补齐 Codex 风格最终用量汇报提示，便于模型在完成预算 Goal 时报告结构化用量。
- Goal MCP 模型可见工具收口为 Codex 对齐的 `get_goal`、`create_goal`、`update_goal` 三件套。
- Goal `update_goal` 工具入参收口为 Codex 风格的 `status` 字段，`blocked` 不再要求模型额外提供 reason。
- Goal `get_goal` 在当前线程没有 Goal 时返回空 Goal 结构化结果，不再把空状态误报为工具失败。
- Goal 创建时不再应用隐式默认 token budget，只有显式传入 `token_budget` 时才设置预算。
- Goal 工具结构化结果改为 Codex 风格 camelCase 字段，包括 `remainingTokens`、`completionBudgetReport` 和 `goal.tokensUsed`。
- Goal 工具结构化结果中的 `goal.tokenBudget` 改为 Codex 一致的显式 nullable 字段，无预算时返回 `null` 而不是省略。
- Goal 工具 schema 与文本结果继续贴近 Codex：`token_budget` 使用 integer 类型，成功结果文本输出 JSON payload。
- Goal round usage 绑定到本轮开始时的 Goal ID，模型在本轮完成/阻塞 Goal 后仍会补记最终用量。
- Goal 自动续跑对齐 Codex 的空进展抑制语义：隐藏续跑轮次未产生可计入工具进展时暂停下一次自动续跑，用户/外部活动或工具进展会恢复续跑。
- Goal app-server `thread/goal/set` 创建路径直接落最终状态，避免创建 paused/blocked/budgetLimited Goal 时短暂广播 active 状态。
- Goal 模型侧完成/阻塞状态更新保留变更前 usage flush，但不再提前清理本轮 runtime accounting，确保 `update_goal` 工具结果后的最终用量仍可补记。
- Goal runtime context 标记对齐 Codex `<goal_context>`，并在前端 Goal 面板直接展示剩余预算与续跑暂停状态。
- Goal 续跑与 steering prompt 移除 Nexus 私有措辞和内部 round ID，进一步贴近 Codex thread goal 模板。
- Goal runtime context 改为优先注入下一轮运行时上下文，bridge 暂不支持时降级为用户输入前缀；前端 Goal 面板新增运行上下文状态。
- Goal runtime context 中的 objective 与 checkpoint 摘要按 Codex 方式转义 XML 分隔符，避免用户目标内容闭合隐藏上下文。
- Goal app-server `thread/goal/updated` 通知补齐 turnId，模型轮次内的 Goal 更新可按触发 round 归因。
- Goal 运行中 objective/budget steering 改为注入 `<goal_context>` 命名上下文，不再包进通用 Nexus guidance。
- Goal `update_goal` 工具描述与续跑 prompt 统一为三轮同一阻塞条件后才能标记 blocked。
- Goal 面板新增独立运行态行，直观展示当前轮次、下轮续跑、空进展暂停、预算耗尽与阻塞状态。
- Goal 面板上下文状态文案从调试式 `goal_context` 改为用户可理解的运行状态表达。
- 聊天输入框新增 Goal 斜杠命令提示，输入 `/goal` 时可直接看到创建、查看、暂停、继续和清除入口。
- Goal `/goal` 斜杠命令移除 Codex 未提供的 `complete/done` 子命令，完成 Goal 仍通过模型工具或面板按钮触发。
- Goal `/goal` 斜杠命令继续收口到 Codex 文档语义，移除 `edit/start` 私有别名，编辑和继续操作分别保留在面板按钮与 `/goal resume`。
- Goal runtime context 移除要求模型记录 checkpoint 的私有提示，避免提示模型使用 Codex 三件套之外的不可见能力。
- Goal budget limit steering 的目标段落改为 Codex 模板一致的 `<objective>`，目标内容仍会进行 XML 转义。
- Goal objective 更新时也会按 Codex 语义尝试填充空会话预览，不再只在创建 Goal 时处理。
- Goal 自动续跑对齐 Codex plan mode 语义，目标 Agent 处于 Plan 模式时不会启动隐藏续跑，并在 Goal 面板显式展示暂停原因。
- Goal runtime 进一步对齐 Codex plan mode 语义，Plan 模式下不再注入 Goal 上下文、记录 Goal usage 或标记 Goal usage limit。
- Goal runtime context 进一步收口到 active Goal，paused/blocked/usage_limited/budget_limited 等停止态不再继续注入隐藏上下文。
- Goal budget_limited 状态下调高预算恢复 active 后，会立即尝试触发下一轮 Goal 续跑。
- Goal objective/budget 更新仅在归一化后实际变化时记录事件和触发 objective-updated steering。
- Goal 外部或用户操作把目标恢复为 active 时，会按当前运行中 round 的 usage 快照重置 Goal accounting 基线，后续用量继续归属该 Goal。
- Goal 面板的上下文状态会跟随 Plan 模式与空进展暂停展示，不再误提示 Plan 模式下会注入 Goal 上下文。
- Goal 面板耗时展示对齐 Codex，超过 24 小时时保留分钟，并将停止态上下文文案收口为不再注入。
- Goal 面板移除用户侧手动完成按钮，完成 Goal 继续交由模型 `update_goal(status=complete)` 审计后触发，对齐 Codex `/goal` 操作面。
- Goal native HTTP 与前端 API 移除用户侧完成/阻塞入口，完成和阻塞 Goal 只保留模型 `update_goal(status=...)` 审计路径。
- Goal active turn 不再额外注入常驻 runtime context，只保留用量/耗时记账；`<goal_context>` 收口到隐藏续跑和运行中 steering。
- Goal runtime 将 budget_limited 继续保留为本轮 usage accounting 目标，但不再注入 Goal 上下文，贴近 Codex 预算耗尽后的收尾结算语义。
- Goal active 状态会在运行时上下文读取和外部 mutation 前结算 wall-clock 用时，没有运行中 round 时也能对齐 Codex 的长程耗时统计。
- Goal 隐藏续跑在启动前会重新校验当前 active Goal，避免用户已暂停或替换目标后继续投递旧续跑。
- Goal `update_goal` 工具描述补齐 Codex 当前 blocked 审计语义，包括 resumed 后重新审计和 usage-limit 由系统控制。
- Room group runtime 中的 Goal MCP 工具改为绑定房间 shared session，房间成员完成/阻塞 Goal 时会更新同一个房间 Goal。

### Fixed
- 修复聊天侧边栏删除确认在删除请求失败时不会关闭的问题。

## [0.1.8] - 2026-05-21

### Added
- Windows 桌面 App 托盘右键菜单新增“检查更新”入口，可手动触发 GitHub Release 检测、下载和 sha256 校验安装链路。

### Changed
- `make app-win-build` 默认使用当前时间戳作为 Windows 桌面 app 构建号，方便未提交改动的本地临时测试；需要固定构建号时仍可通过 `APP_WIN_BUILD_NUMBER` 覆盖。
- 收口 Memory 调度与接口测试，提升记忆动态召回、checkpoint 和 HTTP API 的回归覆盖。
- Windows 桌面 App 点击窗口关闭按钮时改为隐藏到系统托盘，真正退出需通过托盘图标右键菜单执行。
- Windows 桌面 App 托盘右键菜单改为带标题、分组和悬停高亮的样式化菜单。

### Fixed
- 修复桌面 App 在 Windows/macOS 因 sidecar 本地端口变化导致引导完成状态每次启动丢失的问题。
- 修复点击 Nexus 或 DM 入口时未进入最近活跃会话的问题。
- 修复发送附件时同一文件可能被重复存储的问题。
- 修复 Windows 桌面 App 自动更新检查在请求前写入 24 小时节流状态，导致失败后后续启动被误判为近期已检查的问题。
- 修复 Windows 桌面 App 受系统“动画效果”关闭影响时，首页 Nexus 动效被 WebView2 的 reduced-motion 媒体查询完全降级为静态文字的问题，并在启动日志中记录 reduced-motion 状态便于排查。
- 修复 Windows 桌面 App 关闭主窗口后可能仍残留壳进程和 sidecar，导致下一次临时构建覆盖 `.build/app/Nexus` 时文件被占用的问题。
- 修复 Agent 启动失败时 WebSocket 只返回笼统内部错误，缺少 Claude Code 或 Provider 配置指引的问题。
- 修复 Windows 上通过 npm 安装 Claude Code 只提供 `claude.cmd` 时，Agent runtime 仍按 `claude.exe` 查找而初始化失败的问题。
- 修复 Windows 桌面 App 导出日志时，正在写入的 sidecar 日志文件可能因文件共享锁导致导出失败的问题。
- 修复 Windows WebView2 未写入 `nexus_desktop_token` cookie 时，WebSocket 握手可能被 sidecar 判定为桌面会话 token 无效并返回 401 的问题。

## [0.1.7] - 2026-05-20

### Added
- 新增 Nexus Memory v1：支持本地 Markdown 真相源、自动动态召回、候选提升、checkpoint 去重、`nexusctl memory` 管理命令、HTTP API 与 Web Memory 面板。
- 聊天消息完成后新增通知闭环：非激活窗口触发浏览器系统通知，左侧聊天入口和会话行显示未读完成消息数，进入对应会话后自动清除。
- 工作区文件预览支持 Markdown、HTML、Mermaid、图片、SVG、PDF 和普通文本，并在预览区、聊天文件卡和文件右键菜单提供统一下载入口。
- 桌面 App 内置 GitHub OAuth Device Flow：发布包只注入公开 Client ID，用户输入 GitHub 授权码后由本地 sidecar 轮询并保存 token。
- 桌面 App 本地模式默认跳过账号登录，由原生壳注入的本地 session token 保护 sidecar API。

### Changed
- `make logs`、`make logs-all` 与 `make logs-nginx` 默认显示最近 1000 行，便于直接查看启动前后的服务日志。
- 移除 Makefile 中针对 bridge SDK 的额外可访问性预检查，安装、迁移、生成协议和发布包构建直接使用 Go 模块工具链校验依赖。
- 连接器前端不再提供 OAuth App 自助配置入口，统一由后端环境变量或桌面内置配置决定是否可连接。
- 优化 Markdown/预览流式输出：按 block 显式区分已稳定内容和流式尾块，未闭合代码围栏直接对齐真实内容，Mermaid 流式预览保留上一版合法 SVG，代码块流式期间跳过完整高亮，HTML 预览按 head 就绪和节流提交减少重载抖动。
- 优化 Markdown 表格渲染：修正公式与 GFM 表格解析顺序，并让宽表格在自身容器内横向滚动。
- 优化 Markdown 列表渲染：修正列表项段落块导致 marker 后内容另起一行的问题。
- 优化 Markdown 文本渲染：支持安全的行内文本标签与 `<br>` 换行，并改善正文段落换行观感。
- 优化 Mermaid SVG 渲染观感：统一边标签背景、节点圆角、note 配色和菱形节点圆角处理。

### Fixed
- 修复 Markdown 中 `Cron*（...）` 这类标识符星号被误解析为强调标记的问题。
- 修复工作区文件编辑/预览工具栏按钮点击文字区域时先触发编辑器失焦，导致视图跳动的问题。
- 修复 Agent 任务结束后工作区文件状态可能停留在“写入中”的问题。
- 修复用户消息正文在右侧气泡中未按发送方方向对齐的问题。
- 修复用户消息附件打开后文件树误聚焦到 `.nexus/attachments` 内部目录，导致刷新后附件预览路径异常的问题。
- 修复图片附件只作为 `@"path"` 文本传入 runtime，导致首轮对话不稳定触发读图的问题，并对齐 Claude Code 的 `source.base64` 图片内容块。
- 修复聊天未读只记在全局入口、会话行不显示且点击未进入对应未读会话的问题。
- 修复 Windows 安装器在 Windows 11 ARM64 x64 兼容环境下因 Inno Setup 架构约束误报不支持当前 Windows 版本的问题。
- 修复 Windows 桌面 App 内聊天、侧边栏订阅和完成通知 WebSocket 未携带桌面会话 token，导致连接被本地 sidecar 拒绝的问题。
- 移除桌面发布包内 GitHub OAuth Client Secret 注入，避免分发产物暴露 confidential client secret。
- 修复 macOS Dock 点击重新打开时把当前工作台路由重置到 launcher 的问题。

## [0.1.6] - 2026-05-20

### Added
- Windows 桌面应用启动更新检测补齐下载安装链路：按 24 小时节流读取 GitHub Release 的 Windows metadata，发现新版本时可下载 `NexusSetup-*.exe` 与 sha256，校验通过后再提示启动安装器。
- Windows 桌面发布链路新增 Inno Setup 安装包，输出 `NexusSetup-<version>-<build>.exe`、sha256，并注册开始菜单、可选桌面快捷方式和 `nexus://` 协议。
- Windows 桌面应用接入 Nexus app 图标，打包后的 `Nexus.exe` 会显示独立应用图标。
- macOS 原生菜单新增“检查更新...”，启动后会按 24 小时节流后台检测 GitHub Release，并在发现新版本时提示打开下载页。
- Windows 桌面第一阶段新增 WPF/WebView2 原生壳骨架，支持启动 Go sidecar、随机本地端口、runtime config 注入、完整 launcher 默认入口、单实例唤起、`nexus://` 路由、DPAPI 凭据 key、基础桌面 bridge、诊断导出、smoke 脚本、zip/metadata 打包和 GitHub Release app asset 上传。
- 对话输入框支持粘贴图片，并可上传图片、PDF、Office、Markdown、HTML 与常见文本文件作为工作区附件。

### Changed
- 桌面 app 的运行数据目录统一为 `~/.nexus`，macOS 与 Windows 不再分别使用 `Application Support/Nexus` 或 `%LOCALAPPDATA%\Nexus`。
- 聊天附件改为结构化 metadata 传递，正文不再拼接文件清单或内容摘录，DM/Room 待发送队列和历史回放会保留附件信息；Room 群聊附件上传到 conversation 级公共目录。
- 文件类工具执行成功后会写入结构化工作区文件产物，并在聊天区提供单击打开入口。

### Fixed
- 修复 macOS 桌面 smoke 在未登录状态下把 `/login` 误判为启动失败的问题。

## [0.1.5] - 2026-05-19

### Added
- Room 创建与管理支持设置群主，并可启用未 @ 公区消息由群主默认接管后回答或委派成员。
- GitHub Release 发布流程新增 macOS app 构建 job，并把 dmg、sha256、metadata 作为同一个 tag 的 Release assets 上传。
- macOS 桌面 smoke 支持 CI 友好的 launcher 分布式通知兜底和可配置 fallback reveal 容忍度。
- 新增 macOS app QA 清单，并补充 WebView 外链/阻断、launcher 关闭原因和 WebContent 终止诊断记录。
- Makefile 新增 macOS app 开发、构建、运行、smoke 和打包入口。
- macOS 桌面新增 Nexus 概念 App 图标，并接入 `.app` bundle。

### Changed
- 重做侧边栏聊天工作台：联系人、能力入口、最近会话与 launcher 控制台的信息结构更清晰。
- macOS app 默认启动和 `nexus://launcher` 统一打开主窗口完整 launcher 首页，移除独立紧凑 launcher 浮层，关闭 `Option + Space` 默认全局唤起，并移除设置页里的启动器快捷键配置。

### Fixed
- 修复 Room slot 状态并发访问风险，并稳定 Room 异步清理测试。
- 修复 `nexus-server --help` 会提前触发迁移的问题。
- 修复聊天区侧栏 tab 激活态在路由切换后丢失的问题。
- 修复 macOS app 已运行时再次打开不会唤起 launcher 的问题。
- 修正 macOS smoke 对默认 launcher 路由的校验，确保启动和 URL 唤起都落到 `/`。

## [0.1.4] - 2026-05-19

### Added
- 新增 Nexus 版本展示入口：发布包注入版本号、Git commit 与构建时间，`/system/version` 返回当前二进制信息，Web 设置页提供 GitHub Release 下载入口。
- 补充 Windows 发布包运行说明，明确 Claude Code、PowerShell、WinGet 与 Git for Windows 的安装路径。

### Changed
- Agent workspace 目录改为按 `agent_id` 生成，改名时不再移动目录，只同步数据库名称与工作区 `AGENTS.md` 身份标识。
- Workspace 初始化增强 Windows 兼容：补充 `nexusctl.cmd` 入口，Claude skill 链接在目录 symlink 不可用时会镜像目录。
- 跳过新手引导时立即记为已读，避免后续反复出现同一导览。

### Fixed
- 修复发布包首页点击“进入工作台”后仍停留在 Launcher 的问题。
- 修复 Windows 下 Agent 改名时因 workspace 目录被占用导致失败的问题。
- 修复 SQLite URL 中 `~` 与 Windows 路径分隔符展开不完整，以及 SQLite 父目录不存在时打开数据库失败的问题。

## [0.1.3] - 2026-05-15

### Added
- 发布包进入可直接运行阶段：Linux 与 Windows 运行包内置服务端、前端资源、数据库迁移和内置技能，启动后即可通过同一个本地地址访问 Nexus。
- 图片生成能力成型：支持独立的图片生成 Provider、内置 `imagegen` 技能，以及会话内图片结果预览。
- Room 协作动作增强：支持私域消息、请求指定成员回复、小范围受众投递、延迟唤醒和房间级技能规则。
- 桌面端内部验证链路完成第一阶段：本地 sidecar、独立窗口、桌面会话凭据、启动诊断和内部验证包已具备闭环。

### Fixed
- 会话运行态以真实执行中的任务为准，减少异常退出或中断失败后仍显示“对话中”的情况。
- Room 删除会完整清理成员、会话、消息和执行记录，避免残留数据影响后续使用。
- Room 私域动作的来源身份由运行时统一注入，避免模型侧伪造或误填发送者。
- 私域动作默认不在工具结果中回显正文，降低协作过程里的信息泄漏风险。

## [0.1.2] - 2026-05-12

### Added
- DM 与 Room 输入区新增待发送队列：运行中或已有排队消息时，Enter 会把新输入加入队列，队列项支持手动引导、删除与拖拽排序。
- 常规设置新增用户级默认消息行为与新建 Agent 默认权限模式；默认消息行为仅支持排队/打断，偏好写入 workspace JSON，不新增数据库表。
- bypass 权限模式保留 AskUserQuestion 交互通道，其余工具自动放行。
- 会话配置热更新替代 stale 全量淘汰：权限模式与模型支持原地切换，需要重连的配置变更（cwd、MCP servers 等）标记为待重连、下次请求时自动生效。
- Agent 工作区技能管理：显示已安装技能、允许移除并增加移除确认，防止重复提交。
- 定时任务链路完善：智能体选择与回传计数刷新。
- IM 频道与配对管理：新增频道 CRUD、配对绑定与运行层，标记为未上线预览。
- 后端 API 路径前缀统一为 `/nexus/v1`。
- 编辑器面板新增 Markdown 预览/编辑模式切换。
- `task_started` 系统消息类型：后端格式化 + 前端展示样式。

### Changed
- 输入框不再内联展示”排队 / 引导 / 打断”选择，默认消息行为改由常规设置统一控制；引导只保留为待发送队列项的手动动作。
- 常规设置页按外观、常规、权限分区展示，文案与控件更紧凑；偏好项选择后即时保存，权限设置收敛为四种权限模式下拉选择。
- DM 与 Room 的”引导”改为持久队列状态：点击后不会立即消失，只有对应 round 的 PostToolUse hook 真正注入时才消费。
- 引导消息历史改为从 Claude transcript 的 `hook_additional_context` 回放，不再写入 overlay 作为重复真相源。
- Room 公区消息命中正在回复的 Agent 时不再强制中断该 Agent；忙碌目标会通过 SDK streaming input 接收补充上下文，空闲目标仍正常启动新 round。
- Room 公区上下文改为按成员 cursor 投递增量；固定协作规则进入 SDK append system prompt，每轮动态输入仅保留公区增量与一行自然消息形式的触发信息。
- DM 回复中也可继续输入补充要求，新消息会排入当前流式会话，不再默认杀掉正在执行的任务。
- 代码块精简样式：移除红黄绿圆点、缩小圆角、复制按钮改为 icon-only、横向滚动替代自动换行。
- 前端函数与 props 命名统一为 snake_case（126 文件）。
- 前端目录按功能域二次拆分：types/hooks/lib/features/workspace 细化为子域目录。

### Security
- SDK 调试日志内容脱敏。

### Fixed
- 修复引导队列在当前 round 没有工具调用时被提前消费，导致消息既未注入也不再可见的问题。
- 修复 SDK 未返回 `result` 但 assistant 已完整 `end_turn` 时，DM/Room 轮次被误判为提前关闭的问题。
- 修复 Room 公区后续上下文漏掉无 SDK `result` 的完整 assistant 回复，以及手动引导队列项被公区增量覆盖的问题。
- 修复引导队列在特定条件下卡住无法继续消费的问题。
- 修复 DM 流式输出卡住的问题。
- 增强 Room round 断流诊断。
- 修复服务启动时未自动执行数据库迁移的问题。
- 修复 heartbeat 状态在并发访问下的数据竞争。

## [0.1.1] - 2026-04-25

### Added
- Room 公区协作机制收敛：新增 `room-collaboration` system skill、公区 @ 提及唤醒、Agent 公区发言后的后续 @ 触发，以及无需回复标记输出过滤。
- 个人设置页新增头像设置，复用 Agent 头像资源，并将头像同步到个人资料与登录状态。

### Changed
- 前端与 Docker 部署链路切换到 pnpm：新增 `pnpm-lock.yaml`，移除 `package-lock.json`，并同步更新 makefile、Web 构建镜像、运行镜像与容器内工具链 registry 配置。
- Room 公区上下文改为只向 Agent 注入公区用户消息和其他 Agent 的最终公开结果，不再把工具调用、thinking、tool result 等中间过程塞进其他成员上下文。
- Room 输入区恢复为“只限制正在回复的目标 Agent”，其他 Agent 回复中仍可继续发送普通消息；Room Thread 面板不再因结果消息到达自动关闭。
- Agent 改名允许仅大小写变化的合法更新，同时继续阻止真正重复的名称。

### Fixed
- 修复 Docker 多阶段构建并发复用 apt cache 时可能抢占 `/var/cache/apt/archives/lock` 导致安装失败的问题。
- 修复 Docker 构建时 Corepack 通过 npmmirror 拉取 pnpm 元数据返回 404 的问题，改为用 npm 安装固定版本 pnpm。
- 修复 usage 落账遇到 SDK JSON 数字类型时被判空，导致设置页 token 用量无数据的问题。
- 修复个人头像未显示在 DM、Room 主消息区和 Room Thread 用户消息上的问题，并确保头像变更会触发消息项重渲染。
- 修复 Room 中被无需回复标记过滤的轮次没有写入 token usage 台账的问题。
- 修复 Room 公区上下文注入缺少部分成员公开结果、以及中间过程误进入其他 Agent 输入的问题。
- 修复 Room 新公区消息按 shared session 中断整轮的问题，改为只停止被再次点名的目标 Agent。
- 修复 Room 主动中断后 SDK 流提前关闭被误判为 `round stream closed before terminal` 错误的问题。

## [0.1.0] - 2026-04-24

### Added
- Go 后端主线正式落地：新增 `nexus-server`、`nexus-migrate`、`nexusctl`、协议生成、Goose 迁移，以及 `gateway / protocol / runtime / chat / room / session / workspace / skills / connectors / automation` 分层。
- 浏览器登录与多用户体系落地：支持 HttpOnly Cookie 会话、服务端 session 撤销、用户级主智能体、workspace、room、session 与 skill/connector 数据隔离。
- DM / Room 对话链路升级：接入 `transcript + overlay / transcript_ref` 历史真相源、共享 round 执行内核、多观察者单控制端、Room 重连恢复与权限定向派发。
- Capability 能力面成型：技能市场持久化、结构化定时任务 API/UI/MCP 工具、heartbeat/cron 自动化运行时、GitHub Connector OAuth 自助配置与 `nexus_connectors` MCP 工具。
- Workspace 与外部入口扩展：新增 workspace live 订阅、文件资源块展示、Discord / Telegram 通道入口，以及 Agents / Contacts / Rooms / Settings / Scheduled Tasks / Connectors 等主界面能力。
- 部署链路升级：新增 Go 多阶段 Docker 镜像、nginx 网关、生产健康检查、GitHub Release workflow、运行镜像内置 Agent 工具链与 Docker 管理员自举。

### Changed
- 默认开发、构建、迁移、校验与发布链路切换到 Go 后端；`make dev`、`make db-init`、`make check`、Docker 与 release workflow 均围绕当前 Go 主线运行。
- 网关与业务结构全面收口：HTTP handlers 按领域拆包，共享中间件下沉到 `gateway/shared`，DM / Room / ingress / automation / websocket 入站路由统一由 `Dispatcher` 协调。
- 会话与历史模型收口：运行时不再依赖 legacy `messages.jsonl` 正文链路，session / room 目录统一为可读语义路径，历史读取以 Claude transcript 与 Nexus overlay 为边界。
- `nexusctl` 改为 Agent 友好协议：新增全局 `--json` / `--pretty` / `--verbose`，stdout/stderr 职责分离，成功与错误结构统一，并支持 `--password-stdin`。
- 前端应用结构重组：统一同源 API 客户端、WebSocket 绑定语义、conversation identity、运行态状态机、页面级 controller 与更完整的 onboarding/help 入口。
- 自动化工具参数对齐 UI：`schedule`、`execution_mode`、`reply_mode`、agent scope、cron 回翻与 lenient 默认值统一到可编辑、可审计的任务模型。
- 文档同步到当前架构：README、env 示例、部署说明，以及 session key、permission runtime、main agent、message processing、skill、room、frontend design 等 spec 已精简重写。

### Fixed
- 修复 runtime client 失效、provider/model 热更新、`bypassPermissions` 权限处理、工具参数错误展示、文件路径展示、SDK 依赖预检与 Docker 内 skill 根目录解析问题。
- 修复 DM / Room 在权限确认、停止生成、AskUserQuestion、多窗口观察、重连恢复、活跃态判断与输入框状态上的多处不一致。
- 修复多用户部署下 `nexus-manager` / `nexusctl` 作用域丢失，避免跨用户读取或操作 agent、room、session、workspace、skill 数据。
- 修复本地迁移、Alembic 多 head、旧认证域结构、Go 迁移识别、前端依赖安装与 release workflow 仍引用旧 Python 链路的问题。
- 修复安全与并发问题：Zip Slip 路径遍历、Token 时序侧信道、敏感配置脱敏、Resp 全局单例突变、裸 `except` 与异常变量引用错误。

### Removed
- 删除旧 Python 运行链路、legacy sync/backfill、历史迁移 CLI、旧 workspace runtime layout 迁移、成本账本回填和多处旧字段兼容逻辑。
- 删除 `messages.jsonl` 作为运行时正文真相源的路径，移除旧 session 双写、旧 base64/短 hash 目录布局和旧 result 投影迁移。
- 删除前端旧 conversation store、home conversation controller、loading 手动状态、旧 StreamingCursor 组件与过时的 Session/Workspace 辅助结构。

## [0.0.3] - 2026-03-18

### Fixed
- 修复 Markdown 有序列表在消息区渲染时编号与正文被拆成两行的问题，`1.` 之后不再出现异常换行。

### Changed
- 统一前端主界面视觉风格，聊天工作区、侧栏、状态栏、输入区和空状态切换为同一套软拟态设计语言。
- 统一消息内部块样式，`thinking`、工具执行块、问答块、代码块与消息统计条改为同心圆角和一致的面板层级。
- 统一配置与确认流程弹窗样式，`AgentOptions`、权限确认和确认/输入对话框与主界面视觉保持一致。
- 收敛任务浮层与 Markdown 表格等剩余组件的圆角、描边与阴影节奏，减少界面风格割裂。
- 新增 `Agent / Profile / Runtime / Room / Conversation / Session` SQLite ORM 模型与 Alembic 初始迁移，建立新的站内协作数据骨架。

## [0.0.2] - 2026-03-17

### Fixed
- 修复删除 Agent 时仅归档记录、不回收工作区目录与活跃 session 的问题，删除后不再残留旧 workspace。
- 修复 `thinking` 在后续 assistant 快照到达后被覆盖消失的问题，思考块现在会稳定保留在同一轮消息中。
- 修复 `tool_result` 被拆成独立 assistant 气泡的问题，工具结果现在会回填到对应 assistant 段内展示。

### Changed
- 重写后端消息处理器，按 SDK 实际消息节奏收口为更薄的 `ChatMessageProcessor + AssistantSegment + SdkMessageMapper` 结构。
- 收紧前端流式边界：只有 `thinking / text` 参与 `StreamMessage` 增量渲染，工具调用与工具结果统一走完整消息快照。

## [0.0.1] - 2026-03-14

### Fixed
- 修复 `thinking` / 文本流式内容在前端被二次打字机动画延迟显示的问题，恢复按后端真实 chunk 即时渲染。
- 修复消息流式链路中 assistant 段收束、工具结果插入与同一 `message_id` 更新顺序不稳定的问题。
- 修复 `TodoWrite` 提取、会话删除与工作区侧栏在空块/空 `session_key` 场景下的前端异常。

### Changed
- 重构消息协议边界，新增 `StreamMessage`，统一后端流式消息、最终消息与前端消费模型。
- 调整 WebSocket / IM 发送层，显式区分 `message`、`stream` 与 `event` 三类传输。
- 默认向 SDK 透传 `include_partial_messages`，并同步收敛前端无效的流式/轮次配置项。
