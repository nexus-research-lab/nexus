# WorkGraph workflow 沉淀与复用

只在用户要求查看历史工作图、把一张图沉淀为命名命令，或当前 prompt 明确来自一个已保存的 `/<workflow>` 时读取本文件。

## 命令边界

- `/workgraph <request>` 只为当前请求启用 WorkGraph 协作，不创建或更新 workflow。
- `/<workflow> <request>` 是用户已经保存的 prompt-based workflow。它提供责任节点和依赖模板；每次调用仍创建新的 Execution、Plan、Work Item 和运行身份。
- 沉淀动作必须由 `nexus execution invoke --operation distill_workgraph_workflow` 完成。UI 只展示历史、发起沉淀意图并管理结果，不自行判断或落库节点。

## 沉淀一张历史图

1. 先运行 `execution inspect`。若用户指定历史图，使用它的 exact `execution_id` 再运行 `execution inspect --execution-id '<execution-id>'`。不要从标题、时间或聊天正文猜图。
2. 只从 Work Item 中选择可复用语义：
   - `key`：决定交付物、验证结果、汇总结果或关键恢复顺序的节点。
   - `collaboration`：表达独立 owner、handoff、review、跨 Agent 验证或 integration boundary 的节点。
   - Tool、普通本地文件操作、runtime retry、Assignment/Attempt、Submission/Review/Acceptance 是历史事实，永不成为 workflow 节点。
3. 保留完成该 workflow 所需的最小闭合子图。若一个选中节点依赖未选择节点，判断该前置是否是真正可复用结构：是则一并选择；否则让新 workflow 在当前请求中重新推导，不伪造旧边。
4. 形成输入：required `execution_id`、不带 `/` 且匹配 `^[a-z][a-z0-9-]{0,63}$` 的 `slash_name`、非空 `title`、至少一个且不超过 contract 上限的 `nodes[{work_item_id,role}]`；`description` 可选。`role` 只能是 `key|collaboration`，Work Item 不得重复或越出源图，并应保留真实 terminal 交付。不能使用 `compact|goal|model|skills|visualize|workgraph` 等保留 Slash，也不能覆盖 owner 已有同名 workflow；每个 owner 最多保存 128 个。
5. 读取 fresh contract；只有目录实际列出该 operation 才继续：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution contract --operation distill_workgraph_workflow
   ```

6. 按主 Skill 的私有输入槽规则写入 exact schema，再执行：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json execution invoke --operation distill_workgraph_workflow --request-id 'workflow-distill-UNIQUE'
   ```

7. 只有顶层 `is_error=false` 且 `data.outcome=applied` 才表示已保存。向用户返回 `data.command`；同一意图重试复用 request id。相同 request id 是同一次幂等创建，想改变名称、节点或描述必须使用新 ID。

## 复用命名 workflow

动态 Slash 展开会给出当前请求、模板节点和依赖。把它们当作 Plan 设计输入，而不是已发生事实：

1. 根据当前请求调整 subject、objective、deliverable 和 acceptance criteria，但不悄悄删掉模板中的关键交付或协作边界。
2. 生成一份完整的 fresh Nexus Plan Document，按 `prepare_plan_execution -> plan_execution` 两阶段落图。
3. 只通过当前 Session/Room 的实际 authority 分配与执行。永不复制源 Execution/Work Item ID、Agent 身份、状态、结果、Artifact 或审核结论。
4. 如果当前请求不适合模板，先说明不匹配点并选择更小的真实结构；不要为了匹配命令制造无效节点。
