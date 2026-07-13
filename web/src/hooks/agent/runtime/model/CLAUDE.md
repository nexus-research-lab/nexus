# hooks/agent/runtime/model/

L5 | 父级: ../CLAUDE.md

保存运行状态机、公开快照协议、消息/slot 迁移和权限过期策略。这里只处理纯数据与同步状态迁移，不读取浏览器存储，不持有 React 生命周期。

- Snapshot reconcile 按终态收集、旧 tracker 保留和 DM tracker 补建三个阶段执行；Room 不从历史消息反推活跃 tracker。
- 消息终态迁移统一解析为保留、移除或更新状态三种动作，调用方只定义作用域规则。
- Runtime 瞬时状态优先于轮次推断；`compacting` 进入独立阶段，显式 null 或会话重置负责清除。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
