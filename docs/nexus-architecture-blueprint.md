# Nexus 技术架构

| 项目 | 内容 |
| --- | --- |
| 文档状态 | 当前实现基线 |
| 核对日期 | 2026-08-11 |
| 覆盖范围 | Nexus Product、Agent SDK Bridge、Agent Runtime 边界 |
| 适用读者 | 架构师、后端与前端工程师、桌面端工程师、运维与安全评审人员 |

本文描述 Nexus 当前代码中的系统边界、运行流程、状态归属和部署方式。文档以本仓库代码、公开 Bridge 协议和随产品发布的 Runtime 行为为依据，重点说明长期稳定的架构约束。具体接口字段、页面结构和功能清单仍以代码及公开协议定义为准。

## 架构结论

Nexus 的核心由产品宿主和独立 Agent Runtime 组成。产品宿主持有身份、协作、调度、持久化和实时广播，Runtime 持有模型调用、工具循环、上下文压缩、Transcript 与记忆维护。两者通过公开 Bridge 的行分隔 `stream-json` 协议通信。

这套架构有四个关键约束。

1. Product 只依赖开源 Bridge，不直接依赖闭源 Runtime 或其内部 SDK 源码。
2. Agent loop 运行在独立子进程，默认实现为 `nxs`，也可以使用 Claude Code。
3. 产品按 Runtime 声明的 Capability 使用能力，不根据 Runtime 名称推断行为。
4. 数据按权威归属拆分到产品数据库、Agent 工作区、Runtime Transcript 和宿主状态目录。

## 一、系统边界

Nexus 面向用户提供 Web 和原生桌面界面，也可通过外部 IM 通道接收消息。产品后端负责协作控制面，Runtime 负责执行 Agent 回合，Connector 负责外部服务授权与调用，模型请求由 Runtime 发往相应 Provider。

### 总体架构图

![Nexus 总体技术架构](./images/nexus-architecture-diagram.svg)

[打开独立 HTML 版本](./architecture-html/nexus-architecture-diagram.html)

图中 Product Plane 持有业务控制面与数据库，Runtime Plane 通过独立进程持有模型调用、工作区和会话记录。默认 `nxs` 以闭源可执行程序分发，不公开内部 Go SDK 源码。实线表示请求或控制，虚线表示 Runtime 返回的流式事件。

### 组件与公开边界

| 组件 | 可见性 | 主要职责与边界 |
| --- | --- | --- |
| Nexus Product | 开源，本仓库 | Go 后端、React Web、桌面壳与部署资产；持有产品业务和 Runtime 编排，不实现 Agent loop |
| Agent SDK Bridge | 开源，独立仓库 | Go 客户端、进程协议、生命周期与 Capability 协商；持有公开契约，不持有产品业务状态 |
| `nxs` | 闭源 Runtime 发行物 | 执行 Agent loop、Provider 调用、工具、会话、压缩与记忆；只通过 Bridge 与 Product 通信 |
| Claude Code | 外部可选 Runtime | 通过 Bridge 兼容层接入，实际能力以运行时协商结果为准 |

依赖方向固定为 Product 依赖 Bridge，Bridge 通过 `stream-json` 驱动 `nxs` 或 Claude Code。

`nxs` 及其内部 Go SDK 不属于开源仓库。Product 只与 Runtime 可执行程序和公开协议协作，闭源实现不会进入开源产品的源码或构建依赖。

## 二、运行形态

Nexus 复用同一个 Go 后端和同一套 Web 前端，支持桌面 App、Docker 服务和源码服务三种形态。

![Nexus 三种运行形态](./images/nexus-deployment-topologies.svg)

[打开运行形态图的独立 HTML 版本](./architecture-html/nexus-deployment-topologies.html)

### 桌面 App

macOS 使用 AppKit 与 WKWebView，Windows 使用 WPF 与 WebView2。原生壳负责窗口、文件选择、系统打开方式、更新和本机安全存储等 OS 能力。Go `nexus-server` 作为 Sidecar 在回环地址的随机端口启动，WebView 加载同包 Web 资产并访问该 Sidecar。

原生壳为每次启动生成桌面会话 Token，并写入 HTTP 请求或 WebView Cookie。Sidecar 对本地 API 和 WebSocket 握手执行校验。桌面端固定使用 SQLite，数据位于统一状态根 `~/.nexus`。

### Docker 与源码服务

