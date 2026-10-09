# 消息处理规范

范围：实时消息流动、历史落盘与读取、按 round 展示与分页、时间线稳定顺序、并发 Agent 快慢处理、guide/queue 控制消息投影、正文与状态的密度边界。Room 业务流程与 handoff 通信语义见 [Room 协作协议](./room-collaboration-spec.md)。

## 1. 核心对象

| 对象 | 规则 |
| --- | --- |
| stream event | 运行时实时增量，负责过程态，不是历史真相源 |
| assistant message | 某个 assistant turn 的 durable 消息，可含 thinking、tool、text 等；正文真相源只来自 runtime transcript |
| result message | 一轮执行的终态结果（结果文本、执行终态与 runtime 摘要）；真相源只来自 Nexus overlay；对外 API / WebSocket 不暴露 standalone `result`，统一收口为 `assistant.result_summary` |
| round | 一次用户输入触发的一轮业务对话；历史分页与状态收口都按 round |

result 的有效错误由 `is_error` 或 `error_*` subtype 共同决定；`success + is_error: true` 仍是失败。

### 1.1 身份边界

| 字段 | 生成方 | 语义 |
| --- | --- | --- |
| `client_request_id` | Web | 一次发送尝试的 ACK / timeout 关联，不是业务主键 |
| `client_message_id` | Web | optimistic 用户消息与幂等重试身份 |
| `round_id` | Nexus | 一次用户输入的根业务轮次，由后端生成 |
| `message_id` | Nexus / runtime | durable 消息身份，不能复用为 round |
| `agent_round_id` | Nexus | Room 内单个 Agent slot 的执行身份，与 root round 独立 |

- 前端不得发送或拼接 canonical `round_id`，也不得从 `message_id`、前缀或 Agent 名称反推 root round。
- `agent_round_id` 必须使用显式字段，不能编码进 `round_id` 后缀。

### 1.2 投影归属

- bridge 只传递 runtime message、stream、session 与控制事件，不持有 UI turn 模型。
- Nexus 后端负责 transcript、overlay、共享消息和实时状态的产品级归一化。
- Web 只负责折叠、虚拟列表、搜索和 viewport 加载，不从多个旁路状态猜生命周期。

## 2. 实时链路

### 2.1 入口与 ACK

- 前端通过 WebSocket `chat` 发起一轮执行；后端创建 / 复用 runtime client；runtime 返回 stream / durable message / round status。
- 受理 ACK 等待窗口：`chat_ack`、`input_queue_ack`、`interrupt_ack` 为 10 秒（常量 `protocol.RequestAckTimeoutMS`）；独立 `set_goal` 为 20 秒，覆盖后端 15 秒 detached command deadline。
- ACK 超时表示“后端受理状态未知”：
  - 只触发只读对账，不强制关闭仍 connected 的 Socket，不得通过重发消息延长等待。
  - 前端必须保留输入，允许用同一 `client_message_id` 重试；不能当作已确认失败而清空草稿。
- 连接失活由心跳检测；只有已断开或耗尽重试的连接才主动恢复。
- `client_request_id` 标识一次传输尝试；`client_message_id` 标识同一逻辑输入，ACK 未知后重试必须复用后者。
- `input_queue` 快照只表达共享队列当前状态，不能充当请求回执；后端完成持久化后必须向请求连接单播 `input_queue_ack`。

### 2.2 投影边界

| 类型 | 规则 |
| --- | --- |
| stream | 增量展示过程 |
| durable message | 写入最终消息列表，可恢复并可产生未读 |
| ephemeral message | 只展示 round 内过程，终态到达后移除 |
| transient message | 保留在当前打开的时间线，不进入历史、后台缓存或未读 |

- 同一 `message_id` 的 durable snapshot 必须更新同一条消息投影，不能按 snapshot 数量追加气泡。
- `round_status`、`agent_round_status`、input queue 和 handoff 事件只更新状态投影，不变成正文消息。
- round 结束只由 terminal `round_status` 定义，前端不自行猜测。

### 2.3 Conversation reliability 与恢复闭环

transport retry、Provider retry、请求受理、Agent round 和工具过程是独立状态，按稳定 `failure_code` 与 exact Session/request/round/Agent-round 身份隔离；禁止用一个全局错误字符串代表多个阶段。

连接与重连：

