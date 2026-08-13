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
本仓采用三层分形文档：**L1**（本节，项目宪法）→ **L2**（各 Go 包 `doc.go` 的 `L2` 头，成员清单 + 暴露接口）→ **L3**（业务文件顶部 `INPUT/OUTPUT/POS` 契约）。跨包产品语义只在 `docs/specs/` 保留一份当前规范；`internal/protocol` 类型、MCP schema/parser 是线格式真相，Skill 只说明模型决策，API Reference 只说明 transport。未来方案、交付计划和未实现字段必须明确标为 non-normative，不能混入当前规范或由多份文档重复定义。

`nexus` — 用户运行的多 agent 桌面/网页应用；Go 后端 + React web。
技术栈: Go + net/http + WebSocket + SQLite/goose + React19 + Vite + Zustand

```
<directory>
cmd/        - 可执行入口（nexus-server 服务 + 自动迁移；nexusctl；Linux runtime launcher）
web/        - React 前端（features / store / shared / lib，见 web/CLAUDE.md）
desktop/    - macOS AppKit/WKWebView 与 Windows WPF/WebView2 宿主（窗口 chrome、bridge、sidecar 生命周期、状态根整体迁移与重启、本机 workspace 文件打开与 macOS 关联应用发现；Windows 用独立原生标题/菜单栏承载全部拖窗与系统命令，WebView 始终保持客户区，Theme/Dialog 将 Nexus token 投影到原生菜单与反馈窗）
skills/     - 随产品发布的平台内置 Skill（每个目录自含 SKILL.md、元数据、脚本与按需加载的参考资料）
internal/   - 后端核心（各子包 L2 见其 doc.go）:
  protocol/   - 跨 HTTP/WS/前端/运行时的协议真相源（会话/房间/Goal/Execution Graph 模型、NodeRun 历史/可恢复结构化产物/显式 partial/total/控制回连事实与 Room creator/lead 身份、事件、枚举、TS codegen 输入）
  runtime/    - nxs/Claude Code 共用宿主主链（bridge client、manager 生命周期、workspace isolation Hook）
  service/    - 业务服务（agent / communication / dm / room / room/realtime / configuration / session / workspace / skills / connectors / automation / llm ...）
  service/objectivealignment/ - Goal completion 与 Execution loop guard 共用的无状态目标对齐审计契约
  chat/       - 对话领域（dm / room）
  handler/    - HTTP / WebSocket 处理器
  message/    - runtime/SDK 消息 → Nexus 事件与 assistant 快照的映射投影
  automation/ - 定时任务 / heartbeat 调度域（任务级 capability grant、持久审批、run 阻塞与安全恢复）
  service/memorymaintenance/ - Nexus 唤醒 nxs 后台记忆维护的宿主协调器
  cli/        - nexusctl 命令装配（按领域文件组织）
  app/        - HTTP 服务装配与生命周期
  mcp/ connectors/ workspace/ - 能力域；mcp/communication 提供平台通讯录与消息工具，mcp/visualize 提供对话内生成式 UI 的模型工具入口
  config/ storage/ infra/ migration/ version/ - 装配、迁移与基础；infra/runtimeidentity 承载 Linux UID/GID、ACL、Landlock launcher，infra/confinedfs 承载宿主目录 fd 边界
docs/       - 开源文档入口；README.md 是索引，guides/ 面向用户与作者，images/ 保存图片与导出 SVG，operations/ 面向运维，specs/ 保存当前维护者合同，architecture-html/ 保存可独立打开的图解页面
</directory>
```

[PROTOCOL]: 变更时更新此头部，然后检查各 Go 包入口 `doc.go`（L2）

## 状态根契约

