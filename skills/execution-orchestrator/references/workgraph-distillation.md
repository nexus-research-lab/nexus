# 命名 WorkGraph 的保存与复用

只在用户要求查看历史工作图、提取或继续编辑草图、选择草图版本、把一张图保存为可复用命名工作图，或当前 prompt 明确来自一个已保存的命名 Slash 时读取本文件。

## 命令边界

- `/workgraph <request>` 只为当前请求启用 WorkGraph 协作，不创建或更新命名图。
- `/<command> <request>` 是用户已保存的 WorkGraph 命令。它提供抽象责任节点和依赖模板；每次调用仍创建新的 Execution、Plan、Work Item 和运行身份。
- UI 标题栏和普通对话都是同一 WorkGraph Draft 能力的入口。UI 可由宿主直接调用 authoring service；普通对话必须先读取 fresh `inspect_workgraph_library` contract 并调用该 operation，不能凭 transcript、记忆或数量描述推断历史图。返回值会同时列出当前 Session 的 completed source、Draft 和 owner 命名图，因此一个会话有多张图时必须使用 exact `execution_id` / `preview_id`。
- 提取只接受当前 Session 的 completed source Execution。调用 `extract_workgraph_preview` 时，宿主会先按 owner/session/source 查找可恢复 Draft；已有 Draft 直接返回，不重复模型提取。首次提取把完整 source logical-key、父子层级和依赖交给抽象模型，强制保留 required/terminal、拓扑引用、验证/复核和协作边界等关键节点，主要抽象节点内的具体任务语义；只有不影响任何结构语义的非关键孤立节点才可省略，无法确定时保留。`slash_name` 默认用不冲突的短单词，只有准确单词均冲突时才用两个短词。
- Draft 按 source Execution 唯一并跨页面恢复。每次修改追加不可变版本；`head_revision` 是并发 CAS，`selected_revision` 是用户当前偏好版本。选择旧版本不删除新版本，下一次修改从 selected 内容继续但仍提交 fresh head revision。模型不得把“选择 v1”解释成重写一份近似 v1。
- 草图编辑统一由 owner 的 Nexus 主智能体承载在隐藏专用 DM 中。它不进入主智能体普通 DM 目录，不继承来源 DM/Room transcript、连接器或权限；来源只通过完整 Draft 与 source WorkGraph 事实提供。关闭 UI 不删除该 Session，再次打开继续同一对话。该 Session 只开放本 Skill、`revise_workgraph_preview` 和 `select_workgraph_preview_revision`。
- 保存命名图必须由宿主在独立目录隐藏内部 DM 中启动 `HiddenFromUser + Synthetic + purpose=workgraph_distillation` Agent round，并调用 `nexus execution invoke --operation distill_workgraph` 完成；该 Session 不 fork、resume 或续写源 transcript。mutation 只接收用户刚确认、由宿主 capability 绑定的 exact `preview_id`；UI 调度端点不直接落库，Agent 也不得重新读取源图、重选节点或重写草图，更不能向聊天时间线补发保存请求。
- 用户在普通对话中明确要求保存当前 Draft 时，不需要再创建 UI round：读取 `save_workgraph_preview` fresh contract，以 exact preview_id 直接保存 selected version。保存前必须已经向用户展示或概述当前 Draft 且本轮存在明确保存意图；“看看”“比较”“先改一下”不构成保存确认。
- 已保存工作图从能力页“继续编辑”时必须恢复其原 Draft、selected revision 和隐藏编辑 Session；若历史数据尚无 Draft，则从该命名图建立一次可继续编辑的初始版本。再次保存更新同一个命名 WorkGraph 并追加聚合版本，不重复抽取、不创建同名副本。
- 该内部保存 round 的思考摘要、过程状态、工具调用说明和结束文本必须使用简体中文；只有命令、Skill 名称和标识符保留原始形式，禁止输出英文叙述。
- 模型不可用、JSON 无效、输出不是源 logical key 子集、遗漏宿主标记的结构关键节点、缺少 key 主路径/terminal 交付或语义字段不完整时预览失败关闭，绝不回退展示或保存原始具体内容。

## 普通对话中的查询、提取与版本化编辑

