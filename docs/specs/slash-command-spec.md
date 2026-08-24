# Slash 指令统一协议

## 目标

Nexus 同时承载 nxs 与 Claude Code（CC）两种 runtime。两者都通过普通用户文本
执行 Slash 指令。Nexus 需要在不启动 runtime、不让浏览器理解 runtime 私有协议
的前提下，把当前产品版本支持的 runtime 指令和 Nexus 自有指令投影为一个稳定的
Composer 目录。

Composer 固定目录的真相源是 Nexus 仓内的版本化静态清单；owner 从历史 WorkGraph
保存的命名工作图由持久目录在 bind 时追加。Claude Code 的 initialize
control response 与 nxs 的同形能力仍可供 bridge 的其他宿主使用，但 Nexus
不会在 App 或 session 生命周期内读取它们。runtime 新增且尚未进入当前清单的
指令仍可手动输入并原样透传，只是不参与补全。

## 权责边界

| 层 | 责任 | 不负责 |
| --- | --- | --- |
| nxs / Claude Code | 解析收到的 `/name args` 普通用户文本；解析并展开各自的 Skill Slash | 维护 Nexus Composer 目录；识别或执行 Nexus host 指令 |
| `nexus-agent-sdk-bridge` | 统一普通文本发送和单轮隐藏上下文清理；保留通用初始化能力读取 | 合并或同步 Nexus Composer 目录；发明 Slash RPC |
| Nexus `runtime.Manager` | 管理业务 session/runtime 连接与 round 生命周期 | 持有 Slash 目录或为补全请求启动子进程 |
| Nexus `service/slashcommand` | 持有当前 Nexus 版本的 nxs/Claude 静态清单与 `/visualize`、`/workgraph` 固定产品提示 | 读取 runtime 私有 metadata；保存命名工作图；绑定 session |
| Nexus `service/workgraphworkflow` | 从 exact 完成态 managed Execution 生成/复用 durable Draft，统一 UI 与普通对话的查询、版本化编辑、选择和确认保存，并在 runtime 投递时展开动态 Slash | 让模型伪造 owner/session/source；通过 UI HTTP 直接持久化命名图；保存 Tool、Assignment、Attempt、Submission、Review、Acceptance 或旧运行身份；在抽取失败时回退保存原始语义 |
| WebSocket handler | 在 bind 时按当前 Agent runtime 选择内置清单，合并 host/product/runtime 与 owner 命名工作图描述并广播完整快照 | 启动 runtime、让前端查询目录或拼接隐藏上下文 |
| Web Composer | 只消费当前 session 的完整快照，选择后发送原始 Slash 文本 | 启动 runtime、查询目录、判断命令归属 |

Host 命令先进入合并结果，因此与 runtime 同名时 Nexus host 命令保留该名称，
runtime 同名项被丢弃。名称比较不区分大小写，展示名称统一为不带 `/` 的
canonical 名称。

当前版本清单：

- nxs runtime：`compact`、`skills`
- Claude Code runtime：`compact`、`skills`
- Nexus host：`model`
- Nexus 固定产品提示：`visualize`、`workgraph`
- Nexus owner 命名工作图：用户保存的命名项，例如 `deep-research`

两个 runtime 在 Composer 中最终都展示 `compact`、`model`、`skills`、`visualize`
与 `workgraph` 五个核心入口，并追加当前 owner 的命名工作图，但执行归属不同：`compact` 交给当前 runtime，`model` 始终由 Nexus
校验并持久化 Agent 的 Provider/模型选择，`skills` 由 Composer 打开完整 Skill
选择器并替换为选中的具体 `/skill-name`，不会把字面量 `/skills` 发给 runtime；
`visualize` 由 Nexus 在投递 runtime 时展开为简短的 Generative UI 提示；`workgraph`
只要求当前请求使用 fresh WorkGraph 协作，不承担保存语义。动态 `/<command>` 在同一
投递边界展开为语义节点和依赖模板，再由 `execution-orchestrator` Skill 通过
`nexus execution` 创建 fresh Plan/WorkGraph。
nxs 的 session summary 是 runtime 内部自动维护数据，不投影为公开 Slash 指令；
需要立即释放上下文时统一使用 `/compact [instructions]`。

