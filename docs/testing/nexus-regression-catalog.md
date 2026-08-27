# Nexus 维护者回归测试总目录

本文把 Nexus 近期反复出现过的生产故障、跨 runtime 对齐和桌面端回归整理成可重复测试。它是测试入口，不替代代码测试；历史结果只用于说明风险，发布结论必须来自当前构建的重新执行。

详细滚动场景见[会话打开与最新消息滚动回归测试](./conversation-latest-scroll-regression.md)。

## 使用规则

1. 先记录 product、bridge、nxs 三个仓库的 Commit，以及实际运行的 App 和 runtime 二进制路径。
2. 涉及真实 UI 时，同时保留截图、日志时间点和唯一消息标记。DOM、无障碍树或数据库中存在数据，不代表用户当前可见。
3. 对比 runtime 时使用相同 Provider、模型、权限、MCP、Skill、工作区和输入；每个 runtime 建立独立新会话，避免前一个会话污染结果。
4. 缓存一致指应稳定的 system prompt、工具 schema 和消息前缀保持稳定，不要求不同 runtime 的 token 数完全相同。
5. 破坏性测试使用隔离的 `NEXUS_STATE_ROOT`。生产环境只做必要的只读核查，不在文档或测试记录中保存密码、Token、私有记忆正文和完整配置。
6. 历史通过项在新版中仍视为“未测”，不能沿用旧结论。

## 验证层级

| 层级 | 使用时机 | 最低门禁 |
| --- | --- | --- |
| 定向 | 日常窄改动 | 受影响 Go 包或 Web 脚本，加 `git diff --check` |
| 常规 | product 功能闭环 | `make check`，再执行本目录对应的真实 App 用例 |
| 跨 runtime | bridge、nxs、工具、缓存、记忆或压缩改动 | product 定向测试，bridge/nxs 受影响包测试，NXS 与 Claude runtime 的相同输入实测 |
| 发布 | 桌面升级、共享协议、隔离基础设施或正式发布 | `make check-go-full`、Web 构建、桌面构建与 smoke；bridge/nxs 在各自仓执行 `GOWORK=off go test ./...` |

常用入口：

```bash
make check
make app-run
make app-check
```

Windows 桌面 smoke 必须在 Windows 环境执行：

```powershell
make app-win-smoke
```

## 1. Runtime、工具、缓存与压缩

### RUN-01 默认工具面

在全新 NXS 会话读取初始化工具清单，确认默认可见工具为：

```text
Agent
AskUserQuestion
Bash
Edit
EnterPlanMode
ExitPlanMode
Read
Skill
TaskCreate
TaskGet
TaskList
TaskOutput
TaskStop
TaskUpdate
WebFetch
WebSearch
Write
read_result
```

同时确认以下工具没有默认加载，也没有残留在默认提示词和延迟工具公告中：

| 非默认工具 | 当前默认替代或处理 |
| --- | --- |
| `DiscoverSkills` | 使用 `Skill` 发现并调用 Skill |
| `Glob`、`Grep` | 使用 `Bash` 调用随 NXS 发布的 `rg`；`rg` 即 ripgrep |
| `TodoWrite` | 使用 Task v2 的 `TaskCreate/Get/List/Update/Output/Stop` 六个工具 |
| `NotebookEdit` | 无默认等价工具；明确的 Jupyter 场景才单独开启 |
| `EnterWorktree`、`ExitWorktree` | 默认在当前工作区执行；明确编程隔离场景才开启，默认 Agent 提示词不应描述 worktree |

自动化真相源：

```bash
cd ../nexus-agent-sdk-go
go test ./internal/tools/runtime ./internal/query
```

### RUN-02 ToolSearch 默认关闭

分别用默认环境、显式开启和显式关闭三组配置启动会话：

- 默认环境不向模型暴露 `ToolSearch`，也不注入 deferred-tools 公告。
- 只有显式开启时才省略延迟工具 schema，并在发现后稳定恢复对应 schema。
- 关闭实验能力时，即使 Provider 支持，也不得偷偷开启。
- 开启后的公告只存在于请求期，不写入 transcript；同一会话的公告集合冻结，不能因 MCP 热变更破坏缓存前缀。

### RUN-03 NXS、Claude runtime 与原生 Claude 客户端对比

使用同一条输入分别运行：

```text
请先概括任务，再读取一个文件，搜索一个标记，最后只报告结果和使用过的工具。
```

记录以下字段：

| 对比项 | 通过标准 |
| --- | --- |
| runtime/model | App 统计、初始化响应和后台日志一致，不发生静默 fallback |
| 工具名称与 schema | NXS 和 Claude runtime 的共有能力语义一致；产品专属控制面差异有明确来源 |
| 工具顺序 | 同一 runtime 的相邻轮稳定，不能无业务变化地重排缓存前缀 |
| 权限行为 | 相同权限模式下，读、写、Shell 和 Skill 的允许/拒绝边界一致 |
| 工具错误 | 错误只出现一次，模型能按提示恢复，不形成重复调用循环 |
| 最终回复 | 不暴露内部提示、控制事件或 transport 文本 |

原生 Claude 客户端只作为 CC 行为参照，不要求 Nexus UI、产品控制工具和 token 统计完全相同。

### RUN-04 KV cache 稳定性

每个 runtime 使用独立新会话，连续执行三轮：

1. 首轮发送较长且固定的输入，建立缓存。
2. 第二轮发送短追问，不改 Provider、模型、工具、MCP、Skill 和系统配置。
3. 第三轮只改变一个明确变量，例如新增 MCP，再观察预期的缓存边界变化。

通过标准：

- 第二轮应复用可缓存前缀；记录 `cache_creation_input_tokens` 和 `cache_read_input_tokens`。
- 不要求 NXS 与 Claude runtime 的数值相同，只要求各自该缓存的部分能缓存。
- 工具数量、schema 字节数、beta header、system prompt 分段和 compact 元数据应在未变更时稳定。
- 第三轮的缓存变化必须能归因到那个显式变量，不能因 ToolSearch 环境泄漏或工具顺序漂移掉档。

### RUN-05 microcompact、完整压缩与恢复

1. 在长会话中写入两个唯一事实、一个未完成任务和一次成功的文件读取。
2. 让会话依次触发 microcompact 和完整压缩。
3. 压缩后追问两个事实、任务状态，并继续编辑已读取文件。
4. 退出 App 后恢复同一会话，再重复追问和编辑。

预期：事实与当前任务不丢失，工具调用配对完整，已发现工具和缓存边界可恢复；压缩不会把内部续跑提示投影为用户消息。

### RUN-06 Read-before-Write 保护

1. 准备已存在文件，在新会话中直接调用 `Edit`。
2. 确认返回 `File has not been read yet. Read it first before writing to it.`。
3. 调用 `Read` 后再次 `Edit`，应成功。
4. 用失败的 `Read` 重试，确认失败读取不能解锁编辑。
5. 恢复含成功 `Read` 记录的会话，确认读取状态可恢复。

