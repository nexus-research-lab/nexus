# 命名 WorkGraph 的保存与复用

只在用户要求查看历史工作图、提取或继续编辑草图、选择草图版本、把一张图保存为可复用命名工作图，或当前 prompt 明确来自一个内置/已保存的命名 Slash 时读取本文件。

## 命令边界

- `/workgraph <request>` 只为当前请求启用 WorkGraph 协作，不创建或更新命名图。
- `/<command> <request>` 可以是 Nexus 只读内置 WorkGraph 模板，也可以是用户已保存的 WorkGraph 命令。两者都只提供抽象责任节点和依赖模板；每次调用仍创建新的 Execution、Plan、Work Item 和运行身份。内置模板没有 source Execution，不进入 Draft 编辑或删除链路。
- UI 标题栏和普通对话都是同一 WorkGraph Draft 能力的入口。UI 可由宿主直接调用 authoring service；普通对话必须通过 `nexus.execution_read` 调用 `inspect_workgraph_library`，不能凭 transcript、记忆或数量描述推断历史图。返回值会同时列出当前 Session 的 completed source、Draft、只读内置模板和 owner 命名图，因此一个会话有多张图时必须使用 exact `execution_id` / `preview_id`；用户明确询问“自己保存的”图时排除 `built_in=true` 项。
- 提取只接受当前 Session 的 completed source Execution。调用 `extract_workgraph_preview` 时，宿主会先按 owner/session/source 查找可恢复 Draft；已有 Draft 直接返回，不重复模型提取。首次提取把完整 source logical-key、父子层级和依赖交给抽象模型，强制保留 required/terminal、拓扑引用、验证/复核和协作边界等关键节点，主要抽象节点内的具体任务语义；只有不影响任何结构语义的非关键孤立节点才可省略，无法确定时保留。`slash_name` 默认用不冲突的短单词，只有准确单词均冲突时才用两个短词。
- Draft 按 source Execution 唯一并跨页面恢复。每次修改追加不可变版本；`head_revision` 是并发 CAS，`selected_revision` 是用户当前偏好版本。选择旧版本不删除新版本，下一次修改从 selected 内容继续但仍提交 fresh head revision。模型不得把“选择 v1”解释成重写一份近似 v1。
- 草图编辑统一由 owner 的 Nexus 主智能体承载在隐藏专用 DM 中。它不进入主智能体普通 DM 目录，不继承来源 DM/Room transcript、连接器或权限；来源只通过完整 Draft 与 source WorkGraph 事实提供。关闭 UI 不删除该 Session，再次打开继续同一对话。该 Session 只开放本 Skill、`revise_workgraph_preview` 和 `select_workgraph_preview_revision`。
- 保存命名图必须由宿主在独立目录隐藏内部 DM 中启动 `HiddenFromUser + Synthetic + purpose=workgraph_distillation` Agent round，并通过 `nexus.execution_write` 调用 `distill_workgraph`；该 Session 不 fork、resume 或续写源 transcript。mutation 只接收用户刚确认、由宿主 round identity 绑定的 exact `preview_id`；UI 调度端点不直接落库，Agent 也不得重新读取源图、重选节点或重写草图，更不能向聊天时间线补发保存请求。
- 用户在普通对话中明确要求保存当前 Draft 时，不需要再创建 UI round：按 `nexus.execution_write` 的当前 schema 以 exact preview_id 调用 `save_workgraph_preview`，直接保存 selected version。保存前必须已经向用户展示或概述当前 Draft 且本轮存在明确保存意图；“看看”“比较”“先改一下”不构成保存确认。
- 普通 DM/Room 中，成功的 `extract_workgraph_preview`、`get_workgraph_preview`、`revise_workgraph_preview`、`select_workgraph_preview_revision` 和 `save_workgraph_preview` 会由宿主把最后一份完整图自动渲染为当前回复里的草图卡片；卡片可按需打开“来源图 / 当前草图”对照。这是回答“草图在哪看、是否已更新、怎么对照”时使用的界面事实，不是每次回复都要重复的固定话术。不要复述完整节点 JSON，或假装自己另外绘制了界面；没有 applied/ready/selected 成功结果时不得声称卡片已更新。
- 已保存工作图从能力页“继续编辑”时必须恢复其原 Draft、selected revision 和隐藏编辑 Session；若历史数据尚无 Draft，则从该命名图建立一次可继续编辑的初始版本。再次保存更新同一个命名 WorkGraph 并追加聚合版本，不重复抽取、不创建同名副本。
- 该内部保存 round 的思考摘要、过程状态、工具调用说明和结束文本必须使用简体中文；只有命令、Skill 名称和标识符保留原始形式，禁止输出英文叙述。
- 模型不可用、JSON 无效、输出不是源 logical key 子集、遗漏宿主标记的结构关键节点、缺少 key 主路径/terminal 交付或语义字段不完整时预览失败关闭，绝不回退展示或保存原始具体内容。

