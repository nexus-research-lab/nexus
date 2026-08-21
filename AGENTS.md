# AGENTS.md

## Build & Validation Commands
- `make dev`：同时启动 Go 后端（8010）和前端（3000）
- `make check-go`：默认 Go 门禁，只检查相对上游及当前工作树中发生变化的 Go 包
- `make check-go-fresh`：对上述变化包禁用测试结果缓存
- `make check-go-full`：显式运行 Go 全量 vet 与无缓存测试，仅用于发布、跨包基础设施变更或用户明确要求
- `make check`：运行增量 Go 门禁、前端 lint、前端时间线行为测试、前端 typecheck
- `make check-backend`：Go 后端增量校验，等价于 `make check-go`
- `make install`：执行 `go mod tidy` 并安装前端依赖

日常修改先跑目标包测试或 `make check-go`。Agent 不得默认执行 `go test ./...` 或 `make check-go-full`；只有发布、共享协议/基础设施变更、增量范围无法可靠判断，或用户明确要求全量时才执行。

## Commit Style
Use English commit messages with an emoji prefix, for example `:sparkles: Switch to the Go default runtime path`. Keep user-visible changes reflected in `CHANGELOG.md`.

## L1 — 文档地图

代码是机器相，注释是语义相，两相必须同构：任一相变化必须在另一相显现，否则视为未完成。
本仓采用三层分形文档：**L1**（本节，项目宪法）→ **L2**（各 Go 包 `doc.go` 的 `L2` 头，成员清单 + 暴露接口）→ **L3**（业务文件顶部 `INPUT/OUTPUT/POS` 契约）。跨包产品语义只在 `docs/specs/` 保留一份当前规范；`internal/protocol` 类型、runtime command/MCP schema 与 parser 是线格式真相，Skill 只说明模型决策，API Reference 只说明 transport。未来方案、交付计划和未实现字段必须明确标为 non-normative，不能混入当前规范或由多份文档重复定义。

`nexus` — 用户运行的多 agent 桌面/网页应用；Go 后端 + React web。
技术栈: Go + net/http + WebSocket + SQLite/goose + React19 + Vite + Zustand

