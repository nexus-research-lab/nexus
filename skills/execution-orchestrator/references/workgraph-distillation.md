# 命名 WorkGraph 的保存与复用

只在用户要求查看历史工作图、把一张图保存为可复用命名工作图，或当前 prompt 明确来自一个已保存的命名 Slash 时读取本文件。

## 命令边界

- `/workgraph <request>` 只为当前请求启用 WorkGraph 协作，不创建或更新命名图。
- `/<command> <request>` 是用户已保存的 WorkGraph 命令。它提供抽象责任节点和依赖模板；每次调用仍创建新的 Execution、Plan、Work Item 和运行身份。
- 用户只能在完成态 WorkGraph 标题栏请求“保存为草图”。宿主先用 owner 的默认后台模型从完整实际图自动选择、抽象结构，并按界面语言生成有时效的 preview；生成模型会收到当前命令目录与固定保留名，`slash_name` 默认选择一个不重复的短词，只有语义准确的单词候选都冲突时才退到两个词，不使用三个及以上词；若模型仍返回多词候选，服务端按语义核心词优先收敛到未占用的单词，并继续做最终冲突校验。用户可直接修改元信息，或在宿主从源 transcript 最近一个已完成助手轮次创建的短期受限 DM 分支中让模型修改文案、节点、父子结构与依赖；宿主校验完整草图的 revision、DAG、key 主路径与 terminal 交付后，只有用户明确应用才替换原 preview。
- 保存命名图必须由宿主 `HiddenFromUser + Synthetic + purpose=workgraph_distillation` 的内部 Agent round 调用 `nexus execution invoke --operation distill_workgraph` 完成。该 mutation 只接收用户刚确认的 exact `preview_id`；UI 调度端点不直接落库，Agent 也不得重新读取源图、重选节点或重写草图，更不能向聊天时间线补发保存请求。
- 模型不可用、JSON 无效、输出不是源 logical key 子集、缺少 key 主路径/terminal 交付或语义字段不完整时预览失败关闭，绝不回退展示或保存原始具体内容。

## 保存用户已确认的草图

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

4. 只有顶层 `is_error=false` 且 `data.outcome=applied` 才表示已保存。结束内部 round，由宿主目录变更事件刷新 WorkGraph 与 Slash 目录，不在聊天中补发结果；同一意图重试复用 request id。preview 过期或与当前 Session 不符时，由 UI 提示用户回到完成图重新生成并确认，禁止用其他图或字段代替。

## 复用命名 WorkGraph 命令

动态 Slash 展开会给出当前请求、抽象节点和依赖。把它们当作 Plan 设计输入，而不是已发生事实：

1. 根据当前请求具体化 subject、objective、deliverable 和 acceptance criteria，但不悄悄删掉模板中的关键交付或协作边界。
2. 生成一份完整的 fresh Nexus Plan Document，按 `prepare_plan_execution -> plan_execution` 两阶段落图。
3. 只通过当前 Session/Room 的实际 authority 分配与执行。永不复制源 Execution/Work Item ID、Agent 身份、状态、结果、Artifact 或审核结论。
4. 如果当前请求不适合该命令，先说明不匹配点并选择更小的真实结构；不要为了匹配命令制造无效节点。