## 信息补充与草图检查门槛

- 创建或修改 Draft 前，先判断当前 source、用户要求和 selected version 是否足以确定复用目标、范围边界、terminal 交付、完成/验收标准以及必须保留的依赖或协作边界。只有缺口或冲突会实质改变草图或持久化结果时才提问；可从 exact source/Draft 得出的事实、可逆的文案偏好和不影响结构的细节不追问，采用合理假设并在检查问题中简短说明。
- 需要补充时使用 `AskUserQuestion`，一次集中询问最少的关键问题，允许用户提供自定义答案；在收到答案前不猜测、不执行受影响的 Draft mutation，也不保存。若当前通道不支持 `AskUserQuestion`，改为一条简洁的普通问题并结束本轮，等待用户回复。
- `extract_workgraph_preview` 或 `revise_workgraph_preview` 成功后，宿主会把最新完整草图渲染到当前回复的草图卡片；隐藏编辑 Session 则实时刷新右侧预览。此时必须调用 `AskUserQuestion` 暂停，请用户检查目标和范围、节点与依赖、关键交付与验收条件是否准确且无遗漏。问题应提供“确认当前草图”和“需要修改或补充”的清晰路径；原请求已经写过“保存”也不能替代这次草图后的检查。
- 用户指出缺漏或错误时，先读取当前 selected 完整 Draft，把用户补充作为精确修改要求调用 `revise_workgraph_preview`，成功渲染新版本后再次进入同一检查门槛。不要在用户仍要求修改时保存，也不要用聊天中的修正说明代替 durable Draft revision。
- 用户在草图显示后明确确认无遗漏，才算通过检查。若原意图包含保存，可把明确的“确认并保存”答案作为当前草图的保存确认；若原意图只是查看或编辑，确认只结束检查，不自动扩大为保存意图。用户选择暂不保存时保留 Draft，结束当前流程。
- `HiddenFromUser + Synthetic + purpose=workgraph_distillation` 的内部保存 round 不执行这套问答：它收到的 host-bound `preview_id` 已代表用户在可见界面完成检查并确认保存，重复提问会形成不可见阻塞。

## 普通对话中的查询、提取与版本化编辑

不要先运行 `get_execution`；WorkGraph library 能力在当前没有 active Execution 时也可用。

1. 读取并调用 `inspect_workgraph_library`。如果用户没有指明是哪张图，只有返回目录中目标唯一时才能继续；否则列出紧凑候选让用户选择。
2. 如果目标、范围或关键交付仍有会改变草图的缺口，先按“信息补充与草图检查门槛”提问；没有目标 Draft 且用户要求沉淀 completed source 时，通过 `nexus.execution_write` 调用 `extract_workgraph_preview`，提交 exact `source_execution_id` 和界面语言。已有同源 Draft 会直接恢复。
3. 首次提取成功后进入草图检查门槛。用户要求修改时，先通过 `nexus.execution_read` 调用 `get_workgraph_preview`，读取 selected 完整图、head revision 和版本目录；再按 `nexus.execution_write` 的当前 schema 调用 `revise_workgraph_preview` 并提交完整草图。保留用户未要求改变的字段，不能只提交 diff。修改成功后重新进入草图检查门槛。
4. 用户明确选择旧版本时调用 `select_workgraph_preview_revision`，提交 exact preview_id、head revision 和 selected revision。选择成功后把所选版本作为当前可见草图请用户检查；不要继续修改，除非用户同时明确要求。
5. 只有用户在当前草图显示后明确确认保存，才通过 `nexus.execution_write` 调用 `save_workgraph_preview`。只有 applied receipt 才表示保存成功；回复当前用户即可，不向任何来源会话或其他 Session 补发结果。

