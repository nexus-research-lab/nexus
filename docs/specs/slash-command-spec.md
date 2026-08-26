# Slash 指令统一协议

## 目标

Nexus 同时承载 nxs 与 Claude Code（CC）两种 runtime。两者都通过普通用户文本
执行 Slash 指令。Nexus 需要在不启动 runtime、不让浏览器理解 runtime 私有协议
的前提下，把当前产品版本支持的 runtime 指令和 Nexus 自有指令投影为一个稳定的
Composer 目录。

Composer 固定目录的真相源是 Nexus 仓内的版本化静态清单；Nexus 内置 WorkGraph
模板由版本化模板目录追加，owner 从历史 WorkGraph 保存的命名工作图由持久目录在
bind 时追加。Claude Code 的 initialize
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
| Nexus `service/workgraphworkflow` | 维护只读内置 WorkGraph 模板；从 exact 完成态 managed Execution 生成/复用 durable Draft；统一 UI 与普通对话的查询、版本化编辑、选择和确认保存；在 runtime 投递时展开动态 Slash | 让模型伪造 owner/session/source；把内置模板写进 owner 数据；通过 UI HTTP 直接持久化命名图；保存 Tool、Assignment、Attempt、Submission、Review、Acceptance 或旧运行身份；在抽取失败时回退保存原始语义 |
| WebSocket handler | 在 bind 时按当前 Agent runtime 选择内置清单，合并 host/product/runtime、内置模板与 owner 命名工作图描述并广播完整快照 | 启动 runtime、让前端查询目录或拼接隐藏上下文 |
| Web Composer | 只消费当前 session 的完整快照，选择后发送原始 Slash 文本 | 启动 runtime、查询目录、判断命令归属 |

Host 命令先进入合并结果，因此与 runtime 同名时 Nexus host 命令保留该名称，
runtime 同名项被丢弃。名称比较不区分大小写，展示名称统一为不带 `/` 的
canonical 名称。

当前版本清单：

- nxs runtime：`compact`、`skills`
- Claude Code runtime：`compact`、`skills`
- Nexus host：`model`
- Nexus 固定产品提示：`visualize`、`workgraph`
- Nexus 内置工作图模板：`deep-research`、`build-ship`、`decision-brief`、`review-improve`
- Nexus owner 命名工作图：用户保存的命名项，例如 `market-scan`

两个 runtime 在 Composer 中最终都展示 `compact`、`model`、`skills`、`visualize`
与 `workgraph` 五个核心入口，并追加 Nexus 内置模板及当前 owner 的命名工作图，但执行归属不同：`compact` 交给当前 runtime，`model` 始终由 Nexus
校验并持久化 Agent 的 Provider/模型选择，`skills` 由 Composer 打开完整 Skill
选择器并替换为选中的具体 `/skill-name`，不会把字面量 `/skills` 发给 runtime；
`visualize` 由 Nexus 在投递 runtime 时展开为简短的 Generative UI 提示；`workgraph`
只要求当前请求使用 fresh WorkGraph 协作，不承担保存语义。动态 `/<command>` 在同一
投递边界展开为语义节点和依赖模板，再由 `execution-orchestrator` Skill 通过
`nexus.execution_read|write` 创建 fresh Plan/WorkGraph。
nxs 的 session summary 是 runtime 内部自动维护数据，不投影为公开 Slash 指令；
需要立即释放上下文时统一使用 `/compact [instructions]`。

内置模板固定保留“可并行分支 → 显式汇合 Gate → 独立验证 → 终态交付”的最小拓扑：

| Slash | 标准拓扑 |
| --- | --- |
| `/deep-research` | 问题定界 → 第 1 轮拆分与策略 → 权威/对照证据并行收集 → 充分性评估；不足时在同一工作图按需追加第 N+1 轮“缺口诊断 → 策略调整 → 两条定向证据线并行重收集 → 再评估”，充分后才进入综合 → 事实与引用复核 → 报告 |
| `/build-ship` | 范围 → 设计 → 实现 → 验证/审查并行 → 交付就绪 Gate；不足时按 blocker 类型定向修复并重新验证/评审 → 集成交付 |
| `/decision-brief` | 决策定界 → 证据/选项并行 → 权衡评估 → 独立挑战 Gate；不足时按证据/标准/方案/实验缺口追加必要分支 → 建议 |
| `/review-improve` | 基线 → 质量审计/体验审计并行 → 优先级 → 修改 → 独立复核 Gate；不足时按未修复/回归/无改善/rubric 错误选择定向修订、重审计或重建基线 → 改进后交付物 |

验证或审查节点可通过 WorkGraph review 的 `changes_requested` 回到责任节点返工，
模板本身仍保持 DAG，不用静态环伪造迭代。验证记录不是最终交付物，因此终态只标在
报告、集成交付、建议或改进后交付物节点。