Docker Compose 由 Nginx 提供静态资源和反向代理，Go 服务默认监听容器内 `8010` 端口。服务端默认使用 SQLite，也支持 PostgreSQL。生产 TLS 由外层网关或负载均衡器终止。

IM 通道维护进程内长连接或轮询连接。同一个数据库运行多个后端副本时可能产生重复消费，因此启用这类通道的默认部署采用单个后端 Worker。需要横向扩展时，应先为通道连接补充分布式租约与消费归属。

## 三、Product 内部结构

Product 后端遵循单向依赖。命令入口和桌面 Sidecar 负责启动，`internal/app` 完成服务装配，Handler 负责 HTTP 与 WebSocket 边界，Service 持有领域规则，Storage 和 Repository 负责持久化。`internal/protocol` 保存跨 HTTP、WebSocket、前端与运行时投影共享的协议模型。

![Nexus Product 后端分层](./images/nexus-product-layers.svg)

[打开 Product 分层图的独立 HTML 版本](./architecture-html/nexus-product-layers.html)

### 后端主要领域

| 领域 | 职责 |
| --- | --- |
| Agent 与 Session | Agent 身份、工作区、Runtime 配置和会话归属 |
| DM | 单 Agent 持续对话、历史投影和运行状态 |
| Room | 多 Agent 成员、共享消息、定向通信、参与状态和实时调度 |
| Goal 与 Execution | 目标生命周期、执行计划、WorkGraph、分派、提交与评审证据 |
| Runtime | Bridge Client、Session 复用、Round 生命周期、中断和运行时控制 |
| Message | Runtime 消息到产品事件、Assistant 快照和工作区产物的统一投影 |
| Automation | 定时任务、Heartbeat、后台执行和运行记录 |
| Capability | Skill、Connector、Channel、MCP 和权限策略 |
| Storage | 数据库连接、迁移、事务锁和敏感载荷加密存储 |

### 前端结构

React Web 采用领域功能目录。`app` 与 `bootstrap` 负责启动和全局装配，`pages` 负责路由协调，`features` 持有 Agent、Room、DM、Goal、Capability、Memory 与 Settings 等领域界面，`lib` 和 `shared` 提供无业务所有权的协议客户端与 UI 原语，Zustand 保存必要的本地界面状态。

实时通信集中在 WebSocket 客户端和 Agent Session Controller。HTTP 负责资源查询与命令，WebSocket 负责流式消息、权限交互、Session 快照和失效事件。断线重连后，共享通道会重放 Session 绑定，并以服务端快照恢复权威运行状态。

## 四、Agent 回合流程

一次回合同时穿过产品控制面、Bridge 和 Runtime 数据面。产品先确定用户、Agent、Session 与策略，再把输入交给对应 Runtime Client。Runtime 完成模型请求和工具循环，Bridge 将流式消息返回给 Product，Product 统一投影事件并向前端广播。

![Nexus Agent 回合时序](./images/nexus-agent-turn-sequence.svg)

[打开 Agent 回合时序图的独立 HTML 版本](./architecture-html/nexus-agent-turn-sequence.html)

### 消息交付语义

产品协议区分三类交付语义。

| 类型 | 用途 | 恢复行为 |
| --- | --- | --- |
| `durable` | 用户消息、最终结果和需要跨重连保留的状态 | 进入持久状态、后台缓存或重连快照 |
| `ephemeral` | 仅随当前 Round 存活的运行状态 | Round 收口后清理 |
| `transient` | 当前时间线的即时展示 | 不进入后台缓存与未读模型 |

这一区分避免把流式进度当作业务事实，也防止重连后重复制造交互卡片。稳定的 Session ID、Round ID、Tool Use ID 和 Handoff ID 用于幂等接管既有节点。

## 五、Runtime 边界

![Nexus Runtime 公开边界](./images/nexus-runtime-boundary.svg)

[打开 Runtime 边界图的独立 HTML 版本](./architecture-html/nexus-runtime-boundary.html)

### Bridge

Bridge 是 Product 与 Runtime 之间的公开契约层。它提供一次性 Prompt、持久 Session、流式输入输出、控制请求、Hook、权限回调和进程内 MCP。默认 Transport 启动本地 Runtime 进程，也支持宿主管理进程后的直接连接。