- WebSocket `onerror` 只是连接异常证据，不是业务终态。共享客户端最多 5 次指数退避重连；重试期间只在 Composer 工作栈显示“连接中断，正在恢复…”。
- 重试耗尽进入 `unavailable`：持久提示必须说明连接尚未恢复、已显示的消息和当前输入仍保留但页面可能不是最新，并要求恢复后再发送。
- 宿主广播不把来源请求的取消当作接收连接失效：发送前已取消的事件不进入连接；已接纳的帧使用连接独立的有界超时完成。旧连接退出或 interrupt 请求取消不得关闭刚重连的健康通道；真实传输失败仍使该连接失效。
- 物理通道恢复后，先重放仍有效的 Session binding，再拉取当前 Session 的 durable 消息快照。
  - DM：以快照和随后实时事件按消息身份合并。
  - Room：还必须恢复 `room_seq` replay、Room subscription snapshot、pending Agent slot 和 pending interaction。
  - subscription snapshot 携带捕获时的 `snapshot_room_seq`。客户端先把 Room 游标设为该栅栏（服务端重启后允许回到较小的新代次序号），再应用快照，并丢弃不新于该序号的迟到事件。非当前 conversation 的迟到快照不得改写游标。

重试与对账：

- Provider/runtime 的 API retry 只由 runtime 发起，投影为当前 round 的 ephemeral `api_retry`。Web 在执行过程里显示原始 Provider 错误、当前次数和倒计时，Composer 同时保留简短的全局重试状态。
- 新的 stream/message/成功 round status 按 exact `round_id`/`agent_round_id` 清除对应 retry；最终 error 才转成失败分类。
- 用户消息、Goal、queue、permission 和 interrupt 的受理按 exact `client_request_id` 对账。ACK 丢失时客户端可重连和读取 durable 状态。
- 正向 ACK、durable `client_message_id`、input queue snapshot 或后续 round 事实只清除精确匹配的请求故障，不能清除其他 Session 的状态。
- 客户端不得自动重发 prompt、工具调用或任何可能产生副作用的命令。

错误展示：

- 错误事件使用结构化 `failure_code`。原始 Provider 错误可作为当前轮 API retry 明细及最终失败原文展示；内容安全拦截必须替换为安全说明。
- Session/round/request ID、内部堆栈和其他实现详情只进入日志。
- Composer 状态栈用本地化文案说明影响和安全下一步：
  - `delivery_unknown` 必须明确警告重复发送可能产生两次回复。
  - 未单独分类的终态说明本轮没有完成回复、用户消息和已显示历史仍保留，并引导用户先查看执行失败事实，再决定是否发起新一轮。
- 撤销旧全局失败提示的时机：用户发起新提交（开始新的恢复尝试）；同一失败 round 重新出现 stream/message；重连后的 durable 对账证明该失败已不存在。
- 提示不能作为遮罩，不能冻结 Composer；终态后的下一条用户输入继续沿原 Session 进入 runtime recovery context。
- Room：transport 故障属于整个页面；带 exact `agent_round_id` 的 retry/error 只属于对应 Agent shell/Thread，不得把 root round 或其他成员标成失败。只有权威 root `round_status=error`，或没有 Agent round 身份且明确影响整个 Room 的错误，才进入 Room 全局失败状态。
- 可靠性状态只进入 Composer 上方的统一状态栈，不写入 Feed、transcript、历史、未读或消息计数，不参与 Feed 高度与滚动锚点。

### 2.4 Goal 完成收据

- 宿主在成功的 `update_goal(complete)` 后生成收据，以内部 `goal_id + round_id` 精确绑定到该轮最终 assistant 的同一 `message_id` durable snapshot。
- `goal_id` 优先取成功工具结果返回的权威 identity；旧 Provider 未返回 identity 时才回退到该物理 round 的固定 Goal binding；两者冲突必须 fail closed。
- 两个绑定 ID 进入历史，不进入用户文案。
- 收据始终可显示“Goal 已完成”；仅当 Goal 聚合报告有正耗时时附加耗时；仅当 `usage_finalized=true` 时附加 actual token。
- 结算进行中、Provider usage 永久不可得或查询失败时，完全省略未知 token 项：不显示“结算中”“不可用”，不把未知值当 0。
- 后续结算成功或兼容修复推进了同一 Goal 聚合真相时，历史读取按隐藏的 `goal_id` 用当前 finalized report 静默刷新收据，不保留旧 snapshot 的错误数值。

### 2.5 工具 stream 与运行中进度