这个错误本身是防止覆盖陈旧文件的保护。以下情况才是回归：已经成功读取仍报错、恢复会话丢失读取状态、模型不先读而反复写，或错误导致整轮无终态。

### RUN-07 runtime 选择与命令路径

- 为两个 Agent 分别选择 NXS 与 Claude runtime，重启 App 后选择仍保持。
- Session 显式模型优先于 Agent 与全局默认；临时 fallback 后能恢复显式选择。
- 桌面包优先使用随包发布的 NXS；开发覆盖路径只在显式设置时生效。
- 缺少 runtime 时显示可行动错误，不能静默换成另一个 runtime。
- MCP、Connector 或工具面发生变化时，只在工具拓扑确实变化时 fork/restart Session。

自动化入口：

```bash
go test ./internal/service/runtimeselection ./internal/service/nxsruntime ./internal/service/sessionresume ./internal/runtime/clientopts
cd ../nexus-agent-sdk-bridge
go test ./client ./runtimes/nxs
```

## 2. 记忆、Recall 与 Summary

### MEM-01 Recall 相关性与频率

1. 写入三个互不相关的唯一事实，其中只有一个与随后问题相关。
2. 连续发送短确认、寒暄、工具结果续跑和有语义的追问。
3. 记录每轮是否触发 Recall、候选文件、最终引用和 token 开销。

预期：无语义短提示、内部 continuation 和后台来源不触发 Recall；有语义问题只召回相关主题，不能每轮重复召回同一批无关文件。

### MEM-02 明确记住与忘记

- “记住……”和“忘记……”应在当前轮立即执行并给出可见确认。
- 普通对话中的 AutoMemory 使用批量/节流路径，不能把模型未反对的信息直接当成长期事实。
- 忘记后新会话不得再次召回，索引和主题文件不能残留互相矛盾的条目。

### MEM-03 模型注入与 UI 引用

- 模型只通过 `relevant_memories` attachment 获得召回内容，不额外收到 `memory_recalled` 系统事件。
- 产品可以保留不进入模型的展示元数据，在 Assistant 消息底部用简短入口显示“引用的记忆”。
- 展开后只显示实际被引用的条目；未召回时不显示空入口。
- DM、Room、流式终态、历史恢复和重新进入会话后的显示一致。

### MEM-04 AutoDream 与 MEMORY.md

准备至少五个已结束的历史 Session，触发维护后检查：

- Dream agent 自己维护 `<workspace>/MEMORY.md` 和 `memory/**`，不会再被宿主的确定性重建覆盖或来回改写。
- `.consolidation-active` 锁能防并发，成功才更新 `.consolidated-at`，失败或取消可恢复。
- 当前 Session、维护 Session 和不满足间隔/数量 gate 的数据不被错误计数。
- Claude Agent 的后台维护仍由受控 NXS maintenance runtime 执行，不依赖 Claude runtime 暴露维护工具。

### MEM-05 session-memory 与长期记忆隔离

- `session-memory/summary.md` 只服务当前 Session 的 compact/recovery，不自动进入长期 Recall。
- `MEMORY.md` 是长期索引，主题文件按 Recall 选择；两者不能相互冒充。
- 退出、恢复、fork Session 后 Summary 连续，但不同 owner、Agent 和 workspace 不串数据。

### MEM-06 `daily_log` 模式

- 未设置 `memory.mode` 时使用默认 topic 模式，不生成 daily log。
- 显式设置 `daily_log` 后追加 `memory/logs/YYYY/MM/YYYY-MM-DD.md`，并停止普通 AutoMemory topic extraction。
- AutoDream 后续可以蒸馏 daily log，但不能把日志全文无差别注入每轮。

自动化入口：

```bash
go test ./internal/service/dm ./internal/service/room/realtime ./internal/service/memorymaintenance ./internal/message
cd ../nexus-agent-sdk-go
go test ./internal/memory/... ./internal/runtime ./internal/query
```

## 3. 权限、隔离与配置文件

### PERM-01 runtime 参数文件可读性

覆盖 MCP config、追加系统提示和其他 `arg-files`：

1. 在宿主和容器内分别记录文件、父目录的 owner、group、mode 和 ACL。
2. 记录实际 runtime UID/GID，而不是用宿主进程身份代替。
3. 以该 runtime identity 执行真实读取。
4. 以另一 owner 的 runtime identity 读取同一路径，必须拒绝。

Linux 取证命令模板：

```bash
namei -l <arg-file>
stat -c '%U:%G %a %n' <arg-file>
id <runtime-user>
sudo -u <runtime-user> test -r <arg-file>
```

通过标准：目标 runtime 能穿过所有父目录并读取文件，跨 owner 仍拒绝；不能只看到文件是 `0640` 就判定正确。

### PERM-02 新旧 owner 与重启修复

分别覆盖新建 owner、升级前已存在 owner、重启容器、重新生成参数文件和清理后重建。每次都执行 PERM-01，确认修复作用于统一创建路径，不依赖手工 `chmod/chown`。

### PERM-03 人工确认跨重连

1. 触发 `AskUserQuestion`、写文件或 Shell 的人工确认。
2. 卡片出现后断开 WebSocket，再重连同一会话。
3. 确认 request ID、创建时间、顺序和作用域不变。
4. 分别允许、持续允许、拒绝和取消。

预期：请求不因固定超时自动失败；重连从 snapshot 重放；运行态只在明确决策、Session 取消或 runtime 结束后收口。多个 pending 请求不能用无 ID 决策误批。

### PERM-04 Skill 权限拒绝

在允许和拒绝两种 Agent 权限下调用同一 Skill：

- 允许时 Skill 正常加载和执行。
- 拒绝时才显示 `Skill execution blocked by permission rules`，并立即结束该工具卡片。
- Agent/Room 主持人身份本身不能绕过 Skill allow/deny。
- 修改权限后新一轮生效；旧轮不能继承扩大后的能力。

### PERM-05 主智能体控制面边界

- owner 主智能体可在当前 owner scope 使用 `nexusctl`/配置控制能力。
- 普通 Agent、其他 owner、过期 round 或伪造命令必须 fail closed。
- Shell 管道、重定向或拼接不能绕过 managed CLI 的精确命令识别。

### PERM-06 错误码与前端投影

触发拒绝、请求超时兼容事件、runtime 关闭和配置记录失败，确认 `error_code` 从 SDK、bridge、product 到 Web 不丢失。前端应显示可理解原因，不能都折叠成普通 `permission denied`。

自动化入口：

```bash
go test ./internal/runtime/permission ./internal/infra/runtimeidentity ./internal/runtime/workspaceisolation ./internal/service/toolpolicy
cd ../nexus-agent-sdk-bridge
go test ./permission ./protocol ./client
```

## 4. Room、实时消息与任务

### ROOM-01 不带 @ 的消息

在多人 Room 中直接发送普通消息，不指定成员：

- 消息立即获得 ACK，并按当前合同路由到 Room 主持人。
- 用户停留在当前窗口时就能看到 working 状态、流式内容和终态回复。
- 不允许“退出对话再进入后才看到回复”；这种现象要同时查 backend 目标选择、handoff/queue、WebSocket 订阅和前端 snapshot 合并。