Bridge 直接承载两种 Runtime 共用的 mixed-casing control wire，并只在已确认的字段差异处声明兼容别名；它不会对工具输入、Provider payload 或控制消息做全局 snake/camel 转换。上层 Product 只消费统一类型和 Capability，不按 Runtime 名称猜测能力。

### 默认 Runtime：nxs

`nxs` 以闭源 Runtime 可执行程序随 Nexus 分发，其内部 Go Agent SDK 不属于本开源项目。Product 把它视为可替换的独立进程，只依赖 Bridge 契约和运行时声明的 Capability。它持有以下执行职责。

- Runtime 初始化和 Turn 生命周期
- Anthropic Messages、OpenAI Chat Completions 与 OpenAI Responses 适配
- Provider 中立的流式事件
- 工具执行、权限、Hook、MCP 与沙箱
- Context Usage、Microcompact、完整 Compact 和恢复策略
- Transcript 持久化、Session Fork 与恢复
- Summary、AutoMemory 与 AutoDream

产品侧持有 AutoDream 的时钟、并发、重试和进程生命周期。Bridge 传递可取消的控制请求，`nxs` 判断执行资格并完成模型调用和记忆写入。这种分工使后台维护遵守同一套 Runtime 状态和文件锁规则。

### Claude Code

Claude Code 作为兼容 Runtime 由 Bridge 启动和控制。它使用同一 Product Session 与消息投影流程，但 Capability 集合可以不同。产品必须接受 Runtime 返回的实际支持范围。

### 责任矩阵

| 能力 | Product | Bridge | Runtime |
| --- | --- | --- | --- |
| 用户、Agent、Room 和 Goal | 权威所有者 | 传输 | 消费受限上下文 |
| Session 到 Runtime Client 的映射 | 管理 | 提供 Client | 维护执行内会话 |
| 进程启动与协议收发 | 提供配置与生命周期策略 | 执行 | 响应 |
| 模型调用与工具循环 | 提供 Provider 配置和产品工具 | 传输回调 | 权威执行 |
| 权限 | 产品策略和用户决定 | 回调协议 | 调用前检查与恢复 |
| Transcript 与 Compact | 读取投影和关联元数据 | 传输控制 | 权威所有者 |
| 记忆维护 | 调度和展示结果 | 传输控制 | 资格判断和文件写入 |
| 实时 UI 事件 | 统一投影并广播 | 返回类型化消息 | 产生执行事件 |

## 六、Room、Goal 与 Execution

![Nexus Room 与 Execution 分流](./images/nexus-collaboration-execution.svg)

[打开协作执行分流图的独立 HTML 版本](./architecture-html/nexus-collaboration-execution.html)

Room 是共享协作容器，Conversation 是连续消息域，每个参与 Agent 以独立 Slot 运行。公开消息、定向消息、Mention 和 Handoff 保留因果关系。繁忙 Agent 的后续输入进入引导或持久队列，避免并发写入同一 Session。

Goal 提供目标、预算、完成条件和生命周期。需要受管分工时，Execution 将 Goal 物化为不可变 Plan Revision 与 WorkGraph。Assignment 绑定明确的 Agent、Attempt 和输入契约，Submission 与 Acceptance 保存可审计结果。普通聊天和单纯 Mention 不会被人数推断为受管 Execution。

Execution 的分派、评审返回和取消通过持久 Outbox 恢复。取消命令绑定精确目标，区分 Provider 中断、本地 Context 取消、目标已结束和 Runtime 不支持，避免宽泛中断误伤后继 Round。

## 七、数据与状态

统一状态根由 `NEXUS_STATE_ROOT` 指定，默认值为 `~/.nexus`。宿主数据与 owner 数据分开保存。Agent 工作区的真实路径以数据库中的 `agents.workspace_path` 为准，诊断工具不从目录名猜测。

![Nexus 状态权威与恢复责任](./images/nexus-state-ownership.svg)

[打开状态权威图的独立 HTML 版本](./architecture-html/nexus-state-ownership.html)

```text
~/.nexus/
├── app/
│   ├── data/nexus.db
│   └── config/
└── users/<owner>/
    ├── workspace/
    │   └── .rooms/
    ├── runtime/
    │   ├── projects/
    │   └── logs/debug/
    └── state/
        └── rooms/

<agent.workspace_path>/
├── MEMORY.md
├── memory/
├── .nexus/settings.json
└── .agents/sessions/<session>/
    ├── meta.json
    ├── overlay.jsonl
    └── input_queue.jsonl
```