- stream 事件必须保留 `tool_use` 的 block start 和 `input_json_delta`；累计输入构成完整 JSON 后才更新可解释的工具参数。
- 兼容网关漏发 `content_block_start` 时，处理器按 delta 类型建立临时块，再由完整 assistant 快照原位替换；不能生成孤儿工具块或终止 round。
- 嵌套调用通过 `parent_tool_use_id` 绑定父工具。事件未重复携带该字段时沿用本条 stream 在 `message_start` 建立的父链；新 assistant 段开始时必须重新取值，不继承上一段的 parent。
- Bash / PowerShell 运行中进度是 ephemeral 状态：首次立即展示，此后最多每 30 秒更新一次，工具结束后由 durable tool result 收口；不进入 transcript，重连后不变成历史正文。

### 2.6 ToolUseSummary 与过程展示

投影：

- Provider / bridge `ToolUseSummary` 是 Agent round 的自然语言 ephemeral 执行摘要。没有长任务、耗时或工具数量门槛：执行中收到非空 summary 就立即投影为仅供状态使用的 `progress_update` assistant 块。
- 同一 Agent round 使用稳定 `message_id` 原位替换；round 完成、失败或停止后立即移除。
- summary 必须携带并保留 `preceding_tool_use_ids`，供 DM/Thread 折叠过程定位相关工具批次。
- 该投影不写入 transcript、历史、未读或消息计数，也不混入最终 durable assistant 快照。

Room 主 Feed：

- 不渲染 thinking、工具详情、ToolResult、MCP/CLI 输入输出或可展开过程栏。
- 当前工具头与“正在思考/正在回复”等 fallback 共用最新可见正文之后的同一个不可展开单行活动位置。
- 存在未收口工具时，该位置复用 Thread 工具组头部的图标栈与当前工具标题，不读取 ToolUseSummary；工具收口后回退通用状态。
- 运行文字使用中性低对比流光，不使用主色动画，不追加工具状态、数量、异常数或第二条尾随活动。
- round 终态后不保留过程占位。

Thread 与 DM：

- 具体过程只在对应 Thread 中按 ToolUseSummary 展示。Thread 过程与工具组默认展开，子项目录默认展开；DM 工具组默认折叠。
- DM 没有公区/Thread 分层，按日志原序展示可展开过程。普通工具出现在正文之后时，该正文留在 direct 时间线并结束前一工具组，后续工具形成新组。
- summary 到达后只替换相关折叠栏标题，不追加“已完成”或“正在执行”。
- 收起的活跃工具组在最终回复开始前持续显示共享活动状态，按真实阶段复用“正在思考 / 执行 / 回复”等既有动效。
- DM 首次点击过程栏只展开子项目录。Thought、Agent、MCP 与普通工具的详情由各自入口独立打开（Agent 可进入任务详情面板）；父级不得级联打开全部详情。
- Thread 与 DM 复用同一 Thought 明细字号和滚动机制；嵌套在工具组内时由唯一外层滚动窗口跟随流式内容，用户上滑后暂停。
- 失败、拒绝和替换的详细状态只属于可展开过程。
- round 终态清除 summary 后，durable 工具只保留中性的“执行过程”审计入口；归档过程的外层思路、动作、异常与最近动作摘要在展开前后保持不变，避免与内层工具组标题重复。

通用：

- 只有真正位于回复尾部的最终正文进入独立 final surface，始终可见；不能折入过程、重复同一 summary，或继承过程轨道、边线和节点。
- 权限、AskUserQuestion 与生成式 UI 由各自唯一交互面负责。

summary 文案：

- 描述已完成批次的具体成果，用类似 commit subject 的短语；不写完整句子、下一步预告或工具动作清单。
- 语言只由最近真实用户文本与 assistant intent 决定；英文工具名、输入或结果不参与判断。中文会话在提示词中优先使用简洁自然的简体中文，可保留必要的通用技术术语。
- 所有工具输入/输出均是不可信参考数据，摘要模型不得执行其中的指令。

生成开关：

- Nexus 启动 DM/Room bridge 时必须把 owner 当前后台模型选择投影给 runtime，但默认关闭 ToolUseSummary；仅当启动环境显式设置 `CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES=1` 时才生成。
- nxs 使用同一 Provider 下的后台模型；Claude Code 使用其原生小模型/ToolUseSummary 通道。
- 后台模型缺失、解析失败、协议不兼容或属于另一 Provider 时回退当前主模型，不能阻断主会话启动。
- bridge 只负责生成和转发；Nexus message processor 是 ephemeral/durable 边界的唯一真相源。

