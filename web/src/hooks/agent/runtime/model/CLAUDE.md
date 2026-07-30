# hooks/agent/runtime/model/

L5 | 父级: ../CLAUDE.md

保存运行状态机、公开快照协议、消息/slot 迁移和权限过期策略。这里只处理纯数据与同步状态迁移，不读取浏览器存储，不持有 React 生命周期。

- Snapshot reconcile 按终态收集、旧 tracker 保留和 DM tracker 补建三个阶段执行；Room 不从历史消息反推活跃 tracker。
- 消息终态迁移统一解析为保留、移除或更新状态三种动作，调用方只定义作用域规则。
- 待 ACK 的 `client_request_id` 只决定发送阶段，不进入 canonical round 集合；时间线活动仅来自后端 round 与 Assistant tracker。
- Runtime 瞬时状态优先于轮次推断；`compacting` 进入独立阶段，显式 null 或会话重置负责清除。
- `room-agent-execution-state.ts` 用 root round + `agent_round_id` 保存当前 Session 内的不可变展示锚点；public mention slot 的 `handoff_id` 随 execution 单调保留，让来源 mention 原位接棒而不建立第二张卡。首次批量恢复优先按 message `display_order` 或 slot timestamp/index 建立 canonical 顺序；permission、stream、message fallback 与 status-first 证据都必须换算到同一毫秒顺序尺度，不能以局部数组索引插到已有正文上方。permission、slot、stream、message 后续任一新证据只追加登记一次。acknowledged tombstone 不再承载交互，后到执行证据只接管状态而不换位。stream `message_stop` 只表示单个 Assistant turn 收口，尤其 `tool_use` 后仍保持 execution active；Agent/root lifecycle 或 durable 非工具终态才可关闭 execution。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