不要先运行 `execution inspect`；WorkGraph library 能力在当前没有 active Execution 时也可用。

1. 读取并调用 `inspect_workgraph_library`。如果用户没有指明是哪张图，只有返回目录中目标唯一时才能继续；否则列出紧凑候选让用户选择。
2. 没有目标 Draft 且用户要求沉淀 completed source 时，读取 `extract_workgraph_preview` contract，提交 exact `source_execution_id` 和界面语言。已有同源 Draft 会直接恢复。
3. 修改前调用 `get_workgraph_preview`，读取 selected 完整图、head revision 和版本目录；再读取 `revise_workgraph_preview` contract并提交完整草图。保留用户未要求改变的字段，不能只提交 diff。
4. 用户明确选择旧版本时调用 `select_workgraph_preview_revision`，提交 exact preview_id、head revision 和 selected revision。选择成功后不要继续修改，除非用户同时明确要求。
5. 用户明确确认保存时读取并调用 `save_workgraph_preview`。只有 applied receipt 才表示保存成功；回复当前用户即可，不向任何来源会话或其他 Session 补发结果。

## UI 隐藏 round 保存用户已确认的草图

当宿主内部 `workgraph_distillation` round 含有生成的 `preview_id` 时，确认它明确表示用户刚刚看过并选择保存的草图。不要从历史消息、标题、时间或 Execution id 猜 preview；普通可见用户消息不是这条保存链的必要组成。

1. 不运行 `execution inspect`，不读取源图，不分析 key/collaboration，也不修改已确认的命令名或语义。预览和用户确认已经完成这些工作；CLI 会按 owner、当前 Session、有效期严格校验。
2. 读取 fresh contract；只有目录实际列出该 operation 且 schema 只要求 `preview_id` 才继续：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract --operation distill_workgraph
   ```

3. 按主 Skill 的私有输入槽规则只写入 exact `preview_id`，再执行：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution invoke --operation distill_workgraph --request-id 'workgraph-distill-UNIQUE'
   ```

4. 只有顶层 `is_error=false` 且 `data.outcome=applied` 才表示已保存。结束内部 round，由宿主目录变更事件刷新 WorkGraph 与 Slash 目录，不在聊天中补发结果；同一意图重试复用 request id。Draft 不存在或与当前 Session 不符时，由 UI 提示用户回到 exact source 或已保存命名图恢复，禁止用其他图或字段代替。

## 隐藏编辑 Session

宿主明确说明当前是 WorkGraph 草图编辑 Session 时，这是由 Nexus 主智能体承载、可恢复但不进入普通 DM 目录的受限模式：

1. 只响应用户对当前草图的修改或提问，不执行草图任务，不读 workspace，也不调用 MCP。
2. 不运行 `execution inspect`。修改前读取 fresh `revise_workgraph_preview` contract；版本选择前读取 fresh `select_workgraph_preview_revision` contract。
3. 按 contract 的私有输入槽规则写入带当前 head revision 的完整草图；保留所有未被用户要求改变的字段，不能只提交 diff。当前内容是 selected revision，用户切回旧版本后必须从它继续。
4. 只调用单进程 `execution invoke --operation revise_workgraph_preview`。服务端会校验 owner/editor Session、revision CAS、命令冲突、节点类型、父子结构、DAG、key 主路径与 terminal 交付。
5. applied 后简短说明用户可见变化；冲突或过期时说明需要基于最新预览重试，不得转用普通 Execution operation。

## 复用命名 WorkGraph 命令

动态 Slash 展开会给出当前请求、抽象节点和依赖。把它们当作 Plan 设计输入，而不是已发生事实：

1. 根据当前请求具体化 subject、objective、deliverable 和 acceptance criteria，但不悄悄删掉模板中的关键交付或协作边界。
2. 生成一份完整的 fresh Nexus Plan Document，按 `prepare_plan_execution -> plan_execution` 两阶段落图。
3. 只通过当前 Session/Room 的实际 authority 分配与执行。永不复制源 Execution/Work Item ID、Agent 身份、状态、结果、Artifact 或审核结论。
4. 如果当前请求不适合该命令，先说明不匹配点并选择更小的真实结构；不要为了匹配命令制造无效节点。