固定清单只承诺 Nexus 版本内置指令；owner 命名工作图是 Nexus 自己持久化并验证的
动态目录，不等同于扫描用户本机 Skill、插件或 MCP 命令，也不能跨 owner 投影。

## 生命周期

1. `service/slashcommand.Catalog` 在构造时直接装入与当前 Nexus 版本一起维护的
   nxs 和 Claude Code 静态清单；这个过程没有文件 IO、子进程或 runtime session。
2. 浏览器发送 `bind_session`，只携带会话地址和已有的 Room/Agent 作用域信息。
3. WebSocket handler 解析当前 Agent 的 runtime 偏好，选择对应内置清单，与当前
   作用域的 Nexus host、固定产品描述与 owner 命名工作图合并、校验、排序后发送一条完整 `command_catalog`
   权威快照。这个过程不创建 DM/Room runtime session。
4. 浏览器按 session key 和 Agent 身份接收完整快照，只替换本地目录状态；
   浏览器不发送 `get_command_catalog` 或 `ensure_runtime_session`。

绑定时的目录读取和 host 派发都经过当前 owner/session 校验；host registry 只在
命令匹配成功后调用鉴权器，未知 Slash 不会因为 host registry 的存在而改变
runtime 的错误或透传语义。

## 状态

`cold` 只作为前端切换 session 后等待后端快照的本地哨兵；已知 runtime 的绑定
事件直接为 `ready`。`unavailable` 只表示当前 Nexus 版本没有对应 runtime 清单。
后端不再产生 `starting`，因为目录没有同步阶段。Room 当前公开 host 作用域和产品
提示型指令，普通 runtime 命令暂不投影，因此不应通过 Room 目录事件启动或绑定
Room runtime。

## 执行

Composer 选择任意 `host` 或 `runtime` 描述后，仍发送一条普通 `chat` 文本，例如
`/model anthropic/claude-sonnet-4-6`。WebSocket handler 先让 host registry
尝试匹配：

- 匹配成功：执行 host handler，返回其产生的产品事件；
- 未匹配：原样交给 DM runtime，由 nxs/Claude 自己解析；
- 带附件的已知 host 指令：在 handler 执行前拒绝；
- DM 的 runtime Slash 输入标记为 atomic，清除 bridge 尚未消费的隐藏上下文，
  不追加 Goal、recovery 或 emotion context；
- `/skills` 选择器选中具体 Skill 后仍发送原始 `/skill-name args`；bridge 把它当
  普通 user message，nxs 或 Claude Code 在 runtime 内解析并读取自身可访问的
  `SKILL.md`；
- `/visualize [request]` 在 Composer 和用户时间线中保留原始 Slash 文本，仅在
  DM/Room 的 runtime 投递边界展开为一段简短的 Generative UI 提示；
- `/workgraph [request]` 同样保留原始文本，只在投递边界要求模型加载
  `execution-orchestrator` 并为当前请求创建 fresh managed WorkGraph；它不保存模板；
- `/<command> [request]` 从当前 owner 目录读取已保存的抽象责任节点、协作角色和依赖。
  每次调用必须创建新的 Execution/Plan/Work Item identity，
  不得复制源图的 Agent、状态、结果、Artifact 或审核事实；