| 数据 | 权威所有者 | 介质 | 说明 |
| --- | --- | --- | --- |
| 用户、认证、Agent、Room、Goal、Execution、Automation | Product | SQLite 或 PostgreSQL | 业务实体与事务状态 |
| Agent 文件与长期记忆 | Agent Runtime 和受限宿主能力 | 文件系统 | 人和 Agent 都能检查的工作产出 |
| 工作区 Session 投影 | Product | `meta.json` 与 JSONL | 关联 Session、Room、Round 和输入队列 |
| Runtime Transcript | Runtime | `runtime/projects` | Runtime 恢复、Fork 与 Compact 的权威记录 |
| Room Ledger | Product | `state/rooms` | 公开协作、Directed Message、Cursor 与 Handoff 证据 |
| Room 公共资产 | Product | `workspace/.rooms` | Runtime 可读取的共享附件与产物 |
| Provider 诊断日志 | `nxs` Runtime | `runtime/logs/debug` | 默认关闭，开启后供只读诊断 |
| Connector 凭据 | Product | 加密数据库载荷 | 加密密钥由部署环境或桌面安全存储提供 |

拆分存储带来清晰的写入权威，也提高了跨存储恢复的复杂度。系统通过稳定 ID、幂等事件和持久 Outbox 关联这些证据。

## 八、安全与隔离

![Nexus 四层安全边界](./images/nexus-security-boundaries.svg)

[打开安全边界图的独立 HTML 版本](./architecture-html/nexus-security-boundaries.html)

### 身份和入口

- 服务端使用登录 Session，数据库只保存 Session Token 哈希。
- 桌面端使用每次启动生成的本地会话 Token，保护本地 API 与 WebSocket。
- WebSocket 校验允许来源和用户作用域，Session 绑定按身份恢复。
- Connector 与 Channel 的敏感授权流程要求明确的人类交互证据。

### 文件边界

来自工作区、Runtime Artifact、Transcript 和用户 Skill 的相对路径先绑定到 `os.Root` 或目录文件描述符。`internal/infra/confinedfs` 负责遍历、读写、重命名和删除，业务层完成 owner 校验后也不能把未经约束的绝对路径交回普通 `os` API。

### Runtime 隔离

Linux 服务端可通过 root-owned `nexus-runtime-launcher` 为 Runtime 分配不可登录 OS 用户、UID 与 GID、POSIX ACL、cgroup v2 和 Landlock 文件规则。Launcher 只接受受信任宿主配置，按 allowlist 构造环境，并移除宿主秘密和原始控制面能力。

普通 Agent 通过稳定的内建能力使用 Nexus，并通过宿主按 runtime round 签发的 `nexuscfg` capability 管理自己的安全配置子集。定时任务与 heartbeat 使用内置 `automation` Skill 和同样按 round 签发的 Agent-facing `nexus automation`：后台 run 只读且绑定 exact job/run，交互 mutation 经过 plan/revision/digest 与当前会话真人确认。主 Agent 额外获得宿主注入的 `nexusctl` 路径管理 owner 资源，并在私聊中通过 `nexuscfg` 和 `nexus automation` 获得各自明确的 owner 能力；owner、Agent、DM/Room 和 workspace 均由宿主锁定，Runtime Policy 拒绝作用域与 capability 覆盖。

### 凭据

Provider 凭据保留在 Product，启动 Runtime 时通过环境注入，不写入 Agent 工作区。Connector 凭据使用独立的 32 字节密钥加密。macOS 正式签名包优先使用 Keychain 保存该密钥，Windows 使用系统保护能力，本地开发可以回退到权限收紧的密钥文件。

## 九、可靠性与恢复

Nexus 的可靠性设计围绕可恢复 Session 和精确执行身份展开。

![Nexus 故障恢复判定流程](./images/nexus-recovery-flow.svg)

[打开故障恢复图的独立 HTML 版本](./architecture-html/nexus-recovery-flow.html)

- Runtime Manager 按 Session Key 管理 Client、Owner、活动 Round 和启动关闭栅栏。
- Client 换代、连接取消和闲置回收不持有全局锁，避免一个 Session 阻塞其他会话。
- WebSocket 重连会重放逻辑 Session 绑定，并以服务端 Snapshot 覆盖前端推测状态。
- Room 输入队列、Execution Outbox、Attempt 终态和 Acceptance 使用持久记录与幂等键。
- 中断绑定精确 Agent Round，完成事件和中断确认可以幂等竞合。
- Runtime Transcript 支持有界读取、Compact Boundary、Fork 和恢复，展示元数据不污染执行记录。
- 桌面 Sidecar 由原生壳监督，退出、异常和升级时统一回收子进程。