### ROOM-02 显式 @ 路由

- `@AgentName` 只唤醒有效 Room 成员，目标顺序稳定且去重。
- 不存在、非成员或歧义名称必须明确拒绝，不能静默交给其他 Agent。
- Agent 公开回复中的协作 mention 按当前 handoff 合同创建独立目标；legacy fanout marker 只作兼容输入，不向用户显示。

### ROOM-03 排队、并发与回传

- 目标空闲时立即启动；目标忙时进入 durable queue，不丢消息、不重复启动。
- 同一目标来自多个 source 的消息保持顺序；另一个 Agent 正在运行不阻塞空闲目标。
- public handoff、directed message 和 handback 在重启后可恢复，私域正文不泄漏到公区。
- 并行流式回复保持 Agent 到达顺序、卡片归属和最终布局稳定。

### ROOM-04 WebSocket 重连与活动态

- 正常 close、网络中断和 App 后台恢复都会自动重连。
- Room 与 DM 的 working、pending permission 和未读状态按精确 source 重放。
- 空 snapshot 或某个 source 的终态不能清掉同 Room 其他正在运行的 source。
- 用户消息不能因 ACK 丢失重复或消失。

### ROOM-05 Task v2 单一任务视图

- `TaskCreate/Get/List/Update/Output/Stop` 的状态在 DM 和 Room 共用同一任务视图。
- 历史 `TodoWrite`、`task_progress` 和 system task 事件需要合并时，不生成两套列表或重复任务。
- 失败、停止、完成和恢复后的状态稳定；不能触发 React `Maximum update depth exceeded`。

### ROOM-06 成员参与和权限

- 暂停成员不接收新调度，恢复后只处理当前有效任务。
- Room host、普通成员、公开/私域消息和 Skill 权限边界相互独立。
- 关闭会话 tab、删除 Conversation、删除 Room 后，对应 runtime、pending、queue 和订阅都被清理；其他 Room 不受影响。

### ROOM-07 会话打开与滚动

执行[会话打开与最新消息滚动回归测试](./conversation-latest-scroll-regression.md)的 SCROLL-01、SCROLL-03、SCROLL-06；Room/时间线改动执行全部十项。

自动化入口：

```bash
go test ./internal/chat/room ./internal/service/room/... ./internal/handler/websocket
node --test web/scripts/room-collaboration-regression.test.mjs web/scripts/input-queue-ack.test.mjs web/scripts/websocket-reconnect.test.mjs web/scripts/room-member-participation.test.mjs
```

## 5. Session、历史与消息投影

### SESSION-01 transcript 清洗

- 导入旧 transcript 时，内部 token-limit continuation、Skill transport prompt 和诊断行不显示为用户消息。
- assistant 快照按 message ID 合并；工具并行分支、usage 和终态保留。
- 不生成空 Assistant 气泡、虚假 user round 或重复 result 文本。

### SESSION-02 历史分页与 Round Index

- 冷打开、向上加载、清空历史、fork 和删除 lineage 后，消息顺序、计数和标题一致。
- 静态 Feed 切换虚拟 Feed 时，Round Index 的延迟返回不覆盖用户滚动意图。
- 末条消息、未读节点和当前 runtime round 都用稳定身份定位，不能依赖 DOM 索引。

### SESSION-03 工具结果和错误终态

- 成功、拒绝、进程退出、超时、malformed input 和 read-before-write 都有唯一终态。
- 工具执行错误不吞掉后续 Assistant 回复，不让会话永久显示 working。
- 重新进入会话后错误卡片、复制内容和展开状态仍能正确投影。

### SESSION-04 上下文与用量

- token、费用、cache read/create、上下文占用和 runtime/model 标签对应同一轮。
- compact、恢复、Room 并行 Agent 和历史导入不能把上一轮或其他 Agent 的统计挂到当前回复。

自动化入口：

```bash
go test ./internal/storage/workspace ./internal/service/session ./internal/message
node --test web/scripts/conversation-history-indexing.test.mjs web/scripts/timeline-guidance-order.test.mjs web/scripts/workspace-error-scope.test.mjs
```

## 6. Provider、桌面升级与发布

### DESKTOP-01 确认运行当前构建

1. 完全退出旧 Nexus，确认没有旧进程持有单实例锁。
2. 执行 `make app-run`。
3. 记录 App version、product Commit、sidecar PID、NXS 路径和唯一构建标记。
4. 验证 UI 和日志均来自本次构建，再开始功能回归。

新进程启动后立即退出而旧窗口仍存在时，当前测试无效。

### DESKTOP-02 更新体验

- 启动时检查更新，之后按四小时间隔刷新；周期检查不重复弹窗。
- 侧栏持续显示可用更新，下载显示进度、已下载/总大小。
- SHA-256 校验、安装、重启和失败恢复保持有效。
- macOS 与 Windows 分别做原生构建和 smoke，不能用一端结果代替另一端。

### DESKTOP-03 sidecar 启动诊断

人为提供错误 runtime 路径或让 sidecar 提前退出，确认诊断包含 `exit_code`、`stdout_tail`、`stderr_tail` 和 `startup_timeline`。优先从 `stderr_tail` 得到真实根因；macOS 与 Windows artifact 字段一致。

### DESKTOP-04 Provider 初始化与登录

- 内置、自定义 Anthropic-compatible 和 OpenAI Responses Provider 的模型、Token、Base URL 和 runtime 兼容性正确。
- 无 Token、错误 Token、配额、网络和 runtime 不兼容分别显示准确错误，不能统一成 500。
- 首次引导、设置页和对话内配置选择同一真相源；重启后选择保留。

### DESKTOP-05 升级与状态迁移

从仍受支持的旧包升级到当前包，检查：

- owner、Agent、Room、Conversation、Provider 和设置仍存在。
- workspace、transcript、session-memory、`MEMORY.md`、topic/daily-log 文件不丢失、不串 owner。
- runtime 权限迁移幂等，旧 owner 的参数文件和 workspace 在升级后可读。
- bundled NXS、`rg` sidecar 和命令 shim 路径存在并可执行。
- 回退旧包只作为对照，不用旧包写入已经升级的数据根。

### RELEASE-01 发布包闭环

```bash
make check-go-full
pnpm --dir web build
make app-check
cd ../nexus-agent-sdk-bridge && GOWORK=off go test ./...
cd ../nexus-agent-sdk-go && GOWORK=off go test ./...
```

再在 Windows 执行原生 build/smoke。发布记录必须包含 tag、三个仓 Commit、runtime manifest、包 SHA-256 和实际升级结果。

## 7. Nexus 产品功能覆盖矩阵

本节按 Nexus 当前 HTTP 路由、Web feature 和 service 边界列出产品能力。前面章节负责深测高风险主链；这里用于防止发布时漏掉没有刚好发生过故障的功能。

三层职责要分开验证：