```
<directory>
cmd/        - 可执行入口（nexus-server 服务 + 自动迁移；nexusctl 资源控制 CLI；nexuscfg 配置 CLI；nxs/Claude 共用的 Agent-facing nexus CLI；Linux runtime launcher）
web/        - React 前端（features / store / shared / lib，见 web/CLAUDE.md）
desktop/    - macOS AppKit/WKWebView 与 Windows WPF/WebView2 宿主（窗口 chrome、bridge、sidecar 生命周期、状态根整体迁移与重启、本机 workspace 文件打开与 macOS 关联应用发现；Windows 用独立原生标题/菜单栏承载全部拖窗与系统命令，WebView 始终保持客户区，Theme/Dialog 将 Nexus token 投影到原生菜单与反馈窗）
skills/     - 随产品发布的平台内置 Skill（每个目录自含 SKILL.md、元数据、脚本与按需加载的参考资料）
internal/   - 后端核心（各子包 L2 见其 doc.go）:
  protocol/   - 跨 HTTP/WS/前端/运行时的协议真相源（会话/房间/Goal/Execution Graph 与命名工作图模型、NodeRun 历史/可恢复结构化产物/显式 partial/total/控制回连事实与 Room creator/lead 身份、事件、枚举、TS codegen 输入）
  runtime/    - nxs/Claude Code 共用宿主主链（bridge client、manager 生命周期、workspace isolation Hook）
  service/    - 业务服务（agent / communication / dm / room / room/realtime / configuration / session / workspace / skills / connectors / automation / llm ...）
  service/objectivealignment/ - Goal completion 与 Execution loop guard 共用的无状态目标对齐审计契约
  chat/       - 对话领域（dm / room）
  handler/    - HTTP / WebSocket 处理器
  message/    - runtime/SDK 消息 → Nexus 事件与 assistant 快照的映射投影
  automation/ - 定时任务 / heartbeat 调度域（任务级 capability grant、持久审批、run 阻塞与安全恢复）
  service/memorymaintenance/ - Nexus 唤醒 nxs 后台记忆维护的宿主协调器
  cli/        - 命令子系统；根目录无 Go 包，按信任边界分为三个子包：
    agent/          - Agent-facing round-scoped nexus CLI；不打开数据库，只经 broker capability 调用宿主
    host/           - nexusctl / nexuscfg 宿主命令行装配（按领域文件组织）
    runtimecommand/ - Goal/Execution 的 transport-neutral operation contract、round capability 与 typed receipt
  app/        - HTTP 服务装配与生命周期
  mcp/ connectors/ workspace/ - 能力域；模型侧只通过内置 Skill 和 round-scoped `nexus goal|execution` CLI 使用 runtime command；mcp/communication 提供平台通讯录与消息工具，mcp/feishudocx 提供独立飞书云文档语义工具，支持原生 MCP 的其他 Provider 直接挂载自身 server，不提供通用 REST 路由；mcp/visualize 只暴露 show_widget，skills/visualize 承载生成规范；Automation 模型控制复用全 Agent 内置 automation Skill 与 round-scoped `nexus automation` CLI，不再挂载 Automation MCP；owner 资源管理复用 nexus-manager / nexusctl，配置管理复用全 Agent 内置 nexus-configuration Skill 与 round-scoped nexuscfg，不再挂载 manager 或 configuration MCP
  config/ storage/ infra/ migration/ version/ - 装配、迁移与基础；infra/duework 承载后台 durable work 的合并唤醒、精确 deadline timer 与低频审计，infra/runtimeidentity 承载 Linux UID/GID、ACL、Landlock launcher，infra/confinedfs 承载宿主目录 fd 边界
docs/       - 开源文档入口；README.md 是索引，guides/ 面向用户与作者，images/ 保存图片与导出 SVG，operations/ 面向运维，testing/ 保存维护者回归清单，specs/ 保存当前维护者合同，architecture-html/ 保存可独立打开的图解页面
</directory>
```

[PROTOCOL]: 变更时更新此头部，然后检查各 Go 包入口 `doc.go`（L2）

## 状态根契约

- `.nexus` 是统一 `NEXUS_STATE_ROOT`；宿主数据位于 `.nexus/app`。
- 桌面端只迁移完整 `NEXUS_STATE_ROOT`：原生宿主退出 sidecar 后离线复制 `app/`、`users/` 与其余状态，切换宿主外的启动指针并直接重启；业务进程不支持拆分或在线迁移局部子树。
- 用户数据位于 `.nexus/users/<owner>/`，该 owner 的 runtime 对整棵用户数据根拥有读写权限，跨 owner 访问仍拒绝；`workspace/` 保存 Agent 工作目录与 `.rooms/` 公共附件，`runtime/` 同时作为 `NEXUS_CONFIG_DIR` 与 `CLAUDE_CONFIG_DIR`，Room ledger 固定写入 `state/rooms/`。
- nxs 长期记忆固定写入当前 Agent workspace 的 `MEMORY.md` 与 `memory/`；Nexus 管理的 runtime 不接受宿主环境、请求环境或远端记忆配置改写该根目录。会话摘要仍独立位于 owner 的 `runtime/projects/`。
- Unix runtime 额外获得 `/tmp` 共享兼容读写根，以保持 App/Web 命令行为一致；敏感临时数据仍必须写入该 owner 的 `$TMPDIR`。
- 启动只把当前 canonical 布局作为运行时读写路径；新增宿主或 runtime 文件必须直接落在对应的 `app/` 或用户根目录。历史数据只能通过 `internal/migration/state_layout.go` 与 `workspace_layout.go` 这类明确版本化、可重试且不提供旧路径回读的安全迁移进入 canonical 布局；迁移不能因版本发布而提前移除，必须允许用户跨版本直接升级。
- Linux 多用户强隔离由 root-owned `nexus-runtime-launcher` 执行；产品 server 保持 `nexus-host` 普通用户，普通 Agent runtime 只获得自己的私有 GID 和当前项目组。Nexus 主智能体属于宿主控制面主体，保留 host identity 以调用当前 owner scope 的 `nexusctl`；所有交互 Agent 通过宿主签发的 round capability 调用 `nexuscfg`，权限仍由 configuration 角色矩阵收口。
- 宿主代 runtime 操作 workspace、transcript、artifact、用户 Skill 或 Room 状态时必须使用 `internal/infra/confinedfs`；owner 校验后不得重新把用户可控绝对路径直接交给 `os.*`。