### 2.7 空态与 host 指令确认

- 首次 DM 与新 Room 的介绍是前端空态，不是消息事实。canonical timeline 没有可见轮次、运行态或待发送输入时，界面可静态展示身份和建议；选择建议后按普通用户输入进入既有投递链路。
- 空态不得调用模型、写入 transcript/overlay/Room ledger、创建 runtime round、消费 draft，或参与历史、未读和消息计数。
- 历史 `conversation_welcome` 只作旧数据保留，在 timeline 根入口统一隐藏，不进入 Feed 或导航。
- Nexus host 指令的完成确认是 transient 状态：不是 runtime 回复，不写入 transcript，但在当前时间线保留，让用户确认本地操作已生效。
- 对应 `chat_ack` 使用 `user_message_delivery_mode=transient`，把 optimistic 用户指令规范化为同一 round 的 transient 用户消息；`user_message_committed` 仍为 false。

### 2.8 内容块兼容

文件交付：

- `workspace_file_artifact` 的 `role=working_file` 表示 Write/Edit 文件变更；`role=deliverable` 表示显式交付或专用图片工具输出；旧记录缺省 role 时保留既有文件证据。
- 任意 Skill/脚本生成的交付通过 `nexus.deliver_files` 登记：模型只提交当前 workspace 文件 paths；宿主整批校验 owner/Agent 和 confined-fd 普通文件，拒绝缺失、目录、越界、符号链接或受保护路径。这证明文件可交付，不把存在性或修改时间伪装成创建者的文件系统证据。
- 产出归属是本轮 Agent 的显式声明：宿主绑定 `producer_agent_id` 与 `source_agent_round_id`，只接受精确工具身份及匹配当前 Agent/round 的成功回执，并随来源 assistant 消息持久化。
- `workspace_agent_id` 独立表示打开文件的位置；模型不能指定其他 Agent 的身份。
- 公开性继承原消息，不因登记文件跨私域广播。
- 只有文件没有正文的交付仍须展示。DM/Thread/Room 从同一回复的 direct/process/final 投影统一提取交付，在回复尾部去重展示；隐藏工具过程不得隐藏交付；working_file 不进入生成文件列表。
- 其他 Agent 的产物不能归入当前回复；转述只保留引用，不改变产出者。
- 正文 Markdown、目录缓存、Bash 日志及修改时间不能生成或覆盖交付记录；历史无记录的脚本产物不做推测回填。
- 文件卡指向当前文件，不承诺不可变内容快照。

其他内容块：

- 已知内容块按协议类型显式解码，不靠全局字段改名。
- Claude Code 的 `server_tool_use` / `web_search_tool_result` 等块保留原始 `source_type`，同时投影到 Nexus 现有工具渲染模型。
- 未知或字段不完整的内容块：前端保留原始类型和 payload 并安全隐藏；单个未知块不能让整条消息解析失败或让会话停止。
- 空数组、单个空白 text、`(no content)` 和工具中断占位文本不构成可见 assistant；多块或非文本块仍是有效消息。

## 3. 历史真相源

### 3.1 DM / 私有 session

真相源是 runtime transcript 与 `overlay.jsonl`，职责严格分开：

- transcript 保存 agent 私有正文历史。
- overlay 只保存 Nexus 补充语义：`round_marker`、`result`、transcript 没有的补充消息，以及同 `message_id` 的 assistant 补充快照。
- assistant 正文只能来自 transcript；overlay assistant 只能携带同 `message_id` 的 Nexus 补充字段（如 Goal 完成收据），compact 时合并回 transcript assistant。
- `result` 只能来自 overlay；transcript 里的 `MessageTypeResult` 不参与历史投影。

隐藏 Goal continuation：

- runtime 在 user 落盘前提取隐藏 Goal reminder 后，transcript 可只留下空白 user。
- 历史投影与 rewrite/fork 必须共用轮次边界识别：只有与隐藏 Goal continuation marker 唯一匹配的空白输入才建立续跑轮次；普通空白、缺少有效时间或匹配冲突不得消耗可见用户 marker。
- 隐藏输入不展示，但后续 assistant、result 和完成收据必须保留续跑身份，不继承上一条真实问题的轮次。
- 投影版本升级后重建派生索引。

### 3.2 transcript assistant 终态