| 层 | 主要职责 | 不应承担 |
| --- | --- | --- |
| Nexus product | 账户和 owner 隔离、Agent/Room/Session、Provider、Skill、Connector、Automation、Goal/Execution、持久化、Web/桌面 UI | 不直接实现模型 agent loop，不 import 闭源 NXS 内核 |
| bridge | 启动 runtime 子进程、`stream-json` 契约、control/permission/MCP 消息转换 | 不决定产品权限、Room 路由或模型行为 |
| NXS / Go SDK | Provider 请求、上下文与 transcript、工具循环、Skill/MCP/hooks、权限请求、压缩和记忆 | 不管理 Nexus 账户、Room 数据库、桌面更新或产品订阅 |

一个端到端能力往往跨三层。例如“Agent 写文件”要分别证明 Nexus 选对 owner/workspace、bridge 没丢 permission/control 字段、NXS 执行了 freshness 和 sandbox 检查。

### 7.1 系统、账户、订阅与项目

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-CORE-01 | 健康检查与版本 | `/health`、`/system/version` 与实际 App/sidecar/runtime 版本一致；依赖失败时健康状态可诊断 |
| NX-CORE-02 | Runtime options/status | 可用 runtime、模型能力和 NXS 路径与实际环境一致，缺失项不伪装为可用 |
| NX-AUTH-01 | 登录、状态与登出 | 正确密码建立 Session，错误密码不泄漏账户状态；登出后 HTTP 与 WebSocket 都失效 |
| NX-AUTH-02 | 个人资料与密码 | 修改姓名、头像、密码后重登生效；旧密码不能重新登录，现有 Session 的吊销行为符合当前认证合同 |
| NX-PREF-01 | 用户偏好 | 语言、默认模型/runtime、桌面与记忆偏好重启后保留，非法值拒绝而非静默写入 |
| NX-SUB-01 | 用户订阅与配额 | 套餐、额度、到期和 usage 一致；系统/桌面内部流量不误扣普通聊天额度 |
| NX-ADMIN-01 | 订阅计划管理 | 管理员可增改计划和用户订阅，普通用户访问管理路由必须拒绝 |
| NX-ADMIN-02 | 公共 Provider 管理 | Provider/模型创建、测试、更新、删除和订阅可见性一致，密钥不返回前端 |
| NX-PROJECT-01 | 共享项目创建 | 同一路径幂等解析为同一项目，路径变化和重复创建不制造平行记录 |
| NX-PROJECT-02 | 项目成员 ACL | 授权成员可以进入共享项目，未授权 owner、路径穿越和伪造 project ID 均拒绝 |

### 7.2 Agent、Provider 与 Session

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-AGENT-01 | Agent 创建与模板 | 名称校验、默认头像/提示词/模型/runtime/权限正确；重名与非法名称明确拒绝 |
| NX-AGENT-02 | Agent 编辑 | 模型、Provider、runtime、权限和介绍热更新边界正确；需要 restart/fork 的变更不污染旧轮 |
| NX-AGENT-03 | Agent 联系人 | 添加、删除和打开联系人通道幂等；不能把其他 owner Agent 加入私有联系人 |
| NX-AGENT-04 | Agent 通讯 | Agent 间消息保留发送者、目标和私域身份；失败重试不重复投递、不泄漏到 Room 公区 |
| NX-AGENT-05 | Agent 删除 | Session、workspace、Skill 关联、Automation 和 Room 成员关系按合同清理或失效，不留活 runtime |
| NX-PROVIDER-01 | Provider CRUD | 内置和自定义 Provider 的配置、测试、更新、删除闭环；删除被引用 Provider 时给出可行动结果 |
| NX-PROVIDER-02 | 模型发现与默认模型 | 拉取、编辑、测试和设默认模型后，Agent/Session 选择与模型能力卡同步 |
| NX-PROVIDER-03 | 凭据与日志脱敏 | API key、OAuth secret、自定义 header 不进入响应、transcript、diagnostics 和错误正文 |
| NX-PROVIDER-04 | CC Switch 导入 | preview 不写状态，sync 幂等；冲突、失效凭据和未知 Provider 有明确结果 |
| NX-PROVIDER-05 | runtime 兼容性 | Provider/模型只展示兼容 runtime；切换后不产生 `/login`、错误协议或静默 fallback |
| NX-SESSION-01 | Session 创建与列表 | DM、Room、外部通道和后台任务 Session 身份唯一，标题、更新时间和消息数单调更新 |
| NX-SESSION-02 | Session 编辑与删除 | 重命名、标签/元数据和删除即时反映到侧栏；删除停止 runtime 并处理 Automation 绑定 |
| NX-SESSION-03 | Session runtime 设置 | 模型、permission mode 和本地目录按 Session 生效，重启可恢复且不回写其他 Session |
| NX-SESSION-04 | 子 Agent 任务面板 | 列表、消息、继续发送和停止对应同一 task/child Session，终态后不会创建重复任务 |

### 7.3 Workspace、附件、记忆与 Skill

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-WS-01 | 文件树和读取 | 大小、类型、目录层级和错误范围正确；不可读项局部报错，不让整个文件树失败 |
| NX-WS-02 | 文件更新 | HTTP 文件编辑只作用于规范化后的 owner workspace 路径；写入失败不留下半文件，不覆盖其他 owner 内容 |
| NX-WS-03 | 上传与下载 | 文本、图片、PDF、二进制、大文件和同名文件处理一致；下载名与 MIME 正确 |
| NX-WS-04 | 新建、重命名与删除 | 文件/目录操作原子且刷新 UI；非空目录、重复名和非法路径有明确行为 |
| NX-WS-05 | Reveal 与本地应用 | 桌面端能在 Finder/Explorer 定位文件；Web 环境不伪装支持原生动作 |
| NX-WS-06 | 路径限制 | `..`、绝对路径、符号链接、hardlink 和竞态不能越过 owner/workspace 根 |
| NX-WS-07 | Session 本地目录 | 添加、移除和重启恢复正确；目录只扩展显式读取范围，不自动授予写权限 |
| NX-ATTACH-01 | 对话附件 | 上传后 DM/Room 消息引用稳定，成员能读共享附件，其他 owner 和未授权 Agent 不能读 |
| NX-ATTACH-02 | 图片/PDF 投影 | 预览、模型输入、下载和历史恢复使用同一 artifact；不支持视觉的模型明确降级 |
| NX-MEMUI-01 | 记忆工作区 | `MEMORY.md`、topic、daily log 的浏览与更新时间正确，隐藏内部锁和 summary 文件 |
| NX-MEMUI-02 | 清空记忆 | 只删除目标 Agent 长期记忆，Session transcript/summary 和其他 workspace 不受影响 |
| NX-SKILL-01 | Skill 列表与详情 | bundled、用户、项目和外部 Skill 来源、版本、启用状态及 Agent 数准确 |
| NX-SKILL-02 | 本地/Git/源码导入 | preview/导入验证 frontmatter、路径和名称；失败不留下半安装目录 |
| NX-SKILL-03 | 外部搜索与预览 | 搜索、预览和来源管理不执行远端 Skill；网络错误保留来源上下文 |
| NX-SKILL-04 | 更新检查 | 单个/批量更新幂等，用户修改冲突不会被静默覆盖，状态及时投影到前端 |
| NX-SKILL-05 | Agent 安装与启停 | 安装、启用、禁用、卸载改变后续轮工具面；当前运行轮不越权热扩容 |
| NX-SKILL-06 | Skill 文件权限 | 发布、升级和 host link 后目标 runtime 可读，其他 owner 不可读；不依赖手工 chmod |