- 用户在完成态图标题栏请求保存时，Web 通过 `POST /workgraph/previews` 让 service 使用 owner 默认对话模型接收完整 source logical-key、父子层级和依赖，默认保留宿主标记的结构关键节点，并主要抽象具体任务语义。Draft 按 owner/session/source Execution 唯一并进入数据库，不进入命令目录；再次请求同一 source 直接恢复。每次修改保存不可变完整版本，`head_revision` 做 CAS，`selected_revision` 表达用户偏好。一个 Session 有多张 WorkGraph 时通过 exact execution_id/preview_id 区分。
  用户可直接修改元信息，也可进入 Nexus 主智能体承载的目录隐藏专用 DM；该 Session 不 fork 或继承来源 transcript、Connector、workspace 或权限，关闭 UI 不删除，再次打开继续对话。左侧展示本地接待说明和隐藏会话自身的编辑消息，右侧展示实时 preview 与版本目录；只开放 `execution-orchestrator`、`revise_workgraph_preview` 与 `select_workgraph_preview_revision`。服务端校验完整草图、revision、logical key、kind、父子结构、DAG、key 主路径和 terminal 交付。用户确认后，
  Web 调用 preview save 调度端点，宿主在 fresh 目录隐藏内部 DM Session 启动 `HiddenFromUser + Synthetic +
  purpose=workgraph_distillation` 的 Agent round；该 Session 不 fork 或续写源 transcript，只通过 capability 绑定原 source session 与 exact preview，pending、过程、工具与完成事件全程隐藏，不生成聊天消息或改写源 Composer；
  `execution-orchestrator` 读取 fresh contract 并通过 `distill_workgraph` CLI mutation 原样保存。
  Agent 不得重新 inspect、选节点、命名或抽象，HTTP 调度端点不得直接创建命名图；
- 普通 DM/Room 中用户也可以要求智能体沉淀或继续编辑 WorkGraph。模型加载 `execution-orchestrator`，先调用 `inspect_workgraph_library` 读取当前 Session 的 completed sources、Drafts 和 owner 命名图，再用 exact `extract_workgraph_preview`、`get_workgraph_preview`、`revise_workgraph_preview`、`select_workgraph_preview_revision`、`save_workgraph_preview`；这些 operation 不依赖 active Execution，保存只接受当前对话中的明确确认，并且只回复当前会话，不向来源或其他 Session 透传。成功结果自动把最后一份完整图快照渲染为当前回复的草图卡片，来源对照只在用户显式打开后按 exact source identity 加载；
- 已保存命名图可从能力页恢复原 Draft、selected revision 和隐藏编辑 Session；旧数据没有 Draft 时只建立一次初始版本。后续保存更新同一命名图 aggregate 并追加版本，不重复抽取、不制造同名副本；
- inline Skill 的完整正文只作为 runtime 内部 meta user 进入模型上下文，不作为
  tool result、普通用户正文或 Nexus next-turn context；`context: fork` 由 runtime
  自己执行并只回写本地结果。

显式单轮 Skill 可以来自当前 Agent 的“可启用”列表；这只代表用户本轮明确授权使用，
不会改写 `skill_ids`、`disabled_skill_ids` 或 Agent 设置。nxs 把未绑定/未启用集合
只用于模型自动发现，用户显式 Slash 从完整 `user-invocable` 目录解析；Claude Code
沿用自己的直接 Skill command 路径。未知 Slash、不可见 Skill 和 Room scope Skill
仍不在 Nexus 侧截获，继续保持 runtime 透传语义。

`/model` 只接受 Nexus Provider 目录里的模型，更新 Agent 的显式
`provider/model` 配置并广播 `agent_updated`；它不启动或调用 runtime。已存在的
runtime 连接在下一轮发送前按新的 Agent 配置重建/恢复，模型选择因此仍由 Nexus
保持唯一真相源。host handler 产生的确认消息带终止投影，并用规范的 `finished`
round 状态立即收口，不能把 Composer 留在“回复中”；确认消息使用 `transient`
投影保留在当前时间线，但不进入 runtime 历史、后台缓存或未读。对应的用户
`/model ...` 输入同样保留为当前时间线的 `transient` 用户消息，避免确认失去来源。

未知 Slash 不在 Nexus 侧报错，以便 runtime 新增指令时旧版 Nexus 仍能透传。
atomic Slash 发送前会清理 bridge 尚未消费的旧隐藏上下文；Skill 正文由 runtime
在本轮内部生成，不进入产品侧 buffer。

## 维护规则

- runtime 固定指令变化时，在升级 Nexus 支持的 runtime 版本时同步更新静态清单
  和目录测试；项目 Skill、插件命令等动态内容不进入产品补全。owner 命名 WorkGraph 只从
  `workgraph_workflows` 权威目录投影，并按 owner fence 读取、删除。