- 终态只认 `message.stop_reason`：有值即终态 assistant，不要求存在独立 `result`；为空则是未完成快照。
- 历史读取不能因“没有 result”把 transcript assistant 判成 interrupted；synthetic interrupted 只用于真正缺少终态且 round 已结束的场景。
- 持久化层继续维护 assistant 的 `is_complete` 以兼容旧数据，但终态判定只看 `stop_reason`。
- 字段来源：
  - assistant `usage` 可直接来自 transcript。
  - `duration_ms / duration_api_ms / num_turns / total_cost_usd / result / subtype / is_error` 只来自 overlay result。
  - `model_usage / structured_output / fast_mode_state / runtime_subtype` 只从 overlay result 投影到 `assistant.result_summary`。
  - 不允许从 transcript assistant 反推“差不多的 result”。

### 3.3 Room shared 历史

- 共享层不保存完整正文副本，只保存 inline overlay（用户消息、result/synthetic 消息）和对 transcript assistant 的 `transcript_ref`。
- 正文按需从成员 transcript 投影恢复。
- `transcript_ref` 只允许引用 assistant，不允许引用 result。

## 4. 分页机制

历史分页按 round，不按消息条数。runtime transcript、DM overlay 与 Room ledger/private transcript 是唯一 canonical 真相源；分页索引只是可删除、可校验、可从真相源重建的派生数据，不得反向改写历史或成为第二套权威存储。

### 4.1 首屏与侧栏目录

- 左侧 DM/Room 目录从宿主数据库 `room_reply_previews` 一次读取当前 owner 的摘要；`(owner_user_id, room_id)` 唯一，每个 DM/Room 至多一行。
- 完整 assistant 回复落盘后更新摘要，保留来源 conversation、session、message 和回复时间。多 conversation 按回复时间更新同一行；私有成员消息不能更新群聊摘要；流式 delta 不写库。
- 编辑重发在修改历史前清空来源摘要并保留时间栅栏，拒绝此前消息的延迟写回。
- Session 删除在同一个次级数据事务中失效摘要；来源 conversation/Room 删除由外键级联清理。失效后保持空摘要等待新回复，不扫描历史寻找回退内容。
- 前端通过全局 WebSocket 更新共享目录的活动时间及完整回复摘要，不在每轮状态变化时查询 bootstrap。只有冷加载、重连、未知会话和目录失效才对账；请求期间的消息增量与删除保留到 HTTP 快照合并后，避免旧响应覆盖新状态。
- 首屏摘要查询共用 500ms context 预算，不读取历史文件或建立历史索引。
- 旧数据或新摘要写入失败时，用户正常读取的历史页可补齐摘要；不额外全量扫描。投影失败只记诊断日志，不把已落盘的消息误报为失败或触发重复发送。
- Launcher 与侧栏目录只读取 Session metadata/Room catalog，禁止为标题、预览或排序扫描 transcript/history；单个超长或损坏会话不能阻塞目录首屏。
- 对话正文默认加载最近一页 round。

### 4.2 向上翻页与重同步

- 上滚到顶部时再请求更早 round，保持视口位置不跳。
- 重同步只刷新最近一页，不整段全量重拉。

### 4.3 Transcript 缓存

- 内存缓存保存规范化 JSONL 条目，不保存绑定会话身份或 marker 的投影结果。
- 普通、分段与显式读取共用缓存，按受控文件句柄的文件身份、大小、mtime、首尾指纹校验；读取期间文件变化则不发布缓存。
- 缓存与调用方通过深拷贝隔离，分段重编号不污染旧条目。
- 索引重建不主动清空全部 transcript；历史删除按目录失效。
- 采用 12 个文件的 LRU 上限；不变更 canonical JSONL 格式，不做活跃文件的字节级增量解析。

### 4.4 Room 发送受理的上下文读取

- 发送受理不预读全量公区历史。
- slot 在 runtime 确认可恢复状态后，复用消息身份索引读取游标所在轮次及其后消息，再由既有公区 batch 过滤已消费部分；为失败恢复保留目标 Agent 最近终态。
- 普通输入以当前触发消息为上界；当前活跃轮次不物化 synthetic interrupt。
- 轮内引导与公开 mention 共用读取入口。
- 冷启动、无法定位游标、超预算或所选内容包含 UI detail 时保留 canonical 完整正文；UI 页大小和正文预览不是模型上下文预算。

### 4.5 派生读模型

存储与发布：