### 7.4 Connector、Custom MCP 与外部通道

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-CONN-01 | Connector 目录 | 分类、数量、详情和连接状态一致；不可用 Connector 给出原因而非空白页 |
| NX-CONN-02 | OAuth authorization code | client 保存、auth URL、callback、connect/disconnect 闭环；state/PKCE 防跨会话注入 |
| NX-CONN-03 | Device flow | start/poll 的 pending、slow_down、成功、过期和拒绝状态正确，可安全重试 |
| NX-CONN-04 | OAuth 凭据生命周期 | refresh、撤销、重连和删除 client 后不残留可用 token，日志全程脱敏 |
| NX-MCP-01 | Custom MCP CRUD | stdio/HTTP 配置创建、更新、删除和校验完整；非法 command/URL/schema 拒绝 |
| NX-MCP-02 | MCP 热挂载 | Connector/MCP 变化只 fork 受影响 Session；新工具出现、删除工具消失，未改变会话不重启 |
| NX-MCP-03 | MCP 启动失败 | 进程退出、配置不可读、握手超时和协议错误显示 server 名及可行动根因，不泄漏 secret |
| NX-MCP-04 | Connector 授权工具 | 人工批准绑定 exact Connector、Session 和调用；重连/重试不扩大 scope |
| NX-CHAN-01 | 通道配置与账号 | Discord、Telegram、钉钉、飞书、个人微信等配置增删和多账号隔离正确 |
| NX-CHAN-02 | 登录与验证码 | 二维码/验证码登录状态可恢复、可取消、会过期；旧 login ID 不能控制新登录 |
| NX-CHAN-03 | Pairing | 创建、启停、改绑和删除后 inbound 只进入当前绑定 Agent/Session |
| NX-CHAN-04 | Ingress 幂等 | 重复 webhook、乱序事件、平台重试和相同 message ID 不产生重复 user round |
| NX-CHAN-05 | 外部回复 | 文本、工具状态、权限提示和终态按平台能力投递；平台失败不吞 Nexus 内部结果 |
| NX-CHAN-06 | 外部权限命令 | `/y`、`/a`、`/d` 只处理当前会话唯一 pending；歧义或跨 Agent 请求 fail closed |

### 7.5 Automation、Echo 与恢复

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-AUTO-01 | 定时任务 CRUD | cron/时区、Agent、Session、权限快照和投递目标验证完整；修改不隐式改 Agent 配置 |
| NX-AUTO-02 | 手动运行与并发 | run 创建唯一 run ID；重复点击、调度撞车和进程重启不重复执行副作用 |
| NX-AUTO-03 | 启停与状态 | disable 阻止新 run 但不伪造当前 run 终态；re-enable 从正确下次时间继续 |
| NX-AUTO-04 | Run 历史与事件 | 排队、运行、等待权限、成功、失败和投递状态顺序完整，可按 job/run 追踪 |
| NX-AUTO-05 | 持久权限 | 请求冻结到唯一审批 Session，Web/IM 重连重放同一 ID，允许后从原 run 恢复 |
| NX-AUTO-06 | Delivery retry | Agent 执行成功但外部投递失败时只重试 delivery，不重跑模型和工具副作用 |
| NX-AUTO-07 | Crash recovery | server/runtime 在不同阶段退出后，recover 按 durable state 续跑或明确终止 |
| NX-AUTO-08 | Session rebind | 绑定 Session 删除后任务停用为 rebind-required；改绑成功前不能继续运行 |
| NX-AUTO-09 | Daily report | 日期、时区、成功/失败/待确认统计和费用与 run ledger 一致 |
| NX-ECHO-01 | Echo 候选 | 成功 DM round 只产生一个候选；内部 round、Room 与外部 IM 不产生候选 |
| NX-ECHO-02 | Echo 让路 | 新用户输入、全局关闭或会话暂停能取消等待、判断中与生成中的 Echo |
| NX-ECHO-03 | Echo 限频 | 活跃时段、同 Session cooldown、用户级每日上限和 7 天过期边界正确 |
| NX-ECHO-04 | Echo 隔离 | 最终消息只提交一次并显示 Echo 标识；无工具、无权限、无维护任务，超出输出预算时静默 |

### 7.6 Goal、Execution 与配置控制面

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-GOAL-01 | Goal 创建与读取 | Composer 和 `/goal` 走同一控制路径；objective、负责人、criteria 和控制消息一致 |
| NX-GOAL-02 | Goal 修改 | update 使用 revision fence，旧轮和旧 objective revision 不能覆盖当前目标 |
| NX-GOAL-03 | Pause/resume/clear | 状态转换保留 ledger；pause 不启动新工作，resume 不重复已有 continuation |
| NX-GOAL-04 | Goal usage/events | token、费用、事件和协作者事实归属 exact Goal，不混入普通聊天或旧 Goal |
| NX-GOAL-05 | 完成审计 | 先验证 objective alignment 和未终结责任；缺证据返回 domain-qualified 恢复动作 |
| NX-EXEC-01 | WorkGraph 投影 | latest snapshot 与 Assignment/Attempt/Submission/Review/Acceptance 历史一致 |
| NX-EXEC-02 | 分配与接管 | `assign_work`/`take_over_work` 建立独立责任段，不能把上一段 authority 带入新段 |
| NX-EXEC-03 | 提交与评审 | Submission Gate 不重复创建，Review/Acceptance 只更新 exact Submission |
| NX-EXEC-04 | 失败、阻塞和取消 | terminal 状态不可再 mutation；恢复、retry 和 loop-back 保留因果链 |
| NX-EXEC-05 | Runtime command wire | inspect/invoke 返回单层 typed data；重复 request ID 幂等，changed refs 不重复切段 |
| NX-AUTHZ-01 | Round capability | Goal/Execution/Automation/配置能力绑定 exact owner、Session、round 和资源 revision |
| NX-CONFIG-01 | 对话式配置 | inspect → plan → apply 使用 revision/digest fence，模型不能跳过真人确认 |
| NX-CONFIG-02 | 配置审计 | secret 独立于模型 tool input；变更来源、风险、批准人和结果可追踪且脱敏 |

