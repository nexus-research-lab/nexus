# hooks/agent/runtime/

L4 | 父级: ../CLAUDE.md

负责维护后端运行态在前端的唯一投影，包括轮次状态、权限、Room slot 和发送中的请求。

- `model/`: 无 React、无浏览器状态的运行模型、消息迁移与权限策略
- `snapshot/`: 易失会话投影和 `sessionStorage` 边界
- `state/`: 状态机订阅、slot/权限 React 生命周期
- `use-agent-conversation-runtime.ts`: 只编排下层命令，不直接持有状态机或计时器

依赖只能从编排层指向 `state/`、`snapshot/` 和 `model/`；`snapshot/`、`state/` 可依赖 `model/`，`model/` 不得反向依赖。终态迁移必须一次收口所有关联状态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
