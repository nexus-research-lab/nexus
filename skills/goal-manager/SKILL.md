---
name: goal-manager
description: 当用户或系统明确要求创建、查看、纠正、完成或阻塞当前会话 Goal，或当前上下文已经绑定 active Goal 时使用。通过宿主签发的 nexus goal CLI 管理 Goal 生命周期，并按需加载创建、完成或 Room 参考；不从普通任务猜 Goal，不承载 Task、WorkGraph 或 Room 分工选择。
---

# Goal Manager

管理当前会话已经选定的长程 objective。是否需要 Goal 的结构选择由 `execution-orchestrator` 负责；本 Skill 不把普通任务、提醒、Room action 或一次性协作升级为 Goal，也不因 Goal 存在自动建立 WorkGraph。

使用宿主注入的 `NEXUS_COMMAND_PATH`。宿主把命令绑定到当前 owner、Agent、Session、physical round 和 exact Goal revision；不要声明或覆盖这些身份，不要覆盖任何 `NEXUS_COMMAND_*` 环境变量，不使用 `nexusctl`、其他管理入口或 `/goal` 文本命令。

## 命令工作流

1. 首次操作或边界不确定时读取目录。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json goal contract
   ```

   Windows 的 PowerShell runtime 使用 `& "${env:NEXUS_COMMAND_PATH}" --json goal contract`。后续命令保持同一种 shell 变量语法。

2. 用户要求设定、查看或更改 Goal，或当前状态未知时，读取权威状态。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json goal inspect
   ```

   `inspect` 中当前 objective revision 与 completion criteria 是 Objective Alignment 和完成判断的唯一目标边界；不要从 transcript、旧 Plan、聊天正文或本地草稿反查或拼接标准。

3. 确定一个操作后，只读取它的精确 contract，不根据记忆猜输入。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json goal contract --operation create_goal
   ```

4. 从 contract 输出读取 `input_staging.path`。这是宿主预建且初始内容为 `{}` 的文件：每个 physical round 第一次写入前，先用 Read 工具读该路径一次，再用 Write 覆盖为该操作的一个完整 JSON 对象；同轮后续新意图直接覆盖旧内容。不要自行选择路径，不用 Bash、heredoc、`cat`、命令替换或重定向生成 JSON。

5. 用一条单行命令执行操作。每个新意图生成一个 8–128 位稳定 `request_id`；同一意图重试必须复用。

   ```bash
   "${NEXUS_COMMAND_PATH}" --json goal invoke --operation create_goal --input-file "${NEXUS_COMMAND_INPUT_PATH}" --request-id 'goal-create-UNIQUE'
   ```

   PowerShell 对应使用 `& "${env:NEXUS_COMMAND_PATH}" ... --input-file "${env:NEXUS_COMMAND_INPUT_PATH}"`。

6. 只把 `is_error=false` 且宿主返回 applied receipt 的调用视为状态变化。按 `nextAction.domain` 与 `nextAction.operation` 继续；不要解析 shell 文本、伪造 receipt 或把 plan/no-op 当成已写入。

输入槽由宿主按 physical round 创建为 owner 私有 `0600` 文件并在 round 结束清理，只解决 JSON 传输；Goal、revision、责任与完成状态仍以服务端为准。

## 生命周期分流

- `inspect` 返回空且用户有显式 Goal 意图：读取 [references/create-and-retarget.md](references/create-and-retarget.md)，再执行 `create_goal`。
- 已有 active Goal 且用户明确替换 objective：读取同一参考，再执行 `retarget_goal`。
- 判断完成、提交 Objective Alignment 或确认 blocked：读取 [references/complete-and-block.md](references/complete-and-block.md)。
- 当前 Goal 属于 shared Room：额外读取 [references/room-goals.md](references/room-goals.md)。

只完整读取与当前动作相关的参考文件。

## 稳定边界

- 创建前 objective 必须具体到可以执行；缺少会改变结果的信息时先询问，不创建占位 Goal。
- 当前会话已有未结束 Goal 时不创建第二个；明确更换 objective 时 retarget 同一 Goal。
- `update_goal` 只标记 `complete` 或 `blocked`，绝不设定或改写 objective。
- `token_budget` 只在用户明确给出预算时设置。
- 暂停、恢复、预算和用量限制属于用户或系统控制面。
- 提醒、定时和周期任务使用 Automation，不使用 Goal。
- Goal 状态是执行连续性，不是最终成果；完成调用后仍要独立交付 objective 要求的结果。