### 7.7 Launcher、通知、用量与桌面集成

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NX-LAUNCH-01 | Launcher bootstrap | 最近会话、Agent、Room、Provider 可用性和建议在冷启动一次性收敛 |
| NX-LAUNCH-02 | Launcher query | 新任务进入正确 Agent/Room/workspace；重复提交不建立两个 Session |
| NX-TITLE-01 | 自动标题 | 首轮普通消息、Goal 控制消息和恢复 Session 各自生成正确标题，不泄漏内部提示 |
| NX-NOTIFY-01 | 侧栏活动与未读 | working、waiting、未读和终态按 Room 聚合且按 source 隔离，重连 snapshot 权威覆盖 |
| NX-USAGE-01 | token/费用/cache | 每轮和累计 usage 与 Provider 返回、runtime/model 和 cache 字段对应，失败轮也可解释 |
| NX-LOOP-01 | Capability loops | 列表与详情反映真实可用 loop，缺依赖时展示原因，不生成不可执行入口 |
| NX-IMAGE-01 | ImageGen | Provider 选择、图片产物、错误、历史恢复和 workspace artifact 归属一致 |
| NX-DESKTOP-06 | 本地目录与文件动作 | macOS/Windows 的 mount、open、reveal、下载及重启恢复语义一致 |
| NX-DESKTOP-07 | WebView/sidecar 恢复 | renderer/browser/sidecar 分别崩溃时恢复边界明确，不重复启动旧 runtime |
| NX-I18N-01 | 中英文界面 | 错误、权限、Room、Skill、更新和滚动控件无缺键；语言切换不改变协议值 |

Nexus 功能面定向入口：

```bash
go test ./internal/service/auth ./internal/service/agent ./internal/service/provider ./internal/service/workspace ./internal/service/skills
go test ./internal/service/connectors ./internal/service/channels/... ./internal/service/automation
go test ./internal/service/goal/... ./internal/service/orchestration/... ./internal/mcp/command/...
go test ./internal/service/projectpermission ./internal/service/subscription ./internal/service/usage ./internal/service/launcher
```

## 8. NXS Runtime 与 Go SDK 功能覆盖矩阵

NXS 既是 Go SDK agent loop，也是 Nexus 通过 bridge 拉起的 `stream-json` runtime。测试时要区分 standalone SDK、`cmd/nxs` 进程协议和 Nexus 托管模式。

### 8.1 公开 API、Session 与 control wire

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NXS-API-01 | `client.Query` | 单轮流式返回 Assistant、工具和唯一 Result；取消和 Provider 错误正确结束 stream |
| NXS-API-02 | `client.Prompt` | 聚合最终结果但不吞 usage、error、permission denial 和 structured output |
| NXS-API-03 | 多轮 Session | `NewSession`、`Send`、`SendMessage`、`StreamInput` 保持同一 session lineage 和顺序 |
| NXS-API-04 | Session 目录 | list/lookup/load/rename/tag 对 transcript 原子生效，损坏单文件不拖垮全部目录 |
| NXS-API-05 | Resume 与 fork | `ResumeID + ResumeAt + Fork` 从目标消息分支，生成新 transcript 且不修改原 Session |
| NXS-API-06 | 强类型消息 | 文本、图片、tool use/result、system、task、usage 和 result 的解析/序列化可逆 |
| NXS-CTRL-01 | initialize | runtime、命令、Agent、模型、账户、MCP 和 protocol capabilities 返回当前快照 |
| NXS-CTRL-02 | 运行期模型/权限 | `set_model`、`set_permission_mode` 只影响后续请求，响应 ACK 与 effective state 一致 |
| NXS-CTRL-03 | 环境热更新 | Provider、WebSearch 和非敏感 runtime env 更新原子生效，凭据不进入 transcript |
| NXS-CTRL-04 | interrupt | 中断当前 turn 后发唯一终态，后台 task 是否继续由其独立生命周期决定 |
| NXS-CTRL-05 | 异步任务控制 | stop/send-message/cancel 使用稳定 task ID，终态任务可继续同一 thread 时不重复历史 |
| NXS-CTRL-06 | hook ACK | 只有协商 `hook_response_ack_v1` 后发送 `control_ack`，内部关联消息不进模型和 transcript |
| NXS-CTRL-07 | MCP control | status/reconnect/toggle 更新同一 server，连接失败不破坏其他 MCP |
| NXS-WIRE-01 | Mixed casing | initialize/hook output 与 permission/普通消息保持 CC 定义 casing，不双写兼容字段 |

### 8.2 Provider、模型能力与流式协议

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NXS-PROV-01 | Anthropic Messages | 文本、thinking、tool、图片、usage、cache breakpoint 和 beta header 形状正确 |
| NXS-PROV-02 | OpenAI Chat Completions | tool calls、stream delta、reasoning、usage 和兼容端可选字段按能力发送 |
| NXS-PROV-03 | OpenAI Responses | 固定 `store=false`，本地 transcript 为真相源，encrypted reasoning 无状态回放 |
| NXS-PROV-04 | Prompt cache key | Responses 的 session-stable key 只在允许端点发送，fork 后不错误复用原 Session key |
| NXS-PROV-05 | Anthropic-compatible | GLM/Kimi/Qwen/DeepSeek/Doubao 等只发送已验证字段，tool result ID 与 thinking 兼容正确 |
| NXS-PROV-06 | Model card | context、vision、structured output、tool reference、effort 和 cache-edit 能力 fail closed |
| NXS-PROV-07 | 模型别名 | generic size、环境覆盖和 runtime `set_model` 最终解析到同一有效模型 |
| NXS-PROV-08 | Header 与凭据 | API key/auth token/custom headers、client cert 和额外 CA 正确装配且 diagnostics 脱敏 |
| NXS-PROV-09 | Retry 与 backoff | 429、5xx、连接失败、流中断和不可重试错误分类正确，副作用工具不重复执行 |
| NXS-PROV-10 | Idle watchdog | 模型持续有事件时不超时；静默超时给出可行动错误并正确清理请求 |
| NXS-PROV-11 | Usage 汇总 | input/output/cache/reasoning token 在 start/delta/result 不重复、不因零值被清空 |
| NXS-PROV-12 | Live provider | 显式凭据下分别跑 Anthropic、OpenAI Chat、Responses、cache 和 vision live 用例 |

Live 测试只在明确提供测试账户时运行：

```bash
NEXUS_LIVE_TESTS=1 go test ./tests/live -run 'TestNativeAnthropicLive|TestOpenAIChatCompletionsLive|TestOpenAIResponsesLive'
```