- `.nexus` 是统一 `NEXUS_STATE_ROOT`；宿主数据位于 `.nexus/app`。
- 桌面端只迁移完整 `NEXUS_STATE_ROOT`：原生宿主退出 sidecar 后离线复制 `app/`、`users/` 与其余状态，切换宿主外的启动指针并直接重启；业务进程不支持拆分或在线迁移局部子树。
- 用户数据位于 `.nexus/users/<owner>/`：`workspace/` 保存 Agent 工作目录与 runtime 可读的 `.rooms/` 公共附件，`runtime/` 同时作为该 owner 的 `NEXUS_CONFIG_DIR` 与 `CLAUDE_CONFIG_DIR`，宿主托管的 Room ledger 固定写入 `state/rooms/`。
- nxs 长期记忆固定写入当前 Agent workspace 的 `MEMORY.md` 与 `memory/`；Nexus 管理的 runtime 不接受宿主环境、请求环境或远端记忆配置改写该根目录。会话摘要仍独立位于 owner 的 `runtime/projects/`。
- Unix runtime 额外获得 `/tmp` 共享兼容读写根，以保持 App/Web 命令行为一致；敏感临时数据仍必须写入该 owner 的 `$TMPDIR`。
- 启动只把当前 canonical 布局作为运行时读写路径；新增宿主或 runtime 文件必须直接落在对应的 `app/` 或用户根目录。历史数据只能通过 `internal/migration/state_layout.go` 与 `workspace_layout.go` 这类明确版本化、可重试且不提供旧路径回读的安全迁移进入 canonical 布局；迁移不能因版本发布而提前移除，必须允许用户跨版本直接升级。
- Linux 多用户强隔离由 root-owned `nexus-runtime-launcher` 执行；产品 server 保持 `nexus-host` 普通用户，普通 Agent runtime 只获得自己的私有 GID 和当前项目组。Nexus 主智能体属于宿主控制面主体，保留 host identity 以调用当前 owner scope 的 `nexusctl`。
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
- 侧栏的聊天执行态与待确认人工交互只按 Room ID 投影；DM 是 Room 的一种，禁止把 Agent runtime 或持久化 `is_active/status` 混入聊天行，联系人侧栏也不订阅 Agent runtime。
- Goal Composer 的“设定 Goal”和文本 `/goal` 必须进入同一个宿主控制命令：先写 Goal，再持久化一条完成态、用户可见但不进入模型的 `/goal <objective>` 控制记录，最后才启动 Goal continuation。不得把 Goal objective 当普通 chat prompt 直接执行；新会话标题与 started/message count 必须由 Goal/控制记录独立建立，不能依赖首个模型回复。
- 模型通过 `create_goal` 创建的 Goal 必须把经服务端验证的当前 Agent 持久化为负责人；后续新物理 round 只为该负责人解析启动时的 exact Goal/objective revision，并且该快照只进入 `nexus_goal`，不得泄漏为 ambient Execution/WorkGraph authority。Room 协作者只能读 Goal 和交付证据；旧 round、旧 revision、后台/外部来源仍必须 fail closed。
- Room Goal continuation 发出的公区 `@` 或带 wake 的 directed message 必须携带宿主持有的精确 Goal ID/objective revision 协作归因，跨 directed-message fact、handoff ledger、InputQueue 和重启恢复保持；私域消息与 handoff 的两阶段写入必须可从前者按当前 revision 幂等修复。副作用工具重试使用 host-only command identity；immediate/delayed wake 都必须先 schedule、成功入队后 complete，并可在线及重启恢复。该归因只用于等待协作者终态、记录可见证据并重新调度一轮有权限的 continuation，绝不能授予目标 conversation round Goal mutation authority。Goal-attributed handoff 不得折叠为 busy slot 的普通 guide；target terminal 与 Goal handback 必须作为两个 durable 阶段恢复，handback 只解除旧 source 的空进展抑制，不重置 continuation 次数上限。多人 Room Goal 的协作要求在创建时由 owner-scoped Room 成员证明写入，complete 时还必须重新读取当前成员事实；创建入口或旧 metadata 缺失不得绕过 room-visible 非 Lead 证据门槛。该证据在同一 durable Goal ID 的整个生命周期内单调累计，retarget、Lead 连续收口或协作要求短暂变化不得清除；objective revision 只 fence 迟到事件的写入归因。历史无归因数据只能由当前 Goal 的精确 suppression 审计事件、完整终态 root 与同 root 公开证据联合修复，禁止从正文或时间邻近猜测。
- Room-backed Session 中，SQL 只拥有 Room 身份、标题与配置，workspace/Room ledger 拥有运行历史进度；统一读模型必须单调合并。旧 SQL `messages` 计数只能作为兼容下限，禁止覆盖 canonical Goal 控制记录、标题、最近活动、消息数、上下文占用或 transcript lineage。
- Room 首条未读定位必须以完成事件的精确消息身份映射到稳定 `agent_round` 节点，并按真实到达顺序排队；Agent 回复可能插入旧 root，禁止用 Feed 尾部或 DOM 索引猜测未读边界，DM 继续沿用自身的回到底部行为。
- 定时任务创建时把来源 Session 的有效 permission mode（无覆盖则取 Agent）和 Agent 工具 allow/deny 复制为独立任务快照；后续修改任务不回写 Agent，Agent 配置变化也不隐式改写任务。单个 Session 删除时，引用任务必须保留、停用并进入 `rebind_required`，只有执行与投递的全部失效绑定被替换后才能重新启用；Agent/Room 整体删除仍级联删除任务，删除恢复必须重放同一失效投影。
- IM 来源的定时任务只持久化结构化会话键与可长期复用的通道上下文；创建 `Source` 只作不可变 provenance，最近一次明确配置投递的可信 Agent/页面/CLI 授权必须独立保存在非公开 `DeliveryGrant`，旧任务升级时从 Source 精确复制而不改写来源。callback `req_id`/stream 只用于当前即时回复。普通 Agent 的 Automation MCP 只表达 `context_mode=current|isolated` 与 `deliver_result`，动态 schema 隐藏 execution/reply 枚举及 channel/account/target/thread/session 等宿主路由参数，create/update 必须从可信 `ServerContext` 自动绑定且忽略模型伪造值；底层完整枚举仅供数据库、旧任务兼容与 owner main 高级控制。每个 run 固化开始时的投递目标，首次投递使用该快照；用户修正通道后重试使用任务最新目标。结果先以 run_id 幂等投影到逻辑 Nexus 会话，再发送外部 IM 并关联平台回执；正文不承担来源或授权语义。创建、执行投递、审批与重试都必须重新验证 active pairing。企业微信延迟结果使用主动消息命令；IM 权限通知只向用户展示 session-scoped `/y`（本次允许）、`/a`（持续允许）、`/d`（拒绝），历史 `/approve`、`/always`、`/deny` 只作兼容别名，内部请求 ID 不属于用户协议；无 ID 命令只有在当前会话跨普通 runtime 与 Automation 合计恰有一个待确认请求时才执行，多个请求必须 fail closed，且命令不进入 Agent 对话。catalog 中所有外部 IM 的 active-paired 私聊都是同一 Agent 的 transport：共享该 Agent 的 Skill、当前 permission mode、工具 allow/deny 和同 Agent Automation CRUD；普通 runtime 工具确认也通过同一 session 的无 ID Slash 回到唯一 pending request。浏览器查看外部 IM session 只能订阅，不得注入 Web host Slash 或覆盖 IM 投递 route；群聊、失效配对及跨 Agent/owner 控制继续 fail closed。

长流程按业务阶段拆成私有函数，阶段之间传递有语义的结构体；一个产品语义只保留一个投影入口。Go 文件不设机械行数上限，按业务内聚、依赖边界和阅读路径决定拆合；同一业务散落时优先合并，不以透传参数包或多层薄包装掩盖复杂度。
