# hooks/agent/runtime/model/

L5 | 父级: ../CLAUDE.md

保存运行状态机、公开快照协议、消息/slot 迁移和权限过期策略。这里只处理纯数据与同步状态迁移，不读取浏览器存储，不持有 React 生命周期。

- Message snapshot reconcile 按终态收集、旧 tracker 保留和 DM tracker 补建三个阶段执行；Room 历史只能补结构和精确终态，不得从未收口的持久 Assistant 行反推活跃 tracker。Room 订阅恢复的 `pending_snapshot` 是当前 conversation 活跃 slot 的权威集合。
- 消息终态迁移统一解析为保留、移除或更新状态三种动作，调用方只定义作用域规则；round 终态移除 ephemeral 过程消息，但保留已经完成的 transient host 通知及其关联用户指令。host `chat_ack` 即使不声明 durable commit，也可用显式 delivery mode 把 optimistic 用户消息规范化为同 round transient 节点。
- 待 ACK 的 `client_request_id` 只决定发送阶段，不进入 canonical round 集合；时间线活动仅来自后端 round 与 Assistant tracker。
- Runtime 瞬时状态优先于轮次推断；`compacting` 进入独立阶段，显式 null 或会话重置负责清除。
- `room-agent-execution-state.ts` 用 root round + `agent_round_id` 保存当前 Session 内的展示锚点；public mention slot 的 `handoff_id` 随 execution 单调保留，让来源 mention 原位接棒而不建立第二张卡。首次批量恢复优先按 message `display_order` 或 slot timestamp/index 建立 canonical 顺序；permission、stream、message fallback 与 status-first 证据都必须换算到同一毫秒顺序尺度，不能以局部数组索引插到已有正文上方。易失证据后续首次出现时只追加登记一次；唯有持久 message 显式携带的 `display_order` 可以在初始历史请求期间回填更早 execution 或校正同一 execution 的实时锚点，缺少该字段的 legacy message 不得猜测换位。acknowledged tombstone 不再承载交互，后到执行证据只接管状态而不换位。stream `message_stop` 只表示单个 Assistant turn 收口，尤其 `tool_use` 后仍保持 execution active；Agent/root lifecycle 或 durable 非工具终态才可关闭 execution。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