用户询问界面时可回答：提取或修改后的当前版本显示在回复草图卡片中，点“与来源图对照”查看来源图和当前草图；保存成功后卡片会标记为已保存。正常操作回复只需说明对用户有用的结果，不必主动介绍这些界面。不要把“测试草图展示”理解成执行草图中的任务——如果用户是在验证展示链路，只完成最小的提取或修改即可。

## UI 隐藏 round 保存用户已确认的草图

当宿主内部 `workgraph_distillation` round 含有生成的 `preview_id` 时，确认它明确表示用户刚刚看过并选择保存的草图。不要从历史消息、标题、时间或 Execution id 猜 preview；普通可见用户消息不是这条保存链的必要组成。

1. 不运行 `get_execution`，不读取源图，不分析 key/collaboration，也不修改已确认的命令名或语义。预览和用户确认已经完成这些工作；宿主会按 owner、当前 Session、有效期严格校验。
2. 确认当前 `nexus.execution_write` schema 实际列出 `distill_workgraph`，且只要求 `preview_id` 才继续。
3. 只提交 exact `preview_id`：

   ```json
   {"operation":"distill_workgraph","preview_id":"<exact-preview-id>"}
   ```

4. 只有顶层 `is_error=false` 且 `data.outcome=applied` 才表示已保存。结束内部 round，由宿主目录变更事件刷新 WorkGraph 与 Slash 目录，不在聊天中补发结果。Draft 不存在或与当前 Session 不符时，由 UI 提示用户回到 exact source 或已保存命名图恢复，禁止用其他图或字段代替。

## 隐藏编辑 Session

宿主明确说明当前是 WorkGraph 草图编辑 Session 时，这是由 Nexus 主智能体承载、可恢复但不进入普通 DM 目录的受限模式：

1. 只响应用户对当前草图的修改或提问，不执行草图任务，不读 workspace，也不调用 `nexus.execution_read` / `nexus.execution_write` 之外的 MCP。
2. 用户的修改要求缺少会实质改变 Draft 的信息时，先用 `AskUserQuestion` 补齐；不运行 `get_execution`。信息充分后，按 `nexus.execution_write` 的当前 schema 调用 `revise_workgraph_preview` 或 `select_workgraph_preview_revision`。
3. 提交带当前 head revision 的完整草图；保留所有未被用户要求改变的字段，不能只提交 diff。当前内容是 selected revision，用户切回旧版本后必须从它继续。
4. 只通过 `nexus.execution_write` 调用 `revise_workgraph_preview`。服务端会校验 owner/editor Session、revision CAS、命令冲突、节点类型、父子结构、DAG、key 主路径与 terminal 交付。
5. applied 后右侧预览实时刷新，立即用 `AskUserQuestion` 请用户检查当前版本；用户继续补充时重复修改与检查，明确确认后才结束本次编辑检查。冲突或过期时说明需要基于最新预览重试，不得转用普通 Execution operation。

隐藏编辑 Session 的右侧是宿主实时草图预览窗，applied 后宿主会从 durable Draft 刷新它。只有用户询问展示位置或刷新状态时才需要说明这个界面事实；正常修改回复不必反复提右侧。不要在左侧对话重复整张图；只回答问题而未 mutation 时，不得声称右侧已经更新。

## 复用命名 WorkGraph 命令

动态 Slash 展开会给出当前请求、抽象节点和依赖。把它们当作 Plan 设计输入，而不是已发生事实：

1. 根据当前请求具体化 subject、objective、deliverable 和 acceptance criteria，但不悄悄删掉模板中的关键交付或协作边界。
2. 生成一份完整的 fresh Nexus Plan Document，按 `prepare_plan_execution -> plan_execution` 两阶段落图。
3. 只通过当前 Session/Room 的实际 authority 分配与执行。永不复制源 Execution/Work Item ID、Agent 身份、状态、结果、Artifact 或审核结论。
4. 如果当前请求不适合该命令，先说明不匹配点并选择更小的真实结构；不要为了匹配命令制造无效节点。