- 旧用户数据不改写、不搬迁；首次读取只从 canonical 生成宿主 `app/cache/history-read-model.v1.sqlite`。
- 派生库只保留当前 schema，不保留数据迁移链；schema 变化、初始化中断或损坏时直接丢弃并从 canonical 回建。
- DM 与 Room 先完成第 5 节的完整规范化，再在单个 SQLite 事务中发布新 generation。
- 每个 physical round 分开保存 B-Tree 游标元数据、完整 payload 与摘要；不调用模型压缩，不丢弃消息块。
- generation 在单个数据库事务内切换。会话在 build/persist 期间被删除时必须放弃写入，禁止由派生读模型重新创建 canonical session/Room 容器。超过保留期的 scope 可直接淘汰。

Room 增量：

- 在后台单飞任务中优先尝试增量：保存原始尾轮与已处理 ledger 长度；同文件追加且已有引用未变化时，只读取新增完整 JSONL 行，重投影尾轮和新增轮次。
- 分页 payload、导航、消息身份索引和尾轮检查点在同一 SQLite 事务中提交；失败不推进进度。
- 回退完整重建的条件：已有 transcript/private overlay 变化、文件替换/截断、跨旧轮次修改、特殊控制行或超预算。首次建模仍需扫描 canonical。
- 增量保持既有 generation 并清理被替换尾轮的 detail；完整重建才更换 generation。
- 增量成功记录源文件字节数、新增字节数、更新/保留轮次数和耗时；不能增量时记录回退原因。

完整重建：

- 当前任一 canonical source 变化都会触发一次完整 generation 重建。duplicate UUID、parent 主链、marker 对齐和 Room transcript_ref 都可能反向改变旧 round，因此在没有等价性证明前不得用 append-only 增量更新替代完整规范化。

读取：

- 热读先验证全部 canonical source 快照，再通过 B-Tree 读取有界的游标元数据窗口和本页命中的 round payload。
- 单组摘要、scope 元数据或数据库损坏时放弃派生结果并安全回建，不返回未经校验的历史。
- Session Round Navigator 的标题、状态、Agent 和时间元数据与消息页属于同一 generation；热开不得为导航再扫描完整 overlay；冷开复用同一 rebuild future 和 `indexing` 短轮询协议。

并发与 `indexing`：

- 冷建或 source 变化时，每个会话只允许一个后台 rebuild；全局只允许固定数量的 active rebuild，不保留 detached 等待队列。
- HTTP 客户端设置 `defer_index=true` 时，后台槽已满或前景等待超过短预算即返回 `indexing=true` 和 `retry_after_ms`。前端保持 loading 并用短请求重试，禁止把该空 `items` 提交为真实空历史。
- 原请求超时或切页不取消已受理的有界 rebuild；重试必须 attach 同 scope future，不得从零重扫。

容量上限：

- 派生层必须限制 group 数、单 group、generation 和单页 payload 字节。
- detached build 在读取/规范化前限制 canonical source 总字节：DM 先检查全部 overlay/transcript 快照；Room 先检查 ledger，再从有界 ledger 收集 dependency 快照，并在 resolve private transcript 前再次检查。
- source 超限：
  - detached build 只能生成绑定当前 source digest 的 disabled marker；ledger 自身超限的 Room marker 只绑定 ledger snapshot，不能为计算全依赖 digest 读取 oversized ledger。
  - 读取走 request-bound canonical 精确分页；完整规范化回到可由 HTTP context 取消的 request-bound 路径。
  - 不改写 canonical，不向客户端返回截断历史。
  - 同 source 不得反复启动 detached rebuild；source 变化后，有空闲 admission 时才可尝试恢复索引。
  - 该降级保留完整功能，但回到完整规范化的原有成本，不是 append-only 增量方案。

### 4.6 客户端窗口与大型内容

- 首屏、向前分页、around 定位和 `indexing` 重试都必须携带请求级取消信号；切换或清空 Session 时取消旧请求。会话代次是拒绝迟到响应的第二道栅栏，不能替代 transport cancellation。
- 浏览器只保留受 root round 数与估算驻留字节双预算约束的消息窗口：
  - 淘汰以完整 root round 为单位。
  - around 请求必须保留目标锚点；实时、乐观与最新 round 优先于可再次分页取得的历史。
  - 单个不可拆 round 可独自超过字节预算，但不得因此保留更早的大 round。