### 8.3 内建工具、Skill 与子 Agent

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NXS-TOOL-01 | `Read` | 文本、图片、PDF、Notebook、offset/limit/pages 和大文件截断均返回稳定结构 |
| NXS-TOOL-02 | `Edit` | 唯一/多处替换、replace_all、空白和换行规范正确；freshness 变化拒绝覆盖 |
| NXS-TOOL-03 | `Write` | 新建与覆盖区分，原子写入、父目录和权限正确；已有文件遵守读取保护 |
| NXS-TOOL-04 | `Bash` | cwd、env、timeout、stdout/stderr 截断、退出码和 shell 选择稳定 |
| NXS-TOOL-05 | 后台 Bash | 启动、轮询、输出增量、取消和 runtime 退出清理同一 command ID |
| NXS-TOOL-06 | WebFetch | URL 校验、重定向、大小限制、网络权限和内容错误不混成模型错误 |
| NXS-TOOL-07 | WebSearch | anysearch/brave/tavily/exa/firecrawl/searxng 显式选择；失败不暗中 fallback |
| NXS-TOOL-08 | Task v2 | create/get/list/update/output/stop 的依赖、状态、owner 和恢复一致 |
| NXS-TOOL-09 | Agent 子任务 | foreground/background、model/max_turns、resume、fork 和权限收窄正确 |
| NXS-TOOL-10 | 子任务通讯 | parent/child message、stop 和完成后继续同一 thread 不重复上下文 |
| NXS-TOOL-11 | `Skill` | 自动发现与显式调用一致；正文作为隐藏 meta，`context: fork` 只回传结果 |
| NXS-TOOL-12 | Skill allowed tools | Skill 只临时获得声明且获批的最小工具，结束后不残留扩权 |
| NXS-TOOL-13 | `AskUserQuestion` | 多问题 schema、允许/拒绝、取消和重连结果进入 exact tool use |
| NXS-TOOL-14 | Plan mode | enter/exit 与计划上下文稳定；非编程任务不会被 worktree 提示误导 |
| NXS-TOOL-15 | Structured output | schema 校验成功输出 typed result，非法/不支持模型明确降级或失败 |
| NXS-TOOL-16 | `read_result` | 大工具结果分页读取保持 tool use 归属，越界和过期引用明确失败 |
| NXS-TOOL-17 | ViewImage/vision | 支持模型直通图片；不支持模型 fail closed，不把本地路径泄漏到远端文本 |
| NXS-TOOL-18 | 可选工具 | NotebookEdit、worktree、LSP、PowerShell、REPL 只有显式能力/runner 时出现 |
| NXS-TOOL-19 | ToolSearch opt-in | 默认隐藏；开启时权限后检索、schema promotion、历史发现恢复和 MCP 断连失效正确 |
| NXS-TOOL-20 | 工具结果错误 | `is_error`、`error_code`、stderr、结构化结果和模型可见正文各自职责清晰 |

### 8.4 MCP、权限、hooks 与 sandbox

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NXS-MCP-01 | SDK MCP server | Go tool schema、handler、progress、error 和 shutdown 生命周期完整 |
| NXS-MCP-02 | stdio/HTTP MCP | initialize/list/call/reconnect/toggle 兼容；一个 server 崩溃不终止主 Session |
| NXS-MCP-03 | MCP 动态工具 | 新增、更新、删除 schema 后按会话拓扑更新，缓存前缀变化可解释 |
| NXS-MCP-04 | MCP result/resource | text、image、audio、resource、resource_link 和 structured content 不丢字段 |
| NXS-PERM-01 | allow/deny 规则 | exact tool、参数模式、alias 和大小写按 CC 语义匹配，不被 shell 拼接绕过 |
| NXS-PERM-02 | permission mode | default/accept-edits/plan/bypass 等模式只改变授权，不偷偷改变工具目录 |
| NXS-PERM-03 | 宿主 handler | allow、deny、updated input、persist suggestion 和稳定 error code 完整往返 |
| NXS-PERM-04 | Elicitation/OAuth | 用户输入和 token 只交给请求方，取消/过期不落 transcript 或日志 |
| NXS-HOOK-01 | 生命周期 hooks | pre/post tool、user prompt、stop 等触发顺序、matcher 和修改结果正确 |
| NXS-HOOK-02 | hook 故障 | timeout、panic、invalid output 和宿主断开 fail closed，不重复工具副作用 |
| NXS-SBOX-01 | 文件系统 sandbox | workspace、附加只读目录、`/tmp` 和禁止路径符合策略，symlink/hardlink 不越界 |
| NXS-SBOX-02 | 网络 sandbox | 域名/端口 allowlist 生效，重定向、DNS 变化和代理不能绕过限制 |
| NXS-SBOX-03 | 显式关闭 sandbox | `dangerouslyDisableSandbox` 只有宿主允许且权限批准后生效，并留下诊断事实 |
| NXS-SBOX-04 | 平台差异 | Linux/macOS 在支持的 sandbox backend 上执行策略；不支持的平台明确报告能力缺失，不能伪装已隔离 |

### 8.5 配置、指令、Slash、Session 与观测

| 用例 | 功能 | 核心通过标准 |
| --- | --- | --- |
| NXS-CFG-01 | Settings 优先级 | user → project → local → flag → managed policy 深合并，Options/env 保持最高优先级 |
| NXS-CFG-02 | 托管配置投影 | Nexus 只写非敏感 settings，Provider 凭据由宿主注入；standalone 可独立配置 |
| NXS-CFG-03 | 配置热更边界 | 可热更项当前 Session 生效，需要重启项明确报告，不出现半旧半新状态 |
| NXS-INSTR-01 | AGENTS 指令 | user/project/local 指令按目录作用域加载；compact 后重新加载且不重复注入 |
| NXS-INSTR-02 | Slash command 目录 | 用户/项目命令发现、参数替换、未知命令和同名覆盖规则稳定 |
| NXS-SLASH-01 | 内建 Slash | `/model`、`/compact` 等本地命令短路模型但保留可恢复记录和明确输出 |
| NXS-AGDEF-01 | Agent definition | 用户/项目 Agent 的 model/tools/instructions 合并正确，未知 subagent_type 拒绝 |
| NXS-SKDISC-01 | Skill 目录 | `.nexus/skills`、项目/全局 `.agents/skills` 的优先级、禁用和重复名稳定 |
| NXS-SESS-01 | transcript 写入 | parent UUID、timestamp、tool 配对、task、usage 和 result 追加原子且可恢复 |
| NXS-SESS-02 | 中断恢复 | prompt 中断、tool result 中断和 terminal result 分别决定是否注入 continuation |
| NXS-SESS-03 | compact 恢复 | Task、Skill、已发现工具、read state、summary 和 cache metadata 在预算内保留 |
| NXS-SESS-04 | 损坏与截断 | 最后一行截断、未知字段、legacy 字段和单条坏记录不破坏前面有效历史 |
| NXS-SESS-05 | checkpoint/rewind | checkpoint 只选有效 Assistant，rewind 后文件与 transcript 指向同一消息边界 |
| NXS-MEM-01 | Recall | 查询 eligibility、短追问上下文、line/byte/session budget 和 selector failure 正确 |
| NXS-MEM-02 | AutoMemory/Summary | 后台模型、节流、写入范围和错误不阻塞主轮；普通 subagent 不写长期记忆 |
| NXS-MEM-03 | AutoDream | managed/standalone 调度所有权、gate、跨进程锁、取消和原子 success marker 正确 |
| NXS-OBS-01 | diagnostics | runtime/provider/cache/compact/slow-operation 事件可关联 Session，默认不含敏感正文 |
| NXS-OBS-02 | 可选 trace | 开启 `NEXUS_AGENT_SDK_TRACE` 后事件进入同一 JSONL；关闭时无额外持久化开销 |
| NXS-MEDIA-01 | 视觉与媒体 | 本地图片、远端图片 URL、MCP image 和模型能力卡组合正确，超限输入明确失败 |
| NXS-PORT-01 | 文件权限与 ACL | managed runtime 新建 transcript/config/MCP/artifact 保留宿主私有组，standalone 不改原 mode |

