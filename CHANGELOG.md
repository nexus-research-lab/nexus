# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 操作舞台新增待机桌面空态，使用粒子化 `nexus` 标识与时钟展示新 session 的初始状态。

### Changed
- 操作舞台新增工具调用到桌面意图与 app session 的投影层，Bash/open HTML、web_search 和 fetch 会更新并聚焦 Terminal/Safari 持久窗口。
- 操作舞台按工具类型区分读、写、运行、浏览器、知识与任务窗口，并重做 Dock、窗口聚焦、终端输出与文件预览呈现。
- 操作舞台窗口支持通过标题栏拖动位置，并在拖动时保持窗口置顶与内部内容滚动独立。
- 操作舞台新增工作台叙事阶段、事件流轨道、窗口逐步登场与完成沉淀摘要，弱化一次性 demo 布局感。
- 操作舞台完成态升级为执行胶片与关键产物收纳视图，方便从工具调用流回看最终现场。
- 操作舞台终端窗口改为按命令 session 展示 stdout/stderr、耗时、退出状态与运行光标，提升真实执行感。
- 操作舞台浏览器窗口新增导航栏、地址栏、来源标签、加载状态与 live preview/fallback 呈现，强化网页与产物预览的现场感。
- 操作舞台文件窗口升级为工作区文件树与编辑器表面，展示文件状态、diff 统计、行号、语言标签和编辑器状态栏。
- 操作舞台事件流升级为运行态焦点控制台，突出当前工具、窗口焦点、已沉淀步骤与现场窗口数量。
- 操作舞台新增工作台航线视图，将本轮工具调用按真实执行顺序、工具类型与状态串联展示。
- 操作舞台收束态新增结果落盘/交接状态，串联轨迹归档、关键产物和可继续上下文。
- 操作舞台任务、权限与证据窗口升级为子任务控制台、审批检查点和证据检查器，补齐非终端工具调用的真实工作台语义。
- 操作舞台从 `nexus` 字符空态进入执行现场时改为粒子让位式过渡，避免白色遮罩覆盖工作台。
- 操作舞台字符空态退场时新增启动信号，展示首个工具类型、目标和从字符场到执行现场的进度线。
- 操作舞台新增执行幕次引导，将字符场进入、工具接管现场、结果沉淀串成连续叙事，并收窄工具名启发式匹配，避免非工具名被误分类。
- 操作舞台窗口登场改为按已发生工具数量保底显示，避免后续工具运行时前序工具窗口从现场消失。
- 操作舞台完成态新增交接清单，按真实事件、证据和产物判断轨迹归档、产物可打开、证据可追溯和后续可继续状态。
- 操作舞台在新工具事件到达时自动恢复并聚焦对应窗口，保持事件流驱动的现场焦点。
- 操作舞台字符空态新增待机状态面板，明确新 session 进入的是已就绪的工作台入口，而不是空白组件展示。
- 操作舞台等待确认态新增独立幕次表达，将权限检查点、用户介入与继续执行从普通运行态中区分出来。
- 操作舞台等待确认窗口升级为居中的执行检查点，展示请求命令、载荷、证据与继续路径，并在等待态抢占现场焦点。
- 操作舞台完成态新增现场归档条，将窗口现场、关键产物和执行轨迹压缩成可回看的工作台 archive capsule。
- 操作舞台后续工具事件到达时新增接入态过渡信号，并避免同一事件重复刷新打断当前叙事状态。
- 操作舞台真实投影链路将 workspace live 文件事件归回对应工具 round，并优先使用本轮 summary 作为完成态 active event，避免完成场景被后到文件事件切碎。
- 操作舞台命令类工具保留真实 `tool_result` 的内容、错误码和错误状态 envelope，终端窗口可从真实结果中还原输出与退出状态。
- 操作舞台新增 live round 预运行投影，在模型已开始回复但首个工具尚未出现时进入“运行接入中”叙事态，避免舞台停在待机。
- 操作舞台 summary 事件开始时间回溯到同轮首条消息，让状态卡和归档区展示真实 round 耗时。
- 操作舞台新增运行接入窗口，专门承载模型已开始但首个工具尚未出现的阶段，展示请求意图、上下文装载与等待工具事件。
- 操作舞台完成态新增执行清单窗口，将工具回放、交付物、证据和耗时沉淀为可打开的 `run-manifest.md` 工作记录。
- 操作舞台执行回放、事件流与工作台航线支持点击事件聚焦对应窗口，让完成态从静态记录升级为可回放工作现场。
- 操作舞台多文件工具流会按目标文件沉淀多个文档窗口，历史读写事件可回到各自文件现场而不是被最新文件覆盖。
- 操作舞台读写修改类文件事件会优先聚焦具体文档窗口，Workspace 文件树退为背景上下文，让运行中工具更像真实打开文件编辑。
- 操作舞台运行接入窗口新增等待时长与接入等待过久状态，真实运行长时间没有首个工具事件时不再只显示普通连接中。
- 操作舞台运行接入窗口会识别模型 API 重试状态，并在舞台叙事、伪终端和执行幕次中明确显示重试而不是普通等待。
- 操作舞台完成态将运行中的事件流切换为工作台交接账本，展示最后轨迹、产物、异常与可继续状态，让执行收束更像真实工作记录。
- 操作舞台抽出可复用体验状态机，显式标记 `idle`、`awakening`、`running`、`settling`、`completed` 阶段并对核心判定加校验。
- 操作舞台 workspace live 文件事件按当前 session 与工具调用归属过滤，文件树和完成态窗口不再混入同一 Agent 的旧会话文件。
- 操作舞台 pending permission 改为按当前 session/agent 与精确工具输入匹配，避免旧会话权限或同名工具权限误入当前工作台。
- 操作舞台执行清单新增交接摘要，沉淀完成状态、继续上下文、关键产物与检查点，让完成态更像真实可续作的工作档案。
- 操作舞台完成态侧栏新增续作提示，将下一步可继续的产物、状态解释和检查点提升为舞台级信息，不再只藏在执行清单窗口里。
- 操作舞台运行态新增 live episode 叙事，明确刚刚沉淀的工具、当前焦点工具、下一步等待条件与执行检查点，让工具流更像真实逐步发生。
- 操作舞台 active event 选择改为跟随最新执行 round，避免历史未收束工具抢占新一轮完成现场。
- 操作舞台恢复远端快照时会合并同轮次历史事件、工作区产物与证据，避免刷新后首屏消息投影覆盖已沉淀的完整工作现场。
- 新增 `pnpm --dir web verify:operation-stage` 校验脚本，覆盖完成态 summary 焦点、workspace live round 归属和命令结果 envelope 不变量。

### Fixed
- 修复新 Room session 复用旧操作舞台快照的问题，并隔离 workspace live 事件避免旧会话内容污染新舞台。
- 缺失操作舞台快照时返回空结果，避免新 session 打开时产生预期内的 404 告警。
- 修复 synthetic/API 认证失败被操作舞台误判为成功完成的问题，错误收口现在会进入失败叙事并保留错误证据。
- 修复 synthetic/API 认证失败的 summary 预览仍显示成功 envelope 的问题，错误窗口内部数据现在与舞台状态保持一致。
- 修复真实运行失败后旧的 live handoff 仍被标记为执行中的问题，同轮 summary/error 到达后会把早先运行事件沉淀为失败证据。
- 修复完成态终端窗口继承 summary 事件身份的问题，归档后的终端窗口现在保留真实命令事件与输出上下文。
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