- canonical 历史保存完整 Tool result 与内联图片。派生 generation 对单个 `>=256 KiB` 的 Tool result 或图片只保存内容摘要、大小和 opaque detail ref；消息页返回有界预览/引用，Tool 展开或图片实际渲染时才读取完整 detail。
- detail 必须同时绑定 owner、Session scope、source generation 和 payload digest；source 变化、scope 不匹配、派生数据损坏或淘汰后一律返回 unavailable，不回退猜测旧内容。
- 图片 detail 只以受限 raster MIME 和 `nosniff` 响应；桌面客户端必须通过带会话令牌的 fetch 生成临时 Blob URL，不能把认证 URL 直接交给 `<img>`。
- Feed 和定位只使用消息 round 页与同 generation 的 `SessionRoundIndex`。`ConversationTurn`、`TurnPage` 与 turn-index HTTP 投影已删除，禁止恢复先全量读取再切片的第二套历史 API。

## 5. 规范化规则

历史读取依次执行：

1. transcript / overlay 合并
2. transcript user 与 round marker 尾部对齐
3. snapshot 压缩
4. 未完成 round 物化
5. round 归一化
6. round 分页

API 返回“可展示历史”，不是原始文件逐行回放。

- runtime 的 `is_meta` user、Skill 完整正文、连续的 Execution / Goal `<internal_context>` carrier 和其他内部 carrier 必须在可见 round 投影前过滤。旧版 `<internal_context source="explicit_skill">` 包装只用于兼容读取，不能重新显示或进入模型历史。
- marker 对齐按 transcript user 槽位逐个消费；空槽位不能跳过后借用下一轮 marker，否则刷新后旧的 unknown/内部消息会窃取新 Slash 的 round 身份。
- runtime command metadata 统一还原为原始 `/name args`；与 overlay marker 相同时只展示一份用户输入。
- 同一 round 的稳定顺序：user，然后 assistant / system / task_progress。
- `result` 只在 overlay 存储层保留语义；对外投影挂到 assistant 的 `result_summary`，不是前端可见主消息类型。
- 未完成 round 直接物化为 `assistant + stop_reason: cancelled + result_summary.subtype: interrupted`，不经过 `role: result` 中间态。

### 5.1 内部上下文注入

- Nexus 只生产按 priority 降序及 name/content/metadata 确定性排序的内部上下文块。
- bridge 把统一隐藏 reminder 绑定到下一条 runtime user 消息。
  - nxs 在 user 落盘前将其提取到当前 live model history；当前进程后续请求仍能看到已发生的 Goal、Execution、恢复、Automation 与 transport 上下文，transcript 只保存干净的 user。
  - Claude Code 通过 `UserPromptSubmit` hook 的 `additionalContext` 生成同语义 attachment。
  - 两者后续请求继续携带，但不进入 transcript。
- workspace `AGENTS.md` 只由 SDK 启动加载器读取，产品 prompt builder 不重复拼接。

### 5.2 公区时间线顺序

时间线同时满足事实顺序和因果顺序：

1. 同一 root round 的 primary user message 在最前，只展示一份。定向到某个执行槽的 guide user message 属于该槽的附着输入，按 5.4 紧贴目标卡片，不在顶部重复。
2. 已发布的 Agent final reply 按服务端公区发布时间升序展示；同一时间按 root round 内后端在 slot 创建时分配的稳定 `display_order`，再以 `agent_round_id` 兜底。不得按客户端收到事件的先后重排历史。
3. source message 必须先于它触发的 handoff 状态和 target reply；handoff child 可以在 sibling slot 仍运行时出现，但不能插入 source 之前。
4. pending、streaming、等待权限或等待 guide ACK 的 slot 不是公区事实，统一放在已完成回复之后，按 slot 启动顺序排列。
5. 同一 slot 的流式更新只更新该 slot 的状态卡；进入终态后替换为最终回复，不重复追加卡片。

- 同一 Agent 的 active slot 用 `agent_round_id` 归组。
- 回复顺序不由 Agent 名称、`@` 书写顺序或 Skill 预期决定。并行 slot 谁先完成谁先进入已发布回复区；慢 slot 只保留一个紧凑活动状态，不阻塞已完成回复，也不制造空白占位消息。
- 活动卡从活动区进入已完成区是唯一允许的结构变化；已发布的回复之间不因后续 stream 或 guide 互换位置。

示例：A 先启动但较慢，B 后启动且先完成：

```text
用户消息
├─ B 的最终回复
└─ A：执行中（紧凑状态）

A 完成后：
用户消息
├─ B 的最终回复
└─ A 的最终回复
```

