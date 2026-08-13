---
name: goal-manager
description: 当用户或系统明确要求创建、查看、纠正、完成或阻塞当前会话 Goal，或当前上下文已经绑定 active Goal 时使用。负责 Goal 生命周期操作与按需加载创建、完成或 Room 参考；不从普通任务猜 Goal，不承载 Task、WorkGraph 或 Room 分工选择。
---

# Goal Manager

管理当前会话已经选定的长程 objective。是否需要 Goal 的结构选择由 `execution-orchestrator` 负责；本 Skill 不把普通任务、提醒、Room action 或一次性协作升级为 Goal，也不把已选定的 Goal 自动升级为 WorkGraph。

Skill 只加载使用规则，不替代工具调用。根据当前工具列表使用完整 MCP 名称或裸名：

| 操作 | MCP 名称 | 裸名 |
| --- | --- | --- |
| 读取 | `mcp__nexus_goal__get_goal` | `get_goal` |
| 创建 | `mcp__nexus_goal__create_goal` | `create_goal` |
| 纠正 | `mcp__nexus_goal__retarget_goal` | `retarget_goal` |
| 完成审计 | `mcp__nexus_goal__audit_objective_alignment` | `audit_objective_alignment` |
| 终态更新 | `mcp__nexus_goal__update_goal` | `update_goal` |

优先调用当前工具列表实际暴露的名称，不使用 `/goal` 文本命令，也不向用户索要 session key。

## 使用步骤

1. 读取当前 Goal context；用户要求设定或更改 Goal、或状态未知时必须先调用 `get_goal`。
2. 区分本次意图是创建、纠正、完成还是阻塞。普通补充、追问和模型自己的路线调整不等于 retarget。
3. 只读取下面与当前生命周期动作对应的参考文件，并完整读完后再调用工具。
4. 遵守工具返回的 objective revision、Execution transition 和 next action；不要凭文本模拟状态变化。
5. 完成工具调用后继续交付任务本身。Goal 状态是辅助信息，不是最终成果。

## 按需参考

- 创建新 Goal 或明确纠正现有 objective 时，读取 [references/create-and-retarget.md](references/create-and-retarget.md)。
- 判断完成、提交 Objective Alignment、形成最终交付或确认 blocked 时，读取 [references/complete-and-block.md](references/complete-and-block.md)。
- 当前 Goal 属于 shared Room 时，额外读取 [references/room-goals.md](references/room-goals.md)。

## 稳定边界

- 创建前 objective 必须具体到可以执行；缺少会改变结果的信息时先询问，不创建占位 Goal。
- 当前会话已有未结束 Goal 时不创建第二个；用户明确更换 objective 时 retarget 同一个 Goal。
- `get_goal` 返回空时，显式 Goal 意图走 `create_goal`；返回现有 Goal 时，objective 纠正走 `retarget_goal`。`update_goal` 绝不用于设定或改写 objective。
- `token_budget` 只在用户明确给出预算时设置。
- 模型只能用 `update_goal` 标记 `complete` 或 `blocked`；暂停、恢复、预算和用量限制属于用户或系统控制面。
- 提醒、定时和周期任务使用 Automation，不使用 Goal。