`/deep-research` 默认不是单轮或固定两轮搜索：某轮评估即使准确得出“证据不足”，该评估工作
本身仍可验收，但其 verdict 必须阻止综合，并在同一 Execution、同一 WorkGraph 中按需追加
下一段带编号的“缺口诊断—策略调整—并行定向收集—重新评估”节点。每次追加都把综合节点的
前置依赖移到最新评估；只有累计证据充分时才进入下游。迭代次数由证据缺口决定，直到充分或
触发研究策略预先声明的迭代上限/停止条件；不能新建另一张工作图、覆盖前一轮事实，或为了结束
而直接综合。底层 immutable Plan revision 只记录同一工作图的拓扑演进，不是面向用户的“第二版”。

`/build-ship` 的迭代单位是一次有边界的修复，而不是重复完整开发流程。质量 Gate 把 blocker
区分为范围、设计、实现、测试、文档或外部依赖问题，只追加必要修复节点；若发生实质变更，
必须重新并行执行验证与独立评审，再汇合到新的质量 Gate。无法在当前边界解决的外部依赖应
显式 block 或触发 replan，不能用重复尝试伪装进展。

`/decision-brief` 的迭代由挑战性评审诊断：缺证据时追加定向收集或低成本实验，标准有误时
修正 rubric，方案缺失时补方案；只有两个以上独立缺口才并行。随后重新比较并再次挑战。
收口结果可以是稳健建议、明确的有条件/延后决策，或一个有边界的实验计划；不要求为了得出
确定答案而无限收集信息。

`/review-improve` 的迭代由复核失败类型决定：问题未关闭或局部回归进入诊断—定向修订—受影响
检查与回归检查；大范围修改才重新触发质量/体验审计；基线或 rubric 无效则回到基线与优先级。
每轮继续保留前一轮审计、改动与复核事实，最新复核充分后才允许交付。

DM 与 Room 都只把用户输入的 Slash 原文写入可见时间线，并在 runtime 投递边界展开；
Slash runtime 输入是独立原子消息，不追加用户画像等宿主动态后缀。Room 的展开结果不进入
有界 public/private history 拼装，因此不会因公共上下文压缩丢失模板尾部节点；后续普通
Room round 仍只从持久历史看到用户原始命令。

固定清单只承诺 Nexus 版本内置指令；内置 WorkGraph 模板只读且不写 owner 数据；owner
命名工作图是 Nexus 自己持久化并验证的动态目录，不等同于扫描用户本机 Skill、插件
或 MCP 命令，也不能跨 owner 投影。升级前已存在的同名 owner 图优先于新增内置模板，
避免改变既有 Slash 的用户语义；新保存图不能占用内置模板名称。

## 生命周期

1. `service/slashcommand.Catalog` 在构造时直接装入与当前 Nexus 版本一起维护的
   nxs 和 Claude Code 静态清单；这个过程没有文件 IO、子进程或 runtime session。
2. 浏览器发送 `bind_session`，只携带会话地址和已有的 Room/Agent 作用域信息。
3. WebSocket handler 解析当前 Agent 的 runtime 偏好，选择对应内置清单，与当前
   作用域的 Nexus host、固定产品描述、内置模板与 owner 命名工作图合并、校验、排序后发送一条完整 `command_catalog`
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
- `/<command> [request]` 从有效模板目录读取内置或当前 owner 已保存的抽象责任节点、协作角色和依赖。
  每次调用必须创建新的 Execution/Plan/Work Item identity，
  不得复制源图的 Agent、状态、结果、Artifact 或审核事实；
- 用户在完成态图标题栏请求保存时，Web 通过 `POST /workgraph/previews` 让 service 使用 owner 默认对话模型接收完整 source logical-key、父子层级和依赖，默认保留宿主标记的结构关键节点，并主要抽象具体任务语义。Draft 按 owner/session/source Execution 唯一并进入数据库，不进入命令目录；再次请求同一 source 直接恢复。每次修改保存不可变完整版本，`head_revision` 做 CAS，`selected_revision` 表达用户偏好。一个 Session 有多张 WorkGraph 时通过 exact execution_id/preview_id 区分。
  用户可直接修改元信息，也可进入 Nexus 主智能体承载的目录隐藏专用 DM；该 Session 不 fork 或继承来源 transcript、Connector、workspace 或权限，关闭 UI 不删除，再次打开继续对话。左侧展示本地接待说明和隐藏会话自身的编辑消息，右侧展示实时 preview 与版本目录；只开放 `execution-orchestrator`、`revise_workgraph_preview` 与 `select_workgraph_preview_revision`。服务端校验完整草图、revision、logical key、kind、父子结构、DAG、key 主路径和 terminal 交付。对本地格式合法的 `slash_name`，保存确认页通过 owner-scoped availability 查询做防抖预检，并携带 exact `preview_id` 以允许已保存 aggregate 保留自己的命令名；格式错误、名称占用和检查失败必须分开反馈，只有当前输入的查询结果为可用时才能提交。该预检不替代保存调度中的同源检查和数据库唯一索引；并发冲突仍以 HTTP 409 回到命名输入框。用户确认后，
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
  和目录测试；项目 Skill、插件命令等动态内容不进入产品补全。系统 WorkGraph 模板只从
  `internal/service/workgraphworkflow/builtins.go` 投影并保持只读；owner 命名 WorkGraph 只从
  `workgraph_workflows` 权威目录投影，并按 owner fence 读取、删除。