### 5.3 实时与历史的一致性

- 实时订阅用 `room_seq` 做事件重放和缺口检测；它是传输序号，不是历史排序真相源。
- 持久 Assistant 行只提供公区结构、内容与精确终态；即使其 legacy `stream_status` 仍为 pending/streaming，没有当前 slot、pending interaction 或 active lifecycle 时也不得重建“正在执行”。
- 历史按持久化的公区发布时间、稳定 display order 和因果关联归一化。公区发布时间由服务端在消息进入 shared overlay 时确定，不用 runtime 开始时间，也不直接重放 WebSocket 到达顺序。
- 多个并发消息落在同一时间粒度时，持久化层必须提供稳定 tie-breaker；进程重启不能改变已有回复的相对顺序。
- 目标 Agent 的状态事件可以先于它的 final reply 展示，但不能先于 source public message。

### 5.4 guide、queue 与 handoff 的展示

- `delivery_policy=guide` 是投递策略，不是新的 assistant 消息。
- 单目标 guide：用户消息只保留一份，以紧凑的“补充要求”样式紧贴目标 Agent 卡片之前。
- 多目标或无法安全归组的 guide：用户消息保留在原始公区位置，旁边只显示目标 Agent 头像/名称摘要，不为每个目标复制正文。
- guide 的 ACK、fallback、`guided_input` 等控制事件合并为目标卡片的一行轻量状态；详细过程放入 Thread，不生成独立大气泡。
- 尚未消费的用户 queue item 只出现在 composer 的待发送队列；消费后才进入时间线，且只进入一次。
- 用户入队请求只有收到 `input_queue_ack` 后才能清空 composer。队列项即使在 ACK 前后被立即派发，重试也必须由持久化幂等记录返回原 `item_id`，不得创建第二轮。
- Agent public handoff 的 `detected/queued/running` 不生成“系统发言”气泡。源消息中的 `@Agent` chip 是唯一的交接正文；目标卡片只显示排队/运行状态；目标 final reply 到达后才显示完整回复。
- final reply 可用宿主 `handoff_reply` 注解在消息头展示非动作的“回应 `@成员`”因果。这里的 `@` 只是宿主投影的成员来源标识，不得进入正文、`agent_mentions`、mention URI、handoff detector 或 wake。
- no-reply、空 assistant、纯 result 和重复 wake 不占用独立时间线行。

### 5.5 消息渲染的密度边界

- 一份正文只渲染一次；状态、路由和耗时附着在消息头、轻量状态行或 Thread 中。
- 用户消息和 Agent final reply 是主内容，都走 Markdown 渲染链。thinking、tool、permission、AskUserQuestion、guide 控制事件属于过程层，默认折叠或摘要化，不与 final reply 平铺竞争。
- 连续的状态事件合并为最新状态，不逐条堆叠“已发送/已排队/已启动/等待中”；完整事件在 Thread 审计。
- 主 Feed 不显示独立 wake/queue 系统气泡；状态用轻量行或 badge。
- Agent 头像只出现在消息头、Agent mention chip 和必要的状态卡，不为每条控制事件重复放大头像。
- `agent_mentions` 由共享渲染器转成小头像 + 可点击名称的 Agent chip；点击打开 Agent 资料，不触发第二次 handoff。头像 URL 不写入消息，历史按当前成员目录解析。
- 主 Feed 显示事实和一行状态摘要，Thread 显示过程细节；两者使用同一消息注解和同一 Agent 身份映射。

### 5.6 会话内滚动与未读

- Room 与 DM 的会话内滚动只保留统一“回到底部”动作；不把侧栏未读状态注入 Feed，不自动定位首条未读，不渲染“新消息”边界。
- 当前窗口打开精确 Conversation 后立即确认该目标；其他 Conversation 的侧栏未读与系统通知继续隔离保留。

## 6. API 与已删除链路

Room / DM 历史读取统一走 room conversation 语义：

```text
GET /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/messages
```

以下链路已移除，不属于运行时主链：

- `/nexus/v1/sessions/{session_key}/messages`
- 私有 `messages.jsonl` 完整正文副本
- room shared 完整正文副本
- `cost/summary` HTTP 链
- `telemetry_cost.jsonl` / `telemetry_cost_summary.json`

## 实现约束

- Room 主 Feed 只在原活动位置以统一主色显示一条不可展开的 summary/fallback；具体过程进入 Thread 后才使用中文优先的可展开折叠栏。