## 后端依赖方向

```text
cmd -> app -> handler -> service -> domain/storage
                 \-> protocol <- runtime/message
```

- `app` 只负责装配、路由和进程生命周期，不承载业务规则。
- `handler` 在消费侧定义小接口，只依赖当前端点需要的操作；实现返回具体类型。
- `service` 负责业务阶段和事务边界，不依赖 `handler` 或 `app`。
- `service/room` 只持有 Room 的持久化管理；实时聊天与 runtime 编排位于 `service/room/realtime`，依赖方向只能从 realtime 指向 room。
- `service/room/realtime` 测试按 package 与行为聚合：内部状态、Goal、协作测试分别归组，外部交付、生命周期和共享夹具集中管理；queue、guidance、session、directed message 等大场景保持独立。
- `storage` 负责持久化与数据库方言，不保留没有行为的方言门面；共享 SQL 分叉统一进入 `SQLDialect`，领域查询留在各自 repository。
- `runtime` 只描述 bridge 会话与执行生命周期；SDK 系统消息到产品事件的投影统一属于 `message`。
- 测试便利入口优先留在 `_test.go`；只有跨包集成测试需要共享装配时，才在生产包保留窄入口。
- 侧栏的聊天执行态与待确认人工交互只按 Room ID 输出；容器内部必须按精确 Conversation/Session source 隔离后取并集，空快照或终态不得清除其他 source。DM 是 Room 的一种，禁止把 Agent runtime 或持久化 `is_active/status` 混入聊天行，联系人侧栏也不订阅 Agent runtime。
- Goal Composer 的“设定 Goal”和文本 `/goal` 必须进入同一个宿主控制命令：先写 Goal，再持久化一条完成态、用户可见但不进入模型的 `/goal <objective>` 控制记录，最后才启动 Goal continuation。该控制记录必须保留 exact `client_message_id` 作为 ACK 丢失后的 durable acceptance 证据；不得按正文或时间邻近猜测。不得把 Goal objective 当普通 chat prompt 直接执行；新会话标题与 started/message count 必须由 Goal/控制记录独立建立，不能依赖首个模型回复。WebSocket 入口完成同步 scope/owner 校验并受理 `set_goal` 后，必须按原连接顺序在有界 detached context 中完成；切页或断连只能失去 ACK 投影，不能取消已受理的 Goal、控制记录、目录失效或 continuation。
- 模型通过 Goal command 创建的 Goal 必须把经服务端验证的当前 Agent 持久化为负责人；后续新物理 round 只为该负责人解析启动时的 exact Goal/objective revision，并且该快照只进入 round-scoped Goal command authority，不得泄漏为 ambient Execution/WorkGraph authority。Goal read model 必须同时投影当前 objective revision 与服务端权威 completion criteria，完成审计不得从 transcript、旧 Plan 或聊天正文重建。exact Room conversation 的协作者只能读 Goal、共享 WorkGraph 的目标/拓扑/状态和交付证据；只读观察不得授予 Assignment、Review、Submission、Plan mutation、Goal 或 coordination capability。旧 round、旧 revision、后台/外部来源仍必须 fail closed。
- Room Lead 自己执行 Work Item 时，必须由宿主在 self Assignment 持久化后签发 exact WorkBinding，并在同一物理 round 内供 Execution command、Runtime Graph 与 subagent admission 动态读取；Room 成员或 coordinator 身份不得替代该显式 capability。DM 的同轮责任分段保持独立，不得反向成为 Room 的隐式授权条件。
- Runtime Graph 必须复用 tool policy 的严格单进程语法识别 Goal/Execution CLI transport，只持久化分类而不持久化 raw command/input；这些控制调用及 owner-private command input staging 文件操作始终作为 direct owner 的 `detail` 审计事实，不因 Bash/Write 名称、失败、重试、Artifact 或控制边进入主画布。读取历史运行记录时，旧 Goal/Execution MCP 的 exact 工具身份与 canonical staging 路径必须归入同一 transport 语义，但不得恢复旧路由或授权。CLI 节点只能按 exact Execution ID + Execution root round 读取，并由宿主 typed receipt 的 `domain + operation + request_id + changed refs` 核验；同一 exact request 的 provider Tool lifecycle 与 host receipt 是一个语义边界，不能重复切段。DM 同一物理 round 内每个成功 self `assign_work` 或 `take_over_work` 必须据此建立独立 Work Item/Assignment/Attempt 执行段；首个 Assignment 在 block/release 前尚无 round identity 时，只能由同 coordinator、同 exact request、唯一生命周期区间恢复，其他缺失、冲突或跨 scope 引用必须清空边界而非沿用上一段。画布必须用同一 read transaction 读取当前 Snapshot 与 append-only Assignment/Attempt/Submission/Review/Acceptance 历史，每个 root Attempt 和 immutable Submission Gate 保留独立轮次；Acceptance 只更新对应 Submission Gate 的结论，不能因 current Assignment 清空再补建第二个 Gate。运行工具优先按 exact Attempt/Submission/round 归属，不能被最新 Work Item 头像吞并。Work Item/Attempt/Review/Gate 继续表达权威图变化；普通成功 Bash、`MEMORY.md`/`memory/` 维护与无 Artifact/显式重要性的本地文件操作只留在详情，运行中、失败/取消/中断、业务 Artifact、显式可见 hint、retry/loop-back 或失败后的恢复才提升为关键画布节点，外部可观察动作仍按语义展示。
- Agent-facing `nexus goal|execution` 必须由显式命令路径或当前运行宿主的私有 multicall 入口提供，并在每次 server 启动刷新共享 shim；Agent round 不得从源码 `go run` 构建命令，也不得受开发机 `go.work` 影响。Goal/Execution `invoke` 只读取宿主签发、由 physical round 持有的单一 owner-private 输入槽，不接受 inline JSON 或调用方 `--input-file`；每个新 mutation 输入必须以紧邻写入的 fresh exact contract 为路径与 schema 真相，portable schema 在领域读取前校验；同轮 Goal mutation 推进的动态 authority 必须被后续 Execution command 原子读取。`get_goal` 固定映射零输入 Goal inspect，`get_execution` 固定映射 current/显式历史 inspect，读操作不得绕回 invoke。CLI inspect/invoke 只暴露顶层 `data` + 显式 `is_error` 的单层 typed wire，不暴露 MCP Content 镜像或嵌套 `result.data`；受管命令必须是无管道、重定向或自定义后处理的单进程调用，避免状态已提交而后处理失败导致误重试。CLI 结果只能裁掉 Runtime Graph 观察事实和可选 graph digest，不得清空当前 responsibility/review/action/blocker context、伪造 refresh 状态或结束 physical round；只有服务状态机明确返回 `round_refresh_required` 才能换轮。完成态 WorkGraph 的命名保存分为预览与持久化：宿主 HTTP 只用 owner 默认后台模型从完整实际图自动选择 source logical-key 子集、抽象通用语义并按界面语言生成短期 owner/session-scoped 草图，不进入数据库或 Slash 目录；生成模型接收 owner 当前命令目录与固定保留名，默认选择一个不重复的短词 Slash，只有语义准确的单词候选都冲突时才退到两个词，不生成三个及以上词；若模型仍返回多词候选，服务端按语义核心词优先收敛到未占用的单词，并继续做最终冲突校验。用户可直接修改元信息，或进入宿主从源 transcript 最近一个已完成助手轮次创建的目录隐藏短期 DM fork，让模型修改 Slash 命令、标题、描述、objective、节点、父子结构和依赖；Execution root round 与 transcript agent round 属于不同身份，禁止混用。编辑小页只装配既有草图渲染器与标准 DM 面板；Agent 目录或语言刷新不得重建临时 Session，每个模型 revision 直接重绘同页草图。编辑 Session 复用标准 DM 消息、流式、Tool 与 Composer 投影；它不挂载 MCP 或 Connector，只加载 `execution-orchestrator` Skill，并仅通过 round-scoped Execution CLI 暴露 exact owner/session-scoped `revise_workgraph_preview`，同时禁止普通 workspace 读取与其他 command domain/operation；若 source 尾部使用 provider-specific 非兼容消息 ID，runtime 必须复制完整 source transcript，不得传入无效 resume-at。每次 mutation 必须提交带 revision 的完整草图，并由服务端校验 logical key、kind、父子结构、DAG、key 主路径和 terminal 交付。取消只删除临时 Session，明确“应用并返回”才以 exact revision 替换原 preview。用户确认后，宿主冻结完整草图并启动 `HiddenFromUser + Synthetic + purpose=workgraph_distillation` 的内部 Agent round，不能创建可见用户消息或改写 Composer。该 round 内 `execution-orchestrator` Skill 只能把 exact `preview_id` 交给 `distill_workgraph` CLI mutation 原样保存，不得重新 inspect、选节点、命名、抽象或修订结构；HTTP 调度端点本身不得直接持久化。初始抽取模型失败或虚构源节点、编辑后缺 key 主路径/terminal 交付、preview 过期或跨 owner/session 时失败关闭；禁止保存 Tool/运行身份/Assignment/Attempt/结果/Artifact/Submission/Review/Acceptance。固定 `/workgraph` 只启用当前请求的协作；已保存动态 Slash 每次复用都创建 fresh Execution/Plan/Work Item identity，不新增通用 WorkGraph MCP。
- WorkGraph 临时编辑小页固定为左侧标准 DM、右侧实时草图预览的扁平双栏；保存命名图的 `HiddenFromUser + Synthetic + purpose=workgraph_distillation` 内部 round 从 pending、过程状态、工具事实到完成事件都不得进入聊天时间线或占用前端可见执行态。
- Goal 与 Execution command domain 的同名近义操作不得混用：`execution/audit_execution_alignment` 只是 current Execution 的可选 Gate，不能作为 Goal 完成证据，terminal Execution 必须拒绝；确认绑定 WorkGraph 的最终 Acceptance 自动完成无 blocker 的 Execution 后，exact coordinator receipt 必须把同一 physical round 路由到 `goal/audit_objective_alignment`，其 aligned receipt 再路由到 `goal/update_goal`。Goal completion 拒绝必须返回 domain-qualified recovery action，缺对齐证据回 Goal audit，未完成图责任回 Execution inspect；不得靠模型猜测相似 operation 名称。
- 活跃 Nexus Session 的 MCP 工具面只由稳定会话拓扑和用户显式 MCP/Connector 选择决定；内部唤醒、私域回传、Room 角色、WorkBinding/ReviewBinding、Goal authority 与通讯开关只改变逐轮执行权限，不得通过卸载 schema 鉴权。无权轮次必须保留工具定义并在 service 真相源 fail closed；`ToolSearch` 只是默认关闭的 schema 传递优化，不参与挂载或鉴权。
- Room Goal continuation 发出的公区 `@` 或带 wake 的 directed message 必须携带宿主持有的精确 Goal ID/objective revision 协作归因，跨 directed-message fact、handoff ledger、InputQueue 和重启恢复保持；私域消息与 handoff 的两阶段写入必须可从前者按当前 revision 幂等修复。副作用工具重试使用 host-only command identity；immediate/delayed wake 都必须先 schedule、成功入队后 complete，并可在线及重启恢复。该归因只用于等待协作者终态、记录可见审计事实并重新调度一轮有权限的 continuation，绝不能授予目标 conversation round Goal mutation authority。Goal-attributed handoff 不得折叠为 busy slot 的普通 guide；target terminal 与 Goal handback 必须作为两个 durable 阶段恢复，handback 只解除旧 source 的空进展抑制，不重置 continuation 次数上限。当前负责人在 objective 满足且 Room/Execution readiness 通过后拥有 Goal 关闭决定权；成员数量与协作证据不构成完成门槛，但已启动的 slot、handoff、queue、wake 或 WorkGraph work 必须先终态或显式取消。公开非 Lead 实质回复仍可在同一 durable Goal ID 生命周期内单调记录为审计事实，objective revision 只 fence 迟到事件的写入归因。历史无归因数据只能由当前 Goal 的精确 suppression 审计事件、完整终态 root 与同 root 公开证据联合修复，禁止从正文或时间邻近猜测。
- Room-backed Session 中，SQL 只拥有 Room 身份、标题与配置，workspace/Room ledger 拥有运行历史进度；统一读模型必须单调合并。旧 SQL `messages` 计数只能作为兼容下限，禁止覆盖 canonical Goal 控制记录、标题、最近活动、消息数、上下文占用或 transcript lineage。
- Room 与 DM 的会话内滚动只保留统一“回到底部”动作，不再把侧栏未读状态注入 Feed、自动定位首条未读或渲染“新消息”边界；当前窗口打开精确 Conversation 后立即确认该目标，其他 Conversation 的侧栏未读与系统通知继续隔离保留。
- 用户主动打开首次 DM 或显式创建 Room 时，宿主异步追加一条幂等 `conversation_welcome` assistant 消息；内部路由物化不得触发。欢迎语优先使用 owner 后台任务模型，回退用户默认/发言 Agent/Provider 默认模型并保留静态失败兜底，不启动 runtime 或消费 draft。Room 由群主署名，无群主时由主成员只负责介绍；文案必须按 `host_auto_reply_enabled` 准确说明无 `@` 接管或 `@AgentName` 指定规则。Provider `ToolUseSummary` 在执行中收到非空内容即投影为同 Agent round 原位替换的 ephemeral `progress_update`，不设长任务、耗时或工具数门槛；DM 与 Room 在用户可见正文之间只保留一个持续更新的中文优先折叠执行栏，普通 thinking 与连续工具都并入其中，`preceding_tool_use_ids` 只定位包含该批工具的执行段，点击才展开完整过程，权限、用户提问、生成式 UI 和生成文件保持独立可见。终态清除 summary 但保留普通工具的确定性折叠标题，且 summary 不得进入历史、未读、计数或最终 assistant。
- 定时任务创建时，有复用执行 Session 就复制该 Session 的有效 permission mode，否则取执行 Agent；同时复制执行 Agent 工具 allow/deny 为独立任务快照。执行与投递的页面层级统一按真实会话身份选择：DM 都先选 Agent、再选该 Agent 的 DM/active-paired IM Session；Room 都先选真实 Room、再选共享 Room Session、最后选该 Session 的有效成员 Agent，不选成员时在保存阶段解析为当前房主。Room 的执行 Agent 与结果回复/署名 Agent 是两个独立、持久化且在运行/重试时重新验证的身份。来源 Session 只作 provenance，不参与权限继承。后续修改任务不回写 Agent，Agent 配置变化也不隐式改写任务。单个 Session 删除时，引用任务必须保留、停用并进入 `rebind_required`，只有执行与投递的全部失效绑定被替换后才能重新启用；Agent/Room 整体删除仍级联删除任务，删除恢复必须重放同一失效投影。
- IM 来源的定时任务只持久化结构化会话键与可长期复用的通道上下文；创建 `Source` 只作不可变 provenance，最近一次明确配置投递的可信 Agent/页面/CLI 授权必须独立保存在非公开 `DeliveryGrant`，旧任务升级时从 Source 精确复制而不改写来源。callback `req_id`/stream 只用于当前即时回复。执行 Agent 与结果接收 Session 相互独立：结果先以 run_id 幂等投影到接收 Agent 所属的真实 Nexus/Room/IM Session，另存 producer Agent metadata，再发送外部 IM 并关联平台回执；新建与改绑必须引用结构化且已存在的真实 Session，不得只保存裸 channel/chat ID 或创建合成“定时任务收件箱”，旧裸路由与该内部 key 只保留历史读取/投递兼容，编辑时必须重绑。真实 Session 以统一读模型为准：数据库拥有的 Room-backed DM/成员 Session 与 workspace/IM Session 都是合法候选；首次投递可基于该已验证身份物化 workspace 投影，不得以 `room_id` 或 workspace meta 是否已生成作为可投递性边界。普通 Agent 的 Automation CLI 只表达 `context_mode=current|isolated` 与 `deliver_result`，CLI wire 不暴露 channel/account/target/thread/session 等宿主路由；create/update 必须从 round capability 自动绑定，模型伪造字段必须被严格解码或 service 拒绝。owner main 高级控制也只能选择已存在且可验证的真实 Session。所有 Automation mutation 固定走 `inspect -> plan -> apply`、revision/digest 栅栏和当前 Session 原生真人确认；后台 scheduled run capability 只读并绑定 exact job/run。每个 run 固化开始时的投递目标，首次投递使用该快照；用户修正通道后重试使用任务最新目标。正文不承担来源或授权语义。创建、执行投递、审批与重试都必须重新验证 active pairing。持久权限请求创建时冻结唯一审批 Session：优先该 run 的结果接收 Session，没有接收目标时可退到来源 Session；实时投影、重连重放、Composer 决策与 IM Slash 此后只认该冻结身份。企业微信延迟结果使用主动消息命令；IM 权限通知只向用户展示 session-scoped `/y`（本次允许）、`/a`（持续允许）、`/d`（拒绝），历史 `/approve`、`/always`、`/deny` 只作兼容别名，内部请求 ID 不属于用户协议；无 ID 命令只有在当前会话跨普通 runtime 与 Automation 合计恰有一个待确认请求时才执行，多个请求必须 fail closed，且命令不进入 Agent 对话。catalog 中所有外部 IM 的 active-paired 私聊都是同一 Agent 的 transport：共享该 Agent 的 Skill、当前 permission mode、工具 allow/deny 和同 Agent Automation CRUD；普通 runtime 工具确认也通过同一 session 的无 ID Slash 回到唯一 pending request。浏览器查看外部 IM session 可以订阅并处理绑定 exact request 的 Automation 持久权限卡片，但不得注入 Web host Slash 或覆盖 IM 投递 route；群聊、失效配对及跨 Agent/owner 控制继续 fail closed。

长流程按业务阶段拆成私有函数，阶段之间传递有语义的结构体；一个产品语义只保留一个投影入口。Go 文件不设机械行数上限，按业务内聚、依赖边界和阅读路径决定拆合；同一业务散落时优先合并，不以透传参数包或多层薄包装掩盖复杂度。