NXS 定向入口：

```bash
go test ./client ./protocol ./permission ./hook ./mcp ./sandbox ./tools
go test ./internal/runtime ./internal/query ./internal/session ./internal/settings ./internal/instructions
go test ./internal/provider/... ./internal/tools/... ./internal/memory/...
```

## 9. 按改动选择必测集

| 改动范围 | 必测用例 |
| --- | --- |
| nxs 工具或提示词 | RUN-01、RUN-02、RUN-03、RUN-06 |
| Provider、ToolSearch 或缓存 | RUN-02、RUN-03、RUN-04、RUN-05、DESKTOP-04 |
| 记忆或压缩 | RUN-04、RUN-05、MEM-01 至 MEM-06、SESSION-04 |
| runtime/owner 权限 | RUN-07、PERM-01 至 PERM-06、DESKTOP-05 |
| Room realtime | ROOM-01 至 ROOM-07、SESSION-02、SESSION-03 |
| WebSocket/活动态 | PERM-03、ROOM-01、ROOM-04、SESSION-03 |
| 账户、Agent 或 Session | NX-AUTH、NX-AGENT、NX-SESSION 全组 |
| Workspace、附件或 Skill | NX-WS、NX-ATTACH、NX-MEMUI、NX-SKILL 全组 |
| Connector、MCP 或外部通道 | NX-CONN、NX-MCP、NX-CHAN 与 NXS-MCP 全组 |
| Automation 或 Echo | NX-AUTO、NX-ECHO、PERM-03、SESSION-03 |
| Goal、Execution 或配置控制面 | NX-GOAL、NX-EXEC、NX-AUTHZ、NX-CONFIG 全组 |
| NXS 公开 API/control wire | NXS-API、NXS-CTRL、NXS-WIRE 全组 |
| NXS Provider | NXS-PROV 全组，凭据允许时补 live tests |
| NXS 工具/子 Agent | NXS-TOOL 全组，另跑 RUN-01、RUN-06 |
| NXS sandbox/权限/hooks | NXS-PERM、NXS-HOOK、NXS-SBOX、PERM-03 至 PERM-06 |
| NXS Session/配置/观测 | NXS-CFG、NXS-INSTR、NXS-SLASH、NXS-SESS、NXS-OBS 全组 |
| 桌面或升级 | DESKTOP-01 至 DESKTOP-05、RELEASE-01 |
| 发布 | 全部高风险用例；至少完成每节一个正常路径和一个失败路径 |

## 10. 历史风险基线

下表不是当前通过证明，只说明为什么这些用例不能删：

| 日期 | 历史现象 | 后续固定回归 |
| --- | --- | --- |
| 2026-08-17 | 隔离 runtime 无法读取宿主生成的 MCP config；文件为宿主身份 `0640`，实际 owner runtime UID 无读取权 | PERM-01、PERM-02、DESKTOP-05 |
| 2026-08 | NXS 默认工具、ToolSearch、Task v2、Notebook/worktree opt-in 与 CC 出现过漂移 | RUN-01 至 RUN-03 |
| 2026-08 | 缓存掉档与 ToolSearch/request shape 有关；不能只看 transcript 是否 compact | RUN-02、RUN-04、RUN-05 |
| 2026-08 | Recall 过于频繁且相关性差；模型事件与 UI 引用职责混在一起 | MEM-01 至 MEM-04 |
| 2026-08 | Room 普通消息出现“不 @ 不及时回复，退出再进入才显示” | ROOM-01、ROOM-04、SESSION-03 |
| 2026-08 | Skill 调用显示 `Skill execution blocked by permission rules` | PERM-04、PERM-06 |
| 2026-08-20 | 长 Room 冷启动曾通过，但跨 Room 返回停在倒数第二条；手动回底有效 | ROOM-07 与滚动子文档全部必测项 |
| 多次发布 | 旧 App 单实例进程、错误 sidecar 路径或缺失 bundled runtime 会让测试实际跑在旧版本 | DESKTOP-01、DESKTOP-03、DESKTOP-05 |

## 11. 固定测试输入

每次把 `<RUN_ID>` 换成新的时间或随机后缀，避免把历史回复误认成当前结果。

### 构建和基础响应

```text
请只回复 BUILD_LIVE_OK_<RUN_ID>，不要调用工具。
```

### 文件读写保护

先创建内容为 `before_<RUN_ID>` 的 `note_<RUN_ID>.txt`，再发送：

```text
把 note_<RUN_ID>.txt 中的 before_<RUN_ID> 改成 after_<RUN_ID>。
```

记录模型是否先 `Read` 再 `Edit`。需要直接验证保护器时，由测试 harness 跳过 `Read` 调用 `Edit`，不要用提示词强迫模型犯错。

### 记忆相关性

依次发送：

```text
请记住：我的回归代号是 MEMORY_ALPHA_<RUN_ID>。
请记住：演示项目的颜色是 MEMORY_BLUE_<RUN_ID>。
今天天气怎么样？
我的回归代号是什么？
```

预期第三条不召回前两个事实，第四条只引用代号相关记忆。再发送“忘记我的回归代号”，用新 Session 复查。

### Room 无 mention 与精确 mention

```text
请只回复 ROOM_HOST_OK_<RUN_ID>，不要调用工具。
@<AgentName> 请只回复 ROOM_TARGET_OK_<RUN_ID>，不要调用工具。
```

第一条不带 `@`，用于验证主持人 fallback 和当前窗口实时显示；第二条用于验证精确目标路由。

### Skill 权限

```text
请调用 <SkillName> 完成它描述的最小只读动作，并报告是否被权限规则拒绝。
```

使用同一 Skill 分别跑允许和拒绝配置，不要更换 Skill 后比较结果。

### 缓存与跨 runtime

首轮：

```text
请记住本轮校验串 CACHE_PREFIX_<RUN_ID>。列出当前任务需要的三个检查点，不要修改文件。
```

第二轮：

```text
把第二个检查点换一种说法，其余不变。
```

第三轮先显式增减一个 MCP 或 Skill，再重复第二轮。对比三轮请求 shape 和 cache 字段。

## 回归记录模板

```text
日期：
测试人：
product Commit：
bridge Commit：
nxs Commit：
App version/包路径：
runtime 路径与版本：
NEXUS_STATE_ROOT：隔离/现有，只写类别，不记录敏感路径
Provider/模型：
ToolSearch：默认关闭/显式开启
权限模式：
Room/Conversation/Session：
唯一输入与末条标记：

执行用例：
- 用例 ID：通过/失败
- 观察：
- 截图/日志时间点：
- 自动化命令与结果：

缓存记录：
- cache creation：
- cache read：
- 工具数量/schema 变化：

是否确认运行当前构建：是/否
遗留问题：
结论：可发布/不可发布
```