故障恢复遵循权威归属。Product 恢复协作与调度，Runtime 恢复 Transcript 和 Agent loop，前端只从 Snapshot 重建运行态。临时流式进度缺失不会改变最终业务事实。

## 十、日志与诊断

Product 日志覆盖 HTTP、WebSocket、调度、Runtime 生命周期和领域错误。`nxs` 的本地生命周期诊断默认关闭，按需记录 Query、Tool、Provider、Compact、Memory 与 Hook 摘要。请求正文和响应正文需要独立开关。

并发 Provider 调用使用 `diagnostic_call_id` 关联请求开始、响应头和完成事件。诊断接口只返回白名单摘要，不暴露提示词、工具参数值、工具结果正文或 Hook 输出。

## 十一、协议与演进规则

Nexus 有两处协议真相源。

| 协议 | 真相源 | 消费方 |
| --- | --- | --- |
| 产品 HTTP、WebSocket 和事件模型 | `internal/protocol` | Go Handler、Service、React 生成类型、Runtime 消息投影 |
| Product 与 Runtime 的进程协议 | [Agent SDK Bridge `protocol`](https://github.com/nexus-research-lab/nexus-agent-sdk-bridge/tree/main/protocol) | Product Runtime Client、`nxs`、Claude Code 兼容层 |

修改跨边界字段时，应在真相源修改并重新生成前端 TypeScript 类型。产品内部 Service DTO、Repository Codec 和存储模型留在所属领域，避免把内部实现扩散成公共协议。

Runtime 新能力先进入 Capability，再由 Product 按协商结果启用。这个规则允许 `nxs` 和 Claude Code 保持不同能力集，也避免 Runtime 升级顺序影响产品判断。

## 十二、关键架构决策

| 决策 | 原因 | 代价与约束 |
| --- | --- | --- |
| Agent loop 独立进程 | 隔离依赖、故障和闭源实现，支持替换 Runtime | 需要稳定协议、进程监督和跨边界诊断 |
| Product 持有协作，Runtime 持有执行 | 业务事务与模型循环各有单一权威 | 一个回合需要跨 Product、Bridge 与 Runtime 关联 ID |
| Capability 协商 | 支持 Runtime 版本和实现差异 | Product 必须处理能力缺失，不能按名称猜测 |
| 按权威拆分存储 | 数据所有者和恢复路径清晰 | 跨存储排障依赖一致的身份键与明确的诊断入口 |
| 产品协议集中定义 | Go 与 TypeScript 共用事件语义 | 协议变更必须同步生成和验证 |

## 十三、代码导航

| 需求 | 首要位置 |
| --- | --- |
| HTTP、WebSocket 或前端事件 | `internal/protocol`、`internal/handler`、`web/src/lib/api` |
| Product 启动和依赖装配 | `internal/app/server` |
| DM 和 Room 行为 | `internal/chat`、`internal/service/room` |
| Goal 与 Execution | `internal/service/goal`、`internal/service/execution`、`internal/service/room/realtime` |
| Runtime Session 和 Round | `internal/runtime` |
| Runtime 消息到产品事件 | `internal/message` |
| Runtime Wire 和 Capability | [Agent SDK Bridge `client` 与 `protocol`](https://github.com/nexus-research-lab/nexus-agent-sdk-bridge) |
| 桌面 Sidecar 和 OS Bridge | `desktop/macos`、`desktop/windows` |

产品仓的常规验证入口如下。

```bash
make check-go
make lint-web
make test-web
make typecheck-web
```

涉及 Runtime Wire 时还应分别验证 Bridge 和实际 Runtime。涉及状态目录时必须同步检查 Product 的迁移逻辑、工作区 Session 和 Runtime Transcript 布局。

## 参考实现

- [Nexus Product 说明](../README_zh.md)
- [Nexus 文档索引](./README.md)
- [Runtime 隔离部署](./operations/runtime-isolation.md)
- [Agent SDK Bridge](https://github.com/nexus-research-lab/nexus-agent-sdk-bridge)
